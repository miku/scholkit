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
	outputFile = flag.String("o", crossref.DefaultOutputFile, "output file path, use .gz or .zst to enable compression")
	indexFile  = flag.String("i", crossref.DefaultIndexFile, "temporary index file path")
	batchSize  = flag.Int("n", 100000, "number of records to process in memory before writing to index")
	workers    = flag.Int("w", runtime.NumCPU(), "number of worker goroutines for parallel processing")
	keepIndex  = flag.Bool("k", false, "keep the index file after processing")
	verbose    = flag.Bool("v", false, "verbose output")
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
		InputFiles: inputFiles,
		OutputFile: *outputFile,
		IndexFile:  *indexFile,
		BatchSize:  *batchSize,
		Workers:    *workers,
		KeepIndex:  *keepIndex,
		Verbose:    *verbose,
	}
	if err := crossref.CreateSnapshot(opts); err != nil {
		log.Fatalf("error creating snapshot: %v", err)
	}
}
