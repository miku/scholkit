package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"time"

	"github.com/adrg/xdg"
	"github.com/jinzhu/now"
	"github.com/miku/scholkit/atomic"
	"github.com/sethgrid/pester"
)

var (
	defaultDataDir   = path.Join(xdg.DataHome, "skol")
	availableSources = []string{
		"openalex",
		"crossref",
		"datacite",
		"pubmed",
		"oai",
	}
	yesterday = time.Now().Add(-86400 * time.Second)
	oneHour   = 3600 * time.Second
	bNewline  = []byte("\n")
)

var (
	dir         = flag.String("d", defaultDataDir, "the main cache directory to put all data under")
	fetchSource = flag.String("s", "", "name of the the source to update")
	listSources = flag.Bool("l", false, "list available source names")
	endpointURL = flag.String("u", "", "endpoint URL for OAI")
	showStatus  = flag.Bool("a", false, "show status and path")
	date        = flag.String("t", yesterday.Format("2006-01-02"), "date to capture")
	runBackfill = flag.String("B", "", "run a backfill, if possible, from a given day on")
	maxRetries  = flag.Int("r", 3, "max retries")
	timeout     = flag.Duration("T", oneHour, "connectiont timeout")
)

func main() {
	flag.Parse()
	switch {
	case *showStatus:
		fmt.Println(*dir)
	case *listSources:
		for _, s := range availableSources {
			fmt.Println(s)
		}
	case *fetchSource != "":
		log.Printf("fetching %v [...]", *fetchSource)
		switch *fetchSource {
		case "openalex":
			dst := path.Join(*dir, "openalex")
			if err := os.MkdirAll(dst, 0755); err != nil {
				log.Fatal(err)
			}
			cmd := exec.Command("rclone", "sync", "--transfers=8", "--checkers=16", "-P", "aws:/openalex", dst)
			b, err := cmd.CombinedOutput()
			if _, err := os.Stderr.Write(b); err != nil {
				log.Fatal(err)
			}
			if err != nil {
				log.Fatal(err)
			}
		case "crossref":
			client := pester.New()
			client.Backoff = pester.ExponentialBackoff
			client.MaxRetries = *maxRetries
			client.RetryOnHTTP429 = true
			client.Timeout = *timeout
			t, err := time.Parse("2006-01-02", *date)
			if err != nil {
				log.Fatal(err)
			}
			ch := CrossrefHarvester{
				Client:              client,
				ApiEndpoint:         "https://api.crossref.org/works",
				ApiFilter:           "index",
				ApiEmail:            "martin.czygan@gmail.com",
				Rows:                1000,
				UserAgent:           "scholkit/dev",
				AcceptableMissRatio: 0.1,
				MaxRetries:          3,
			}
			if err := ch.WriteDaySlice(t, *dir, "sample"); err != nil {
				log.Fatal(err)
			}
		case "datacite":
			// run dcdump
		case "pubmed":
			// fetch a file from URL
			// https://ftp.ncbi.nlm.nih.gov/pubmed/updatefiles/
			client := pester.New()
			client.Backoff = pester.ExponentialBackoff
			client.MaxRetries = *maxRetries
			client.RetryOnHTTP429 = true
			client.Timeout = *timeout
			t, err := time.Parse("2006-01-02", *date)
			if err != nil {
				log.Fatal(err)
			}
			req, err := http.NewRequest("GET", "https://ftp.ncbi.nlm.nih.gov/pubmed/updatefiles/", nil)
			if err != nil {
				log.Fatal(err)
			}
			resp, err := client.Do(req)
			if err != nil {
				log.Fatal(err)
			}
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			var pat = fmt.Sprintf(`(?mi)(?P<Filename>pubmed[^"]*gz).*%s`, t.Format("2006-01-02"))
			var re = regexp.MustCompile(pat)
			matches := re.FindStringSubmatch(string(b))
			filenameIndex := re.SubexpIndex("Filename")
			filename := matches[filenameIndex]
			link, err := url.JoinPath("https://ftp.ncbi.nlm.nih.gov/pubmed/updatefiles/", filename)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf(link)
		case "oai":
			// use metha
		}
	}
}

// Doer abstracts https://pkg.go.dev/net/http#Client.Do.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// CrossrefHarvester fetches data from crossref API and by default will write
// it to files on disk.
type CrossrefHarvester struct {
	Client              Doer
	ApiEndpoint         string
	ApiFilter           string
	ApiEmail            string
	Rows                int
	UserAgent           string
	MaxRetries          int
	AcceptableMissRatio float64 // recommended: 0.1
}

// WorksResponse, stripped of the actual messages, as we only need the status
// and mayby total results.
type WorksResponse struct {
	Message struct {
		Facets struct {
		} `json:"facets"`
		Items        []json.RawMessage `json:"items"`
		ItemsPerPage int64             `json:"items-per-page"`
		NextCursor   string            `json:"next-cursor"` // iterate
		Query        struct {
			SearchTerms interface{} `json:"search-terms"`
			StartIndex  int64       `json:"start-index"`
		} `json:"query"`
		TotalResults int64 `json:"total-results"` // want to estimate total results (and verify download)
	} `json:"message"`
	MessageType    string `json:"message-type"`
	MessageVersion string `json:"message-version"`
	Status         string `json:"status"`
}

