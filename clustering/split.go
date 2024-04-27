package clustering

import (
	"bytes"
	"io"
)

// FieldSplitter applies a key func to each line and groups lines with the same
// key.
type FieldSplitter struct {
	KeyFunc    func(line []byte) []byte
	currentKey []byte
	buf        bytes.Buffer // batch buffer
}

// Split will group a bunch of lines that share the same key.
func (s *FieldSplitter) Split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF {
		return 0, nil, io.EOF
	}
	nextN := bytes.Index(data, []byte{10}) // newline
	if nextN == -1 {
		return len(data), nil, nil
	}
	return len(data), nil, nil
}
