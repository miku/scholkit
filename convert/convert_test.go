package convert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miku/scholkit/schema/crossref"
)

func TestConvertCrossref(t *testing.T) {
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
