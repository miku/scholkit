// Package crossref provides tools for processing Crossref metadata.
package crossref

import (
	"bufio"
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
	"time"

	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/segmentio/encoding/json"
)

var (
	DefaultIndexFile = path.Join(os.TempDir(),
		fmt.Sprintf("crossref-snapshot-index-%v.idx", time.Now().Format("2006-01-02")))
	DefaultOutputFile = path.Join(os.TempDir(),
		fmt.Sprintf("crossref-snapshot-%s.json.zst", time.Now().Format("2006-01-02")))
)

// Record represents the JSON structure of a crossref record we are interested in.
type Record struct {
	DOI     string `json:"DOI"`
	Indexed struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"indexed"`
}

// IndexEntry is an entry in the index.
type IndexEntry struct {
	DOI        string `json:"d"` // DOI
	Timestamp  int64  `json:"t"` // Timestamp
	Filename   string `json:"f"` // Filename
	LineNumber int64  `json:"l"` // LineNumber
}

// SnapshotOptions contains configuration for the snapshot process.
type SnapshotOptions struct {
	InputFiles []string
	OutputFile string
	IndexFile  string
	BatchSize  int
	Workers    int
	KeepIndex  bool
	Verbose    bool
}

// DefaultSnapshotOptions returns a SnapshotOptions with sensible defaults
func DefaultSnapshotOptions() SnapshotOptions {
	return SnapshotOptions{
		OutputFile: DefaultOutputFile,
		IndexFile:  DefaultIndexFile,
		BatchSize:  100_000,
		Workers:    runtime.NumCPU(),
		KeepIndex:  false,
		Verbose:    false,
	}
}

// CreateSnapshot processes crossref records and creates a snapshot with the latest version of each
func CreateSnapshot(opts SnapshotOptions) error {
	if len(opts.InputFiles) == 0 {
		return fmt.Errorf("no input files provided")
	}
	if opts.Verbose {
		log.Printf("processing %d files with %d workers", len(opts.InputFiles), opts.Workers)
		log.Printf("output file: %s", opts.OutputFile)
		log.Printf("index file: %s", opts.IndexFile)
		log.Printf("batch size: %d records", opts.BatchSize)
	}
	if err := buildIndex(opts.InputFiles, opts.IndexFile, opts.BatchSize, opts.Workers, opts.Verbose); err != nil {
		return fmt.Errorf("error building index: %w", err)
	}
	if err := extractLatestRecords(opts.IndexFile, opts.OutputFile, opts.Verbose); err != nil {
		return fmt.Errorf("error extracting latest records: %w", err)
	}
	if !opts.KeepIndex {
		_ = os.Remove(opts.IndexFile)
	} else if opts.Verbose {
		log.Printf("index file kept at: %s", opts.IndexFile)
	}
	return nil
}

// openFile opens a file and returns a reader, detecting if the file is
// compressed.
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
	const maxScanTokenSize = 100 * 1024 * 1024 // 100MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)
	var lineNum int64 = 0
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

// buildIndex processes all input files and creates an index of DOIs and their latest versions
func buildIndex(inputFiles []string, indexFilePath string, batchSize, numWorkers int, verbose bool) error {
	indexFile, err := os.Create(indexFilePath)
	if err != nil {
		return fmt.Errorf("error creating index file: %v", err)
	}
	defer indexFile.Close()
	var indexMutex sync.Mutex
	if verbose {
		log.Printf("building index with %d workers...", numWorkers)
	}
	err = processFilesParallel(inputFiles, numWorkers, func(inputPath string) error {
		if verbose {
			log.Printf("processing file: %s", inputPath)
		}
		var (
			doiMap           = make(map[string]IndexEntry, batchSize)
			entriesProcessed = 0
		)
		err := processFile(inputPath, func(line string, record Record, lineNum int64) error {
			existing, ok := doiMap[record.DOI]
			if !ok || record.Indexed.Timestamp > existing.Timestamp {
				doiMap[record.DOI] = IndexEntry{
					DOI:        record.DOI,
					Timestamp:  record.Indexed.Timestamp,
					Filename:   inputPath,
					LineNumber: lineNum,
				}
			}
			entriesProcessed++
			if entriesProcessed >= batchSize {
				entries := make([]IndexEntry, 0, len(doiMap))
				for _, entry := range doiMap {
					entries = append(entries, entry)
				}
				indexMutex.Lock()
				err := writeIndexEntries(indexFile, entries)
				indexMutex.Unlock()
				if err != nil {
					return fmt.Errorf("error writing to index file: %v", err)
				}
				doiMap = make(map[string]IndexEntry, batchSize)
				entriesProcessed = 0

				if verbose {
					log.Printf("processed %d records from %s", entriesProcessed, inputPath)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error processing file %s: %v", inputPath, err)
		}
		if len(doiMap) > 0 {
			entries := make([]IndexEntry, 0, len(doiMap))
			for _, entry := range doiMap {
				entries = append(entries, entry)
			}
			indexMutex.Lock()
			err := writeIndexEntries(indexFile, entries)
			indexMutex.Unlock()
			if err != nil {
				return fmt.Errorf("error writing to index file: %v", err)
			}
			if verbose {
				log.Printf("processed final %d records from %s", len(doiMap), inputPath)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if verbose {
		log.Print("index building complete")
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
		go func(workerID int) {
			defer wg.Done()
			for filename := range filesCh {
				if err := processor(filename); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return fmt.Errorf("error during processing: %v", err)
		}
	}
	return nil
}

// writeIndexEntries writes out an index extra as TSV.
func writeIndexEntries(indexFile *os.File, entries []IndexEntry) error {
	writer := bufio.NewWriter(indexFile)
	for _, entry := range entries {
		line := fmt.Sprintf("%s\t%d\t%s\t%d\n", entry.DOI, entry.Timestamp, entry.Filename, entry.LineNumber)
		if _, err := writer.WriteString(line); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// sortIndexFile sorts the index file using external sort program.
func sortIndexFile(indexFilePath string, verbose bool) (string, error) {
	sortedIndexPath := indexFilePath + ".sorted"
	if verbose {
		log.Printf("sorting index file with external sort: %s -> %s", indexFilePath, sortedIndexPath)
	}
	cmd := exec.Command("sort", "-k1,1", "-S70%", "-o", sortedIndexPath, indexFilePath)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to sort index file: %v", err)
	}

	return sortedIndexPath, nil
}

// compositeWriteCloser ensures both the compression writer and file are closed properly
type compositeWriteCloser struct {
	writer       io.WriteCloser
	outFile      *os.File
	isCompressed bool
}

func (c *compositeWriteCloser) Write(p []byte) (n int, err error) {
	return c.writer.Write(p)
}

func (c *compositeWriteCloser) Close() error {
	if c.isCompressed {
		if err := c.writer.Close(); err != nil {
			c.outFile.Close()
			return err
		}
	}
	return c.outFile.Close()
}

// createOutputWriter creates a writer for the output file, with optional compression
func createOutputWriter(outputFilePath string) (io.WriteCloser, error) {
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return nil, fmt.Errorf("error creating output file: %v", err)
	}
	switch {
	case strings.HasSuffix(outputFilePath, ".zst"):
		zw, err := zstd.NewWriter(outFile)
		if err != nil {
			_ = outFile.Close()
			return nil, fmt.Errorf("error creating zstd writer: %v", err)
		}
		return &compositeWriteCloser{
			writer:       zw,
			outFile:      outFile,
			isCompressed: true,
		}, nil
	case strings.HasSuffix(outputFilePath, ".gz"):
		gzw := gzip.NewWriter(outFile)
		return &compositeWriteCloser{
			writer:       gzw,
			outFile:      outFile,
			isCompressed: true,
		}, nil
	}
	return outFile, nil
}

// extractLatestRecords extracts data via chunked approach.
func extractLatestRecords(indexFilePath string, outputFilePath string, verbose bool) error {
	// Sort the index file
	sortedIndexPath, err := sortIndexFile(indexFilePath, verbose)
	if err != nil {
		return err
	}
	defer os.Remove(sortedIndexPath)
	chunkSize := 1_000_000 // adjust based on memory constraints
	return extractLatestRecordsChunked(sortedIndexPath, outputFilePath, chunkSize, verbose)
}

// New function to process the sorted index in chunks
func extractLatestRecordsChunked(sortedIndexPath, outputFilePath string, chunkSize int, verbose bool) error {
	if verbose {
		log.Printf("processing sorted index in chunks of %d records", chunkSize)
	}
	// Open sorted index file
	indexFile, err := os.Open(sortedIndexPath)
	if err != nil {
		return fmt.Errorf("error opening sorted index: %v", err)
	}
	defer indexFile.Close()
	// Create output file
	outWriter, err := createOutputWriter(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outWriter.Close()
	bufWriter := bufio.NewWriter(outWriter)
	defer bufWriter.Flush()
	scanner := bufio.NewScanner(indexFile)
	var (
		currentChunk    = make(map[string]IndexEntry)
		lastDOI         string
		extractedCount  = 0
		processedChunks = 0
	)
	// Process the index file one line at a time
	for scanner.Scan() {
		var (
			line      = scanner.Text()
			parts     = strings.Split(line, "\t")
			timestamp int64
			lineNum   int64
			err       error
		)
		if len(parts) != 4 {
			return fmt.Errorf("invalid index line format: %s", line)
		}
		doi := parts[0]
		if timestamp, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			return err
		}
		filename := parts[2]
		if lineNum, err = strconv.ParseInt(parts[3], 10, 64); err != nil {
			return err
		}
		entry := IndexEntry{
			DOI:        doi,
			Timestamp:  timestamp,
			Filename:   filename,
			LineNumber: lineNum,
		}

		// If we've moved to a new DOI and current DOI is different from last DOI we processed
		if lastDOI != "" && doi != lastDOI {
			// Process the current chunk if it's full or we've encountered a new DOI
			if len(currentChunk) >= chunkSize {
				if err := processChunk(currentChunk, bufWriter, verbose); err != nil {
					return err
				}
				extractedCount += len(currentChunk)
				processedChunks++
				if verbose {
					log.Printf("processed chunk %d with %d records, total: %d",
						processedChunks, len(currentChunk), extractedCount)
				}
				currentChunk = make(map[string]IndexEntry)
			}
		}
		lastDOI = doi
		// For each DOI, keep only the entry with the latest timestamp
		existingEntry, ok := currentChunk[doi]
		if !ok || entry.Timestamp > existingEntry.Timestamp {
			currentChunk[doi] = entry
		}
	}
	// Process the last chunk
	if len(currentChunk) > 0 {
		if err := processChunk(currentChunk, bufWriter, verbose); err != nil {
			return err
		}
		extractedCount += len(currentChunk)
		processedChunks++
		if verbose {
			log.Printf("processed final chunk %d with %d records, total: %d",
				processedChunks, len(currentChunk), extractedCount)
		}
	}
	if err := bufWriter.Flush(); err != nil {
		return fmt.Errorf("error flushing output buffer: %v", err)
	}
	if verbose {
		log.Printf("extraction complete, wrote %d records to %s", extractedCount, outputFilePath)
	}
	return nil
}

// processChunk works on a chunk of index entries.
func processChunk(chunk map[string]IndexEntry, writer *bufio.Writer, verbose bool) error {
	fileEntries := make(map[string][]IndexEntry)
	for _, entry := range chunk {
		fileEntries[entry.Filename] = append(fileEntries[entry.Filename], entry)
	}
	for filename, entries := range fileEntries {
		if verbose {
			log.Printf("extracting %d records from %s", len(entries), filename)
		}
		lineMap := make(map[int64]string)
		for _, entry := range entries {
			lineMap[entry.LineNumber] = entry.DOI
		}
		r, err := openFile(filename)
		if err != nil {
			return fmt.Errorf("error opening file %s: %v", filename, err)
		}
		scanner := bufio.NewScanner(r)
		var lineNum int64 = 0
		for scanner.Scan() {
			line := scanner.Text()
			doi, shouldExtract := lineMap[lineNum]
			if shouldExtract {
				var record Record
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					log.Printf("warning: invalid JSON at %s:%d, skipping", filename, lineNum)
				} else if record.DOI == doi {
					if _, err := writer.WriteString(line + "\n"); err != nil {
						r.Close()
						return fmt.Errorf("error writing to output: %v", err)
					}
				}
			}
			lineNum++
		}
		r.Close()
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading file %s: %v", filename, err)
		}
	}
	return nil
}
