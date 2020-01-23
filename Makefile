###############################################
#
# Makefile
#
###############################################

.DEFAULT_GOAL := build

.PHONY: test

VERSION := 1.3.1

ver:
	@sed -i '' 's/^const Version = "[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}"/const Version = "${VERSION}"/' src/daemon/daemon.go

lint:
	golint .

build:
	go build ./...

demo: build
	go build -o demo cmd/demo.go
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
