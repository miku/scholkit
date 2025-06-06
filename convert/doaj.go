package convert

import (
	"errors"
	"fmt"
	"strings"

	"github.com/miku/scholkit/schema/doaj"
	"github.com/miku/scholkit/schema/fatcat"
)

var (
	ErrMissingDOAJIdentifier = errors.New("missing DOAJ ID")
	ErrNilRecord             = errors.New("nil DOAJ record")
)

// NameParts represents the parsed components of a person's name
type NameParts struct {
	GivenName string
	Surname   string
}

// parseFullName splits a full name into given name and surname
// It handles edge cases and provides more reliable name parsing
func parseFullName(fullName string) NameParts {
	names := strings.Fields(fullName)
	if len(names) == 0 {
		return NameParts{}
	}

	if len(names) == 1 {
		return NameParts{Surname: names[0]}
	}

	return NameParts{
		GivenName: strings.Join(names[:len(names)-1], " "),
		Surname:   names[len(names)-1],
	}
}

// DOAJRecordToFatcatRelease converts a DOAJ record to a Fatcat release
// It includes validation and more robust error handling
func DOAJRecordToFatcatRelease(record *doaj.Record) (*fatcat.Release, error) {
	if record == nil {
		return nil, ErrNilRecord
	}

	id := record.ID()
	if id == "" {
		return nil, ErrMissingDOAJIdentifier
	}

	release := &fatcat.Release{
		ID:     fmt.Sprintf("doaj-%s", hashString(id)),
		Source: "doaj",
		Title:  cleanTitle(record.Metadata.Dc.Title),
		Ident:  id,
		ExtIDs: fatcat.ExtID{
			OAI:  record.Header.Identifier,
			DOI:  record.DOI(),
			DOAJ: id,
		},
	}

	// Enhanced date handling
	if record.Metadata.Dc.Date != "" {
		release.ReleaseDate = parseDate(record.Metadata.Dc.Date)
		if year, err := extractYear(record.Metadata.Dc.Date); err == nil {
			release.ReleaseYear = year
		}
	}

	// Enhanced contributor processing
	for i, creator := range record.Metadata.Dc.Creator {
		if creator.Text == "" {
			continue
		}

		contrib := fatcat.Contrib{
			Index:   int64(i),
			RawName: strings.TrimSpace(creator.Text),
			Role:    "author",
		}

		given, family := parseContributorName(creator.Text)
		contrib.GivenName = given
		contrib.Surname = family

		release.Contribs = append(release.Contribs, contrib)
	}

	// Enhanced subject/keyword handling
	for _, subject := range record.Metadata.Dc.Subject {
		if subject.Text != "" {
			// Store subjects using existing OAI extra fields
			release.Extra.OAI.SetSpec = append(release.Extra.OAI.SetSpec, "subject:"+strings.TrimSpace(subject.Text))
		}
	}

	// Publisher
	if record.Metadata.Dc.Publisher != "" {
		release.Publisher = strings.TrimSpace(record.Metadata.Dc.Publisher)
	}

	// Language
	if len(record.Metadata.Dc.Language) > 0 && record.Metadata.Dc.Language[0] != "" {
		release.Language = strings.TrimSpace(record.Metadata.Dc.Language[0])
	}

	// Enhanced description/abstract handling
	if record.Metadata.Dc.Description != "" && len(record.Metadata.Dc.Description) > 20 {
		release.Abstracts = append(release.Abstracts, fatcat.Abstract{
			Content:  strings.TrimSpace(record.Metadata.Dc.Description),
			Mimetype: "text/plain",
			SHA1:     hashString(record.Metadata.Dc.Description),
		})
	}

	// Set spec information
	release.Extra.OAI.SetSpec = record.Header.SetSpec

	// Alternative URL
	if url := record.URL(); url != "" {
		release.Extra.OAI.URL = append(release.Extra.OAI.URL, url)
	}

	return release, nil
}
