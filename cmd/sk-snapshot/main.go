// sk-snapshot turns feeds into snapshots, for simplicity often with external tools.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/miku/scholkit"
	"github.com/miku/scholkit/config"
	"github.com/miku/scholkit/crossref"
)

var (
	defaultDataDir   = path.Join(xdg.DataHome, "schol") // TODO: this is more general, move this out
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
	tempDir    = flag.String("T", os.TempDir(), "temporary directory")
	numWorkers = flag.Int("w", runtime.NumCPU(), "number of workers")
	verbose    = flag.Bool("v", false, "verbose output")
	batchSize  = flag.Int("n", 100000, "batch size for crossref processing")
)

func main() {
	flag.Parse()
	if *source == "" {
		fmt.Fprintf(os.Stderr, "source must be set with -s flag\n")
		fmt.Fprintf(os.Stderr, "available sources: %v\n", availableSources)
		os.Exit(1)
	}
	config := &config.Config{
		DataDir:     *dir,
		FeedDir:     path.Join(*dir, scholkit.FeedsDir),
		SnapshotDir: path.Join(*dir, scholkit.SnapshotsDir),
		Source:      *source,
		TempDir:     *tempDir,
	}
	if err := os.MkdirAll(config.SnapshotDir, 0755); err != nil {
		log.Fatal(err)
	}
	outputFile := *output
	if outputFile == "" {
		fn := fmt.Sprintf("snapshot-%s-%s.jsonl.zst", *source, time.Now().Format("2006-01-02"))
		outputFile = path.Join(config.SnapshotDir, fn)
	}
	switch *source {
	case "crossref":
		err := createCrossrefSnapshot(config, outputFile)
		if err != nil {
			log.Fatalf("error creating crossref snapshot: %v", err)
		}
	case "openalex":
		worksDir := path.Join(config.FeedDir, "openalex/data/works/")
		script := fmt.Sprintf(`find %s -type f -name "*.gz" | parallel --block 10M --line-buffer -j %d -I {} unpigz -c {} | pv -l | zstd -c -T0 > %s`,
			worksDir, *numWorkers, outputFile)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("TMPDIR=%s", *tempDir))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	case "datacite":
		worksDir := path.Join(config.FeedDir, "datacite/")
		script := fmt.Sprintf(`fd . %s -x cat | parallel --block 10M --lb --pipe -j %d 'jq -rc .data[]' | LC_ALL=C sort -T %s -S 40%% -u --compress-program=zstd | zstd -c -T0 > %s`,
			worksDir, *numWorkers, *tempDir, outputFile)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("TMPDIR=%s", *tempDir))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	case "oai":
		dir := path.Join(config.FeedDir, "metha")
		script := fmt.Sprintf(`find %s -type f -name "*.zst" | parallel -j %d "zstdcat -T0" | sk-oai-records | sk-oai-dctojsonl-stream > %s`,
			dir, *numWorkers, outputFile)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("TMPDIR=%s", *tempDir))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	case "pubmed":
		worksDir := path.Join(config.FeedDir, "pubmed")
		script := fmt.Sprintf(`find %s -type f -name "pubmed*.xml.gz" | parallel --block 10M --line-buffer -j %d -I {} unpigz -c {} | zstd -c -T0 > %s`,
			worksDir, *numWorkers, outputFile)
		log.Println(script)
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("TMPDIR=%s", *tempDir))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("source not implemented")
	}
	log.Printf("snapshot created at: %s", outputFile)
}

// createCrossrefSnapshot handles the crossref-specific snapshot creation
func createCrossrefSnapshot(config *config.Config, outputFile string) error {
	crossrefDir := path.Join(config.FeedDir, "crossref")
	var inputFiles []string
	err := filepath.Walk(crossrefDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".zst") || strings.HasSuffix(path, ".gz")) {
			inputFiles = append(inputFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error finding crossref files: %w", err)
	}
	if len(inputFiles) == 0 {
		return fmt.Errorf("no crossref files found in %s", crossrefDir)
	}
	if *verbose {
		log.Printf("found %d crossref files", len(inputFiles))
	}
	opts := crossref.SnapshotOptions{
		InputFiles: inputFiles,
		OutputFile: outputFile,
		BatchSize:  *batchSize,
		NumWorkers: *numWorkers,
		Verbose:    *verbose,
		TempDir:    config.TempDir,
	}
	return crossref.CreateSnapshot(opts)
}
