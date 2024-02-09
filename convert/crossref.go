package convert

import (
	"encoding/json"
	"fmt"
	"slices"
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
	if len(work.Title) == 0 {
		return nil, ErrSkipNoTitle
	}
	if slices.Contains(CrossrefTypeBlacklist, work.Type) {
		return nil, ErrSkipCrossrefReleaseType
	}
	// note: partially generated by chatgpt
	var rel fatcat.Release
	var doi = cleanDOI(work.DOI)
	if len(doi) == 0 {
		return nil, ErrSkipNoDOI
	}
	rel.ID = fmt.Sprintf("crossref-%s", hashString(doi)) // crossref-sha1(DOI)
	// Set title.
	rel.Title = work.Title[0]
	if len(work.Subtitle) > 0 {
		// TODO: maybe get rid of brackets
		rel.Subtitle = string(work.Subtitle)
	}
	if len(work.OriginalTitle) > 0 {
		rel.OriginalTitle = string(work.OriginalTitle)
	}
	// Convert Authors, Editors, Translators
	var vs []crossref.Author
	if len(work.Author) > 0 {
		if err := json.Unmarshal(work.Author, &vs); err != nil {
			return &rel, fmt.Errorf("author: %w", err)
		}
		rel.Contribs = doContribs(vs, "author")
	}
	if len(work.Editor) > 0 {
		if err := json.Unmarshal(work.Editor, &vs); err != nil {
			return &rel, fmt.Errorf("editor: %w", err)
		}
		rel.Contribs = append(rel.Contribs, doContribs(vs, "editor")...)
	}
	if len(work.Translator) > 0 {
		if err := json.Unmarshal(work.Translator, &vs); err != nil {
			return &rel, fmt.Errorf("translator: %w", err)
		}
		rel.Contribs = append(rel.Contribs, doContribs(vs, "translator")...)
	}
	// Set DOI
	rel.ExtIDs.DOI = doi
	// Set Release Date
	if len(work.Created.DateParts) > 0 {
		timestamp := work.Created.Timestamp
		if timestamp != 0 {
			t := time.Unix(timestamp, 0)
			rel.ReleaseDate = t.Format("2006-01-02")
		}
	}
	if t, ok := CrossrefTypeMap[work.Type]; ok {
		rel.ReleaseType = t
	}
	if len(work.Abstract) > 20 && len(work.Abstract) < 1000 {
		rel.Abstracts = append(rel.Abstracts, fatcat.Abstract{
			Content: work.Abstract,
		})
	}
	return &rel, nil
}
