# Contributing

Use Go 1.27.3 or newer and Conventional Commits. Before committing, run:

```console
go mod verify
go vet ./...
golangci-lint run ./...
go test -race ./...
```

Library code must return errors instead of panicking. Do not add testnet URLs, secret flags, or any endpoint that can mutate a Binance account.

New REST routes require an explicit reviewed entry in `internal/binance/endpoints`; the registry is intentionally GET-only. Keep exact endpoint behavior and representative fixtures in tests. Never place secrets in config files, fixtures, or diagnostics archives. Run `make bench` for changes to decoding, order-book updates, or rendering hot paths.
