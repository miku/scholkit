// Package snapshot contains types that can create snapshots from incrementally
// harvested upstream metadata sources.
package snapshot

import (
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Config is a snapshot configuration. Takes a list of readers to read data
// from and a target file to write the snapshot to.
type Config struct {
	Sources       []io.Reader
	Dst           string
	BatchSize     int
	TempDir       string
	MaxErrorCount int64
}

type Snapshotter interface {
	Snapshot(c Config) error
}

// FieldFunc is a function that extracts an identifier and date from a document
type FieldFunc func([]byte) (id string, date time.Time, err error)

// WriteFunc is a function that writes a document to a writer
type WriteFunc func(w io.Writer, doc []byte) error

// SkipFunc determines whether a document should be skipped
type SkipFunc func(id string) bool

// GenericSnapshotter implements the Snapshotter interface with customizable extraction logic
type GenericSnapshotter struct {
	ExtractFieldFunc FieldFunc
	WriteFunc        WriteFunc
	SkipFunc         SkipFunc
}

// extractMetadata performs the first stage of the snapshot process: Read all
// documents and extract relevant metadata (ID and date, plus location of the
// data in the readers) to a temporary file.
func (s *GenericSnapshotter) extractMetadata(c Config) (string, error) {

	f, err := os.CreateTemp(c.TempDir, "snapshot-metadata-")
	if err != nil {
		return "", err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	defer bw.Flush()

	var wg sync.WaitGroup
	var errCount int64

	for i, source := range c.Sources {
		wg.Add(1)
		go func(idx int, src io.Reader) {
			defer wg.Done()

			scanner := bufio.NewScanner(src)
			lineNumber := 0

			for scanner.Scan() {
				lineNumber++
				docBytes := scanner.Bytes()

				id, date, err := s.ExtractMetadata(docBytes)
				if err != nil {
					log.WithFields(log.Fields{
						"source": idx,
						"line":   lineNumber,
						"error":  err,
					}).Warn("failed to extract document metadata")

					errCount++
					if errCount > c.MaxErrorCount {
						log.Error("maximum error count exceeded")
						return
					}
					continue
				}
				if s.ShouldSkip != nil && s.ShouldSkip(id) {
					continue
				}
				metadata := strings.Join([]string{
					string(rune(idx)),
					string(rune(lineNumber)),
					date.Format(time.RFC3339),
					id,
				}, "\t") + "\n"

				if _, err := bw.WriteString(metadata); err != nil {
					log.WithError(err).Error("failed to write metadata")
					return
				}
			}
			if err := scanner.Err(); err != nil {
				log.WithError(err).Error("scanner error")
			}
		}(i, source)
	}
	wg.Wait()
	if errCount > c.MaxErrorCount {
		return "", err
	}
	return f.Name(), nil
}
