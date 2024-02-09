package oaiscrape

import (
	"regexp"
	"strings"
)

// TODO: move this out
var issnPattern = regexp.MustCompile(`[0-9]{4,4}-[0-9X]{4,4}`)

// doiPreceding are some possible strings preceding a DOI
var doiPreceding = []string{
	"doi:",
	"http://doi.org/",
	"http://dx.doi.org/",
	"https://doi.org/",
	"https://dx.doi.org/",
}

// Document is a minimal JSON document already converted from XML through other means.
type Document struct {
	Creators     []string `json:"creators"`
	Datestamp    string   `json:"datestamp"`
	Descriptions []string `json:"descriptions"`
	IDs          []string `json:"ids"`
	Languages    []string `json:"languages"`
	OAI          string   `json:"oai"`
	Rights       []string `json:"rights"`
	Sets         []string `json:"sets"`
	Titles       []string `json:"titles"`
	Types        []string `json:"types"`
	URLs         []string `json:"urls"`
}

// DOI returns the first DOI, will attempt slight guessing at various strings.
func (doc *Document) DOI() string {
	for _, f := range doc.IDs {
		if strings.HasPrefix(f, "10.") {
			return f
		}
		for _, p := range doiPreceding {
			if strings.HasPrefix(f, p) {
				return strings.Replace(f, p, "", 1)
			}
		}
	}
	return ""
}

// URL returns the first URL found.
func (doc *Document) URL() string {
	for _, f := range doc.IDs {
		if strings.HasPrefix(f, "http") {
			return f
		}
	}
	return ""
}

// ISSN returns the first ISSN found.
func (doc *Document) ISSN() string {
	for _, f := range doc.IDs {
		if issnPattern.MatchString(f) {
			return f
		}
	}
	return ""
}
