// Package crossref provides tools for processing Crossref metadata.
package crossref

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/segmentio/encoding/json"
)

var (
	DefaultIndexFile = path.Join(os.TempDir(),
		fmt.Sprintf("crossref-snapshot-index-%v.dat", time.Now().Format("2006-01-02")))
	DefaultOutputFile = path.Join(os.TempDir(),
		fmt.Sprintf("crossref-snapshot-%s.json.zst", time.Now().Format("2006-01-02")))
)

// Record represents the JSON structure we're interested in
type Record struct {
	DOI     string `json:"DOI"`
	Indexed struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"indexed"`
}

// IndexEntry uses shorter field names to reduce JSON size
type IndexEntry struct {
	DOI        string `json:"d"` // DOI
	Timestamp  int64  `json:"t"` // Timestamp
	Filename   string `json:"f"` // Filename
	LineNumber int64  `json:"l"` // LineNumber
}

// SnapshotOptions contains configuration for the snapshot process
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
		BatchSize:  100000,
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
	if err := extractLatestRecords(opts.IndexFile, opts.InputFiles, opts.OutputFile, opts.Verbose); err != nil {
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
func processFile(filename string, fn func(string, Record, int64) error) error {
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

// writeIndexEntries writes index entries with compressed JSON.
func writeIndexEntries(indexFile *os.File, entries []IndexEntry) error {
	zw, err := zstd.NewWriter(indexFile)
	if err != nil {
		return fmt.Errorf("error creating zstd writer: %v", err)
	}
	defer zw.Close()
	encoder := json.NewEncoder(zw)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

// readIndexFile reads a compressed, compact JSON index file.
func readIndexFile(indexFilePath string, verbose bool) (map[string]IndexEntry, error) {
	indexFile, err := os.Open(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening index file: %v", err)
	}
	defer indexFile.Close()
	zr, err := zstd.NewReader(indexFile)
	if err != nil {
		return nil, fmt.Errorf("error creating zstd reader: %v", err)
	}
	defer zr.Close()
	var (
		decoder     = json.NewDecoder(zr)
		latestMap   = make(map[string]IndexEntry)
		recordsRead = 0
	)
	for {
		var entry IndexEntry
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error decoding index entry: %v", err)
		}
		recordsRead++
		if verbose && recordsRead%1_000_000 == 0 {
			log.Printf("read %d index entries so far", recordsRead)
		}
		// Keep only the latest version of each DOI
		existing, ok := latestMap[entry.DOI]
		if !ok || entry.Timestamp > existing.Timestamp {
			latestMap[entry.DOI] = entry
		}
	}

	if verbose {
		log.Printf("index consolidation complete. Found %d unique DOIs.", len(latestMap))
	}
	return latestMap, nil
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

// extractLatestRecords reads the index file and extracts the latest version of each record
func extractLatestRecords(indexFilePath string, inputFiles []string, outputFilePath string, verbose bool) error {
	if verbose {
		log.Println("extracting latest records...")
	}
	latestMap, err := readIndexFile(indexFilePath, verbose)
	if err != nil {
		return fmt.Errorf("failed to read index: %v", err)
	}
	outWriter, err := createOutputWriter(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outWriter.Close()
	bufWriter := bufio.NewWriter(outWriter)
	defer bufWriter.Flush()
	fileEntries := make(map[string][]IndexEntry)
	for _, entry := range latestMap {
		fileEntries[entry.Filename] = append(fileEntries[entry.Filename], entry)
	}
	var (
		extractedCount = 0
		numFiles       = len(fileEntries)
		processedFiles = 0
	)
	for filename, entries := range fileEntries {
		processedFiles++
		if verbose {
			log.Printf("[%d/%d] extracting from file: %s", processedFiles, numFiles, filename)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].LineNumber < entries[j].LineNumber
		})
		lineMap := make(map[int64]string)
		for _, entry := range entries {
			lineMap[entry.LineNumber] = entry.DOI
		}
		fileExtracted := 0
		err := processFile(filename, func(line string, record Record, lineNum int64) error {
			doi, wanted := lineMap[lineNum]
			if wanted && doi == record.DOI {
				// Write the raw line directly to preserve original JSON format without re-encoding
				if _, err := bufWriter.WriteString(line + "\n"); err != nil {
					return fmt.Errorf("error writing to output file: %v", err)
				}
				extractedCount++
				fileExtracted++
				if verbose && fileExtracted > 0 && fileExtracted%100000 == 0 {
					log.Printf("extracted %d records from current file so far", fileExtracted)
					bufWriter.Flush() // Flush periodically to free buffer memory
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error extracting from file %s: %v", filename, err)
		}
		if verbose {
			log.Printf("extracted %d records from this file", fileExtracted)
		}
	}
	if err := bufWriter.Flush(); err != nil {
		return fmt.Errorf("error flushing output buffer: %v", err)
	}
	if verbose {
		log.Printf("extraction complete. Wrote %d records to %s", extractedCount, outputFilePath)
	}
	return nil
}
