SHELL := /bin/bash
TARGETS := urlstream strnorm bibconv cdxlookup clowder fcid fifi
MAKEFLAGS := --jobs=$(shell nproc)

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -o $@ $<

.PHONY: test
test:
	go test -cover ./...

.PHONY: clean
clean:
	rm -f $(TARGETS)

.PHONY: update-all-deps
update-all-deps:
	go get -u -v ./... && go mod tidy

