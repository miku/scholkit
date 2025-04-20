// sk-crossref-snapshot creates a snapshot from a set of crossref records, as harvested.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/segmentio/encoding/json"
	log "github.com/sirupsen/logrus"
)

var (
	outputFile = flag.String("o", "latest_records.json", "output file path, use .gz or .zst to enable compression")
	indexFile  = flag.String("i", "temp_index.dat", "temporary index file path")
	batchSize  = flag.Int("n", 100000, "number of records to process in memory before writing to index")
	workers    = flag.Int("w", runtime.NumCPU(), "number of worker goroutines for parallel processing")
	keepIndex  = flag.Bool("k", false, "keep the index file after processing")
	verbose    = flag.Bool("v", false, "verbose output")
)

// Record represents the JSON structure we're interested in
type Record struct {
	DOI     string `json:"DOI"`
	Indexed struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"indexed"`
}

// IndexEntry stores information about where to find a record
type IndexEntry struct {
	DOI        string
	Timestamp  int64
	Filename   string
	LineNumber int64
}

func main() {
	flag.Parse()
	inputFiles := flag.Args()
	if len(inputFiles) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No input files provided\n")
		fmt.Fprintf(os.Stderr, "Usage: sk-crossref-snapshot [options] file1.zst file2.zst ...\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	finalOutputFile := *outputFile
	buildIndex(inputFiles, *indexFile, *batchSize)
	extractLatestRecords(*indexFile, inputFiles, finalOutputFile)
	if !*keepIndex {
		os.Remove(*indexFile)
	} else {
		fmt.Printf("index file kept at: %s\n", *indexFile)
	}
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
func buildIndex(inputFiles []string, indexFilePath string, batchSize int) {
	indexFile, err := os.Create(indexFilePath)
	if err != nil {
		log.Fatal("error creating index file: %v", err)
	}
	defer indexFile.Close()
	var indexMutex sync.Mutex
	numWorkers := runtime.NumCPU()
	processFilesParallel(inputFiles, numWorkers, func(inputPath string) error {
		doiMap := make(map[string]IndexEntry, batchSize)
		entriesProcessed := 0
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
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("error processing file %s: %v", inputPath, err)
		}
		// Write any remaining entries
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
		}
		return nil
	})
}

// processFilesParallel processes files in parallel using worker goroutines
func processFilesParallel(inputFiles []string, numWorkers int, processor func(string) error) {
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
			log.Fatal("error during processing: %v\n", err)
		}
	}
}

// writeIndexEntries writes a batch of index entries to the index file
func writeIndexEntries(indexFile *os.File, entries []IndexEntry) error {
	encoder := json.NewEncoder(indexFile)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

// readIndexFile reads the index file and returns a map with the latest version of each DOI
func readIndexFile(indexFilePath string) (map[string]IndexEntry, error) {
	indexFile, err := os.Open(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening index file: %v", err)
	}
	defer indexFile.Close()
	var (
		decoder     = json.NewDecoder(indexFile)
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
		if *verbose && recordsRead%1000000 == 0 {
			log.Printf("read %d index entries so far\n", recordsRead)
		}
		existing, ok := latestMap[entry.DOI]
		if !ok || entry.Timestamp > existing.Timestamp {
			latestMap[entry.DOI] = entry
		}
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
func extractLatestRecords(indexFilePath string, inputFiles []string, outputFilePath string) {
	latestMap, err := readIndexFile(indexFilePath)
	if err != nil {
		log.Fatalf("failed to read index: %v", err)
	}
	outWriter, err := createOutputWriter(outputFilePath)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
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
		if *verbose {
			log.Printf("[%d/%d] extracting from file: %s\n", processedFiles, numFiles, filename)
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
				if *verbose && fileExtracted > 0 && fileExtracted%100000 == 0 {
					log.Printf("extracted %d records from current file so far\n", fileExtracted)
					bufWriter.Flush() // Flush periodically to free buffer memory
				}
			}
			return nil
		})
		if err != nil {
			log.Fatalf("error extracting from file %s: %v", filename, err)
		}
		if *verbose {
			log.Printf("extracted %d records from this file\n", fileExtracted)
		}
	}
	if err := bufWriter.Flush(); err != nil {
		log.Fatalf("Error flushing output buffer: %v", err)
	}
	log.Printf("extraction complete. Wrote %d records to %s\n", extractedCount, outputFilePath)
}
