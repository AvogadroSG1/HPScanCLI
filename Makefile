VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test lint clean release

build:
	go build -ldflags "$(LDFLAGS)" -o hpscan ./cmd/hpscan

test:
	go test ./...

lint:
	gofmt -l .
	go vet ./...

clean:
	rm -f hpscan hpscan.exe
	rm -rf dist/

release:
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-linux-amd64       ./cmd/hpscan
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-darwin-arm64       ./cmd/hpscan
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-windows-amd64.exe  ./cmd/hpscan
