package convert

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/miku/scholkit/schema/fatcat"
	"github.com/miku/scholkit/schema/oaiscrape"
)

var (
	ErrOaiDeleted           = errors.New("oai deleted record")
	ErrOaiMissingTitle      = errors.New("oai missing title")
	ErrOaiMissingIdentifier = errors.New("oai missing identifier")
)

func OaiRecordToFatcatRelease(record *oaiscrape.Record) (*fatcat.Release, error) {
	if record.Header.Status == "deleted" {
		return nil, ErrOaiDeleted
	}
	var release fatcat.Release
	release.ID = fmt.Sprintf("oaiscrape-%s", hashString(record.Header.Identifier))
	release.Source = "oaiscrape"
	var dc = record.Metadata.Dc
	// Set title
	if dc.Title == "" {
		return nil, ErrOaiMissingTitle
	}
	release.Title = strings.TrimSpace(dc.Title)
	// Set contributor
	for _, creator := range dc.Creator {
		release.Contribs = append(release.Contribs, fatcat.Contrib{
			RawName: strings.TrimSpace(creator),
		})
	}
	// Set DOI
	release.ExtIDs.DOI = record.DOI()
	if u := record.URL(); len(u) != 0 {
		release.Extra.OAI.URL = u
	}
	release.ExtIDs.OAI = record.Header.Identifier
	release.Language = dc.Language
	// Set release date
	if len(dc.Date) > 0 {
		release.ReleaseDate = dc.Date[0]
	}
	release.Extra.OAI.SetSpec = record.Header.SetSpec
	return &release, nil

}

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
	// Set release date
	release.ReleaseDate = doc.Datestamp
	release.Extra.OAI.SetSpec = doc.Sets
	return &release, nil
}

// FlatRecordToRelease converts a MetadataRecord to a fatcat.Release
func FlatRecordToRelease(metadata *oaiscrape.FlatRecord) (*fatcat.Release, error) {
	if metadata.Identifier == "" {
		return nil, ErrOaiMissingIdentifier
	}
	var release fatcat.Release
	release.Source = "oaiscrape"
	release.ID = fmt.Sprintf("oaiscrape-%v", hashString(metadata.Identifier))

	if len(metadata.Title) > 0 {
		release.Title = metadata.Title[0]
		if len(metadata.Title) > 1 {
			release.Subtitle = metadata.Title[1]
		}
	}
	if len(metadata.Publisher) > 0 {
		release.Publisher = metadata.Publisher[0]
	}
	for i, creator := range metadata.Creator {
		names := strings.Split(creator, ", ")
		var contrib fatcat.Contrib
		contrib.Index = int64(i)
		contrib.RawName = creator
		if len(names) > 1 {
			contrib.Surname = names[0]
			contrib.GivenName = names[1]
		} else {
			contrib.RawName = creator
		}
		release.Contribs = append(release.Contribs, contrib)
	}
	if len(metadata.Date) > 0 {
		release.ReleaseDate = metadata.Date[0]
		if year, err := extractYear(metadata.Date[0]); err == nil {
			release.ReleaseYear = year
		}
	}
	if len(metadata.Language) > 0 && metadata.Language[0] != "" {
		release.Language = metadata.Language[0]
	}
	if len(metadata.DCIdentifier) > 0 {
		for _, id := range metadata.DCIdentifier {
			if strings.Contains(id, "doi.org") || strings.HasPrefix(id, "10.") {
				doi := extractDOI(id)
				if doi != "" {
					release.ExtIDs.DOI = doi
				}
			}
			if strings.Contains(id, "ark:") {
				release.ExtIDs.Ark = id
			} else if strings.Contains(id, "hdl.handle.net") {
				release.ExtIDs.HDL = extractHDL(id)
			}
			// XXX: Add more identifier extractors
		}
	}
	if len(metadata.Description) > 0 && metadata.Description[0] != "" {
		abstract := fatcat.Abstract{
			Content:  metadata.Description[0],
			Mimetype: "text/plain",
		}
		release.Abstracts = append(release.Abstracts, abstract)
	}
	if len(metadata.Type) > 0 {
		release.ReleaseType = mapType(metadata.Type[0])
	}
	if len(metadata.SetSpec) > 0 {
		release.Extra.OAI.SetSpec = metadata.SetSpec
	}
	if len(metadata.Rights) > 0 {
		for _, right := range metadata.Rights {
			if strings.Contains(strings.ToLower(right), "openaccess") ||
				strings.Contains(strings.ToLower(right), "open access") {
				release.LicenseSlug = "CC-BY" // This is a simplification
				break
			}
		}
	}
	return &release, nil
}

// extractYear attempts to extract a year from a date string
func extractYear(date string) (int64, error) {
	// Try parsing as full date
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, date); err == nil {
			return int64(t.Year()), nil
		}
	}
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	match := yearPattern.FindString(date)
	if match != "" {
		year, err := strconv.ParseInt(match, 10, 64)
		return year, err
	}
	return 0, fmt.Errorf("could not extract year from date: %s", date)
}

// extractDOI extracts a DOI from a string that might contain one
func extractDOI(id string) string {
	doiPattern := regexp.MustCompile(`10\.\d{4,}[./]\S+`)
	match := doiPattern.FindString(id)
	return match
}

// extractHDL extracts a handle from a handle.net URL
func extractHDL(id string) string {
	if strings.Contains(id, "hdl.handle.net/") {
		parts := strings.Split(id, "hdl.handle.net/")
		if len(parts) > 1 {
			return parts[1]
		}
	}
	return id
}

// mapType maps from metadata type to fatcat release type
func mapType(metadataType string) string {
	metadataType = strings.ToLower(metadataType)
	if strings.Contains(metadataType, "article") {
		return "article-journal"
	} else if strings.Contains(metadataType, "thesis") || strings.Contains(metadataType, "dissertation") {
		return "thesis"
	} else if strings.Contains(metadataType, "conference") {
		return "paper-conference"
	} else if strings.Contains(metadataType, "book") {
		return "book"
	} else if strings.Contains(metadataType, "report") {
		return "report"
	} else if strings.Contains(metadataType, "post") || strings.Contains(metadataType, "blog") {
		return "post"
	}
	return "article" // Default fallback
}
