// schol-agg-links takes one or more links to gzip compressed files and will stream
// them into a single zstd compressed file.
//
// $ cat urls.txt
// https://archive.org/download/openalex_snapshot_2023-07-11/data/works/updated_date=2023-05-09/part_000.gz
// https://archive.org/download/openalex_snapshot_2023-07-11/data/works/updated_date=2023-05-01/part_000.gz
// https://archive.org/download/openalex_snapshot_2023-07-11/data/works/updated_date=2023-04-16/part_000.gz
//
// $ cat urls.txt | schol-agg-links | zstd -c > data.zst
//
// Turn pubmed baseline into a single zst file (concatenated, most likely invalid XML):
//
//	$ curl -sL "https://ftp.ncbi.nlm.nih.gov/pubmed/baseline/" | \
//			pup 'a[href] text{}' | \
//	        grep -o 'pubmed.*[.]xml[.]gz' | \
//	        awk '{print "https://ftp.ncbi.nlm.nih.gov/pubmed/baseline/"$0}' | \
// 			schol-agg-links -v | \
//          zst -c > pubmed.xml.zst
//
// Alternatively:
//
// $ curl -sL https://is.gd/NuDPca | combray -v -F pubmedtojson | zstd -c > pubmed.ndjson.zst

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var decompMap = map[string]string{
	"7z":  "7z e -so",
	"bz2": "pbzip2 -cd",
	"gz":  "pigz -cd",
	"lz4": "lz4 -cd",
	"rar": "unrar px",
	"zst": "zstd -cd -T0",
}

var (
	timeout      = flag.Duration("t", 600*time.Second, "timeout for a single command")
	retry        = flag.Int("r", 5, "curl: --retry")
	retryMaxTime = flag.Int("m", 300, "curl: --retry-max-time")
	filterProg   = flag.String("F", "", "run each file through this filter command")
	verbose      = flag.Bool("v", false, "debug output")
	forceDecomp  = flag.String("d", "", "force a decompression tool: 7z, bz2, gz, rar, zst")

	externalDependencies = []string{"curl", "bash"}

	help = `The combray utility reads, transforms and streams content from
multiple URLs. The content is decompressed (gz, 7z, bz2), optionally filtered
and then streamed to stdout.

Many datasets are scattered across many files, e.g. wikipedia, pubmed, openalex
and others.

For example, pubmed is spread over 1165 gzip-compressed XML files. We can
collect the list and feed it to combray and convert it to nice JSON on the fly
with this one-liner:

    $ curl -sL is.gd/NuDPca | combray -F pubmedtojson
`
)

func main() {
	for _, p := range externalDependencies {
		if _, err := exec.LookPath(p); err != nil {
			log.Fatalf("%s required", p)
		}
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, help)
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	br := bufio.NewReader(os.Stdin)
	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()
	var i int
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		i++
		var decomp string
		switch {
		case *forceDecomp != "":
			var ok bool
			decomp, ok = decompMap[*forceDecomp]
			if !ok {
				log.Fatal("unsupported: %v", *forceDecomp)
			}
		default:
			switch {
			case strings.HasSuffix(line, ".gz"):
				decomp = decompMap["gz"]
			case strings.HasSuffix(line, ".bz2"):
				decomp = decompMap["bz2"]
			case strings.HasSuffix(line, ".7z"):
				decomp = decompMap["7z"]
			case strings.HasSuffix(line, ".zst"):
				decomp = decompMap["zst"]
			case strings.HasSuffix(line, ".rar"):
				decomp = decompMap["rar"]
			default:
				log.Fatal("don't know how to decompress: %v", line)
			}
		}
		var s string
		if *filterProg == "" {
			s = fmt.Sprintf("(set -euo pipefail; curl --retry %d --retry-max-time %d --fail -sL '%s' | %s)",
				*retry, *retryMaxTime, line, decomp)
		} else {
			s = fmt.Sprintf("(set -euo pipefail; curl --retry %d --retry-max-time %d --fail -sL '%s' | %s | %s)",
				*retry, *retryMaxTime, line, decomp, *filterProg)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "-c", s)
		cmd.Stdout = bw
		if *verbose {
			log.Printf("%06d %s", i, cmd.String())
		}
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}
