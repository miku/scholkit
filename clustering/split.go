package clustering

import (
	"bytes"
	"io"
	"reflect"
)

// FieldSplitter applies a key func to each line and groups lines with the same
// key.
type FieldSplitter struct {
	// KeyFunc extracts a key from a line.
	KeyFunc func(line []byte) []byte
	// currentKey holds the currently found key or nil.
	currentKey []byte
	// buf is an internal buffer for accumulating lines
	buf bytes.Buffer
}

// Split will group a bunch of lines that share the same key.
func (s *FieldSplitter) Split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF {
		return 0, nil, io.EOF
	}
	// We at most read one line.
	if idx := bytes.Index(data, []byte{10}); idx != -1 {
		var (
			line = data[:idx]
			key  = s.KeyFunc(line)
		)
		switch {
		case s.currentKey == nil:
			// This is the first line, we add that in any case.
			_, _ = s.buf.Write(data[:idx])
		case reflect.DeepEqual(s.currentKey, key):
			// Keep this line.
			_, _ = s.buf.Write(data[:idx]) // next line
		default:
			// Key change, emit the current buffer.
		}
	}
	return len(data), nil, nil
}
