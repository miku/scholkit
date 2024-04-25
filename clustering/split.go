package clustering

import "bytes"

// FieldSplitter applies a key func to each line and groups lines with the same
// key.
type FieldSplitter struct {
	CurrentKey []byte
	KeyFunc    func(line []byte) []byte
	buf        bytes.Buffer // batch buffer
}

// Split will group a bunch of lines that share the same key.
func (s *FieldSplitter) Split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	nextN := bytes.Index(data, byte{10}) // newline
	if nextN == -1 {
		return len(data), nil, nil
	}
}
