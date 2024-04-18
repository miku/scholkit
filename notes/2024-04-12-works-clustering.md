# works clustering

input: joint dataset of fatcat release entities, want: clustered version
(similar to google scholar "N versions"), no need to merge; just group

Currently, the input is about 845950447 records across 8 inputs:

```
$ ls -1 input/
arxiv.ndj.zst
crossref.ndj.zst
datacite.ndj.zst
dblp.ndj.zst
doaj.ndj.zst
oaiscrape.ndj.zst
openalex.ndj.zst
pubmed.ndj.zst
```

## dumb approach

* [ ] group first by DOI, there are overlaps
* [ ] group by other ids

We can use doi.duck database for that.

```
D select doi from crossref intersect select doi from openalex;
┌─────────────────────────────┐
│             doi             │
│           varchar           │
├─────────────────────────────┤
│ 10.1002/9781118164792       │
│ 10.1002/9781118164860.ch1   │
│ 10.1002/9781118164860.ch8   │
│            ·                │
│            ·                │
│            ·                │
│ 10.7748/paed.19.4.7.s14     │
│ 10.7748/paed.2.9.5.s9       │
├─────────────────────────────┤
│  146044108 rows (40 shown)  │
└─────────────────────────────┘
```

We would need one "own" id for each document. We could then try to just curate
a simple DB for the clustering, like `(cluster, id)` e.g. `(1, C123)`, `(1,
O123)`, ...

We start with an empty database.

First, take fatcat.ndj.zst and extract all ids into tables; source would be
nice as well.

```
source | id | doi | pmid | pmcid | arxiv | oai | any
```

Also, create a table per `match_key` algorithm.

```
id | match_key
```

Then find all groups and run a finer comparison, similar to citation graph
clustering. For that we would need a fast way to iterate over the keys. In
refcat, we use the "zipkey" structure for that. Can we generalize this a bit?

----

# clustering releases

Input size, 800M records, growing, should work with 1B or more records as well.
Should work on a single machine, with moderate RAM. Should support multiple
algorithms; precision first, recall second.

Can we use generic similitity indices like faiss or annoy for this task? Would
love to just dump (title, author, year) into a db and then do lookups. Would
work for updates as well, as this could be a some "API" over a file, etc.

## naive approach

* match key: normalize title then group by that
* [ex: DNB](https://wiki.dnb.de/download/attachments/146377939/2019-04-03_KIMWS19_GVI.pdf#page=14)

three match keys

```
1. Material:ISBN:Pubdate
2. Material:Author:Title:Pubdate:Publisher
3. Material:Author:Title:Pubdate
```

* solr: [https://solr.apache.org/guide/8_11/de-duplication.html](https://solr.apache.org/guide/8_11/de-duplication.html)

## numbers

we would need to index 1B docs, we could generate embeddings for the record

## faiss

* memory issues: https://github.com/facebookresearch/faiss/issues/3020


