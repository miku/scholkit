# Conversion test data

Add examples for inputs and expected outputs, suggested format:

* arxiv-2025-05-29-0000.input
* arxiv-2025-05-29-0000.golden

In general:

* SOURCE-DATE-ID.input
* SOURCE-DATA-ID.golden

## Misc

Random metha XML output (mostly oai-dc), to records, then split into separate
files.

    $ zstdcat 2021-10-31-00000000.xml.zst | \
        sk-oai-records | \
        awk 'BEGIN{RS="\x1E"} {print > "oaiscrape-xml-fatcat-2025-05-01-" sprintf("%04d", NR-1) ".input"}'

    $ for file in oaiscrape-fatcat-2025-05-01-*.txt; do
        xmllint --format "$file" | sponge "$file"
    done
