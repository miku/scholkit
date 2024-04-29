package clustering

import (
	"bufio"
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func keyFuncFromColumnIndex(col int) func(b []byte) []byte {
	return func(b []byte) []byte {
		fields := bytes.Fields(b)
		if col <= len(fields) {
			return fields[col-1]
		}
		return nil
	}
}

var emptyKeyFunc = func([]byte) []byte { return nil }

func TestFieldSplitter(t *testing.T) {
	var cases = []struct {
		help    string
		keyFunc func([]byte) []byte
		input   string
		result  []string
		err     error
	}{
		{"empty string yields nil", emptyKeyFunc, "", nil, nil},
		// {"single value, single line", keyFuncFromColumnIndex(1), "a\n", []string{"a\n"}, nil},
	}
	for _, c := range cases {
		fs := FieldSplitter{KeyFunc: c.keyFunc}
		scanner := bufio.NewScanner(strings.NewReader(c.input))
		scanner.Split(fs.Split)
		var result []string
		for scanner.Scan() {
			result = append(result, scanner.Text())
		}
		if scanner.Err() != c.err {
			t.Fatalf("got %v, want %v", scanner.Err(), c.err)
		}
		if !reflect.DeepEqual(result, c.result) {
			t.Fatalf("got %v, want %v", result, c.result)
		}
	}
}
