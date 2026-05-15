BINARY_WIN := dist/monarch-mcp.exe
BINARY_LINUX := dist/monarch-mcp
GO ?= $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)

-include .env.local
export

.PHONY: build clean push

build:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.monarchToken=$(MONARCH_TOKEN)" -o $(BINARY_WIN) .
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.monarchToken=$(MONARCH_TOKEN)" -o $(BINARY_LINUX) .

clean:
	rm -rf dist/

push: build
	mv $(BINARY_LINUX) /mnt/mcp/

