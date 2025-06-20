package convert

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/miku/scholkit/schema/crossref"
	"github.com/miku/scholkit/schema/fatcat"
)

var CrossrefTypeMap = map[string]string{
	"book":                "book",
	"book-chapter":        "chapter",
	"book-part":           "chapter",
	"book-section":        "chapter",
	"component":           "component",
	"dataset":             "dataset",
	"dissertation":        "thesis",
	"edited-book":         "book",
	"journal-article":     "article-journal",
	"monograph":           "book",
	"peer-review":         "peer_review",
	"posted-content":      "post",
	"proceedings-article": "paper-conference",
	"reference-book":      "book",
	"reference-entry":     "entry",
	"report":              "report",
	"standard":            "standard",
}

// CrossrefTypeBlacklist lists entities we are not interested in.
var CrossrefTypeBlacklist = []string{
	"journal",
	"proceedings",
	"standard-series",
	"report-series",
	"book-series",
	"book-set",
	"book-track",
	"proceedings-series",
}

var ContainerTypeMap = map[string]string{
	"article-journal":  "journal",
	"paper-conference": "conference",
	"book":             "book-series",
}

// re = ReleaseEntity(
//     work_id=None,
//     container_id=container_id,
//     title=title,
//     subtitle=subtitle,
//     original_title=original_title,
//     release_type=release_type,
//     release_stage=release_stage,
//     release_date=release_date,
//     release_year=release_year,
//     publisher=publisher,
//     ext_ids=fatcat_openapi_client.ReleaseExtIds(
//         doi=doi,
//         isbn13=isbn13,
//     ),
//     volume=clean_str(obj.get("volume")),
//     issue=clean_str(obj.get("issue")),
//     pages=clean_str(obj.get("page")),
//     language=clean_str(obj.get("language")),
//     license_slug=license_slug,
//     extra=extra or None,
//     abstracts=abstracts or None,
//     contribs=contribs or None,
//     refs=refs or None,
// )

func doContribs(authors []crossref.Author, role string) (contribs []fatcat.Contrib) {
	for _, author := range authors {
		contrib := fatcat.Contrib{
			GivenName: author.Given,
			Surname:   author.Family,
			Role:      role,
		}
		if author.Given != "" && author.Family != "" {
			contrib.RawName = fmt.Sprintf("%s %s", author.Given, author.Family)
		}
		if len(author.Affiliation) > 0 {
			contrib.RawAffiliation = author.Affiliation[0].Name
			for _, more := range author.Affiliation[1:] {
				contrib.Extra.MoreAffiliation = append(contrib.Extra.MoreAffiliation, more.Name)
			}
		}
		contribs = append(contribs, contrib)
	}
	return contribs
}

