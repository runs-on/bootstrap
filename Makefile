PREVIOUS_TAG ?= $(shell git tag --sort=-v:refname | head -n 1)
TAG=v0.1.14

.PHONY: build test bump tag release

build:
	mkdir -p dist
	go build -o dist/bootstrap .

test:
	go test -v ./...

bump:
	gsed -i "s/$(PREVIOUS_TAG)/$(TAG)/g" README.md
	git diff --exit-code README.md || (git commit -m "Update README" README.md)
	git diff --exit-code Makefile || (git commit -m "Update Makefile" Makefile)
	git push origin main

tag: bump
	git diff --exit-code || (echo "Error: uncommitted changes detected" && exit 1)
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)

release: tag
	gh release create $(TAG) --generate-notes
