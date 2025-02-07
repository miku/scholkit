# oaiscrape Q1 2025

We find about 477,368,047 end or record tags. The uncompressed XML file
contains 1.25TiB of metadata.

Conversion steps:

Tool: `dumprecords.go` to go from invalid concatenaded XML to a valid XML file
(albeit large), the `oaijson.go` to go from XML to JSON lines.

During `oaijson.go` we encountered a problem with conversion, some unexpected
"dump" element. So while we find 477M end of record markers, we only have
128,863,625 lines in the ndj file.

We retry with a byte counter to identify the data issue.

```
$ time zstdcat -T0 2025-01-16-metha-oai-dump-record.xml.zst | pv | ./oaijson | \
    zstd -c -T0 > 2025-01-16-metha-oai-dump-record-update-1.ndjson.zst
```
