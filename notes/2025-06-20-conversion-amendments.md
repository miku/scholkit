# crossref

* contains license data, e.g. for TDM (ignored)
* is-referenced-by count (ignored)
* authors can have a sequence field: "first", "additional", in fatcat we have an index
* container-title; is put into an extra.crossref.subject field, but that should be a container title
* there is an ISBN field, which can contain multiple ISBN, that belong to the container => extra.crossref.isbn[]
* there is a URL field => extra.crossref.urls[]
* we need an issn field?

maybe need to run work => release and work => container conversions
