BINARY := dist/monarch-mcp.exe
GO ?= $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)

-include .env.local
export

.PHONY: build clean push

build:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.monarchToken=$(MONARCH_TOKEN)" -o $(BINARY) .

clean:
	rm -rf dist/

push: build
	mv $(BINARY) /mnt/mcp/