// CrossrefWorkToFatcatRelease converts a crossref work document to a fatcat release document.
func CrossrefWorkToFatcatRelease(work *crossref.Work) (*fatcat.Release, error) {
	if work == nil {
		return nil, fmt.Errorf("crossref work cannot be nil")
	}

	if len(work.Title) == 0 {
		return nil, ErrSkipNoTitle
	}

	if slices.Contains(CrossrefTypeBlacklist, work.Type) {
		return nil, ErrSkipCrossrefReleaseType
	}

	doi := cleanDOI(work.DOI)
	if len(doi) == 0 {
		return nil, ErrSkipNoDOI
	}

	rel := fatcat.Release{
		ID:     fmt.Sprintf("crossref-%s", hashString(doi)),
		Source: "crossref",
		Title:  cleanTitle(work.Title[0]),
		ExtIDs: fatcat.ExtID{
			DOI: doi,
		},
	}

	// Enhanced title handling
	if len(work.Subtitle) > 0 {
		if subtitle := strings.TrimSpace(string(work.Subtitle)); subtitle != "" {
			rel.Subtitle = subtitle
		}
	}

	if len(work.OriginalTitle) > 0 {
		if origTitle := strings.TrimSpace(string(work.OriginalTitle)); origTitle != "" {
			rel.OriginalTitle = origTitle
		}
	}

	// Enhanced contributor processing
	rel.Contribs = processCrossrefContributors(work)

	// Enhanced date handling - prioritize published date over created date
	var releaseDate string
	var releaseYear int64

	// First try to use published date (this is what we want for the release)
	if len(work.Published.DateParts) > 0 && len(work.Published.DateParts[0]) > 0 {
		year := work.Published.DateParts[0][0]
		releaseYear = year
		dateStr := fmt.Sprintf("%d", year)
		if len(work.Published.DateParts[0]) > 1 {
			dateStr += fmt.Sprintf("-%02d", work.Published.DateParts[0][1])
			if len(work.Published.DateParts[0]) > 2 {
				dateStr += fmt.Sprintf("-%02d", work.Published.DateParts[0][2])
			} else {
				dateStr += "-01" // Default to first day if day not specified
			}
		} else {
			dateStr += "-01-01" // Default to January 1st if month not specified
		}
		releaseDate = parseDate(dateStr)
	} else if work.Created.Timestamp != 0 {
		// Fallback to created date if published date is not available
		t := time.Unix(work.Created.Timestamp, 0)
		releaseDate = t.Format("2006-01-02")
		releaseYear = int64(t.Year())
	} else if len(work.Created.DateParts) > 0 && len(work.Created.DateParts[0]) > 0 {
		// Fallback to created date parts if timestamp is not available
		year := work.Created.DateParts[0][0]
		releaseYear = year
		dateStr := fmt.Sprintf("%d", year)
		if len(work.Created.DateParts[0]) > 1 {
			dateStr += fmt.Sprintf("-%02d", work.Created.DateParts[0][1])
			if len(work.Created.DateParts[0]) > 2 {
				dateStr += fmt.Sprintf("-%02d", work.Created.DateParts[0][2])
			} else {
				dateStr += "-01"
			}
		} else {
			dateStr += "-01-01"
		}
		releaseDate = parseDate(dateStr)
	}

	rel.ReleaseDate = releaseDate
	rel.ReleaseYear = releaseYear

	// Enhanced type mapping
	if releaseType, ok := CrossrefTypeMap[work.Type]; ok {
		rel.ReleaseType = releaseType
	}

	// Enhanced abstract handling
	if len(work.Abstract) > 20 && len(work.Abstract) < 100000 {
		// Clean abstract HTML/XML tags
		abstract := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(work.Abstract, "")
		abstract = strings.TrimSpace(abstract)
		if abstract != "" {
			rel.Abstracts = append(rel.Abstracts, fatcat.Abstract{
				Content:  abstract,
				Mimetype: "text/plain",
				SHA1:     hashString(abstract),
			})
		}
	}

	// Enhanced metadata preservation
	rel.Volume = strings.TrimSpace(work.Volume)
	rel.Issue = strings.TrimSpace(work.Issue)
	rel.Pages = strings.TrimSpace(work.Page)
	rel.Publisher = strings.TrimSpace(work.Publisher)

	// Container title mapping
	rel.Extra.Crossref.ContainterTitle = work.ContainerTitle

	// Enhanced license handling
	if len(work.License) > 0 {
		for _, license := range work.License {
			if license.URL != "" {
				rel.LicenseSlug = inferLicenseSlug(license.URL)
				break
			}
		}
	}

	return &rel, nil
}

func processCrossrefContributors(work *crossref.Work) []fatcat.Contrib {
	var contribs []fatcat.Contrib

	// Process authors
	if len(work.Author) > 0 {
		var authors []crossref.Author
		if err := json.Unmarshal(work.Author, &authors); err == nil {
			for i, author := range authors {
				contrib := fatcat.Contrib{
					GivenName: strings.TrimSpace(author.Given),
					Surname:   strings.TrimSpace(author.Family),
					Role:      "author",
					Index:     int64(i),
				}

				if contrib.GivenName != "" && contrib.Surname != "" {
					contrib.RawName = fmt.Sprintf("%s %s", contrib.GivenName, contrib.Surname)
				} else if contrib.Surname != "" {
					contrib.RawName = contrib.Surname
				}

				if author.ORCID != "" {
					// Store ORCID in a way that works with the existing struct
					// We can add it to the raw name or use sequence field
					cleanOrcid := cleanORCID(author.ORCID)
					contrib.Extra.Seq = cleanOrcid
				}

				// Enhanced affiliation handling
				if len(author.Affiliation) > 0 {
					contrib.RawAffiliation = author.Affiliation[0].Name
					for _, more := range author.Affiliation[1:] {
						if more.Name != "" {
							contrib.Extra.MoreAffiliation = append(contrib.Extra.MoreAffiliation, more.Name)
						}
					}
				}

				contribs = append(contribs, contrib)
			}
		}
	}

	// Process editors
	if len(work.Editor) > 0 {
		var editors []crossref.Author
		if err := json.Unmarshal(work.Editor, &editors); err == nil {
			for i, editor := range editors {
				contrib := fatcat.Contrib{
					GivenName: strings.TrimSpace(editor.Given),
					Surname:   strings.TrimSpace(editor.Family),
					Role:      "editor",
					Index:     int64(len(contribs) + i),
				}

				if contrib.GivenName != "" && contrib.Surname != "" {
					contrib.RawName = fmt.Sprintf("%s %s", contrib.GivenName, contrib.Surname)
				} else if contrib.Surname != "" {
					contrib.RawName = contrib.Surname
				}

				if editor.ORCID != "" {
					// Store ORCID in sequence field for now
					cleanOrcid := cleanORCID(editor.ORCID)
					contrib.Extra.Seq = cleanOrcid
				}

				contribs = append(contribs, contrib)
			}
		}
	}

	return contribs
}
