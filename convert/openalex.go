package convert

import (
	"errors"
	"fmt"
	"strings"

	"github.com/miku/scholkit/schema/fatcat"
	"github.com/miku/scholkit/schema/openalex"
)

var (
	ErrMissingOpenAlexIdentifier = errors.New("missing openalex identifier")
	ErrEmptyDoc                  = errors.New("empty doc")
)

func OpenAlexWorkToFatcatRelease(work *openalex.Work, release *fatcat.Release) error {
	if work == nil || release == nil {
		return ErrEmptyDoc
	}

	if work.IDs.Openalex == "" {
		return ErrMissingOpenAlexIdentifier
	}

	release.ID = fmt.Sprintf("openalex-%s", hashString(work.IDs.Openalex))
	release.Source = "openalex"
	release.Title = cleanTitle(work.Title)

	// Enhanced contributor processing with position information
	for _, authorship := range work.Authorships {
		contrib := fatcat.Contrib{
			Role: authorship.AuthorPosition,
		}

		if authorship.RawAuthorName != "" {
			contrib.RawName = strings.TrimSpace(authorship.RawAuthorName)
			given, family := parseContributorName(authorship.RawAuthorName)
			contrib.GivenName = given
			contrib.Surname = family
		}

		// Enhanced affiliation handling
		if authorship.RawAffiliationString != "" {
			contrib.RawAffiliation = strings.TrimSpace(authorship.RawAffiliationString)
		}

		if len(authorship.RawAffiliationStrings) > 1 {
			for _, affil := range authorship.RawAffiliationStrings[1:] {
				if affil != nil {
					if affiliStr, ok := affil.(string); ok && affiliStr != "" {
						contrib.Extra.MoreAffiliation = append(contrib.Extra.MoreAffiliation, affiliStr)
					}
				}
			}
		}

		release.Contribs = append(release.Contribs, contrib)
	}

	// Enhanced identifier handling
	release.ExtIDs.DOI = cleanDOI(work.DOI)
	release.ExtIDs.OpenAlex = strings.Replace(work.IDs.Openalex, "https://openalex.org/", "", 1)

	if work.IDs.PMID != "" {
		release.ExtIDs.PMID = strings.Replace(work.IDs.PMID, "https://pubmed.ncbi.nlm.nih.gov/", "", 1)
	}
	if work.IDs.PMCID != "" {
		release.ExtIDs.PMCID = strings.Replace(work.IDs.PMCID, "https://www.ncbi.nlm.nih.gov/pmc/articles/", "", 1)
	}
	if work.IDs.Arxiv != "" {
		release.ExtIDs.Arxiv = work.IDs.Arxiv
	}
	if work.IDs.Mag != 0 {
		release.ExtIDs.MAG = fmt.Sprintf("%d", work.IDs.Mag)
	}

	// Enhanced bibliographic data
	release.Volume = strings.TrimSpace(work.Biblio.Volume)
	release.Issue = strings.TrimSpace(work.Biblio.Issue)
	release.Pages = work.Pages()

	// Enhanced date handling
	if work.PublicationDate != "" {
		release.ReleaseDate = parseDate(work.PublicationDate)
	}
	if work.PublicationYear != 0 {
		release.ReleaseYear = work.PublicationYear
	}

	// Language and publisher
	if work.Language != "" {
		release.Language = strings.TrimSpace(work.Language)
	}
	if work.PrimaryLocation.Source.Publisher != "" {
		release.Publisher = strings.TrimSpace(work.PrimaryLocation.Source.Publisher)
	}

	// Enhanced open access information
	release.Extra.OpenAlex.OpenAccess.AnyRepositoryHasFulltext = work.OpenAccess.AnyRepositoryHasFulltext
	release.Extra.OpenAlex.OpenAccess.IsOa = work.OpenAccess.IsOa
	release.Extra.OpenAlex.OpenAccess.OaStatus = work.OpenAccess.OaStatus
	release.Extra.OpenAlex.OpenAccess.OaUrl = work.OpenAccess.OaUrl
	release.Extra.OpenAlex.PdfUrls = work.PdfUrls()

	return nil
}
