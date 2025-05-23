package crossref

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/segmentio/encoding/json"
)

// Some optimization ideas: If an input file is large, we could split it up into
// smaller chunks; albeit we would have to split at line boundaries.
//
// Extract relevant info from JSON w/o parsing JSON.
//
// The final extraction stage could run also in parallel, albeit i/o may also
// be a bottleneck, depending. But could try to run extraction over say 8 files
// in parallel and do a final concatenation of all zstd files; as zstd allows streaming.
//
// Currently, we open one temp file for each input file, to write line numebers
// to extract to. Either write to separate files directly, or be more cautions
// regarding the number of open files.
//
// Represent line numbers as a bitset; could keep 1B lines in 16MB.

var (
	MaxScanTokenSize  = 104_857_600 // 100MB, note: each thread will allocate a buffer of this size
	Today             = time.Now().Format("2006-01-02")
	TempfilePrefix    = "sk-crossref-snapshot"
	DefaultOutputFile = path.Join(os.TempDir(), fmt.Sprintf("%s-%s.json.zst", TempfilePrefix, Today))

	// fallback awk script is used if the filterline executable is not found;
	// compiled filterline is about 3x faster;
	// https://unix.stackexchange.com/q/209404/376, cf.
	// https://github.com/miku/filterline
	fallbackFilterlineScript = `#!/bin/bash
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
)

// Record represents the JSON structure we're interested in
type Record struct {
	DOI     string `json:"DOI"`
	Indexed struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"indexed"`
}

// SnapshotOptions contains configuration for the snapshot process.
type SnapshotOptions struct {
	InputFiles     []string // InputFiles, following a Record structure.
	OutputFile     string   // OutputFile is the file the snapshot is written to.
	TempDir        string   // Directory for temporary files.
	BatchSize      int      // BatchSize is the number records we process at once, affect memory usage.
	NumWorkers     int      // Number of threads, each thread may allocate buffers.
	SortBufferSize string   // For sort -S parameter (e.g. "25%"), curcial for faster sort.
	KeepTempFiles  bool     // For debugging.
	Verbose        bool     // Verbose output.
}

type LineNumberEntry struct {
	LineNumbersFilename string
	NumLines            int64
}

// LineNumbersFileMap maps a filename to the associated filename that contains
// the line numbers to extract and the number of lines in that file.
type LineNumbersFileMap map[string]*LineNumberEntry

// DefaultSnapshotOptions returns default options.
func DefaultSnapshotOptions() SnapshotOptions {
	return SnapshotOptions{
		OutputFile:     DefaultOutputFile,
		TempDir:        os.TempDir(),
		BatchSize:      100000,
		NumWorkers:     runtime.NumCPU(),
		SortBufferSize: "25%",
		KeepTempFiles:  false,
		Verbose:        false,
	}
}

