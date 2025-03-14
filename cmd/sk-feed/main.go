// sk-feed retrieves various upstream data sources. We start with using
// external programs, but aim towards less shelling out in the future.
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
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/jinzhu/now"
	"github.com/klauspost/compress/zstd"
	"github.com/miku/scholkit"
	"github.com/miku/scholkit/atomic"
	"github.com/miku/scholkit/dateutil"
	"github.com/miku/scholkit/xflag"
	"github.com/sethgrid/pester"
)

var docs = strings.TrimLeft(`
# skfeed - fetch data feeds

Uses mostly external tools to fetch raw bibliographic data from the web:
rclone, metha, dcdump.  NOTE: not all flags may work, e.g. -B backfill is not
fully implemented yet.

## external tools

$ sudo apt install rclone
$ go install -v github.com/miku/metha/cmd/...@latest
$ go install -v github.com/miku/dcdump/cmd/...@latest

## openalex

Hardcoded "aws" prefix, please add it to rclone.conf, cf.
https://docs.openalex.org/download-all-data/download-to-your-machine

	$ cat ~/.config/rclone/rclone.conf

	[aws]
	type = s3

## list feeds

$ sk-feed -l
openalex
crossref
datacite
pubmed
oai

## fetch feed

$ sk-feed -s openalex
$ sk-feed -s crossref

## flags

`, "\n")

var (
	defaultDataDir   = path.Join(xdg.DataHome, "schol")
	availableSources = []string{
		"openalex",
		"crossref",
		"datacite",
		"pubmed",
		"oai",
	}
	yesterday = time.Now().Add(-86400 * time.Second)
	oneDay    = 86400 * time.Second
	oneHour   = 3600 * time.Second
	bNewline  = []byte("\n")
)

// Config for feeds, TODO(martin): move to config file and environment variables.
type Config struct {
	// DataDir is the generic data dir for all scholkit tools.
	DataDir string
	// FeedDir is the directory specifically for raw data feeds only. A subdir
	// of DataDir.
	FeedDir            string
	Source             string
	EndpointURL        string
	Date               time.Time
	MaxRetries         int
	Timeout            time.Duration
	CrossrefApiEmail   string
	CrossrefUserAgent  string
	CrossrefFeedPrefix string
	CrossrefApiFilter  string
	RcloneTransfers    int
	RcloneCheckers     int
	DataciteSyncStart  string
}

var (
	dir         = flag.String("d", defaultDataDir, "the main cache directory to put all data under") // TODO: use env var
	fetchSource = flag.String("s", "", "name of the the source to update")
	listSources = flag.Bool("l", false, "list available source names")
	showStatus  = flag.Bool("a", false, "show status and path")
	dateStr     = flag.String("t", yesterday.Format("2006-01-02"), "date to capture")
	runBackfill = flag.String("B", "", "run a backfill, if possible, from a given day (YYYY-MM-DD) on")
	maxRetries  = flag.Int("r", 3, "max retries")
	timeout     = flag.Duration("T", oneHour, "connectiont timeout")
	showVersion = flag.Bool("version", false, "show version")
	// rclone is used for openalex
	rcloneTransfers = flag.Int("rclone-transfers", 8, "number of parallel transfers for rclone")
	rcloneCheckers  = flag.Int("rclone-checkers", 16, "number of parallel checkers for rclone")
	// crossref specific options
	crossrefApiEmail              = flag.String("crossref-api-email", "martin.czygan@gmail.com", "crossref api email")
	crossrefApiFilter             = flag.String("crossref-api-filter", "index", "api filter to use with crossref")
	crossrefUserAgent             = flag.String("crossref-user-agent", "scholkit/dev", "crossref user agent")
	crossrefFeedPrefix            = flag.String("crossref-feed-prefix", "crossref-feed-0-", "prefix for filename to distinguish different runs")
	crossrefSyncStart  xflag.Date = xflag.Date{Time: dateutil.MustParse("2021-01-01")}
	crossrefSyncEnd    xflag.Date = xflag.Date{Time: yesterday}
	// datacite specific options
	dataciteSyncStart = flag.String("datacite-sync-start", "2020-01-01", "when to start datacite fetch")
	// oai specific options
	endpointURL = flag.String("u", "", "endpoint URL for OAI")
)

