// sk-crossref-snapshot creates a snapshot from a set of crossref records, as harvested.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/miku/scholkit/crossref"
)

var (
	outputFile     = flag.String("o", crossref.DefaultOutputFile, "output file path, use .gz or .zst to enable compression")
	batchSize      = flag.Int("n", 100000, "number of records to process in memory before writing to index")
	workers        = flag.Int("w", runtime.NumCPU(), "number of worker goroutines for parallel processing")
	keepTempFiles  = flag.Bool("k", false, "keep temporary files (for debugging)")
	verbose        = flag.Bool("v", false, "verbose output")
	sortBufferSize = flag.String("S", "25%", "sort buffer size")
)

func main() {
	flag.Parse()
	inputFiles := flag.Args()
	if len(inputFiles) == 0 {
		fmt.Fprintf(os.Stderr, "error: no input files provided\n")
		fmt.Fprintf(os.Stderr, "usage: sk-crossref-snapshot [options] file1.zst file2.zst ...\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	opts := crossref.SnapshotOptions{
		InputFiles:     inputFiles,
		OutputFile:     *outputFile,
		BatchSize:      *batchSize,
		NumWorkers:     *workers,
		Verbose:        *verbose,
		KeepTempFiles:  *keepTempFiles,
		SortBufferSize: *sortBufferSize,
	}
	if err := crossref.CreateSnapshot(opts); err != nil {
		log.Fatalf("error creating snapshot: %v", err)
	}
}
