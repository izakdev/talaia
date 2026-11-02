VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build web test run clean e2e screenshots docs-check dist test-installer

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

# Binarios para Linux y sus sumas de comprobación, como en una versión publicada.
dist: web
	rm -rf dist
	mkdir -p dist
	for arch in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/talaia-linux-$$arch ./cmd/talaia; \
	done
	cd dist && (command -v sha256sum >/dev/null && sha256sum talaia-linux-* || shasum -a 256 talaia-linux-*) > SHA256SUMS

test-installer:
	shellcheck scripts/talaia-proxmox.sh scripts/test/*.sh scripts/test/stubs/*
	scripts/test/proxmox-integration.sh

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
