VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build web test run clean e2e screenshots docs-check

build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/talaia ./cmd/talaia

web:
	npm --prefix web ci
	npm --prefix web run build

test:
	go vet ./...
	go test -race ./...

run: build
	./bin/talaia

clean:
	rm -rf bin
	find web/dist -mindepth 1 ! -name .gitkeep -delete

e2e: build
	npm --prefix e2e ci
	npm --prefix e2e test

screenshots: build
	npm --prefix e2e ci
	npm --prefix e2e run screenshots

docs-check:
	python3 scripts/check_docs.py
