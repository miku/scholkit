# scholkit

```
   ,   ,
  /////|
 ///// |
|~~~|  |
|===|  |
|j  |  |
| g |  |
|  s| /
|===|/
'---'
```

Sketch project, assorted utitlies around scholarly metadata.

status: unstable, wip

## Try

```
$ git clone https://github.com/miku/scholkit.git
$ cd scholkit
$ make
```

This builds a couple of executables:

* catshape (data format conversions)
* urlstream (stream data from many urls)
* cdxlookup (ad-hoc cdx api lookup)
* strnorm (quick string normalization)

Example dataset to work with, e.g. convert arxiv to fatcat release:

```sh
$ curl -sL https://archive.org/download/arxiv-2024-02-15/arxiv-2024-02-15.xml.zst | \
    zstd -dc | \
    catshape -f arxiv
```

## Tools

### Conversions

We want conversions from various formats to one single format (e.g. release
entities). Source formats include:

* [x] crossref
* [x] datacite
* [x] pubmed
* [x] arxiv
* [x] oaiscrape
* [x] openalex
* [x] dblp

Target:

* [ ] fatcat entities (release, work, container, file, contrib, abstract)

For each format, try to find the smallest conversion unit, e.g. one record.
Then add convenience layers on top, e.g. for streams.

No bulk conversion should take longer than an 1 hour, roughly (slowest
currently is openalex - 250M records - which takes about 45 min).

### Clustering

Create a "works" view from releases.

### Misc

The `urlstream` utility streams content from multiple URLs to stdout. Can help
to create single file versions of larger datasets like pubmed, openalex, etc.

```
$ curl -s "https://www.gutenberg.org/browse/scores/top" | \
    grep -Eo "/ebooks/[0-9]+" | \
    awk '{print "https://gutenberg.org"$0".txt.utf-8"}' > top100.txt

$ urlstream < top100.txt > top100books.txt
```

## Notes

* [upstream id comparison](notes/2024-02-09-upstream-ids.md)
* [stub releases](notes/2024-03-15-first-stub-releases.md)
* [url list](notes/2024-03-27-url-list.md)

## TODO

* [x] map basic fields to fatcat release entities
* [ ] map all fields to fatcat release entities
* [ ] basic clustering algorithm

