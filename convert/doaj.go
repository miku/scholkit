package convert

import (
	"errors"
	"fmt"
	"strings"

	"github.com/miku/scholkit/schema/doaj"
	"github.com/miku/scholkit/schema/fatcat"
)

var ErrMissingDOAJIdentifier = errors.New("missing DOAJ ID")

func DOAJRecordToFatcatRelease(record *doaj.Record) (*fatcat.Release, error) {
	var release fatcat.Release
	// Set title
	if record.ID() == "" {
		return nil, ErrMissingDOAJIdentifier
	}
	release.ID = fmt.Sprintf("doaj-%s", record.ID())
	release.Title = record.Metadata.Dc.Title
	// Set contributors
	for _, creator := range record.Metadata.Dc.Creator {
		names := strings.Split(creator.Text, " ")
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
	release.ExtIDs.OAI = record.Header.Identifier
	release.ExtIDs.DOI = record.DOI()
	release.ExtIDs.DOAJ = record.ID()
	// Set URL
	release.Extra.Crossref.AlternativeId = append(release.Extra.Crossref.AlternativeId, record.URL())
	// Set release date
	release.ReleaseDate = record.Metadata.Dc.Date
	// Set ident
	release.Ident = record.ID()
	return &release, nil
}
