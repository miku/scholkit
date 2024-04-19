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


----

## 2024-02-18

Take releases file and create tabular version.

* line number, source, title normalized, doi, other ids ...

----

We could:

* try to find a way to turn parts of our JSON docs into an d-dimensional vector
  and then index that into e.g. annoy
* what kind of embedding should we use?
* similar docs should be have small distance

Could use something like this:

* [https://github.com/typesense/showcase-ecommerce-store/blob/master/scripts/vector-generation/main.py](https://github.com/typesense/showcase-ecommerce-store/blob/master/scripts/vector-generation/main.py)
* [https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2)

Boils down to SBERT: [https://www.sbert.net](https://www.sbert.net)

* sentence transformer has a limit of 256 tokens
* [https://www.reddit.com/r/LocalLLaMA/comments/176u53g/is_the_075_tokens_per_word_rule_of_thumb_general/](https://www.reddit.com/r/LocalLLaMA/comments/176u53g/is_the_075_tokens_per_word_rule_of_thumb_general/)

Let's see how long the titles of the records are on average. We would need
title, author, embeddings. If we are above 192 words too often, we cannot use
this approach.

In order to see, whether we get some sensible results, we could try to
calculate embeddings for arxiv; and then see, whether we are getting all the
different versions of a paper; most likely they are almost exact matches.

We could distort the data to get some better idea how far an embedding goes.
Could try to evaluate a list of embeddings for this task.

Summing a file with 10M integers, one per line?

```
$ paste -sd+ 10M.len | bc
712219434
```

On average 71 bytes. But that is just the title.

```
In [7]: df.describe()
Out[7]:
               92
count 9999999.000
mean       71.222
std        49.205
min         3.000
25%        39.000
50%        66.000
75%        94.000
max     20612.000
```

However, we would need to skip about 5% of the data:

```
In [15]: df.quantile([0.5, 0.9, 0.95, 0.98, 0.99, 0.999, 0.9999])
Out[15]:
           92
0.500  66.000
0.900 124.000
0.950 147.000
0.980 183.000
0.990 217.000
0.999 387.000
1.000 898.000
```

Try to match by ID first.

Extract table from combined data, `zstdcat fatcat.ndj.zst | clowder -T >
fatcat.tsv` - should list all ids and a normalized title.

