package main

import (
	"flag"
	"fmt"
	"log"
	"path"

	"github.com/adrg/xdg"
)

var (
	defaultDataDir   = path.Join(xdg.DataHome, "skol")
	availableSources = []string{
		"openalex",
		"crossref",
		"datacite",
		"pubmed",
		"oai:endpoint-url",
	}
)

var (
	dir         = flag.String("d", defaultDataDir, "the main cache directory to put all data under")
	fetchSource = flag.String("s", "", "name of the the source to update")
	listSources = flag.Bool("l", false, "list available source names")
	showStatus  = flag.Bool("a", false, "show status and path")
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
		log.Printf("fetching %v", *fetchSource)
	}
}
