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
	"sort"
	"strconv"
	"strings"
	"sync"
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
// Line numbers are kept in memory; this should work up to the 10M line numbers
// range for a single file, but we can circumvent this by just writing line
// numbers to a file directly.
//
// Represent line numbers as a bitset; could keep 1B lines in 16MB.

var (
	MaxScanTokenSize  = 104857600 // 100MB, note each thread will allocate a buffer of this size
	Today             = time.Now().Format("2006-01-02")
	TempfilePrefix    = "sk-crossref-snapshot"
	DefaultOutputFile = path.Join(os.TempDir(), fmt.Sprintf("%s-%s.json.zst", TempfilePrefix, Today))

	// fallback awk script is used if the filterline executable is not found;
	// compiled filterline is about 3x faster.
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

// SnapshotOptions contains configuration for the snapshot process
type SnapshotOptions struct {
	InputFiles     []string
	OutputFile     string
	TempDir        string // Directory for temporary files
	BatchSize      int
	Workers        int
	SortBufferSize string // For sort -S parameter (e.g. "25%")
	KeepTempFiles  bool
	Verbose        bool
}

// DefaultSnapshotOptions returns default options.
func DefaultSnapshotOptions() SnapshotOptions {
	return SnapshotOptions{
		OutputFile:     DefaultOutputFile,
		TempDir:        os.TempDir(),
		BatchSize:      100000,
		Workers:        runtime.NumCPU(),
		SortBufferSize: "25%",
		KeepTempFiles:  false,
		Verbose:        false,
	}
}

// CreateSnapshot implements a three-stage metadata snapshot approach.
func CreateSnapshot(opts SnapshotOptions) error {
	if len(opts.InputFiles) == 0 {
		return fmt.Errorf("no input files provided")
	}
	indexTempFile, err := os.CreateTemp("", fmt.Sprintf("%s-index-*.txt", TempfilePrefix))
	if err != nil {
		return fmt.Errorf("error creating temporary index file: %v", err)
	}
	defer func() {
		indexTempFile.Close()
		if !opts.KeepTempFiles {
			_ = os.Remove(indexTempFile.Name())
		}
	}()
	lineNumsTempFile, err := os.CreateTemp(opts.TempDir, fmt.Sprintf("%s-lines-*.txt", TempfilePrefix))
	if err != nil {
		return fmt.Errorf("error creating temporary line numbers file: %v", err)
	}
	defer func() {
		lineNumsTempFile.Close()
		if !opts.KeepTempFiles {
			_ = os.Remove(lineNumsTempFile.Name())
		}
	}()
	if opts.Verbose {
		fmt.Printf("processing %d files with %d workers\n", len(opts.InputFiles), opts.Workers)
		fmt.Printf("output file: %s\n", opts.OutputFile)
		fmt.Printf("temporary index file: %s\n", indexTempFile.Name())
		fmt.Printf("temporary line numbers file: %s\n", lineNumsTempFile.Name())
		fmt.Printf("batch size: %d records\n", opts.BatchSize)
		fmt.Printf("sort buffer size: %s\n", opts.SortBufferSize)
	}
	// Stage 1: Extract DOI, timestamp, filename, and line number to temp file
	if opts.Verbose {
		fmt.Println("stage 1: extracting minimal information from input files")
	}
	started := time.Now()
	if err := extractMinimalInfo(opts.InputFiles, indexTempFile, opts.Workers, opts.BatchSize, opts.Verbose); err != nil {
		return fmt.Errorf("error in stage 1: %v", err)
	}
	// Close the index file to ensure all data is flushed
	if err := indexTempFile.Close(); err != nil {
		return fmt.Errorf("error closing index temp file: %v", err)
	}
	if opts.Verbose {
		// stage 1 completed in 1h9m26.202802464s
		fmt.Printf("stage 1 completed in %s\n", time.Since(started))
	}
	// Stage 2: Sort and find latest version of each DOI
	if opts.Verbose {
		fmt.Println("stage 2: identifying latest versions of each DOI")
	}
	started = time.Now()
	if err := identifyLatestVersions(indexTempFile.Name(), lineNumsTempFile.Name(), opts.SortBufferSize, opts.Verbose); err != nil {
		return fmt.Errorf("error in stage 2: %v", err)
	}
	if opts.Verbose {
		// stage 2 completed in 14m53.753035045s
		fmt.Printf("stage 2 completed in %s\n", time.Since(started))
	}
	// Stage 3: Extract identified lines to create final output
	if opts.Verbose {
		fmt.Println("stage 3: extracting relevant records to output file")
	}
	started = time.Now()
	if err := extractRelevantRecords(lineNumsTempFile.Name(), opts.InputFiles, opts.OutputFile, opts.Verbose); err != nil {
		return fmt.Errorf("error in Stage 3: %v", err)
	}
	if opts.Verbose {
		// stage 3 completed in 5h29m47.206642707s
		fmt.Printf("stage 3 completed in %s\n", time.Since(started))
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
	var lineNum int64 = 1
	for scanner.Scan() {
		line := scanner.Text()
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			fmt.Printf("skipping invalid JSON at %s:%d: %v\n", filename, lineNum, err)
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
	var indexMutex sync.Mutex
	if verbose {
		fmt.Printf("extracting minimal information with %d workers\n", numWorkers)
	}
	return processFilesParallel(inputFiles, numWorkers, func(inputPath string) error {
		if verbose {
			fmt.Printf("processing file: %s\n", inputPath)
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
				_, err := indexFile.Write(buffer.Bytes())
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
		// Write any remaining entries
		if buffer.Len() > 0 {
			indexMutex.Lock()
			_, err := indexFile.Write(buffer.Bytes())
			indexMutex.Unlock()
			if err != nil {
				return err
			}
			if verbose {
				fmt.Printf("done %s\n", inputPath)
			}
		}
		return nil
	})
}

// identifyLatestVersions sorts the index and identifies the latest version of each DOI
func identifyLatestVersions(indexFilePath, lineNumsFilePath, sortBufferSize string, verbose bool) error {
	if verbose {
		fmt.Println("sorting and identifying latest versions")
	}
	if sortBufferSize == "" {
		sortBufferSize = "25%"
	}
	pipeline := fmt.Sprintf(
		"LC_ALL=C sort -S%s -t $'\\t' -k4,4 -k3,3nr %s | LC_ALL=C sort -S%s -t $'\\t' -k4,4 -u | cut -f1,2 > %s",
		sortBufferSize, indexFilePath, sortBufferSize, lineNumsFilePath)

	if verbose {
		fmt.Printf("executing sort pipeline: %s\n", pipeline)
	}
	cmd := exec.Command("bash", "-c", pipeline)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing sort pipeline: %v\nOutput: %s", err, string(output))
	}
	fileInfo, err := os.Stat(lineNumsFilePath)
	if err != nil {
		return fmt.Errorf("error checking line numbers file: %v", err)
	}
	if fileInfo.Size() == 0 {
		return fmt.Errorf("error: line numbers file is empty after sorting")
	}
	if verbose {
		fmt.Printf("sorting and filtering complete, output file size: %d bytes\n", fileInfo.Size())
	}
	return nil
}

// extractRelevantRecords extracts the identified lines from the original
// files; XXX: this is slower than a filterline/zstd (external) approach.
func extractRelevantRecords(lineNumsFilePath string, inputFiles []string, outputFilePath string, verbose bool) error {
	if verbose {
		fmt.Println("extracting relevant records")
	}
	fileLineMap, err := groupLineNumbersByFile(lineNumsFilePath, verbose)
	if err != nil {
		return err
	}
	totalExtracted := 0
	for _, inputFile := range inputFiles {
		lineNumbers, ok := fileLineMap[inputFile]
		if !ok {
			if verbose {
				fmt.Printf("no records to extract from %s\n", inputFile)
			}
			continue
		}
		if verbose {
			fmt.Printf("extracting %d records from %s\n", len(lineNumbers.numbers), inputFile)
		}
		lineNumbers.sort()
		extracted, err := extractLinesFromFile(inputFile, lineNumbers, outputFilePath, verbose)
		if err != nil {
			return err
		}
		totalExtracted += extracted
		if verbose {
			fmt.Printf("extracted %d records from %s\n", extracted, inputFile)
		}
	}
	if verbose {
		fmt.Printf("total records extracted: %d\n", totalExtracted)
	}
	return nil
}

// LineNumbers represents a collection of line numbers to extract from a file
type LineNumbers struct {
	numbers []int64
}

func (ln *LineNumbers) add(num int64) {
	ln.numbers = append(ln.numbers, num)
}

func (ln *LineNumbers) sort() {
	sort.Slice(ln.numbers, func(i, j int) bool {
		return ln.numbers[i] < ln.numbers[j]
	})
}

// groupLineNumbersByFile reads the line numbers file and groups by filename
func groupLineNumbersByFile(lineNumsFilePath string, verbose bool) (map[string]*LineNumbers, error) {
	file, err := os.Open(lineNumsFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening line numbers file: %v", err)
	}
	defer file.Close()
	var (
		fileLineMap = make(map[string]*LineNumbers)
		scanner     = bufio.NewScanner(file)
		linesRead   = 0
	)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line format: %s", line)
		}
		filename := parts[0]
		lineNum, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid line number: %s", parts[1])
		}
		if _, ok := fileLineMap[filename]; !ok {
			fileLineMap[filename] = &LineNumbers{numbers: make([]int64, 0)}
		}
		fileLineMap[filename].add(lineNum)
		linesRead++
		if verbose && linesRead%1000000 == 0 {
			fmt.Printf("read %d lines from line numbers file\n", linesRead)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading line numbers file: %v", err)
	}
	if verbose {
		fmt.Printf("read total of %d lines from line numbers file\n", linesRead)
		fmt.Printf("found lines for %d files\n", len(fileLineMap))
	}

	return fileLineMap, nil
}

// extractLinesFromFile uses external tools to perform the slicing.
func extractLinesFromFile(filename string, lineNumbers *LineNumbers, outputFile string, verbose bool) (int, error) {
	lineNumTempFile, err := os.CreateTemp("", fmt.Sprintf("%s-line-numbers-*.txt", TempfilePrefix))
	if err != nil {
		return 0, fmt.Errorf("error creating temp file: %w", err)
	}
	defer os.Remove(lineNumTempFile.Name())
	for _, num := range lineNumbers.numbers {
		_, err := fmt.Fprintf(lineNumTempFile, "%d\n", num)
		if err != nil {
			return 0, err
		}
	}
	if err := lineNumTempFile.Close(); err != nil {
		return 0, err
	}
	filterlineExe := `filterline`
	if !isCommandAvailable(filterlineExe) {
		filterlineExe, err = createFallbackScript()
		if err != nil {
			return 0, err
		}
	}
	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(filename, ".zst"):
		cmd = exec.Command("bash", "-c", fmt.Sprintf("%s %s <(zstd -cd -T0 %s) | zstd -c9 -T0 >> %s", filterlineExe, lineNumTempFile.Name(), filename, outputFile))
	case strings.HasSuffix(filename, ".gz"):
		cmd = exec.Command("bash", "-c", fmt.Sprintf("%s %s <(gzip -cd %s) | gzip -c9 >> %s", filterlineExe, lineNumTempFile.Name(), filename, outputFile))
	default:
		cmd = exec.Command("bash", "-c", fmt.Sprintf("%s %s %s >> %s", filterlineExe, lineNumTempFile.Name(), filename, outputFile))
	}
	if verbose {
		log.Printf("extracting lines with: %v", cmd)
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("command failed: %v", string(b))
	}
	return len(lineNumbers.numbers), err
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
