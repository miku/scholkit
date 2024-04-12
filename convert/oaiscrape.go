package convert

import (
	"fmt"
	"strings"

	"github.com/miku/scholkit/schema/fatcat"
	"github.com/miku/scholkit/schema/oaiscrape"
)

func OaiScrapeToFatcatRelease(doc *oaiscrape.Document) (*fatcat.Release, error) {
	var release fatcat.Release
	release.ID = fmt.Sprintf("oaiscrape-%s", hashString(doc.OAI))
	release.Source = "oaiscrape"
	// Set title
	if len(doc.Titles) > 0 {
		release.Title = doc.Titles[0]
	}
	// Set contributors
	for _, creator := range doc.Creators {
		names := strings.Split(creator, " ")
		var givenName, surname string
		if len(names) > 0 {
			surname = names[len(names)-1]
		}
		if len(names) > 1 {
			givenName = strings.Join(names[:len(names)-1], " ")
		}
		release.Contribs = append(release.Contribs, fatcat.Contrib{
			GivenName: givenName,
			Surname:   surname,
		})
	}
	// Set DOI
	release.ExtIDs.DOI = doc.DOI()
	release.ExtIDs.OAI = doc.OAI
	// Set URL
	release.Extra.Crossref.AlternativeId = append(release.Extra.Crossref.AlternativeId, doc.URL())
	// Set release date
	release.ReleaseDate = doc.Datestamp
	release.Extra.OAI.SetSpec = doc.Sets
	return &release, nil
}
