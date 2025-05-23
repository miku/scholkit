// todo: adjust code
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"unicode"
)

type Contrib struct {
	CreatorId string `json:"creator_id,omitempty"`
	Extra     struct {
		Seq             string   `json:"seq,omitempty"`
		MoreAffiliation []string `json:"more_affiliation,omitempty"`
	} `json:"extra,omitempty"`
	GivenName      string `json:"given_name,omitempty"`
	Index          int64  `json:"index,omitempty"`
	RawName        string `json:"raw_name,omitempty"`
	Role           string `json:"role,omitempty"`
	Surname        string `json:"surname,omitempty"`
	RawAffiliation string `json:"raw_affiliation,omitempty"`
}

type Ref struct {
	ContainerName string          `json:"container_name,omitempty"`
	Extra         json.RawMessage `json:"extra,omitempty"`
	Index         int64           `json:"index,omitempty"`
	Key           string          `json:"key,omitempty"`
	Locator       string          `json:"locator,omitempty"`
	Title         string          `json:"title,omitempty"`
	Year          int64           `json:"year,omitempty"`
}

type ExtID struct {
	Ark                string `json:"ark,omitempty"`
	Arxiv              string `json:"arxiv,omitempty"`
	Core               string `json:"core,omitempty"`
	DBLP               string `json:"dblp,omitempty"`
	DOAJ               string `json:"doaj,omitempty"`
	DOI                string `json:"doi,omitempty"`
	FatcatReleaseIdent string `json:"release_ident,omitempty"`
	FatcatWorkIdent    string `json:"work_ident,omitempty"`
	HDL                string `json:"hdl,omitempty"`
	ISBN13             string `json:"isbn13,omitempty"`
	JStor              string `json:"jstor,omitempty"`
	MAG                string `json:"mag,omitempty"`
	MID                string `json:"mid,omitempty"`
	OAI                string `json:"oai,omitempty"`
	OpenAlex           string `json:"openalex,omitempty"`
	PII                string `json:"pii,omitempty"`
	PMCID              string `json:"pmcid,omitempty"`
	PMID               string `json:"pmid,omitempty"`
	WikidataQID        string `json:"wikidata_qid,omitempty"`
}

type Abstract struct {
	Content  string `json:"content,omitempty"`
	Mimetype string `json:"mimetype,omitempty"`
	SHA1     string `json:"sha1,omitempty"`
}

type Release struct {
	Source      string     `json:"source"`
	Abstracts   []Abstract `json:"abstracts,omitempty"`
	ContainerId string     `json:"container_id,omitempty"`
	Contribs    []Contrib  `json:"contribs,omitempty"`
	ExtIDs      ExtID      `json:"ext_ids,omitempty"`
	Extra       struct {
		Arxiv struct {
			Subjects []string `json:"subjects,omitempty"`
		} `json:"arxiv,omitempty"`
		DBLP struct {
			EE []struct {
				Type string `json:"type,omitempty"`
				Text string `json:"text,omitempty"`
			} `json:"ee,omitempty"`
		} `json:"dblp,omitempty"`
		OpenAlex struct {
			PdfUrls        []string `json:"pdf_urls,omitempty"`
			LocationsCount int64    `json:"locations_count,omitempty"`
			HasFulltext    bool     `json:"has_fulltext,omitempty"`
			OpenAccess     struct {
				AnyRepositoryHasFulltext bool        `json:"any_repository_has_fulltext,omitempty"`
				IsOa                     bool        `json:"is_oa,omitempty"`
				OaStatus                 string      `json:"oa_status,omitempty"`
				OaUrl                    interface{} `json:"oa_url,omitempty"`
			} `json:"open_access,omitempty"`
		} `json:"openalex,omitempty"`
		Crossref struct {
			AlternativeId []string `json:"alternative-id,omitempty"`
			Funder        []struct {
				Award         []string `json:"award,omitempty"`
				DOI           string
				DOIAssertedBy string `json:"doi-asserted-by,omitempty"`
				Name          string `json:"name,omitempty"`
			} `json:"funder,omitempty"`
			License []struct {
				ContentVersion string `json:"content-version,omitempty"`
				DelayInDays    int64  `json:"delay-in-days,omitempty"`
				Start          string `json:"start,omitempty"`
				URL            string
			} `json:"license,omitempty"`
			Subject []string `json:"subject,omitempty"`
			Type    string   `json:"type,omitempty"`
		} `json:"crossref,omitempty"`
		OAI struct {
			SetSpec []string `json:"set_spec,omitempty"`
			URL     []string `json:"url,omitempty"`
		} `json:"oai,omitempty"`
	} `json:"extra,omitempty"`
	Ident           string `json:"ident,omitempty"`
	ID              string `json:"id,omitempty"`
	Issue           string `json:"issue,omitempty"`
	Language        string `json:"language,omitempty"`
	LicenseSlug     string `json:"license_slug,omitempty"`
	Number          string `json:"number,omitempty"`
	OriginalTitle   string `json:"original_title,omitempty"`
	Pages           string `json:"pages,omitempty"`
	Publisher       string `json:"publisher,omitempty"`
	Refs            []Ref  `json:"refs,omitempty"`
	ReleaseDate     string `json:"release_date,omitempty"`
	ReleaseStage    string `json:"release_stage,omitempty"`
	ReleaseType     string `json:"release_type,omitempty"`
	ReleaseYear     int64  `json:"release_year,omitempty"`
	Revision        string `json:"revision,omitempty"`
	State           string `json:"state,omitempty"`
	Subtitle        string `json:"subtitle,omitempty"`
	Title           string `json:"title,omitempty"`
	Version         string `json:"version,omitempty"`
	Volume          string `json:"volume,omitempty"`
	WithdrawnStatus string `json:"withdrawn_status,omitempty"`
	WithdrawnDate   string `json:"withdrawn_date,omitempty"`
	WithdrawnYear   string `json:"withdrawn_year,omitempty"`
	WorkID          string `json:"work_id,omitempty"`
}

