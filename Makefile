IMAGE ?= luukevin87/monarch-mcp
BINARY := monarch-mcp
GO ?= $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)

-include .env.local
export

.PHONY: build clean run docker-build docker-clean docker-push docker-run

build:
	$(GO) build -ldflags="-s -w" -o $(BINARY) .

run: clean build
	./$(BINARY)

clean:
	rm -f $(BINARY)

docker-build:
	docker build -t $(IMAGE):latest .

docker-clean:
	docker rmi $(IMAGE):latest

docker-push: docker-build
	docker push $(IMAGE):latest

docker-run:
	docker run -d -p $(PORT):$(PORT) -e MONARCH_TOKEN=$(MONARCH_TOKEN) -e OAUTH_CLIENT_SECRET=$(OAUTH_CLIENT_SECRET) -e PORT=$(PORT) -e BASE_URL=$(BASE_URL) $(IMAGE):latest
