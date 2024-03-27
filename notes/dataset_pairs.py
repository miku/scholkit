#!/usr/bin/env python
#
# Report DOI overlaps between a number of large bibliographic data sources.
#
# 8 datasets, 28 combinations
#
# 614,536,131 identifiers in total
#
# Creating the duck database (5.6GB) takes a while, but queries are fast, and
# cpu-intensive. Each count corresponds to a query. This is for DOI comparisons
# only. The oaiscrape may have many invalid DOI and report a higher number of
# unique things that there actually are.
#
# (.venv) tir@k9:/var/data/share $ time python ~/code/miku/scholkit/notes/dataset_pairs.py | column -t
# fatcat     crossref   183_017_029  153_842_785  134_042_379  48_974_650   19_800_406
# fatcat     datacite   183_017_029  53_335_897   40_322_809   142_694_220  13_013_088
# fatcat     dblp       183_017_029  5_898_952    5_416_373    177_600_656  482_579
# fatcat     doaj       183_017_029  8_073_056    6_957_139    176_059_890  1_110_538
# fatcat     oaiscrape  183_017_029  26_917_651   21_124_085   161_892_944  5_793_566
# fatcat     openalex   183_017_029  154_390_913  136_692_169  46_324_860   17_698_744
# fatcat     pubmed     183_017_029  29_059_848   27_730_123   155_286_906  1_329_719
# crossref   datacite   153_842_785  53_335_897   89_895       153_752_890  53_246_002
# crossref   dblp       153_842_785  5_898_952    5_597_997    148_244_788  300_955
# crossref   doaj       153_842_785  8_073_056    7_537_060    146_305_725  530_617
# crossref   oaiscrape  153_842_785  26_917_651   19_454_526   134_388_259  7_463_125
# crossref   openalex   153_842_785  154_390_913  146_044_108  7_798_677    8_346_805
# crossref   pubmed     153_842_785  29_059_848   28_590_950   125_251_835  468_892
# datacite   dblp       53_335_897   5_898_952    225_137      53_110_760   5_673_815
# datacite   doaj       53_335_897   8_073_056    125_722      53_210_175   7_941_955
# datacite   oaiscrape  53_335_897   26_917_651   3_071_013    50_264_884   23_846_638
# datacite   openalex   53_335_897   154_390_913  3_333_986    50_001_911   151_056_927
# datacite   pubmed     53_335_897   29_059_848   10_995       53_324_902   29_048_847
# dblp       doaj       5_898_952    8_073_056    344_904      5_554_048    7_722_773
# dblp       oaiscrape  5_898_952    26_917_651   1_211_927    4_687_025    25_705_724
# dblp       openalex   5_898_952    154_390_913  5_805_104    93_848       148_585_809
# dblp       pubmed     5_898_952    29_059_848   413_802      5_485_150    28_646_040
# doaj       oaiscrape  8_073_056    26_917_651   5_088_632    2_979_045    21_829_019
# doaj       openalex   8_073_056    154_390_913  7_901_602    166_075      146_489_311
# doaj       pubmed     8_073_056    29_059_848   3_445_970    4_621_707    25_613_872
# oaiscrape  openalex   26_917_651   154_390_913  20_514_042   6_403_609    133_876_871
# oaiscrape  pubmed     26_917_651   29_059_848   6_490_899    20_426_752   22_568_943
# openalex   pubmed     154_390_913  29_059_848   28_854_453   125_536_460  205_389
#
# real    3m22.630s # 32-core i9-13900T
# user    60m51.817s
# sys     5m31.354s


import itertools
import duckdb

# set paths here, all files contain one DOI per line from that dataset; files
# required some sorting, uniq-ing and lowercasing

fileset = {
    "crossref": "/var/data/share/crossref/crossref-2024-01-01-doi-sorted-lower.tsv.zst",
    "datacite": "/var/data/share/datacite/datacite-2024-01-26-doi.tsv.zst",
    "dblp": "/var/data/share/dblp/dblp-2024-02-01-doi.tsv.zst",
    "doaj": "/var/data/share/doaj/doaj-oaipmh-2024-01-01-doi-lower.tsv.zst",
    "oaiscrape": "/var/data/share/oaiscrape/2023-11-01-metha-oai-doi.ndjson.zst",
    "openalex": "/var/data/share/openalex-works/openalex-works-2024-02-09-doi-sorted-lower.tsv.zst",
    "pubmed": "/var/data/share/pubmed/pubmed-2024-02-09-doi-lower.tsv.zst",
    "fatcat": "/var/data/share/fatcat/release_doi.tsv.zst",
}

dbfile = "/var/data/share/doi.duck"

# "fatcat",     # 183_017_029
# "crossref",   # 153_842_785
# "datacite",   #  53_335_897
# "dblp",       #   5_898_952
# "doaj",       #   8_073_056
# "oaiscrape",  #  26_917_651
# "openalex",   # 154_390_913
# "pubmed",     #  29_059_848

if __name__ == '__main__':
    con = duckdb.connect(dbfile)
    for name, path in fileset.items():
        con.execute(
            """
            create table if not exists %s as
            select * from read_csv('%s',
                sep="\t",
                compression="zstd",
                names=['doi'],
                columns={'doi': 'VARCHAR'})
            """
            % (name, path))

    # for name in fileset.keys():
    #     result = con.execute(""" select count(*) from %s """ % name).fetchone()[
    #         0
    #     ]  # (153842785,)
    #     print("\t".join([name, str(result)]))

    for a, b in itertools.combinations(fileset.keys(), r=2):
        q = """ select count(*) from %s """ % (a,)
        a_count = con.execute(q).fetchone()[0]
        q = """ select count(*) from %s """ % (b,)
        b_count = con.execute(q).fetchone()[0]
        q = """ select count(*) from (select doi from %s intersect select doi from %s) """ % (a, b)
        intersect_count = con.execute(q).fetchone()[0]
        q = """ select count(*) from (select doi from %s except select doi from %s) """ % (a, b)
        aonly_count = con.execute(q).fetchone()[0]
        q = """ select count(*) from (select doi from %s except select doi from %s) """ % (b, a)
        bonly_count = con.execute(q).fetchone()[0]
        values = [a, b, f'{a_count:_}', f'{b_count:_}', f'{intersect_count:_}', f'{aonly_count:_}', f'{bonly_count:_}']
        print("\t".join(values))