// CreateSnapshot implements a three-stage metadata snapshot approach, given
// snapshot options. Tihs allows to create a current view of crossref our of a
// continously harvested set of files.
//
// On a machine with fast i/o, many parts of this process can be cpu bound,
// whereas on spinning disks, this will likely be i/o bound.
//
// An error is returned, if the snapshot options do not contain any files to
// process.
//
// On a 2011 dual-socket Xeon E5645 with spinning disk, the whole process runs
// in: 78189.57user 6229.72system 7:59:19elapsed 293%CPU -- or about 21 hours.
// On a 2023 i9-13900T with raid0 nvme disks the process runs in about 3-4 hours.
func CreateSnapshot(opts SnapshotOptions) error {
	if len(opts.InputFiles) == 0 {
		return fmt.Errorf("no input files provided")
	}
	indexTempFile, err := os.CreateTemp("", fmt.Sprintf("%s-index-*.txt.zst", TempfilePrefix))
	if err != nil {
		return fmt.Errorf("error creating temporary index file: %v", err)
	}
	defer func() {
		_ = indexTempFile.Close()
		if !opts.KeepTempFiles {
			_ = os.Remove(indexTempFile.Name())
		}
	}()
	lineNumsTempFile, err := os.CreateTemp(opts.TempDir, fmt.Sprintf("%s-lines-*.txt", TempfilePrefix))
	if err != nil {
		return fmt.Errorf("error creating temporary line numbers file: %v", err)
	}
	defer func() {
		_ = lineNumsTempFile.Close()
		if !opts.KeepTempFiles {
			_ = os.Remove(lineNumsTempFile.Name())
		}
	}()
	if opts.Verbose {
		log.Printf("processing %d files with %d workers\n", len(opts.InputFiles), opts.NumWorkers)
		log.Printf("output file: %s\n", opts.OutputFile)
		log.Printf("temporary index file: %s\n", indexTempFile.Name())
		log.Printf("temporary line numbers file: %s\n", lineNumsTempFile.Name())
		log.Printf("batch size: %d records\n", opts.BatchSize)
		log.Printf("sort buffer size: %s\n", opts.SortBufferSize)
	}
	// Stage 1: Extract DOI, timestamp, filename, and line number to temp file
	if opts.Verbose {
		fmt.Println("stage 1: extracting minimal information from input files")
	}
	started := time.Now()
	if err := extractMinimalInfo(opts.InputFiles, indexTempFile, opts.NumWorkers, opts.BatchSize, opts.Verbose); err != nil {
		return fmt.Errorf("error in stage 1: %v", err)
	}
	// Close the index file to ensure all data is flushed
	if err := indexTempFile.Close(); err != nil {
		return fmt.Errorf("error closing index temp file: %v", err)
	}
	if opts.Verbose {
		// stage 1 completed in 1h9m26.202802464s
		log.Printf("stage 1 completed in %s", time.Since(started))
	}
	// Stage 2: Sort and find latest version of each DOI
	if opts.Verbose {
		log.Println("stage 2: identifying latest versions of each DOI")
	}
	started = time.Now()
	if err := identifyLatestVersions(indexTempFile.Name(), lineNumsTempFile.Name(), opts.SortBufferSize, opts.Verbose); err != nil {
		return fmt.Errorf("error in stage 2: %v", err)
	}
	if opts.Verbose {
		// stage 2 completed in 14m53.753035045s
		log.Printf("stage 2 completed in %s", time.Since(started))
	}
	// Stage 3: Extract identified lines to create final output
	if opts.Verbose {
		log.Println("stage 3: extracting relevant records to output file")
	}
	started = time.Now()
	if err := extractRelevantRecords(lineNumsTempFile.Name(), opts.InputFiles, opts.OutputFile, opts.SortBufferSize, opts.Verbose); err != nil {
		return fmt.Errorf("error in Stage 3: %v", err)
	}
	if opts.Verbose {
		// [i7-13900T] stage 3 completed in 1h40m0.654023657s (previously, with pure Go zstd and filtering it took 5h29m)
		// [E5645] 2025/04/28 15:43:46 stage 3 completed in 4h15m39.224463716s
		log.Printf("stage 3 completed in %s", time.Since(started))
	}
	return nil
}

// openFile opens a file and returns a reader, detecting if the file is compressed
func openFile(filename string) (io.ReadCloser, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasSuffix(filename, ".gz"):
		zr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return zr, nil
	case strings.HasSuffix(filename, ".zst"):
		zr, err := zstd.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return io.NopCloser(zr), nil
	default:
		return f, nil
	}
}

// processFile reads a file line by line, parsing JSON and calling the provided function
func processFile(filename string, fn func(line string, record Record, lineNum int64) error) error {
	r, err := openFile(filename)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", filename, err)
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, MaxScanTokenSize)
	scanner.Buffer(buf, MaxScanTokenSize)
	var lineNum int64 = 1 // downstream filterline and tools like sed use 1-based line numbers
	for scanner.Scan() {
		line := scanner.Text()
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			log.Printf("skipping invalid JSON at %s:%d: %v\n", filename, lineNum, err)
			lineNum++
			continue
		}
		if record.DOI == "" {
			lineNum++
			continue
		}
		if err := fn(line, record, lineNum); err != nil {
			return err
		}
		lineNum++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %v", filename, err)
	}
	return nil
}

