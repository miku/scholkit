SHELL := /bin/bash
MAKEFLAGS := --jobs=$(shell nproc)
PKGNAME := scholkit

# the "sk" toolkit binaries to build
TARGETS := sk-cat \
		   sk-cdx \
		   sk-cluster \
		   sk-convert \
		   sk-feed \
		   sk-id \
		   sk-norm

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
	rm -f $(PKGNAME)_*deb

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

