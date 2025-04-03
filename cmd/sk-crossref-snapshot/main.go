package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime/pprof"
	"strings"
	"sync/atomic"

	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/miku/clam"
	"github.com/miku/span/formats/crossref"
	"github.com/miku/span/parallel"
	"github.com/miku/span/xio"
	"github.com/segmentio/encoding/json"
	log "github.com/sirupsen/logrus"
)

// fallback awk script is used, if the filterline executable is not found;
// compiled filterline is about 3x faster.
var fallback = `
#!/bin/bash
LIST="$1" LC_ALL=C awk '
  function nextline() {
    if ((getline n < list) <=0) exit
  }
  BEGIN{
    list = ENVIRON["LIST"]
    nextline()
  }
  NR == n {
    print
    nextline()
  }' < "$2"
`

var (
	excludeFile       = flag.String("x", "", "a list of DOI to further ignore")
	outputFile        = flag.String("o", "", "output file")
	compressed        = flag.Bool("z", false, "input file is compressed (see: -compress-program)")
	batchsize         = flag.Int("b", 40000, "batch size")
	compressProgram   = flag.String("compress-program", "zstd", "compress program")
	cpuProfile        = flag.String("cpuprofile", "", "write cpuprofile to file")
	verbose           = flag.Bool("verbose", false, "be verbose")
	pathFile          = flag.String("f", "", "path to a file naming all inputs files to be considered, one file per line")
	errCountThreshold = flag.Int64("E", 1, "number of json unmarshal errors to tolerate")
	sortBufferSize    = flag.String("S", "25%", "passed to sort")
)

// writeFields writes a variable number of values separated by sep to a given
// writer. Returns bytes written and error.
func writeFields(w io.Writer, sep string, values ...interface{}) (int, error) {
	var ss = make([]string, len(values))
	for i, v := range values {
		switch v.(type) {
		case int, int8, int16, int32, int64:
			ss[i] = fmt.Sprintf("%d", v)
		case uint, uint8, uint16, uint32, uint64:
			ss[i] = fmt.Sprintf("%d", v)
		case float32, float64:
			ss[i] = fmt.Sprintf("%f", v)
		case fmt.Stringer:
			ss[i] = fmt.Sprintf("%s", v)
		default:
			ss[i] = fmt.Sprintf("%v", v)
		}
	}
	s := fmt.Sprintln(strings.Join(ss, sep))
	return io.WriteString(w, s)
}

// getInputReader returns a proper reader for the specified file based on compression settings
func getInputReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	if !*compressed {
		return f, nil
	}

	switch *compressProgram {
	case "gzip", "pigz":
		g, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &readCloserWrapper{g, f}, nil
	case "zstd":
		g, err := zstd.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &readCloserWrapper{g, f}, nil
	default:
		f.Close()
		return nil, fmt.Errorf("only gzip and zstd supported currently")
	}
}

// readCloserWrapper combines a reader and a closer to create a proper ReadCloser
type readCloserWrapper struct {
	reader io.Reader
	closer io.Closer
}

func (w *readCloserWrapper) Read(p []byte) (int, error) {
	return w.reader.Read(p)
}

