# europepmc

https://europepmc.org/downloads

>  Nearly 7.1 million full text open access articles available to download using the Europe PMC web services or FTP site.

Metadata: https://europepmc.org/ftp/pmclitemetadata/PMCLiteMetadata.tgz

* [https://europepmc.org/docs/Information_poster_Europe_PMC_vs_PubMed_and_PMC.pdf](https://europepmc.org/docs/Information_poster_Europe_PMC_vs_PubMed_and_PMC.pdf)

> Europe PMC has more content.

> PubMed comprises more than 25 million abstracts for biomedical literature.
> PMC is a full-text archive of over 3 million biomedical and life sciences
> journal articles.

> Europe PMC provides a single point of access to the abstracts available
> through PubMed, PMC full-text articles, and an additional 5 million other
> relevant resources, such as patent records.

----

```
$ curl -sL https://europepmc.org/ftp/oa/ | pup 'td > a[href^=PMC] text{}' | awk '{print "https://europepmc.org/ftp/oa/"$0}' > europepmc_ftp_oa_urls.txt
$ wc -l europepmc_ftp_oa_urls.txt
1146 europepmc_ftp_oa_urls.txt
$ head europepmc_ftp_oa_urls.txt
https://europepmc.org/ftp/oa/PMC13900_PMC17829.xml.gz
https://europepmc.org/ftp/oa/PMC25774_PMC29831.xml.gz
https://europepmc.org/ftp/oa/PMC30703_PMC37548.xml.gz
https://europepmc.org/ftp/oa/PMC45583_PMC48155.xml.gz
https://europepmc.org/ftp/oa/PMC55319_PMC60000.xml.gz
https://europepmc.org/ftp/oa/PMC60001_PMC65704.xml.gz
https://europepmc.org/ftp/oa/PMC77373_PMC79679.xml.gz
https://europepmc.org/ftp/oa/PMC80153_PMC89010.xml.gz
https://europepmc.org/ftp/oa/PMC90187_PMC99087.xml.gz
https://europepmc.org/ftp/oa/PMC100320_PMC107849.xml.gz
```

Crawl the page (metadata, XML):

```
$ wget -rkc https://europepmc.org/ftp/oa/
```

Or archive into a WARC:

```
$ wget -rkc https://europepmc.org/ftp/oa/
```

Fulltext:

```
$ wget -rkc https://europepmc.org/ftp/pdf/
```

Fulltext is in https://europepmc.org/ftp/pdf/PMCxxxx001/PMC6840001.zip files,
just a PDF, zipped.
