package convert

import (
	"fmt"
	"strings"

	"github.com/miku/scholkit/schema/datacite"
	"github.com/miku/scholkit/schema/fatcat"
)

func DataCiteToFatcatRelease(doc *datacite.Document) (*fatcat.Release, error) {
	if doc == nil {
		return nil, fmt.Errorf("datacite document cannot be nil")
	}

	doi := cleanDOI(doc.Attributes.DOI)
	if doi == "" {
		return nil, ErrSkipNoDOI
	}

	release := fatcat.Release{
		ID:     fmt.Sprintf("datacite-%s", hashString(doi)),
		Source: "datacite",
		ExtIDs: fatcat.ExtID{
			DOI: doi,
		},
	}

	// Enhanced title handling
	if len(doc.Attributes.Titles) > 0 {
		release.Title = cleanTitle(doc.Attributes.Titles[0].Title)

		// Look for subtitle in additional titles
		for _, title := range doc.Attributes.Titles[1:] {
			if title.TitleType == "Subtitle" {
				release.Subtitle = strings.TrimSpace(title.Title)
				break
			}
		}
	}

	// Enhanced contributor processing
	for i, creator := range doc.Attributes.Creators {
		contrib := fatcat.Contrib{
			Index: int64(i),
			Role:  "author",
		}

		if creator.GivenName != "" && creator.FamilyName != "" {
			contrib.GivenName = strings.TrimSpace(creator.GivenName)
			contrib.Surname = strings.TrimSpace(creator.FamilyName)
			contrib.RawName = fmt.Sprintf("%s %s", contrib.GivenName, contrib.Surname)
		} else if creator.Name != "" {
			contrib.RawName = strings.TrimSpace(creator.Name)
			given, family := parseContributorName(creator.Name)
			contrib.GivenName = given
			contrib.Surname = family
		}

		release.Contribs = append(release.Contribs, contrib)
	}

	// Enhanced contributor types from contributors field
	for i, contributor := range doc.Attributes.Contributors {
		contrib := fatcat.Contrib{
			Index: int64(len(release.Contribs) + i),
			Role:  strings.ToLower(contributor.ContributorType),
		}

		if contributor.GivenName != "" && contributor.FamilyName != "" {
			contrib.GivenName = strings.TrimSpace(contributor.GivenName)
			contrib.Surname = strings.TrimSpace(contributor.FamilyName)
			contrib.RawName = fmt.Sprintf("%s %s", contrib.GivenName, contrib.Surname)
		} else if contributor.Name != "" {
			contrib.RawName = strings.TrimSpace(contributor.Name)
			given, family := parseContributorName(contributor.Name)
			contrib.GivenName = given
			contrib.Surname = family
		}

		release.Contribs = append(release.Contribs, contrib)
	}

	// Enhanced date handling
	if doc.Attributes.Published != "" {
		release.ReleaseDate = parseDate(doc.Attributes.Published)
		if year, err := extractYear(doc.Attributes.Published); err == nil {
			release.ReleaseYear = year
		}
	} else if doc.Attributes.PublicationYear != 0 {
		release.ReleaseYear = doc.Attributes.PublicationYear
		release.ReleaseDate = fmt.Sprintf("%d-01-01", doc.Attributes.PublicationYear)
	}

	// Enhanced description/abstract handling
	for _, desc := range doc.Attributes.Descriptions {
		if desc.DescriptionType == "Abstract" && len(desc.Description) > 20 {
			release.Abstracts = append(release.Abstracts, fatcat.Abstract{
				Content:  strings.TrimSpace(desc.Description),
				Mimetype: "text/plain",
				SHA1:     hashString(desc.Description),
			})
			break // Only take the first abstract
		}
	}

	// Enhanced type mapping
	if doc.Attributes.Types.ResourceType != "" {
		release.ReleaseType = mapDataCiteType(doc.Attributes.Types.ResourceType)
	}

	// Publisher information
	if doc.Attributes.Publisher != "" {
		release.Publisher = strings.TrimSpace(doc.Attributes.Publisher)
	}

	// Language
	if doc.Attributes.Language != "" {
		release.Language = strings.TrimSpace(doc.Attributes.Language)
	}

	return &release, nil
}