// Normalize title for clustering - lowercase, remove/replace non-letter chars, trim whitespace
func normalizeTitle(title string) string {
	if title == "" {
		return ""
	}

	// Convert to lowercase
	normalized := strings.ToLower(title)

	// Replace sequences of non-letter/non-digit characters with single space
	re := regexp.MustCompile(`[^a-z0-9]+`)
	normalized = re.ReplaceAllString(normalized, " ")

	// Trim whitespace
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// Remove all non-ASCII letters and digits, replace with nothing
func normalizeStrict(title string) string {
	if title == "" {
		return ""
	}

	var result strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r <= 127 { // ASCII only
				result.WriteRune(r)
			}
		}
	}
	return result.String()
}

// Create a sortable key from words (for clustering)
func createSortedKey(title string) string {
	if title == "" {
		return ""
	}

	words := strings.Fields(normalizeTitle(title))
	if len(words) == 0 {
		return ""
	}

	// Sort words to handle word order variations
	for i := 0; i < len(words); i++ {
		for j := i + 1; j < len(words); j++ {
			if words[i] > words[j] {
				words[i], words[j] = words[j], words[i]
			}
		}
	}

	return strings.Join(words, "")
}

func main() {
	// Command line flags
	var (
		inputFile  = flag.String("i", "", "input NDJSON file (default: stdin)")
		outputFile = flag.String("o", "", "output file (default: stdout)")
		separator  = flag.String("s", ";", "field separator (default: semicolon)")
		useTab     = flag.Bool("tab", false, "use tab as separator (overrides -s)")
	)
	flag.Parse()

	// Determine separator
	sep := *separator
	if *useTab {
		sep = "\t"
	}

	// Open input (stdin or file)
	var input *os.File
	var err error

	if *inputFile == "" {
		input = os.Stdin
	} else {
		input, err = os.Open(*inputFile)
		if err != nil {
			log.Fatal("Error opening input file:", err)
		}
		defer input.Close()
	}

	var w *os.File
	var bw = bufio.NewWriter(w)
	defer bw.Flush()

	if *outputFile == "" {
		w = os.Stdout
	} else {
		w, err = os.Create(*outputFile)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Close()
	}

	header := []string{
		"id",
		"source",
		"title",
		"subtitle",
		"original_title",
		"title_normalized",
		"title_strict",
		"title_sorted_key",
		"release_year",
		"release_type",
		"publisher",
		"language",
		"doi",
		"arxiv",
		"pmid",
		"pmcid",
		"isbn13",
		"hdl",
		"ark",
		"openalex",
		"dblp",
		"wikidata_qid",
		"first_author",
	}
	fmt.Fprintln(w, strings.Join(header, sep))

	scanner := bufio.NewScanner(input)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var release Release
		if err := json.Unmarshal([]byte(line), &release); err != nil {
			log.Printf("Error parsing line %d: %v", lineNum, err)
			continue
		}

		firstAuthor := ""
		if len(release.Contribs) > 0 {
			firstAuthor = release.Contribs[0].RawName
			if firstAuthor == "" && release.Contribs[0].GivenName != "" && release.Contribs[0].Surname != "" {
				firstAuthor = release.Contribs[0].GivenName + " " + release.Contribs[0].Surname
			}
		}

		titleNormalized := normalizeTitle(release.Title)
		titleStrict := normalizeStrict(release.Title)
		titleSortedKey := createSortedKey(release.Title)

		row := []string{
			release.ID,
			release.Source,
			release.Title,
			release.Subtitle,
			release.OriginalTitle,
			titleNormalized,
			titleStrict,
			titleSortedKey,
			fmt.Sprintf("%d", release.ReleaseYear),
			release.ReleaseType,
			release.Publisher,
			release.Language,
			release.ExtIDs.DOI,
			release.ExtIDs.Arxiv,
			release.ExtIDs.PMID,
			release.ExtIDs.PMCID,
			release.ExtIDs.ISBN13,
			release.ExtIDs.HDL,
			release.ExtIDs.Ark,
			release.ExtIDs.OpenAlex,
			release.ExtIDs.DBLP,
			release.ExtIDs.WikidataQID,
			firstAuthor,
		}
		for i, field := range row {
			field = strings.ReplaceAll(field, "\n", " ")
			field = strings.ReplaceAll(field, "\r", " ")
			if sep != "\t" {
				field = strings.ReplaceAll(field, sep, " ")
			} else {
				field = strings.ReplaceAll(field, "\t", " ")
			}
			row[i] = field
		}
		fmt.Fprintln(w, strings.Join(row, sep))
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
