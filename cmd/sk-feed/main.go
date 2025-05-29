// sk-feed retrieves various upstream data sources. We start with using
// external programs, but aim towards less shelling out in the future.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/miku/scholkit"
	"github.com/miku/scholkit/config"
	"github.com/miku/scholkit/dateutil"
	"github.com/miku/scholkit/exdep"
	"github.com/miku/scholkit/feeds"
	"github.com/miku/scholkit/xflag"
	"github.com/sethgrid/pester"
)

var docs = strings.TrimLeft(`
# sk-feed - fetch data feeds

Uses mostly external tools to fetch raw bibliographic data from the web:
rclone, metha, dcdump.

## openalex

By default we use "aws" as S3 prefix (-rclone-s3-prefix), please add it to
rclone.conf, cf. https://docs.openalex.org/download-all-data/download-to-your-machine

	$ cat ~/.config/rclone/rclone.conf

	[aws]
	type = s3

## list feeds

$ sk-feed -l

## fetch feed

$ sk-feed -s openalex
$ sk-feed -s crossref

## flags

`, "\n")

const FeedsDir = "feeds"

var deps = []exdep.Dep{
	{
		Name:  "metha-sync",
		Links: []string{"https://github.com/miku/metha/releases/"},
		Docs:  `oai-pmh harvester`,
	}, {
		Name:  "dcdump",
		Links: []string{"https://github.com/miku/dcdump/releases/"},
		Docs:  `datacite harvester`,
	}, {
		Name:  "rclone",
		Links: []string{"https://rclone.org"},
		Docs:  `cloud storage cli`,
	}, {
		Name:  "curl",
		Links: []string{"https://curl.org"},
		Docs:  `http cli`,
	},
}

var (
	defaultDataDir   = path.Join(xdg.DataHome, "schol")
	availableSources = []string{
		"openalex",
		"crossref",
		"datacite",
		"pubmed",
		"oai",
		// TODO: add dblp, doaj, wikicite (maybe), JALC, ...
	}
	yesterday = time.Now().Add(-86400 * time.Second)
	oneDay    = 86400 * time.Second
	oneHour   = 3600 * time.Second
)

var (
	dir         = flag.String("d", defaultDataDir, "the main cache directory to put all data under") // TODO: use env var
	fetchSource = flag.String("s", "", "name of the the source to update")
	listSources = flag.Bool("l", false, "list available source names")
	showStatus  = flag.Bool("a", false, "show status and path")
	// dateStr is deprecated, we should make it so we can always request
	// everything and skip things we have cached, already
	dateStr     = flag.String("t", yesterday.Format("2006-01-02"), "date to capture")
	maxRetries  = flag.Int("r", 3, "max retries")
	timeout     = flag.Duration("T", oneHour, "connectiont timeout")
	showVersion = flag.Bool("version", false, "show version")
	// rclone is used for openalex
	rcloneTransfers = flag.Int("rclone-transfers", 8, "number of parallel transfers for rclone")
	rcloneCheckers  = flag.Int("rclone-checkers", 16, "number of parallel checkers for rclone")
	rcloneS3Prefix  = flag.String("rclone-s3-prefix", "aws", "rclone s3 storage designation")
	// crossref specific options
	crossrefApiEmail              = flag.String("crossref-api-email", "martin.czygan@gmail.com", "crossref api email")
	crossrefApiFilter             = flag.String("crossref-api-filter", "index", "api filter to use with crossref")
	crossrefUserAgent             = flag.String("crossref-user-agent", "scholkit/dev", "crossref user agent")
	crossrefFeedPrefix            = flag.String("crossref-feed-prefix", "crossref-feed-0-", "prefix for filename to distinguish different runs")
	crossrefSyncStart  xflag.Date = xflag.Date{Time: dateutil.MustParse("2021-01-01")}
	crossrefSyncEnd    xflag.Date = xflag.Date{Time: yesterday}
	// datacite specific options
	dataciteSyncStart xflag.Date = xflag.Date{Time: dateutil.MustParse("2020-01-01")}
	// oai specific options
	endpointURL = flag.String("oai-endpoint", "", "endpoint URL for OAI")
)

