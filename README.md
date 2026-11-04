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

When a release is published, download the Windows archive for your CPU architecture (`amd64` or `arm64`) from the repository's **GitHub Releases** page. Verify it against `checksums.txt` before extracting. Releases currently provide Windows builds only.

In Windows PowerShell, this `curl.exe` installer detects the CPU architecture, downloads the latest Windows release, verifies its SHA-256 checksum, and installs `bnx.exe` under your local programs directory:

```powershell
$ErrorActionPreference = 'Stop'
$repo = 'talha-shah2031/BNX'
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$version = $release.tag_name.TrimStart('v')
$archive = "bnx_${version}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$($release.tag_name)"
$tmp = Join-Path $env:TEMP ([guid]::NewGuid().ToString())
$installDir = Join-Path $env:LOCALAPPDATA 'Programs\bnx'
New-Item -ItemType Directory -Force $tmp, $installDir | Out-Null
try {
    curl.exe -fL "$base/$archive" -o (Join-Path $tmp $archive)
    curl.exe -fL "$base/checksums.txt" -o (Join-Path $tmp 'checksums.txt')
    $expected = ((Select-String -Path (Join-Path $tmp 'checksums.txt') -Pattern ([regex]::Escape($archive))).Line -split '\s+')[0]
    $actual = (Get-FileHash (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected.ToLowerInvariant()) { throw 'Checksum verification failed.' }
    Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp
    Copy-Item (Join-Path $tmp 'bnx.exe') (Join-Path $installDir 'bnx.exe') -Force
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $installDir) {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
    }
    Write-Host "Installed to $installDir. Open a new terminal and run: bnx"
}
finally {
    Remove-Item $tmp -Recurse -Force
}
```

If no release has been published yet, build from source using the instructions below. You can also download the Windows ZIP manually, verify its SHA-256 value from `checksums.txt`, and extract `bnx.exe`.

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
