VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build web test run clean

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
