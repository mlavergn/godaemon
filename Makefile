###############################################
#
# Makefile
#
###############################################

.DEFAULT_GOAL := build

.PHONY: test

VERSION := 1.5.0

ver:
	@sed -i '' 's/^const Version = "[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}"/const Version = "${VERSION}"/' src/daemon/daemon.go

lint:
	golint .

build:
	go build ./...

demo: build
	go build -o demo cmd/demo.go

clean:
	rm -f demo

test: build
	go test -v -count=1 ./src/...

github:
	open "https://github.com/mlavergn/godaemon"

release:
	zip -r godaemon.zip LICENSE README.md Makefile cmd src
	hub release create -m "${VERSION} - GoDaemon" -a godaemon.zip -t master "v${VERSION}"
	open "https://github.com/mlavergn/godaemon/releases"

package:
	GOARCH=amd64 GOOS=linux go build -o demo cmd/demo.go
	zip -r demo-linux-amd64.zip LICENSE README.md demo
	GOARCH=arm GOARM=5 GOOS=linux go build -o demo cmd/demo.go
	zip -r demo-linux-arm.zip LICENSE README.md demo
	GOARCH=amd64 GOOS=darwin go build -o demo cmd/demo.go
	zip -r demo-darwin-amd64.zip LICENSE README.md demo
	hub release edit -m "" -a demo-linux-amd64.zip -a demo-darwin-amd64.zip -a demo-linux-arm.zip v${VERSION}
	open "https://github.com/mlavergn/godaemon/releases"
