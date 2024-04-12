package convert

import (
	"fmt"
	"strings"
	"time"

	"github.com/miku/scholkit/schema/arxiv"
	"github.com/miku/scholkit/schema/fatcat"
)

func ArxivRecordToFatcatRelease(record *arxiv.Record) (*fatcat.Release, error) {
	var rel fatcat.Release
	rel.ID = fmt.Sprintf("arxiv-%s", hashString(record.ID())) // arxiv-sha1(id)
	// Setting contributors
	var contribs []fatcat.Contrib
	for _, creator := range record.Metadata.Dc.Creator {
		contribs = append(contribs, fatcat.Contrib{
			RawName: creator,
			Role:    "author",
		})
	}
	rel.Contribs = contribs
	// Setting DOI
	rel.ExtIDs.DOI = record.DOI()
	rel.ExtIDs.OAI = record.Header.Identifier
	rel.ExtIDs.Arxiv = record.ID()
	rel.Source = "arxiv"
	// Setting title
	rel.Title = record.Metadata.Dc.Title
	// Setting release date
	dateStamp := record.Header.Datestamp
	if dateStamp != "" {
		t, err := time.Parse("2006-01-02", dateStamp)
		if err == nil {
			rel.ReleaseDate = t.Format("2006-01-02")
		}
	}
	rel.Language = record.Metadata.Dc.Language
	for _, desc := range record.Metadata.Dc.Description {
		desc = strings.TrimSpace(desc)
		abstract := fatcat.Abstract{
			Content:  desc,
			Mimetype: "text/plain",
			SHA1:     hashString(desc),
		}
		rel.Abstracts = append(rel.Abstracts, abstract)
	}
	var subjects []string
	for _, s := range record.Metadata.Dc.Subject {
		subjects = append(subjects, s)
	}
	rel.Extra.Arxiv.Subjects = subjects
	rel.Extra.OAI.SetSpec = record.Header.SetSpec
	return &rel, nil
}
