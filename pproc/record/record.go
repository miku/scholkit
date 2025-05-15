package record

import (
	"errors"
	"io"
	"strings"
)

var (
	ErrMaxTokenSizeExceeded = errors.New("max token size exceeded")
	ErrInvalidSplitter      = errors.New("invalid splitter")
	errInvalidSplitterFunc  = func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		return 0, nil, ErrInvalidSplitter
	}
)

// xmlTagSplitter provides a split function that can be used in a bufio.Scanner
// to split on XML tag boundaries.
type xmlTagSplitter struct {
	tagName       string
	maxBufferSize int    // Threshold to start looking for complete tags
	maxTokenSize  int    // Hard limit for a single token
	buffer        []byte // Internal buffer
}

// TagSplitter returns a bufio.SplitFunc that will split an XML stream on
// elements of a given name, up to approximately maxBufferSize as a soft limit.
// If a single XML tag will span maxTokenSize as a hard limit, the split
// function will return an error.
func TagSplitter(tagName string, maxBufferSize, maxTokenSize int) func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(tagName) == 0 || maxBufferSize < 0 || maxTokenSize < 0 || maxTokenSize < maxBufferSize {
		return errInvalidSplitterFunc
	}
	splitter := &xmlTagSplitter{
		tagName:       tagName,
		maxBufferSize: maxBufferSize,
		maxTokenSize:  maxTokenSize,
	}
	return splitter.split
}

func (s *xmlTagSplitter) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// If we have a leftover buffer to process first
	if len(s.buffer) > 0 {
		start, end := findFirstCompleteTag(string(s.buffer), s.tagName)
		switch {
		case start == -1:
			// No valid start tag found in buffer: clear it and continue
			s.buffer = nil
		case end == -1:
			// Found start but no end in buffer, need more data
			if !atEOF {
				s.buffer = append(s.buffer, data...)
				if len(s.buffer) > s.maxTokenSize {
					return 0, nil, ErrMaxTokenSizeExceeded
				}
				return len(data), nil, nil
			} else {
				s.buffer = nil
				return len(data), nil, io.EOF
			}
		default:
			// Found complete tag in buffer
			token = s.buffer[start:end]
			s.buffer = s.buffer[end:]
			return 0, token, nil
		}
	}
	// If we arrive here, the buffer is empty so we work with incoming data
	if atEOF {
		if len(data) == 0 {
			return 0, nil, io.EOF
		}
		// Try to find a complete tag in the final data
		start, end := findFirstCompleteTag(string(data), s.tagName)
		if start == -1 || end == -1 {
			return len(data), nil, io.EOF
		}
		// Found a complete tag in the final data
		token = data[start:end]
		if end < len(data) {
			// Store remaining data in buffer for next call
			s.buffer = data[end:]
		}
		return len(data), token, nil
	}
	s.buffer = append(s.buffer, data...)
	if len(s.buffer) < s.maxBufferSize {
		return len(data), nil, nil
	}
	start, end := findFirstCompleteTag(string(s.buffer), s.tagName)
	if start == -1 {
		s.buffer = nil
		return len(data), nil, nil
	}
	if end == -1 {
		if len(s.buffer) > s.maxTokenSize {
			return len(data), nil, ErrMaxTokenSizeExceeded
		}
		return len(data), nil, nil
	}
	// Found complete tag
	token = s.buffer[start:end]
	s.buffer = s.buffer[end:]
	// We've processed the accumulated data, advance by the original data length
	return len(data), token, nil
}

func isValidTagTerminator(ch byte) bool {
	switch ch {
	case '>', ' ', '/', '\n', '\t', '\r':
		return true
	}
	return false
}

// findFirstCompleteTag finds the first complete tag of the given type
func findFirstCompleteTag(input string, tagName string) (start, end int) {
	var (
		openTag  = "<" + tagName
		closeTag = "</" + tagName + ">"
		i        = 0
	)
	for i < len(input) {
		openStart := strings.Index(input[i:], openTag)
		if openStart == -1 {
			return -1, -1
		}
		openStart += i
		// Check if it's a valid opening tag
		if tagNameEnd := openStart + len(openTag); tagNameEnd < len(input) {
			nextChar := input[tagNameEnd]
			if !isValidTagTerminator(nextChar) {
				i = openStart + 1
				continue
			}
		}
		// Find the end of the opening tag
		openEnd := strings.Index(input[openStart:], ">")
		if openEnd == -1 {
			return openStart, -1
		}
		openEnd += openStart
		// Check for self-closing tag
		if openEnd > 0 && input[openEnd-1] == '/' {
			return openStart, openEnd + 1
		}
		// Find matching closing tag
		var (
			depth = 1
			j     = openEnd + 1
		)
		for j < len(input) && depth > 0 {
			var (
				nextOpen  = strings.Index(input[j:], openTag)
				nextClose = strings.Index(input[j:], closeTag)
			)
			if nextClose == -1 {
				return openStart, -1
			}
			if nextOpen != -1 && nextOpen+j < nextClose+j {
				// Found another opening tag before closing tag
				nextOpen += j
				// Verify it's a valid opening tag
				if tagNameEnd := nextOpen + len(openTag); tagNameEnd < len(input) {
					nextChar := input[tagNameEnd]
					if isValidTagTerminator(nextChar) {
						depth++
						j = nextOpen + 1
						continue
					}
				}
				j = nextOpen + 1
			} else {
				// Found closing tag
				nextClose += j
				depth--
				if depth == 0 {
					return openStart, nextClose + len(closeTag)
				}
				j = nextClose + len(closeTag)
			}
		}
		// If we get here, we didn't find a complete element
		i = openEnd + 1
	}
	return -1, -1
}
