// sk-oai-records was used as a first step to go from concatenated metha OAI
// XML file to more valid XML. The issue was that there was an imbalance of
// opening and closing record tags in the raw concatenated data; ultimalely
// conversions failed after 128M record, while the there were about 477M
// opening/closing tags in the data.
//
// This script finds a record and writes it out, and adding "RECORD SEPARATOR"
// (https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators) in
// between.
package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
)

var (
	start = []byte("<record ")
	end   = []byte("</record>")
	sep   = []byte{'\n', 30, '\n'}
)

func main() {
	var (
		br        = bufio.NewReader(os.Stdin)
		readBuf   = make([]byte, 8192)
		buf       []byte
		insideTag = false
		bw        = bufio.NewWriter(os.Stdout)
	)
	defer bw.Flush()
LOOP:
	for {
		n, err := br.Read(readBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		buf = append(buf, readBuf[:n]...)
		for len(buf) > 0 {
			if !insideTag {
				idx := bytes.Index(buf, start)
				if idx == -1 {
					buf = buf[0:0]
					continue LOOP
				}
				insideTag = true
				buf = buf[idx:]
			} else {
				idx := bytes.Index(buf, end)
				if idx == -1 {
					continue LOOP
				}
				record := buf[:idx+len(end)]
				bw.Write(record)
				bw.Write(sep)
				buf = buf[idx+len(end):]
				insideTag = false
			}
		}
	}
}
