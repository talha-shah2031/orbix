BINARY := bnx
PACKAGE := ./cmd/bnx
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run test lint bench vuln verify

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PACKAGE)

run:
	go run $(PACKAGE)

test:
	go test -race -cover ./...

lint:
	go vet ./...
	golangci-lint run ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

vuln:
	govulncheck ./...

verify:
	go mod verify
