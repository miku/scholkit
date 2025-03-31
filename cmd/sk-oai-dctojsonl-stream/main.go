// sk-oai-dctojsonl-streaming converts a stream of XML records, where each
// record is separated by a record separator "1E". This version supports
// streaming data from stdin or directly from compressed files.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/segmentio/encoding/json"
)

var (
	input      = flag.String("i", "-", "input file (use - for stdin, supports .gz files)")
	output     = flag.String("o", "-", "output file (use - for stdout, will compress if ends with .gz)")
	bufferSize = flag.Int("b", 8*1024*1024, "buffer size for reading chunks")
	numWorkers = flag.Int("w", runtime.NumCPU(), "number of worker goroutines")
	printStats = flag.Bool("stats", false, "print processing statistics")
)

// Record and other type definitions remain the same as before
type Record struct {
	XMLName  xml.Name `xml:"record"`
	Header   Header   `xml:"header"`
	Metadata struct {
		DC DublinCore `xml:"dc"`
	} `xml:"metadata"`
}

type Header struct {
	Status     string   `xml:"status,attr"`
	Identifier string   `xml:"identifier"`
	Datestamp  string   `xml:"datestamp"`
	SetSpec    []string `xml:"setSpec"`
}

type DublinCore struct {
	Title       []string `xml:"title"`
	Creator     []string `xml:"creator"`
	Subject     []string `xml:"subject"`
	Description []string `xml:"description"`
	Publisher   []string `xml:"publisher"`
	Contributor []string `xml:"contributor"`
	Date        []string `xml:"date"`
	Type        []string `xml:"type"`
	Format      []string `xml:"format"`
	Identifier  []string `xml:"identifier"`
	Source      []string `xml:"source"`
	Language    []string `xml:"language"`
	Relation    []string `xml:"relation"`
	Rights      []string `xml:"rights"`
}

type FlatRecord struct {
	Identifier   string   `json:"identifier"`
	Datestamp    string   `json:"datestamp"`
	SetSpec      []string `json:"set_spec"`
	Title        []string `json:"title"`
	Creator      []string `json:"creator"`
	Subject      []string `json:"subject"`
	Description  []string `json:"description"`
	Publisher    []string `json:"publisher"`
	Contributor  []string `json:"contributor"`
	Date         []string `json:"date"`
	Type         []string `json:"type"`
	Format       []string `json:"format"`
	DCIdentifier []string `json:"dc_identifier"`
	Source       []string `json:"source"`
	Language     []string `json:"language"`
	Relation     []string `json:"relation"`
	Rights       []string `json:"rights"`
}

const (
	recordSep = 0x1E // ASCII record separator
)

// Statistics for reporting
type Stats struct {
	BytesRead      int64
	RecordsRead    int64
	RecordsWritten int64
	mu             sync.Mutex
}

func (s *Stats) IncrementBytesRead(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BytesRead += n
}

func (s *Stats) IncrementRecordsRead() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecordsRead++
}

func (s *Stats) IncrementRecordsWritten() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecordsWritten++
}

func convertRecord(record *Record) *FlatRecord {
	flat := &FlatRecord{
		Identifier:   record.Header.Identifier,
		Datestamp:    record.Header.Datestamp,
		SetSpec:      record.Header.SetSpec,
		Title:        record.Metadata.DC.Title,
		Creator:      record.Metadata.DC.Creator,
		Subject:      record.Metadata.DC.Subject,
		Description:  record.Metadata.DC.Description,
		Publisher:    record.Metadata.DC.Publisher,
		Contributor:  record.Metadata.DC.Contributor,
		Date:         record.Metadata.DC.Date,
		Type:         record.Metadata.DC.Type,
		Format:       record.Metadata.DC.Format,
		DCIdentifier: record.Metadata.DC.Identifier,
		Source:       record.Metadata.DC.Source,
		Language:     record.Metadata.DC.Language,
		Relation:     record.Metadata.DC.Relation,
		Rights:       record.Metadata.DC.Rights,
	}
	return flat
}

// Split the data into XML records, separated by the record separator
func splitIntoRecords(data []byte) [][]byte {
	var records [][]byte
	var start int

	for i, b := range data {
		if b == recordSep {
			// Only add non-empty records
			if i > start {
				records = append(records, data[start:i])
			}
			start = i + 1
		}
	}

	// Add the last record if it's not empty and doesn't end with a separator
	if start < len(data) {
		records = append(records, data[start:])
	}

	return records
}

func processRecords(records [][]byte, stats *Stats) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	for _, record := range records {
		stats.IncrementRecordsRead()

		var r Record
		err := xml.Unmarshal(record, &r)
		if err != nil {
			// Skip malformed records but log the error
			log.Printf("Error unmarshaling record: %v", err)
			continue
		}

		flat := convertRecord(&r)
		err = enc.Encode(flat)
		if err != nil {
			return nil, fmt.Errorf("error encoding record: %w", err)
		}

		stats.IncrementRecordsWritten()
	}

	return buf.Bytes(), nil
}