func (w *readCloserWrapper) Close() error {
	var err error
	if w.reader != nil {
		if rc, ok := w.reader.(io.Closer); ok {
			err = rc.Close()
		}
	}
	if w.closer != nil {
		if e := w.closer.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// processFiles processes a list of files and writes extracted data to a temporary file
func processFiles(files []string, excludes map[string]struct{}) (string, error) {
	// Stage 1: Extract minimum amount of information from the raw data, write to tempfile.
	log.WithFields(log.Fields{
		"prefix":       "stage 1",
		"excludesFile": *excludeFile,
		"excludes":     len(excludes),
		"files":        len(files),
	}).Info("preparing extraction")

	tf, err := ioutil.TempFile("", "span-crossref-snapshot-")
	if err != nil {
		return "", err
	}
	defer tf.Close()

	var (
		bw      = bufio.NewWriter(tf)
		numErrs atomic.Int64 // error count across threads
		lineno  int64        // global line counter
	)

	for _, filePath := range files {
		log.WithFields(log.Fields{
			"prefix": "stage 1",
			"file":   filePath,
		}).Debug("processing file")

		reader, err := getInputReader(filePath)
		if err != nil {
			return "", fmt.Errorf("error opening file %s: %w", filePath, err)
		}
		defer reader.Close()

		br := bufio.NewReader(reader)
		pp := parallel.NewProcessor(br, bw, func(localLineno int64, b []byte) ([]byte, error) {
			var (
				// This was a crossref.Document, but we only need a few fields.
				doc struct {
					DOI       string
					Deposited crossref.DateField `json:"deposited"`
					Indexed   crossref.DateField `json:"indexed"`
				}
				buf bytes.Buffer
			)
			if err := json.Unmarshal(b, &doc); err != nil {
				numErrs.Add(1)
				if n := numErrs.Load(); n > *errCountThreshold {
					return nil, err
				} else {
					log.Printf("skipping error (#err: %d <= max: %d): %v", n, *errCountThreshold, err)
				}
				return nil, nil
			}
			date, err := doc.Indexed.Date()
			if err != nil {
				numErrs.Add(1)
				if n := numErrs.Load(); n > *errCountThreshold {
					return nil, err
				} else {
					log.Printf("skipping error (#err: %d <= max: %d): %v", n, *errCountThreshold, err)
				}
				return nil, nil
			}
			if _, ok := excludes[doc.DOI]; ok {
				return nil, nil
			}
			// Use global line number for consistent ordering
			globalLineno := atomic.AddInt64(&lineno, 1)
			if _, err := writeFields(&buf, "\t", globalLineno, date.Format("2006-01-02"), doc.DOI); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		})
		pp.BatchSize = *batchsize
		log.WithFields(log.Fields{
			"prefix":    "stage 1",
			"batchsize": *batchsize,
			"file":      filePath,
		}).Info("starting extraction")
		if err := pp.Run(); err != nil {
			return "", err
		}
	}

	if err := bw.Flush(); err != nil {
		return "", err
	}

	return tf.Name(), nil
}

// readFileList reads a list of file paths from a file
func readFileList(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var files []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path != "" {
			files = append(files, path)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// concatenateFiles concatenates multiple files into a single temporary file
func concatenateFiles(files []string) (string, error) {
	tf, err := ioutil.TempFile("", "span-crossref-snapshot-concat-")
	if err != nil {
		return "", err
	}
	defer tf.Close()

	for _, filePath := range files {
		reader, err := getInputReader(filePath)
		if err != nil {
			return "", fmt.Errorf("error opening file %s: %w", filePath, err)
		}

		_, err = io.Copy(tf, reader)
		reader.Close()
		if err != nil {
			return "", fmt.Errorf("error concatenating file %s: %w", filePath, err)
		}
	}

	return tf.Name(), nil
}

func main() {
	flag.Parse()
	if *verbose {
		log.SetLevel(log.DebugLevel)
	}
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	var (
		inputFiles []string
		excludes   = make(map[string]struct{})
	)

	// Handle excludes file if provided
	if *excludeFile != "" {
		file, err := os.Open(*excludeFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		if err := xio.LoadSet(file, excludes); err != nil {
			log.Fatal(err)
		}
		log.Debugf("excludes: %d", len(excludes))
	}

	// Check output file is specified
	if *outputFile == "" {
		log.Fatal("output filename required")
	}

	// Determine input files
	switch {
	case *pathFile != "":
		// Read list of files from the specified file
		files, err := readFileList(*pathFile)
		if err != nil {
			log.Fatalf("failed to read file list: %v", err)
		}
		if len(files) == 0 {
			log.Fatal("no input files found in file list")
		}
		inputFiles = files
		log.Debugf("using %d files from list", len(inputFiles))
	case flag.NArg() > 0:
		// Use files provided as command line arguments
		inputFiles = flag.Args()
		log.Debugf("using %d files from arguments", len(inputFiles))
	default:
		log.Fatal("input file(s) required, use -f for a file list or provide file(s) as arguments")
	}

	// Process the input files in two possible ways
	var (
		stage1Output string
		err          error
	)

	if len(inputFiles) == 1 {
		// For a single file, we can use the original approach (more memory efficient)
		reader, err := getInputReader(inputFiles[0])
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer reader.Close()

		tf, err := ioutil.TempFile("", "span-crossref-snapshot-")
		if err != nil {
			log.Fatal(err)
		}
		defer tf.Close()

		var (
			br      = bufio.NewReader(reader)
			bw      = bufio.NewWriter(tf)
			numErrs atomic.Int64 // error count across threads
		)

		pp := parallel.NewProcessor(br, bw, func(lineno int64, b []byte) ([]byte, error) {
			var (
				// This was a crossref.Document, but we only need a few fields.
				doc struct {
					DOI       string
					Deposited crossref.DateField `json:"deposited"`
					Indexed   crossref.DateField `json:"indexed"`
				}
				buf bytes.Buffer
			)
			if err := json.Unmarshal(b, &doc); err != nil {
				numErrs.Add(1)
				if n := numErrs.Load(); n > *errCountThreshold {
					return nil, err
				} else {
					log.Printf("skipping error (#err: %d <= max: %d): %v", n, *errCountThreshold, err)
				}
				return nil, nil
			}
			date, err := doc.Indexed.Date()
			if err != nil {
				numErrs.Add(1)
				if n := numErrs.Load(); n > *errCountThreshold {
					return nil, err
				} else {
					log.Printf("skipping error (#err: %d <= max: %d): %v", n, *errCountThreshold, err)
				}
				return nil, nil
			}
			if _, ok := excludes[doc.DOI]; ok {
				return nil, nil
			}
			if _, err := writeFields(&buf, "\t", lineno+1, date.Format("2006-01-02"), doc.DOI); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		})

		pp.BatchSize = *batchsize
		log.WithFields(log.Fields{
			"prefix":    "stage 1",
			"batchsize": *batchsize,
		}).Info("starting extraction")
		if err := pp.Run(); err != nil {
			log.Fatal(err)
		}
		if err := bw.Flush(); err != nil {
			log.Fatal(err)
		}

		stage1Output = tf.Name()
	} else {
		// For multiple files, process them one by one
		stage1Output, err = processFiles(inputFiles, excludes)
		if err != nil {
			log.Fatalf("error processing input files: %v", err)
		}
	}

	// Stage 2: Identify relevant records. Sort by DOI (3), then date reversed (2);
	// then unique by DOI (3). Should keep the entry of the last update (filename,
	// document date, DOI).
	fastsort := fmt.Sprintf(`LC_ALL=C sort -S%s`, *sortBufferSize)
	cmd := `{{ f }} -k3,3 -rk2,2 {{ input }} | {{ f }} -k3,3 -u | cut -f1 | {{ f }} -n > {{ output }}`
	log.WithFields(log.Fields{
		"prefix":    "stage 2",
		"batchsize": *batchsize,
	}).Info("identifying relevant records")
	output, err := clam.RunOutput(cmd, clam.Map{"f": fastsort, "input": stage1Output})
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(output)

	// External tools and fallbacks for stage 3. comp, decomp, filterline.
	comp, decomp := fmt.Sprintf(`%s -c`, *compressProgram), fmt.Sprintf(`%s -d -c`, *compressProgram)
	filterline := `filterline`
	if _, err := exec.LookPath("filterline"); err != nil {
		if _, err := exec.LookPath("awk"); err != nil {
			log.Fatal("filterline (git.io/v7qak) or awk is required")
		}
		tf, err := ioutil.TempFile("", "span-crossref-snapshot-filterline-")
		if err != nil {
			log.Fatal(err)
		}
		if _, err := io.WriteString(tf, fallback); err != nil {
			log.Fatal(err)
		}
		if err := tf.Close(); err != nil {
			log.Fatal(err)
		}
		if err := os.Chmod(tf.Name(), 0755); err != nil {
			log.Fatal(err)
		}
		defer os.Remove(tf.Name())
		filterline = tf.Name()
	}

	// Stage 3: Extract relevant records.
	// For multiple files, we need to concatenate them first
	var inputForStage3 string
	if len(inputFiles) == 1 {
		inputForStage3 = inputFiles[0]
	} else {
		var concatErr error
		inputForStage3, concatErr = concatenateFiles(inputFiles)
		if concatErr != nil {
			log.Fatalf("error concatenating files: %v", concatErr)
		}
		defer os.Remove(inputForStage3)
	}

	log.WithFields(log.Fields{
		"prefix":     "stage 3",
		"comp":       comp,
		"decomp":     decomp,
		"filterline": filterline,
	}).Info("extract relevant records")
	cmd = `{{ filterline }} {{ L }} {{ F }} > {{ output }}`
	if *compressed {
		switch *compressProgram {
		case "zstd":
			cmd = `{{ filterline }} {{ L }} <({{ decomp }} -T0 {{ F }}) | {{ comp }} -T0 > {{ output }}`
		default:
			cmd = `{{ filterline }} {{ L }} <({{ decomp }} {{ F }}) | {{ comp }} > {{ output }}`
		}
	}

	output, err = clam.RunOutput(cmd, clam.Map{
		"L":          output,
		"F":          inputForStage3,
		"filterline": filterline,
		"decomp":     decomp,
		"comp":       comp,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Rename(output, *outputFile); err != nil {
		if err := CopyFile(*outputFile, output, 0644); err != nil {
			log.Fatal(err)
		}
		os.Remove(output)
	}
}

// CopyFile copies the contents from src to dst using io.Copy.  If dst does not
// exist, CopyFile creates it with permissions perm; otherwise CopyFile
// truncates it before writing. From: https://codereview.appspot.com/152180043
func CopyFile(dst, src string, perm os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()
	_, err = io.Copy(out, in)
	return
}
