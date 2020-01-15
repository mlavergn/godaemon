###############################################
#
# Makefile
#
###############################################

.DEFAULT_GOAL := build

.PHONY: test

VERSION := 1.0.0

lint:
	golint .

build:
	go build ./...

demo: build
	go build -o demo cmd/demo.go
	cp demo test
	./demo

clean:
	rm -f demo

test: build
	go test -v ./src/...

github:
	open "https://github.com/mlavergn/godaemon"

release:
	zip -r godaemon.zip LICENSE README.md Makefile cmd src
	hub release create -m "${VERSION} - GoDaemon" -a godaemon.zip -t master "v${VERSION}"
	open "https://github.com/mlavergn/godaemon/releases"
