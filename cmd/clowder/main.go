// clowder is a release entity clusterer. It takes release
// entities and will group similar items together. WIP.
//
// $ cat releases.ndj | clowder
package main

import (
	"bufio"
	"encoding/base32"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/miku/scholkit/normal"
	"github.com/miku/scholkit/parallel"
	"github.com/miku/scholkit/schema/fatcat"
)

var (
	batchSize       = flag.Int("b", 20000, "batch size")
	makeTable       = flag.Bool("T", false, "releases to tabular form")
	includeBlob     = flag.Bool("I", false, "include source document as last column (base32 encoded)")
	runGroupVerify  = flag.Bool("G", false, "group and run verification on a cluster")
	groupFieldIndex = flag.Int("f", 0, "group by column given by index (starting at 1, like cut) ")
)

func main() {
	flag.Parse()
	switch {
	case *makeTable:
		var C = normal.ReplaceNewlineAndTab // cleanup function for ids
		// normalizer pipeline for title
		var normalizer = &normal.Pipeline{
			Normalizer: []normal.Normalizer{
				&normal.SimpleNormalizer{},
				&normal.RemoveWSNormalizer{},
				&normal.LettersOnlyNormalizer{},
			},
		}
		pp := parallel.NewProcessor(os.Stdin, os.Stdout, func(p []byte) ([]byte, error) {
			var r fatcat.Release
			if err := json.Unmarshal(p, &r); err != nil {
				return nil, err
			}
			// tabularize data
			fields := []string{
				C(r.ID),
				C(r.Source),
				C(r.ExtIDs.Ark),
				C(r.ExtIDs.Arxiv),
				C(r.ExtIDs.Core),
				C(r.ExtIDs.DBLP),
				C(r.ExtIDs.DOI),
				C(r.ExtIDs.FatcatReleaseIdent),
				C(r.ExtIDs.FatcatWorkIdent),
				C(r.ExtIDs.HDL),
				C(r.ExtIDs.ISBN13),
				C(r.ExtIDs.JStor),
				C(r.ExtIDs.MAG),
				C(r.ExtIDs.MID),
				C(r.ExtIDs.OAI),
				C(r.ExtIDs.OpenAlex),
				C(r.ExtIDs.PII),
				C(r.ExtIDs.PMCID),
				C(r.ExtIDs.PMID),
				C(r.ExtIDs.WikidataQID),
				C(normalizer.Normalize(r.Title)),
			}
			if *includeBlob {
				// Running this took about 25min, result is 1TB uncompressed; 230GB compressed.
				encoded := base32.StdEncoding.EncodeToString(p)
				fields = append(fields, encoded)
			}
			b := []byte(strings.Join(fields, "\t"))
			b = append(b, '\n')
			return b, nil
		})
		pp.BatchSize = *batchSize
		if err := pp.Run(); err != nil {
			log.Fatal(err)
		}
	case *runGroupVerify && *groupFieldIndex > 0:
		// read line until we find all sharing the given key, then pass off to
		// verification thread
		// TODO: custom Split function, a mix of size (e.g. 16MB) and complete
		// set of keys
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
		}
		if scanner.Err() != nil {
			log.Fatal("scan: %v", scanner.Err())
		}
	default:
		log.Printf("use -T to create a table")
	}
}