func main() {
	flag.Var(&crossrefSyncStart, "crossref-sync-start", "start date for crossref harvest")
	flag.Var(&crossrefSyncEnd, "crossref-sync-end", "end date for crossref harvest")
	flag.Usage = func() {
		io.WriteString(os.Stderr, docs)
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println(scholkit.Version)
		os.Exit(0)
	}
	date, err := time.Parse("2006-01-02", *dateStr)
	if err != nil {
		log.Fatalf("invalid date: %v", err)
	}
	config := &Config{
		DataDir:            *dir,
		FeedDir:            path.Join(*dir, "feeds"),
		Source:             *fetchSource,
		EndpointURL:        *endpointURL,
		Date:               date,
		MaxRetries:         *maxRetries,
		Timeout:            *timeout,
		CrossrefApiEmail:   *crossrefApiEmail,
		CrossrefApiFilter:  *crossrefApiFilter,
		CrossrefUserAgent:  *crossrefUserAgent,
		CrossrefFeedPrefix: *crossrefFeedPrefix,
		RcloneTransfers:    *rcloneTransfers,
		RcloneCheckers:     *rcloneCheckers,
		DataciteSyncStart:  *dataciteSyncStart,
	}
	// HTTP client
	client := pester.New()
	client.Backoff = pester.ExponentialBackoff
	client.MaxRetries = *maxRetries
	client.RetryOnHTTP429 = true
	client.Timeout = *timeout
	switch {
	case *showStatus:
		fmt.Printf("feeds: \n", config.FeedDir)
	case *listSources:
		for _, s := range availableSources {
			fmt.Println(s)
		}
	case config.Source != "":
		log.Printf("fetching %v [...]", config.Source)
		switch config.Source {
		case "openalex":
			// openalex is updated in roughly monthly intervals; after an
			// update an rclone sync may take a few hours to fetch data from
			// AWS bucket
			dst := path.Join(config.FeedDir, "openalex")
			if err := os.MkdirAll(dst, 0755); err != nil {
				log.Fatal(err)
			}
			cmd := exec.Command("rclone",
				"sync",
				fmt.Sprintf("--transfers=%d", config.RcloneTransfers),
				fmt.Sprintf("--checkers=%d", config.RcloneCheckers),
				"-P",
				"aws:/openalex",
				dst)
			log.Println(cmd)
			b, err := cmd.CombinedOutput() // TODO(martin): show live update w/ pipe
			if _, err := os.Stderr.Write(b); err != nil {
				log.Fatal(err)
			}
			if err != nil {
				log.Fatal(err)
			}
		case "crossref":
			ch := CrossrefHarvester{
				Client:              client,
				ApiEndpoint:         "https://api.crossref.org/works",
				ApiFilter:           config.CrossrefApiFilter,
				ApiEmail:            config.CrossrefApiEmail,
				Rows:                1000,
				UserAgent:           config.CrossrefUserAgent,
				AcceptableMissRatio: 0.1,
				MaxRetries:          3,
			}
			dstDir := path.Join(config.FeedDir, "crossref")
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				log.Fatal(err)
			}
			log.Println(ch)
			ivs := dateutil.Daily(crossrefSyncStart.Time, crossrefSyncEnd.Time)
			for _, iv := range ivs {
				// TODO: we only need the start date, because we limit
				// ourselves to day slices
				if err := ch.WriteDaySlice(iv.Start, dstDir, config.CrossrefFeedPrefix); err != nil {
					log.Fatalf("crossref day slice: %v", err)
				}
			}
		case "datacite":
			dstDir := path.Join(config.FeedDir, "datacite")
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				log.Fatal(err)
			}
			cmd := exec.Command("dcdump",
				"-s", config.DataciteSyncStart,
				"-e", date.Add(oneDay).Format("2006-01-02"),
				"-i", "e", // most fine granular, takes a while to backfill
				"-d", dstDir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			log.Println(cmd)
			if err = cmd.Run(); err != nil {
				log.Fatal(err)
			}
		case "pubmed":
			// fetch a file from URL
			// https://ftp.ncbi.nlm.nih.gov/pubmed/updatefiles/
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
			var pat = fmt.Sprintf(`(?mi)(?P<Filename>pubmed[^"]*gz).*%s`, date.Format("2006-01-02"))
			var re = regexp.MustCompile(pat)
			matches := re.FindStringSubmatch(string(b))
			filenameIndex := re.SubexpIndex("Filename")
			filename := matches[filenameIndex]
			link, err := url.JoinPath("https://ftp.ncbi.nlm.nih.gov/pubmed/updatefiles/", filename)
			if err != nil {
				log.Fatal(err)
			}
			dstDir := path.Join(config.FeedDir, "pubmed")
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				log.Fatal(err)
			}
			dstFile := path.Join(dstDir, filename)
			if _, err := os.Stat(dstFile); os.IsNotExist(err) {
				cmd := exec.Command("curl", "-sL", "-O", "--output-dir", dstDir, link)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				log.Println(cmd)
				if err = cmd.Run(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Printf("already synced: %v", dstFile)
			}
		case "oai":
			baseDir := path.Join(config.FeedDir, "metha")
			cmd := exec.Command("metha-sync",
				"-base-dir", baseDir,
				*endpointURL)
			log.Println(cmd)
			if _, err = cmd.CombinedOutput(); err != nil {
				log.Fatal(err)
			}
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
		enc, err := zstd.NewWriter(f)
		if err != nil {
			return err
		}
		if err := c.WriteSlice(enc, start, end); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
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
