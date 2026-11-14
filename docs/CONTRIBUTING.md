# Contributing

Use Go 1.27.3 or newer and Conventional Commits. Start from `dev`, keep changes focused, and open pull requests against `main` only when they are release-ready.

Install the local CI tools once:

```console
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
go install golang.org/x/vuln/cmd/govulncheck@latest
```

Before committing, run:

```console
go mod verify
go vet ./...
golangci-lint run ./...
go test -race ./...
```

Or run the equivalent Make target:

```console
make ci
```

Library code must return errors instead of panicking. Do not add testnet URLs, secret flags, or any endpoint that can mutate a Binance account.

New REST routes require an explicit reviewed entry in `internal/binance/endpoints`; the registry is intentionally GET-only. Keep exact endpoint behavior and representative fixtures in tests. Never place secrets in config files, fixtures, or diagnostics archives. Run `make bench` for changes to decoding, order-book updates, or rendering hot paths.

Changes to terminal rendering must remain usable in narrow terminals and on Windows Terminal, macOS Terminal, and common Linux terminals. Changes to packaging must preserve all six targets documented in [RELEASING.md](RELEASING.md).
