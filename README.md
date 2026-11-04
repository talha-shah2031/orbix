# BNX

`bnx` is a keyboard-driven terminal dashboard and scriptable CLI for **Binance mainnet**. It provides public Spot, USDⓈ-M, COIN-M, and basic Options market data, plus optional read-only account monitoring.

## Safety

- Mainnet only. There are no testnet URLs or code paths.
- Read-only by design: no order placement, edit, or cancellation; no withdrawal, transfer, or margin-borrow operations.
- Use a dedicated API key with **Enable Reading** only and IP restrictions. Never pass API secrets as command-line flags.
- Credentials use the operating system keychain, with `BNX_API_KEY` and `BNX_API_SECRET` environment variables as fallback.
- No telemetry phone-home. Network calls go to Binance endpoints only.

See [SECURITY.md](docs/SECURITY.md) for the threat model and [RELEASING.md](docs/RELEASING.md) for maintainers preparing a release.

## Install

### Download a release

When a release is published, download the archive for your operating system and CPU architecture from the repository's **GitHub Releases** page. Extract it, then place `bnx` (or `bnx.exe` on Windows) somewhere on your `PATH`. Verify the archive with `checksums.txt` before use. Release assets include Linux, macOS, and Windows builds for amd64 and arm64.

On Linux or macOS, this `curl` installer detects the OS and architecture, downloads the latest release, verifies its SHA-256 checksum, and installs the binary under `~/.local/bin`:

```sh
set -eu
REPO="talha-shah2031/BNX"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac
TAG="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
if [ -z "$TAG" ]; then echo "No published BNX release found" >&2; exit 1; fi
VERSION="${TAG#v}"
ARCHIVE="bnx_${VERSION}_${OS}_${ARCH}.tar.gz"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
BASE="https://github.com/$REPO/releases/download/$TAG"
curl -fsSL "$BASE/$ARCHIVE" -o "$TMP/$ARCHIVE"
curl -fsSL "$BASE/checksums.txt" -o "$TMP/checksums.txt"
grep -F "  $ARCHIVE" "$TMP/checksums.txt" > "$TMP/checksum.txt"
(cd "$TMP" && if command -v sha256sum >/dev/null 2>&1; then sha256sum -c checksum.txt; else shasum -a 256 -c checksum.txt; fi)
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
mkdir -p "$HOME/.local/bin"
install -m 0755 "$TMP/bnx" "$HOME/.local/bin/bnx"
echo "Installed at $HOME/.local/bin/bnx; ensure ~/.local/bin is on your PATH."
```

This installer requires `curl`, `tar`, and either `sha256sum` or `shasum`. Releases are created from version tags; if no release has been published yet, use the source build below.

### Build from source

Requires Go 1.24 or newer. From the repository root:

```sh
go mod download
go build -trimpath -o bin/bnx ./cmd/bnx
./bin/bnx version
```

Windows PowerShell:

```powershell
go mod download
go build -trimpath -o bin/bnx.exe ./cmd/bnx
.\bin\bnx.exe version
```

With GNU Make installed, `make build` creates `bin/bnx` (Windows users can build directly with the PowerShell commands above).

## Quick start

Launching with no arguments opens the full-screen TUI:

```sh
bnx
bnx tui --symbol BTCUSDT
```

If running the freshly built binary from this repository, use `./bin/bnx` on macOS/Linux or `.\bin\bnx.exe` in PowerShell.

Public CLI examples:

```sh
bnx price BTCUSDT --output json
bnx ticker BTCUSDT --window 1h
bnx book BTCUSDT --depth 20
bnx trades BTCUSDT --limit 50 --output csv
bnx klines BTCUSDT --interval 1h --limit 100 --output ndjson
bnx info BTCUSDT --output yaml
bnx time
bnx funding BTCUSDT --market usdm
bnx oi BTCUSDT --market usdm
bnx stream trade BTCUSDT
```

Supported output formats are `table`, `json`, `ndjson`, `csv`, and `yaml`. Data goes to stdout; diagnostics go to stderr. Use `--help` on the root command or any subcommand for available flags.

## TUI controls

Keys `1`–`8` switch views. In Markets, use `/` to search, `j`/`k` or arrows to move, `s` to sort, `f` to favorite, and `enter` to open a symbol. On the Symbol view, `[` and `]` change interval, `+` and `-` zoom, and `a` toggles indicator summaries. `m` changes market, `t` changes theme, `r` refreshes, `:` or `Ctrl+P` opens the command palette, and `?` opens help. Press `q` or `Ctrl+C` to quit.

The dashboard includes market breadth, top movers, volume leaders, and sparklines. Markets supports filtering, sorting, pagination, and watchlists. Symbol detail combines candles, order book, recent trades, and indicators. Futures, alerts, diagnostics, and help are available in their numbered views. Account data is display-only.

## Configuration and account access

`bnx config init` creates the local configuration interactively. `bnx config show` prints the effective configuration with secrets masked. Configuration is stored in the platform's user configuration directory; typical Unix systems use `~/.config/bnx/config.yaml`. Precedence is command-line flags, `BNX_*` environment variables, config file, then defaults. Keep config files private; credentials belong in the OS keychain or environment, not YAML.

Public market data works without credentials. To enable account monitoring, run `bnx auth login` in an interactive terminal or set `BNX_API_KEY` and `BNX_API_SECRET` in the environment. Then:

```sh
bnx auth status
bnx account balances --output json
bnx account orders
bnx account orders --history BTCUSDT --limit 100
bnx account trades BTCUSDT --limit 100
bnx account positions --market usdm
bnx account income --market usdm --limit 100
```

Signed requests are HMAC-SHA256 and pass through the same GET-only endpoint allowlist. Futures account events can update live; Spot account data refreshes over REST. For details, see [SECURITY.md](docs/SECURITY.md).

## Local data and diagnostics

SQLite stores watchlists, alerts, cache entries, and UI preferences in the platform configuration directory. If the database is unavailable, market views continue without persistence. Alerts can be managed with `bnx alerts` and watchlists with `bnx watchlist`. `bnx doctor` checks configuration, keyring availability, and Spot connectivity. `bnx diagnostics --file diagnostics.zip` exports a redacted diagnostic bundle; it excludes credentials, account records, configuration files, and the local database.

Generate shell completions with `bnx completion bash`, `zsh`, `fish`, or `powershell`.

## Development

```sh
go test ./...
go vet ./...
golangci-lint run ./...
```

See [ARCHITECTURE.md](docs/ARCHITECTURE.md), [DECISIONS.md](docs/DECISIONS.md), and [CONTRIBUTING.md](docs/CONTRIBUTING.md). CI runs checks on Linux, macOS, and Windows. Release builds are prepared from version tags; see [RELEASING.md](docs/RELEASING.md). No release is published by these local setup instructions.
