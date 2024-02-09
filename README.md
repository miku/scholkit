# scholkit

Assorted utitlies around scholarly metadata.

## Pubmed Single File Dump

```shell
$ curl -sL "https://ftp.ncbi.nlm.nih.gov/pubmed/baseline/" | \
    pup 'a[href] text{}' \
    grep -o 'pubmed.*[.]xml[.]gz' | \
    awk '{print "https://ftp.ncbi.nlm.nih.gov/pubmed/baseline/"$0}' | \
    schol-agg-links -v | \
    zstd -T0 -c > pubmed-$(date +"%Y-%m-%d").xml.zst
```
