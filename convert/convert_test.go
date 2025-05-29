package convert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miku/scholkit/schema/crossref"
	"github.com/miku/scholkit/schema/datacite"
	"github.com/miku/scholkit/schema/fatcat"
	"github.com/miku/scholkit/schema/openalex"
)

func TestConvertCrossrefWorkToFatcatRelease(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "crossref-*.input"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var work crossref.Work
			if err := json.Unmarshal(b, &work); err != nil {
				t.Fatal(err)
			}
			release, err := CrossrefWorkToFatcatRelease(&work)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(release, "", "    ")
			if err != nil {
				t.Fatal(err)
			}
			goldenfile := filepath.Join("testdata", name+".golden")
			want, err := os.ReadFile(goldenfile)
			if err != nil {
				if os.IsNotExist(err) {
					if err := os.WriteFile(goldenfile, got, 0644); err != nil {
						t.Fatal(err)
					}
					t.Logf("created golden file: %s", goldenfile)
					return
				}
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("%s: got: %s want: %s", name, string(got), string(want))
			}
		})
	}
}

func TestConvertDataciteToFatcatRelease(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "datacite-*.input"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var doc datacite.Document
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatal(err)
			}
			release, err := DataCiteToFatcatRelease(&doc)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(release, "", "    ")
			if err != nil {
				t.Fatal(err)
			}
			goldenfile := filepath.Join("testdata", name+".golden")
			want, err := os.ReadFile(goldenfile)
			if err != nil {
				if os.IsNotExist(err) {
					if err := os.WriteFile(goldenfile, got, 0644); err != nil {
						t.Fatal(err)
					}
					t.Logf("created golden file: %s", goldenfile)
					return
				}
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("%s: got: %s want: %s", name, string(got), string(want))
			}
		})
	}
}

func TestConvertOpenAlexWorkToFatcatRelease(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "openalex-*.input"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var work openalex.Work
			if err := json.Unmarshal(b, &work); err != nil {
				t.Fatal(err)
			}
			var release fatcat.Release
			if err := OpenAlexWorkToFatcatRelease(&work, &release); err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(release, "", "    ")
			if err != nil {
				t.Fatal(err)
			}
			goldenfile := filepath.Join("testdata", name+".golden")
			want, err := os.ReadFile(goldenfile)
			if err != nil {
				if os.IsNotExist(err) {
					if err := os.WriteFile(goldenfile, got, 0644); err != nil {
						t.Fatal(err)
					}
					t.Logf("created golden file: %s", goldenfile)
					return
				}
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("%s: got: %s want: %s", name, string(got), string(want))
			}
		})
	}
}

func ConvertOaiScrapeToFatcatRelease(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "openalex-*.input"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var work openalex.Work
			if err := json.Unmarshal(b, &work); err != nil {
				t.Fatal(err)
			}
			var release fatcat.Release
			if err := OpenAlexWorkToFatcatRelease(&work, &release); err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(release, "", "    ")
			if err != nil {
				t.Fatal(err)
			}
			goldenfile := filepath.Join("testdata", name+".golden")
			want, err := os.ReadFile(goldenfile)
			if err != nil {
				if os.IsNotExist(err) {
					if err := os.WriteFile(goldenfile, got, 0644); err != nil {
						t.Fatal(err)
					}
					t.Logf("created golden file: %s", goldenfile)
					return
				}
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("%s: got: %s want: %s", name, string(got), string(want))
			}
		})
	}
	OaiScrapeToFatcatRelease()
}