func main() {
	flag.Var(&crossrefSyncStart, "crossref-sync-start", "start date for crossref harvest")
	flag.Var(&crossrefSyncEnd, "crossref-sync-end", "end date for crossref harvest")
	flag.Var(&dataciteSyncStart, "datacite-sync-start", "start date for datacite harvest")
	flag.Usage = func() {
		io.WriteString(os.Stderr, docs)
		flag.PrintDefaults()
	}
	flag.Parse()
	if err := exdep.Check(deps); err != nil {
		for _, e := range err {
			log.Println(e)
		}
		os.Exit(1)
	}
	if *showVersion {
		fmt.Println(scholkit.Version)
		os.Exit(0)
	}
	date, err := time.Parse("2006-01-02", *dateStr)
	if err != nil {
		log.Fatalf("invalid date: %v", err)
	}
	config := &config.Config{
		DataDir:            *dir,
		FeedDir:            path.Join(*dir, FeedsDir),
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
		DataciteSyncStart:  dataciteSyncStart.Format("2006-01-02"),
	}
	// Ensure feeds directory exists
	if err := os.MkdirAll(config.FeedDir, 0755); err != nil {
		log.Fatal(err)
	}
	// HTTP client
	client := pester.New()
	client.Backoff = pester.ExponentialBackoff
	client.MaxRetries = *maxRetries
	client.RetryOnHTTP429 = true
	client.Timeout = *timeout
	switch {
	case *showStatus:
		fmt.Printf("feeds: %s\n", config.FeedDir)
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
				fmt.Sprintf("%s:/openalex", *rcloneS3Prefix), // TODO: sanitize and validate
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
			ch := feeds.CrossrefHarvester{
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
			// TODO: fix GOAWAY
			//
			// Mar 25 08:31:53 sk-feed[3837740]: time="2025-03-25T08:31:53Z" level=info msg="batch done: https://api.datacite.org/dois?affiliation=true&page%5Bcursor%5D=1&page%5Bsize%5D=100&query=updated%3A%5B2022-05-02T18%3A>
			// Mar 25 08:31:54 sk-feed[3837740]: time="2025-03-25T08:31:54Z" level=info msg="requests=1, pages=1, total=47"
			// Mar 25 08:31:54 sk-feed[3837740]: time="2025-03-25T08:31:54Z" level=info msg="batch done: https://api.datacite.org/dois?affiliation=true&page%5Bcursor%5D=1&page%5Bsize%5D=100&query=updated%3A%5B2022-05-02T18%3A>
			// Mar 25 08:31:54 sk-feed[3837740]: time="2025-03-25T08:31:54Z" level=info msg="failed to create file for https://api.datacite.org/dois?affiliation=true&page%5Bcursor%5D=1&page%5Bsize%5D=100&query=updated%3A%5B20>
			// Mar 25 08:31:54 sk-feed[3837740]: time="2025-03-25T08:31:54Z" level=warning msg="incomplete harvest - maybe rm -f /var/data/schol/feeds/datacite/dcdump-*.ndjson"
			// Mar 25 08:31:54 sk-feed[3837740]: time="2025-03-25T08:31:54Z" level=fatal msg="http2: server sent GOAWAY and closed the connection; LastStreamID=18849, ErrCode=NO_ERROR, debug=\"\""
			// Mar 25 08:31:54 sk-feed[3837724]: 2025/03/25 08:31:54 exit status 1
			// Mar 25 08:31:54 systemd[1]: sk-feed-datacite.service: Main process exited, code=exited, status=1/FAILURE
			// Mar 25 08:31:54 systemd[1]: sk-feed-datacite.service: Failed with result 'exit-code'.
			// Mar 25 08:31:54 systemd[1]: Failed to start Harvest metadata from api.datacite.org.
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
			// Download baseline files.
			log.Println("syncing pubmed baseline...")
			fetcher, err := feeds.NewPubMedFetcher("https://ftp.ncbi.nlm.nih.gov/pubmed/baseline/")
			if err != nil {
				log.Fatal(err)
			}
			pmfs, err := fetcher.FetchFiles()
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("found %d pubmed baseline files", len(pmfs))
			dstDir := path.Join(config.FeedDir, "pubmed")
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				log.Fatal(err)
			}
			for _, pmf := range pmfs {
				dstFile := path.Join(dstDir, pmf.Filename)
				wip := dstFile + ".wip"
				if _, err := os.Stat(dstFile); os.IsNotExist(err) {
					cmd := exec.Command("curl", "-sL", "--retry", "10", "--max-time", "1800", "-o", wip, pmf.URL)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					log.Println(cmd)
					if err = cmd.Run(); err != nil {
						log.Fatal(err)
					}
					if err := os.Rename(wip, dstFile); err != nil {
						log.Fatal(err)
					}
				} else {
					log.Printf("already synced: %v", dstFile)
				}
			}
			log.Println("syncing pubmed updates...")
			fetcher, err = feeds.NewPubMedFetcher("https://ftp.ncbi.nlm.nih.gov/pubmed/updatefiles/")
			if err != nil {
				log.Fatal(err)
			}
			pmfs, err = fetcher.FetchFiles()
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("found %d pubmed update files", len(pmfs))
			for _, pmf := range pmfs {
				dstFile := path.Join(dstDir, pmf.Filename)
				wip := dstFile + ".wip"
				if _, err := os.Stat(dstFile); os.IsNotExist(err) {
					cmd := exec.Command("curl", "-sL", "--retry", "10", "--max-time", "600", "-o", wip, pmf.URL)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					log.Println(cmd)
					if err = cmd.Run(); err != nil {
						log.Fatal(err)
					}
					if err := os.Rename(wip, dstFile); err != nil {
						log.Fatal(err)
					}
				} else {
					log.Printf("already synced: %v", dstFile)
				}
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
