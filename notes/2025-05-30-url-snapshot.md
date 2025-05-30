# 05/2025 url snapshot

from oaiscrape: 228283455

sample of 3000 against CDX:

* 1511 no capture
* 1488 at least one capture

From no capture:

$ jq -rc 'select(.count==1) | .url' 2025-05-01-url-cdxlookup-sample-3000.jsonl  | awk -F / '{print $3}'| sort | uniq -c | sort -nr | head -30
    123 doi.org
     84 hdl.handle.net
     60 hispana.mcu.es
     33 arxiv.org
     32 hal.science
     29 dx.doi.org
     18 www.redalyc.org
     17 gallica.bnf.fr
     11 www.sciencedirect.com
     11 lawcat.berkeley.edu
     10 www.scielo.br
     10 publications.rwth-aachen.de
     10 espace.library.uq.edu.au
     10 escholarship.org
      9 texashistory.unt.edu
      9 dialnet.unirioja.es
      9 curis.ku.dk
      7 www.mdpi.com
      7 www.biodiversitylibrary.org
      7 shs.hal.science
      7 hal.inrae.fr
      7 doi.pangaea.de
      6 www.ajol.info
      6 treatment.plazi.org
      6 theses.hal.science
      6 t2r2.star.titech.ac.jp
      6 media.hal.science
      6 juser.fz-juelich.de
      6 inria.hal.science
      6 dspace.nplg.gov.ge