// processFilesParallel processes files in parallel using worker goroutines
func processFilesParallel(inputFiles []string, numWorkers int, processor func(string) error) error {
	filesCh := make(chan string, len(inputFiles))
	for _, file := range inputFiles {
		filesCh <- file
	}
	close(filesCh)
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	errCh := make(chan error, numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for filename := range filesCh {
				if err := processor(filename); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// extractMinimalInfo processes all input files and extracts DOI, timestamp,
// filename, and line number; this data will go into an indexFile;
// uncompressed, this file is about 70GB in size, but could probably compressed
// as well
func extractMinimalInfo(inputFiles []string, indexFile *os.File, numWorkers, batchSize int, verbose bool) error {
	var (
		indexMutex   sync.Mutex
		numProcessed atomic.Uint64
	)
	if verbose {
		log.Printf("extracting minimal information with %d workers", numWorkers)
	}
	zw, err := zstd.NewWriter(indexFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = zw.Flush()
		_ = zw.Close()
	}()
	return processFilesParallel(inputFiles, numWorkers, func(inputPath string) error {
		if verbose {
			log.Printf("processing file: %s", inputPath)
		}
		var (
			buffer           bytes.Buffer
			entriesProcessed = 0
		)
		err := processFile(inputPath, func(line string, record Record, lineNum int64) error {
			// Format: filename \t lineNumber \t timestamp \t DOI
			_, _ = fmt.Fprintf(&buffer, "%s\t%d\t%d\t%s\n",
				inputPath, lineNum, record.Indexed.Timestamp, record.DOI)
			entriesProcessed++
			if entriesProcessed >= batchSize {
				indexMutex.Lock()
				_, err := zw.Write(buffer.Bytes())
				indexMutex.Unlock()
				if err != nil {
					return err
				}
				buffer.Reset()
				entriesProcessed = 0
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error processing file %s: %v", inputPath, err)
		}
		numProcessed.Add(1)
		// Write any remaining entries, report progress.
		if buffer.Len() > 0 {
			indexMutex.Lock()
			_, err := zw.Write(buffer.Bytes())
			indexMutex.Unlock()
			if err != nil {
				return err
			}
			if verbose {
				donePct := float64(numProcessed.Load()) / float64(len(inputFiles)) * 100
				log.Printf("done [%d/%d][%0.2f%%]: %s", numProcessed.Load(), len(inputFiles), donePct, inputPath)
			}
		}
		return nil
	})
}

// identifyLatestVersions sorts the index and identifies the latest version of each DOI
func identifyLatestVersions(indexFilePath, lineNumsFilePath, sortBufferSize string, verbose bool) error {
	if verbose {
		log.Println("sorting and identifying latest versions")
	}
	if sortBufferSize == "" {
		// Initial buffer size, as percentage of RAM.
		sortBufferSize = "25%"
	}
	var pipeline string
	// Takes the index file and sorts it by first ID (4,4) and date (3,3)
	// reversed; keeps the latest and writes out (filename, linenumber) pairs.
	switch {
	case isZstdCompressed(indexFilePath):
		pipeline = fmt.Sprintf(
			"zstd -cd -T0 %s | LC_ALL=C sort --compress-program=zstd --parallel %d -S%s -t $'\\t' -k4,4 -k3,3nr | LC_ALL=C sort --compress-program=zstd --parallel %d -S%s -t $'\\t' -k4,4 -u | cut -f1,2 > %s",
			indexFilePath, runtime.NumCPU(), sortBufferSize, runtime.NumCPU(), sortBufferSize, lineNumsFilePath)
	default:
		pipeline = fmt.Sprintf(
			"LC_ALL=C sort --compress-program=zstd --parallel %d -S%s -t $'\\t' -k4,4 -k3,3nr %s | LC_ALL=C sort --compress-program=zstd --parallel %d -S%s -t $'\\t' -k4,4 -u | cut -f1,2 > %s",
			runtime.NumCPU(), sortBufferSize, indexFilePath, runtime.NumCPU(), sortBufferSize, lineNumsFilePath)
	}
	if verbose {
		log.Printf("executing sort pipeline: %s", pipeline)
	}
	cmd := exec.Command("bash", "-c", pipeline)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing sort pipeline: %v\nOutput: %s", err, string(b))
	}
	fileInfo, err := os.Stat(lineNumsFilePath)
	if err != nil {
		return fmt.Errorf("error checking line numbers file: %v", err)
	}
	if fileInfo.Size() == 0 {
		return fmt.Errorf("error: line numbers file is empty after sorting")
	}
	if verbose {
		log.Printf("sorting and filtering complete, output file size: %d bytes", fileInfo.Size())
	}
	return nil
}

func isZstdCompressed(filename string) bool {
	f, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer f.Close()
	zr, err := zstd.NewReader(f)
	if err != nil {
		return false
	}
	defer zr.Close()
	var dummy = make([]byte, 64)
	if _, err := zr.Read(dummy); err != nil {
		return false
	}
	return true
}

// extractRelevantRecords extracts the identified lines from the original
// files. After successful completion, all temporary files will be cleaned up.
func extractRelevantRecords(lineNumsFilePath string, inputFiles []string, outputFilePath string, sortBufferSize string, verbose bool) error {
	if verbose {
		fmt.Println("extracting relevant records")
	}
	fileLineMap, err := groupLineNumbersByFile(lineNumsFilePath, sortBufferSize, verbose)
	if err != nil {
		return err
	}
	var totalExtracted int64 = 0
	for _, inputFile := range inputFiles {
		entry, ok := fileLineMap[inputFile]
		if !ok {
			if verbose {
				log.Printf("no records to extract from %s", inputFile)
			}
			continue
		}
		err := extractLinesFromFile(inputFile, entry.LineNumbersFilename, outputFilePath, verbose)
		if err != nil {
			return err
		}
		totalExtracted += entry.NumLines
		if verbose {
			log.Printf("extracted %d records from %s", entry.NumLines, inputFile)
		}
	}
	if verbose {
		log.Printf("total records extracted: %d", totalExtracted)
	}
	// Cleanup temporary line number files.
	for _, entry := range fileLineMap {
		_ = os.Remove(entry.LineNumbersFilename)
	}
	return nil
}

// groupLineNumbersByFile reads the line numbers file, groups data by filename
// and will write out one TSV file with just the line number for each file.
// XXX: This may fail, if the number of input files gets closer to
// /proc/sys/fs/file-max.
func groupLineNumbersByFile(lineNumsFilePath string, sortBufferSize string, verbose bool) (LineNumbersFileMap, error) {
	file, err := os.Open(lineNumsFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening line numbers file: %v", err)
	}
	defer file.Close()
	var (
		fileLineMap = make(LineNumbersFileMap)       // map input file name to line number filename
		tempFileMap = make(map[string]*bufio.Writer) // map input file name to the line number file descriptor
		scanner     = bufio.NewScanner(file)         // the "global" line numbers file
		linesRead   = 0                              // number of lines read so far
		tempFiles   = make([]*os.File, 0)            // keep track of all temporary files; only for cleanup
	)
	for scanner.Scan() {
		var (
			line  = scanner.Text()
			parts = strings.Fields(line)
		)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line format: %s", line)
		}
		var (
			filename = parts[0]
			lineNum  = parts[1]
		)
		if _, ok := fileLineMap[filename]; !ok {
			safeName := strings.Replace(path.Base(filename), ".", "-", -1)
			tempFile, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("%s-lines-%s-*.txt", TempfilePrefix, safeName))
			if err != nil {
				// We cleanup all previously created temporary files.
				for _, tf := range tempFiles {
					_ = tf.Close()
					_ = os.Remove(tf.Name())
				}
				return nil, fmt.Errorf("error creating temp file: %w", err)
			}
			tempFiles = append(tempFiles, tempFile)
			bw := bufio.NewWriter(tempFile)
			defer func() {
				_ = tempFile.Close()
			}()
			tempFileMap[filename] = bw
			fileLineMap[filename] = &LineNumberEntry{
				LineNumbersFilename: tempFile.Name(),
				NumLines:            0,
			}
		}
		_, err := fmt.Fprintln(tempFileMap[filename], lineNum)
		if err != nil {
			return nil, err
		}
		linesRead++
		fileLineMap[filename].NumLines++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading line numbers file: %v", err)
	}
	for _, tf := range tempFileMap {
		if err := tf.Flush(); err != nil {
			return fileLineMap, err
		}
	}
	for _, tf := range tempFiles {
		if err := tf.Close(); err != nil {
			return fileLineMap, err
		}
	}
	// We need to sort the file with the line numbers for the "filterline"
	// approach to work.
	for _, entry := range fileLineMap {
		cmd := exec.Command("sort", "-n", "-S", sortBufferSize,
			"--parallel", strconv.Itoa(runtime.NumCPU()),
			"-o", entry.LineNumbersFilename, entry.LineNumbersFilename)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "LC_ALL=C")
		b, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("sorting line numbers file failed: %v", string(b))
			return nil, err
		}
	}
	return fileLineMap, nil
}

