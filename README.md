# ORBIX

`orbix` is a keyboard-driven terminal dashboard and scriptable CLI. Its Spot Market heatmap supports public Binance, WEEX, and MEXC ticker data; the CLI also provides Binance Spot, USDⓈ-M, COIN-M, and basic Options market data, plus optional read-only Binance account monitoring.

## Safety

- Mainnet only. There are no testnet URLs or code paths.
- Read-only by design: no order placement, edit, or cancellation; no withdrawal, transfer, or margin-borrow operations.
- Use a dedicated API key with **Enable Reading** only and IP restrictions. Never pass API secrets as command-line flags.
- Credentials use the operating system keychain, with `ORBIX_API_KEY` and `ORBIX_API_SECRET` environment variables as fallback.
- No telemetry phone-home. The Spot heatmap contacts only the selected exchange's public ticker API (Binance, WEEX, or MEXC); the other CLI and account features use Binance endpoints.

See [SECURITY.md](docs/SECURITY.md) for the threat model and key setup guidance.

## Install

### Download the Windows executable

Each version tag creates a GitHub Release with one downloadable Windows x64 executable, `orbix.exe`. After a release is available, install it from PowerShell with:

```powershell
$installDir = Join-Path $env:LOCALAPPDATA 'Programs\orbix'
New-Item -ItemType Directory -Force $installDir | Out-Null
curl.exe -fL 'https://github.com/talha-shah2031/orbix/releases/latest/download/orbix.exe' -o (Join-Path $installDir 'orbix.exe')
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
}
Write-Host "Installed to $installDir. Open a new terminal and run: orbix"
```

If no release has been published yet, build from source using the instructions below.

### Build from source

Requires Go 1.27.3 or newer. From the repository root:

```sh
go mod download
go build -trimpath -o bin/orbix ./cmd/orbix
./bin/orbix version
```

Windows PowerShell:

```powershell
go mod download
go build -trimpath -o bin/orbix.exe ./cmd/orbix
.\bin\orbix.exe version
```

With GNU Make installed, `make build` creates `bin/orbix` (Windows users can build directly with the PowerShell commands above).

## Quick start

Launching with no arguments opens the full-screen TUI:

```sh
orbix
orbix tui --symbol BTCUSDT
```

If running the freshly built binary from this repository, use `./bin/orbix` on macOS/Linux or `.\bin\orbix.exe` in PowerShell.

Public CLI examples:

```sh
orbix price BTCUSDT --output json
orbix ticker BTCUSDT --window 1h
orbix book BTCUSDT --depth 20
orbix trades BTCUSDT --limit 50 --output csv
orbix klines BTCUSDT --interval 1h --limit 100 --output ndjson
orbix info BTCUSDT --output yaml
orbix time
orbix funding BTCUSDT --market usdm
orbix oi BTCUSDT --market usdm
orbix stream trade BTCUSDT
```

Supported output formats are `table`, `json`, `ndjson`, `csv`, and `yaml`. Data goes to stdout; diagnostics go to stderr. Use `--help` on the root command or any subcommand for available flags.

## TUI controls

Keys `1`–`8` switch views. In Markets, use `/` to search, `j`/`k` or arrows to move, `s` to sort, `f` to favorite, and `enter` to open a symbol. On the Symbol view, `[` and `]` change interval, `+` and `-` zoom, and `a` toggles indicator summaries. `m` changes market, `t` changes theme, `r` refreshes, `:` or `Ctrl+P` opens the command palette, and `?` opens help. Press `q` or `Ctrl+C` to quit.

The TUI opens directly to Spot Market. Press `e` to choose Binance, WEEX, or MEXC; `q` cycles quote assets and `[` / `]` changes the 100-pair page. Pairs are ranked by 24-hour base-asset volume, while tile size and color represent 24-hour percentage change. The CLI retains Binance market commands and optional read-only account commands.

## Configuration and account access

`orbix config init` creates the local configuration interactively. `orbix config show` prints the effective configuration with secrets masked. Configuration is stored in the platform's user configuration directory; typical Unix systems use `~/.config/orbix/config.yaml`. Precedence is command-line flags, `ORBIX_*` environment variables, config file, then defaults. Keep config files private; credentials belong in the OS keychain or environment, not YAML.

Public market data works without credentials. To enable account monitoring, run `orbix auth login` in an interactive terminal or set `ORBIX_API_KEY` and `ORBIX_API_SECRET` in the environment. Then:

```sh
orbix auth status
orbix account balances --output json
orbix account orders
orbix account orders --history BTCUSDT --limit 100
orbix account trades BTCUSDT --limit 100
orbix account positions --market usdm
orbix account income --market usdm --limit 100
```

Signed requests are HMAC-SHA256 and pass through the same GET-only endpoint allowlist. Futures account events can update live; Spot account data refreshes over REST. For details, see [SECURITY.md](docs/SECURITY.md).

## Local data and diagnostics

SQLite stores watchlists, alerts, cache entries, and UI preferences in the platform configuration directory. If the database is unavailable, market views continue without persistence. Alerts can be managed with `orbix alerts` and watchlists with `orbix watchlist`. `orbix doctor` checks configuration, keyring availability, and Spot connectivity. `orbix diagnostics --file diagnostics.zip` exports a redacted diagnostic bundle; it excludes credentials, account records, configuration files, and the local database.

Generate shell completions with `orbix completion bash`, `zsh`, `fish`, or `powershell`.

## Development

```sh
go test ./...
go vet ./...
golangci-lint run ./...
```

See [ARCHITECTURE.md](docs/ARCHITECTURE.md), [DECISIONS.md](docs/DECISIONS.md), and [CONTRIBUTING.md](docs/CONTRIBUTING.md). CI runs checks on Linux, macOS, and Windows. Pushing a version tag such as `v0.1.0` triggers the single workflow at `.github/workflows/release.yml`, which publishes `orbix.exe`.