// IsLast returns true, if there are no more records to fetch.
func (wr *WorksResponse) IsLast() bool {
	return wr.Message.NextCursor == ""
}

// WriteDaySlice is a helper function to atomically write crossref data for a
// single day to file on disk under dir. Idempotent, once the data has been
// captured. TODO: add compression.
func (c *CrossrefHarvester) WriteDaySlice(t time.Time, dir string, prefix string) error {
	start := now.With(t).BeginningOfDay()
	end := now.With(t).EndOfDay()
	fn := fmt.Sprintf("%s%s-%s-%s.json.zst",
		prefix,
		c.ApiFilter,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"))
	cachePath := path.Join(dir, fn)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		f, err := atomic.New(cachePath, 0644)
		if err != nil {
			return err
		}
		if err := c.WriteSlice(f, start, end); err != nil {
			return err
		}
	}
	return nil
}

// addOptionalEmail appends mailto parameter.
func (c *CrossrefHarvester) addOptionalEmail(vs url.Values) {
	if c.ApiEmail != "" {
		vs.Add("mailto", c.ApiEmail)
	}
}

// logSeenRatio logs some progress stats for the current works response.
func (c *CrossrefHarvester) logSeenRatio(seen int, wr *WorksResponse) {
	if wr == nil {
		return
	}
	var pct float64
	if wr.Message.TotalResults == 0 {
		pct = 0.0
	} else {
		pct = 100 * (float64(seen) / float64(wr.Message.TotalResults))
	}
	log.Printf("crossref: status=%s, total=%d, seen=%d (%0.2f%%), cursor=%s",
		wr.Status, wr.Message.TotalResults, seen, pct, wr.Message.NextCursor)
}

// WriteSlice writes a slice of data from the API into a writer.
func (c *CrossrefHarvester) WriteSlice(w io.Writer, from, until time.Time) error {
	filter := fmt.Sprintf("from-%s-date:%s,until-%s-date:%s",
		c.ApiFilter,
		from.Format("2006-01-02"),
		c.ApiFilter,
		until.Format("2006-01-02"))
	vs := url.Values{}
	vs.Add("filter", filter)
	vs.Add("cursor", "*")
	vs.Add("rows", fmt.Sprintf("%d", c.Rows))
	c.addOptionalEmail(vs)
	var seen int
	var i int // for retries
	for {
		link := fmt.Sprintf("%s?%s", c.ApiEndpoint, vs.Encode())
		log.Printf("crossref: attempting to fetch: %s", link)
		req, err := http.NewRequest("GET", link, nil)
		if err != nil {
			return err
		}
		req.Header.Add("User-Agent", c.UserAgent)
		resp, err := c.Client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("crossref: HTTP %d while fetching %s", resp.StatusCode, link)
		}
		var wr WorksResponse
		if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
			if i < c.MaxRetries {
				i++
				log.Printf("crossref: decode failed with %v, retrying [%d/%d]", err, i, c.MaxRetries)
				continue
			} else {
				return fmt.Errorf("crossref: decode failed with %v", err)
			}
		}
		if wr.Status != "ok" {
			return fmt.Errorf("crossref failed with status: %s", wr.Status)
		}
		for _, item := range wr.Message.Items {
			item = append(item, bNewline...)
			if _, err := w.Write(item); err != nil {
				return err
			}
		}
		seen += len(wr.Message.Items)
		c.logSeenRatio(seen, &wr)
		if wr.IsLast() || seen >= int(wr.Message.TotalResults) {
			log.Printf("crossref slice done: seen=%d, total=%d", seen, wr.Message.TotalResults)
			return nil
		}
		vs = url.Values{}
		cursor := wr.Message.NextCursor
		if cursor == "" {
			return nil
		}
		vs.Add("cursor", cursor)
		c.addOptionalEmail(vs)
		// status: ok, total: 55818, seen: 47818 (85.67%)
		// We had repeated requests, with a seemingly new cursor, but no new
		// messages and seen < total; we assume, we have got all we could and
		// move on. Note: this may be a temporary glitch; rather retry.
		if len(wr.Message.Items) == 0 {
			numMissOk := int(c.AcceptableMissRatio * float64(wr.Message.TotalResults))
			if int(wr.Message.TotalResults)-seen < numMissOk {
				log.Printf("crossref: assuming ok to skip, seen=%d, total=%d", seen, wr.Message.TotalResults)
				break
			} else {
				return fmt.Errorf("crossref: no more messages, api may have changed, total=%d, seen=%d",
					wr.Message.TotalResults, seen)
			}
		}
		i = 0
	}
	return nil
}
