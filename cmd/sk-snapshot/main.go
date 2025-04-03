package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/adrg/xdg"
	"github.com/miku/scholkit/config"
)

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
)

var (
	dir        = flag.String("d", defaultDataDir, "the main cache directory to put all data under") // TODO: use env var
	source     = flag.String("s", "", "name of the the source to snapshot")
	output     = flag.String("o", "", "output file, if empty, a sensible output file path will be derived from source and date")
	numWorkers = flag.Int("w", runtime.NumCPU(), "number of workers")
)

func main() {
	flag.Parse()
	config := &config.Config{
		DataDir:     *dir,
		FeedDir:     path.Join(*dir, "feeds"),
		SnapshotDir: path.Join(*dir, "snapshots"),
		Source:      *source,
	}
	if err := os.MkdirAll(config.SnapshotDir, 0755); err != nil {
		log.Fatal(err)
	}
	if *output == "" {
		fn := fmt.Sprintf("snapshot-%s-%s.jsonl.zst", *source, time.Now().Format("2006-01-02"))
		*output = path.Join(config.SnapshotDir, fn)
	}
	switch *source {
	case "crossref":
		// this is a bit more involved
	case "openalex":
		worksDir := path.Join(config.FeedDir, "openalex/data/works/")
		script := fmt.Sprintf(`find %s -type f -name "*.gz" | parallel --block 10M --line-buffer -j %d -I {} unpigz -c {} | pv -l | zstd -c -T0 > %s`,
			worksDir, *numWorkers, *output)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	case "datacite":
		worksDir := path.Join(config.FeedDir, "datacite/")
		script := fmt.Sprintf(`fd . %s -x cat | parallel --block 10M --lb --pipe -j %d 'jq -rc .data[]' | zstd -c -T0 > %s`,
			worksDir, *numWorkers, *output)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	case "oai":
		dir := path.Join(config.FeedDir, "metha")
		script := fmt.Sprintf(`find %s -type f -name "*.gz" | parallel -j %d "unpigz -c" | sk-oai-records | sk-oai-dctojsonl-stream > %s`,
			dir, *numWorkers, *output)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	case "pubmed":
		// maybe convert first, then snapshot
	default:
		log.Fatal("source not implemented")
	}
}
