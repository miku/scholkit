SHELL := /bin/bash
TARGETS := sk-cat sk-norm sk-convert cdxlookup sk-cluster sk-id sk-feed
MAKEFLAGS := --jobs=$(shell nproc)
PKGNAME := scholkit

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
	rm -f scholkit_*deb

.PHONY: update-all-deps
update-all-deps:
	go get -u -v ./... && go mod tidy

.PHONY: deb
deb: $(TARGETS)
	mkdir -p packaging/deb/$(PKGNAME)/usr/local/bin
	cp $(TARGETS) packaging/deb/$(PKGNAME)/usr/local/bin
	mkdir -p packaging/deb/$(PKGNAME)/usr/lib/systemd/system
	cd packaging/deb && fakeroot dpkg-deb --build $(PKGNAME) .
	mv packaging/deb/$(PKGNAME)_*.deb .