func worker(jobs <-chan []byte, results chan<- []byte, wg *sync.WaitGroup, stats *Stats) {
	defer wg.Done()

	for data := range jobs {
		records := splitIntoRecords(data)
		result, err := processRecords(records, stats)
		if err != nil {
			log.Printf("Error processing records: %v", err)
			continue
		}
		results <- result
	}
}

func createReader(filename string) (io.ReadCloser, error) {
	if filename == "-" {
		return os.Stdin, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	// If file has .gz extension, use gzip reader
	if strings.HasSuffix(filename, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("error creating gzip reader: %w", err)
		}
		return &readCloserPair{gzReader, file}, nil
	}

	return file, nil
}

func createWriter(filename string) (io.WriteCloser, error) {
	if filename == "-" {
		return os.Stdout, nil
	}

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	// If file has .gz extension, use gzip writer
	if strings.HasSuffix(filename, ".gz") {
		gzWriter := gzip.NewWriter(file)
		return &writeCloserPair{gzWriter, file}, nil
	}

	return file, nil
}

// Helper struct to handle closing both the gzip reader and the underlying file
type readCloserPair struct {
	reader io.ReadCloser
	file   io.Closer
}

func (rc *readCloserPair) Read(p []byte) (n int, err error) {
	return rc.reader.Read(p)
}

func (rc *readCloserPair) Close() error {
	err1 := rc.reader.Close()
	err2 := rc.file.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// Helper struct to handle closing both the gzip writer and the underlying file
type writeCloserPair struct {
	writer io.WriteCloser
	file   io.Closer
}

func (wc *writeCloserPair) Write(p []byte) (n int, err error) {
	return wc.writer.Write(p)
}

func (wc *writeCloserPair) Close() error {
	err1 := wc.writer.Close()
	err2 := wc.file.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// readChunks reads from the reader and ensures chunks end with a record separator
func readChunks(r io.Reader, chunkSize int, jobs chan<- []byte, stats *Stats) error {
	buffer := make([]byte, chunkSize)
	carryover := make([]byte, 0)

	for {
		n, err := r.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading from input: %w", err)
		}

		if n == 0 {
			// End of file - send any remaining carryover
			if len(carryover) > 0 {
				jobs <- carryover
				stats.IncrementBytesRead(int64(len(carryover)))
			}
			break
		}

		stats.IncrementBytesRead(int64(n))

		// Find the last record separator in this chunk
		lastSepIdx := -1
		for i := n - 1; i >= 0; i-- {
			if buffer[i] == recordSep {
				lastSepIdx = i
				break
			}
		}

		if lastSepIdx == -1 {
			// No separator found, carry over the entire chunk
			carryover = append(carryover, buffer[:n]...)
			continue
		}

		// Send the chunk ending with the last separator
		chunk := make([]byte, len(carryover)+lastSepIdx+1)
		copy(chunk, carryover)
		copy(chunk[len(carryover):], buffer[:lastSepIdx+1])
		jobs <- chunk

		// Save the remainder for the next iteration
		carryover = make([]byte, n-(lastSepIdx+1))
		copy(carryover, buffer[lastSepIdx+1:n])

		if err == io.EOF {
			// End of file - send any remaining carryover
			if len(carryover) > 0 {
				jobs <- carryover
			}
			break
		}
	}

	return nil
}

func main() {
	flag.Parse()
	stats := &Stats{}

	// Create reader
	reader, err := createReader(*input)
	if err != nil {
		log.Fatalf("Error creating reader: %v", err)
	}
	defer reader.Close()

	// Create writer
	writer, err := createWriter(*output)
	if err != nil {
		log.Fatalf("Error creating writer: %v", err)
	}
	defer writer.Close()

	// Create channels for the pipeline
	jobs := make(chan []byte, *numWorkers)
	results := make(chan []byte, *numWorkers)
	done := make(chan struct{})

	// Start the workers
	var wg sync.WaitGroup
	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg, stats)
	}

	// Start the writer goroutine
	go func() {
		bufWriter := bufio.NewWriter(writer)
		defer bufWriter.Flush()

		for result := range results {
			_, err := bufWriter.Write(result)
			if err != nil {
				log.Printf("Error writing result: %v", err)
			}
		}
		close(done)
	}()

	// Start reading chunks
	log.Printf("Starting processing with %d workers and %d byte buffer", *numWorkers, *bufferSize)
	err = readChunks(reader, *bufferSize, jobs, stats)
	if err != nil {
		log.Fatalf("Error reading chunks: %v", err)
	}

	// Close the jobs channel to signal workers we're done
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	// Close the results channel to signal the writer goroutine
	close(results)

	// Wait for the writer to finish
	<-done

	if *printStats {
		log.Printf("Processing complete - Read: %d bytes, Records read: %d, Records written: %d",
			stats.BytesRead, stats.RecordsRead, stats.RecordsWritten)
	}
}