// extractLinesFromFile uses external tools to perform the slicing. If the
// filterline program can be found, it will use that. Otherwise fall back to a
// small awk snippet (that is a bit slower).
func extractLinesFromFile(filename string, lineNumbersFile string, outputFile string, verbose bool) error {
	var (
		filterlineExe = "filterline"
		err           error
	)
	if !isCommandAvailable(filterlineExe) {
		filterlineExe, err = createFallbackScript()
		if err != nil {
			return err
		}
	}
	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(filename, ".zst"):
		cmd = exec.Command("bash", "-c", fmt.Sprintf("%s %s <(zstd -cd -T0 %s) | zstd -c9 -T0 >> %s", filterlineExe, lineNumbersFile, filename, outputFile))
	case strings.HasSuffix(filename, ".gz"):
		cmd = exec.Command("bash", "-c", fmt.Sprintf("%s %s <(gzip -cd %s) | gzip -c9 >> %s", filterlineExe, lineNumbersFile, filename, outputFile))
	default:
		cmd = exec.Command("bash", "-c", fmt.Sprintf("%s %s %s >> %s", filterlineExe, lineNumbersFile, filename, outputFile))
	}
	if verbose {
		log.Println(cmd)
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("command failed: %v", string(b))
	}
	return err
}

// isCommandAvailable checks if a command is available in the system PATH
func isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// createFallbackScript creates a temporary script file with the fallback awk script
func createFallbackScript() (string, error) {
	scriptFile, err := os.CreateTemp("", fmt.Sprintf("%s-fallback-*.sh", TempfilePrefix))
	if err != nil {
		return "", fmt.Errorf("error creating fallback script: %v", err)
	}
	if _, err := scriptFile.WriteString(fallbackFilterlineScript); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptFile.Name())
		return "", fmt.Errorf("error writing fallback script: %v", err)
	}
	_ = scriptFile.Close()
	if err := os.Chmod(scriptFile.Name(), 0755); err != nil {
		_ = os.Remove(scriptFile.Name())
		return "", fmt.Errorf("error making fallback script executable: %v", err)
	}
	return scriptFile.Name(), nil
}
