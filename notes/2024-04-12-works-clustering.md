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

