// clowder is a release entity clusterer. It takes a big bunch of release
// entities and will group similar items together.
//
// $ cat releases.ndj | clouder
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/miku/scholkit/normal"
	"github.com/miku/scholkit/parallel"
	"github.com/miku/scholkit/schema/fatcat"
)

var makeTable = flag.Bool("T", false, "releases to tabular form")

func main() {
	pp := parallel.NewProcessor(os.Stdin, os.Stdout, func(p []byte) ([]byte, error) {
		// tabularize release
		var release fatcat.Release
		if err := json.Unmarshal(p, &release); err != nil {
			return nil, err
		}
		C := normal.ReplaceNewlineAndTab
		fields := []string{
			C(release.ID),
			C(release.Source),
			C(release.ExtIDs.Ark),
			C(release.ExtIDs.Arxiv),
			C(release.ExtIDs.Core),
			C(release.ExtIDs.DBLP),
			C(release.ExtIDs.DOI),
			C(release.ExtIDs.FatcatReleaseIdent),
			C(release.ExtIDs.FatcatWorkIdent),
			C(release.ExtIDs.HDL),
			C(release.ExtIDs.ISBN13),
			C(release.ExtIDs.JStor),
			C(release.ExtIDs.MAG),
			C(release.ExtIDs.MID),
			C(release.ExtIDs.OAI),
			C(release.ExtIDs.OpenAlex),
			C(release.ExtIDs.PII),
			C(release.ExtIDs.PMCID),
			C(release.ExtIDs.PMID),
			C(release.ExtIDs.WikidataQID),
		}
		b := []byte(strings.Join(fields, "\t"))
		b = append(b, '\n')
		return b, nil
	})
	if err := pp.Run(); err != nil {
		log.Fatal(err)
	}
}
