BINARY := orbix
PACKAGE := ./cmd/orbix
ifeq ($(OS),Windows_NT)
GO ?= $(if $(wildcard .tools/go/bin/go.exe),.tools/go/bin/go.exe,go)
BINARY_EXT := .exe
COMMIT ?= local
DATE ?= unknown
else
GO ?= go
BINARY_EXT :=
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
endif
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run test lint bench vuln verify

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY)$(BINARY_EXT) $(PACKAGE)

run:
	$(GO) run $(PACKAGE)

test:
	$(GO) test -race -cover ./...

lint:
	$(GO) vet ./...
	golangci-lint run ./...

bench:
	$(GO) test -run '^$$' -bench . -benchmem ./...

vuln:
	govulncheck ./...

verify:
	$(GO) mod verify
