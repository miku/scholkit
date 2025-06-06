package convert

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Skip struct {
	err error
}

func (s Skip) Error() string {
	return s.err.Error()
}

var (
	ErrSkipNoTitle             = Skip{err: errors.New("no title")}
	ErrSkipCrossrefReleaseType = Skip{err: errors.New("blacklisted crossref release type")}
	ErrSkipNoDOI               = Skip{err: errors.New("no doi")}
)

// TODO: need to pass various conversion options to functions

// hashString returns a hex-encoded hash of a string.
func hashString(s string) string {
	h := sha1.New()
	_, _ = io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Enhanced helper functions
func parseDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// Try different date formats
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01",
		"2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// Extract year if possible
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	if match := yearPattern.FindString(dateStr); match != "" {
		return match + "-01-01"
	}

	return ""
}

// extractYear attempts to extract a year from a date string
func extractYear(date string) (int64, error) {
	// Try parsing as full date
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, date); err == nil {
			return int64(t.Year()), nil
		}
	}
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	match := yearPattern.FindString(date)
	if match != "" {
		year, err := strconv.ParseInt(match, 10, 64)
		return year, err
	}
	return 0, fmt.Errorf("could not extract year from date: %s", date)
}

func cleanTitle(title string) string {
	if title == "" {
		return ""
	}

	// Remove excessive whitespace and clean up
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	// Remove common prefixes/suffixes that don't belong in titles
	prefixes := []string{"Title:", "TITLE:", "Abstract:", "ABSTRACT:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(title, prefix) {
			title = strings.TrimSpace(strings.TrimPrefix(title, prefix))
		}
	}

	return title
}

func parseContributorName(fullName string) (given, family string) {
	if fullName == "" {
		return "", ""
	}

	// Handle "LastName, FirstName" format
	if strings.Contains(fullName, ",") {
		parts := strings.Split(fullName, ",")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
		}
	}

	// Handle "FirstName LastName" format
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	} else if len(parts) == 1 {
		return "", parts[0]
	} else {
		return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}
}

// Enhanced helper functions
func cleanORCID(orcid string) string {
	// Remove URL prefix if present
	orcid = strings.Replace(orcid, "https://orcid.org/", "", 1)
	orcid = strings.Replace(orcid, "http://orcid.org/", "", 1)
	return strings.TrimSpace(orcid)
}

func inferLicenseSlug(licenseURL string) string {
	licenseURL = strings.ToLower(licenseURL)

	if strings.Contains(licenseURL, "creativecommons.org") {
		if strings.Contains(licenseURL, "by-nc-sa") {
			return "cc-by-nc-sa"
		} else if strings.Contains(licenseURL, "by-nc-nd") {
			return "cc-by-nc-nd"
		} else if strings.Contains(licenseURL, "by-nc") {
			return "cc-by-nc"
		} else if strings.Contains(licenseURL, "by-sa") {
			return "cc-by-sa"
		} else if strings.Contains(licenseURL, "by-nd") {
			return "cc-by-nd"
		} else if strings.Contains(licenseURL, "by") {
			return "cc-by"
		} else if strings.Contains(licenseURL, "cc0") {
			return "cc0"
		}
	}

	return ""
}

func mapDataCiteType(resourceType string) string {
	typeMap := map[string]string{
		"text":              "article",
		"article":           "article-journal",
		"book":              "book",
		"chapter":           "chapter",
		"conference paper":  "paper-conference",
		"conference poster": "article",
		"dataset":           "dataset",
		"thesis":            "thesis",
		"report":            "report",
		"software":          "software",
		"image":             "graphic",
		"video":             "motion_picture",
		"sound":             "song",
	}

	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if mapped, ok := typeMap[resourceType]; ok {
		return mapped
	}

	// Fuzzy matching
	for key, value := range typeMap {
		if strings.Contains(resourceType, key) {
			return value
		}
	}

	return "article" // default fallback
}
