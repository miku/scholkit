package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/adrg/xdg"
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
)

var (
	dir         = flag.String("d", defaultDataDir, "the main cache directory to put all data under")
	fetchSource = flag.String("s", "", "name of the the source to update")
	listSources = flag.Bool("l", false, "list available source names")
	endpointURL = flag.String("u", "", "endpoint URL for OAI")
	showStatus  = flag.Bool("a", false, "show status and path")
	date        = flag.String("t", yesterday.Format("2006-01-02"), "date to capture")
	runBackfill = flag.Bool("B", false, "run a backfill, if possible")
	maxRetries  = flag.Int("r", 3, "max retries")
	timeout     = flag.Duration("t", oneHour, "connectiont timeout")
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
		case "datacite":
			// run dcdump
		case "pubmed":
			// fetch a file from URL
		case "oai":
			// use metha

		}
	}
}

// Doer abstracts https://pkg.go.dev/net/http#Client.Do.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type CrossrefHarvester struct {
	Client      Doer
	ApiEndpoint string
	ApiFilter   string
	ApiEmail    string
	Rows        int
	UserAgent   string
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
