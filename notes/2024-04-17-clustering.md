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


