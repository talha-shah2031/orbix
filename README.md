# Orbix

[![CI](https://github.com/talha-shah2031/orbix/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/talha-shah2031/orbix/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/talha-shah2031/orbix?display_name=tag)](https://github.com/talha-shah2031/orbix/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.27.3-00ADD8?logo=go)](go.mod)

Orbix is a fast, keyboard-driven crypto market terminal and scriptable CLI for Windows, macOS, and Linux. It combines a full-screen TUI with stable JSON, NDJSON, CSV, YAML, and table output for automation.

Orbix is mainnet-only and read-only by design. It cannot place, edit, or cancel orders, withdraw funds, transfer assets, or borrow on margin.

> Orbix is an independent project, is not affiliated with Binance, WEEX, or MEXC, and is not financial advice.

## Highlights

- Interactive dashboard, market heatmap, symbol detail, indicators, order book, trades, alerts, and account views
- Binance Spot, USD-M Futures, COIN-M Futures, and Options market data
- Public Binance, WEEX, and MEXC Spot heatmaps
- Reconnecting WebSocket streams and sequence-checked local order books
- Optional read-only account monitoring with credentials stored in the operating-system keychain
- Profile-scoped watchlists, alerts, cache, and UI state in local SQLite
- Exact GET-only endpoint allowlist, mainnet host validation, bounded retries, and redacted logs
- Native x64 and Arm64 releases with SHA-256 checksums and GitHub build provenance

## Supported platforms

| Operating system | Architecture | Release archive |
| --- | --- | --- |
| Windows | x64 | `orbix-VERSION-windows-amd64.zip` |
| Windows | Arm64 | `orbix-VERSION-windows-arm64.zip` |
| macOS | Intel | `orbix-VERSION-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `orbix-VERSION-darwin-arm64.tar.gz` |
| Linux | x64 | `orbix-VERSION-linux-amd64.tar.gz` |
| Linux | Arm64 | `orbix-VERSION-linux-arm64.tar.gz` |

## Install

Download the archive for your platform from [GitHub Releases](https://github.com/talha-shah2031/orbix/releases/latest). Replace `VERSION` below with the release tag, such as `v1.0.0`.

### Linux

```sh
tar -xzf orbix-VERSION-linux-amd64.tar.gz
sudo install orbix-VERSION-linux-amd64/orbix /usr/local/bin/orbix
orbix version
```

Use the `arm64` archive on an Arm64 machine.

### macOS

Apple Silicon:

```sh
tar -xzf orbix-VERSION-darwin-arm64.tar.gz
sudo install orbix-VERSION-darwin-arm64/orbix /usr/local/bin/orbix
orbix version
```

Intel Macs use the `darwin-amd64` archive. Release binaries are checksummed and provenance-attested but are not currently Apple-notarized; macOS may ask you to confirm the first launch in **System Settings → Privacy & Security**.

### Windows

Extract the x64 or Arm64 ZIP, then move `orbix.exe` to a stable directory:

```powershell
$installDir = Join-Path $env:LOCALAPPDATA 'Programs\Orbix'
New-Item -ItemType Directory -Force $installDir | Out-Null
Copy-Item .\orbix.exe (Join-Path $installDir 'orbix.exe')
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
}
```

Open a new terminal and run:

```powershell
orbix version
```

### Verify a download

Every release includes `SHA256SUMS`.

Linux:

```sh
sha256sum -c SHA256SUMS --ignore-missing
```

macOS:

```sh
shasum -a 256 -c SHA256SUMS
```

Windows PowerShell:

```powershell
Get-FileHash .\orbix-VERSION-windows-amd64.zip -Algorithm SHA256
```

With the GitHub CLI installed, verify build provenance:

```sh
gh attestation verify orbix-VERSION-linux-amd64.tar.gz --repo talha-shah2031/orbix
```

## Quick start

Launch the TUI:

```sh
orbix
```

Public commands work without an API key:

```sh
orbix price BTCUSDT
orbix ticker BTCUSDT --window 1h --output json
orbix book BTCUSDT --depth 20
orbix trades BTCUSDT --limit 50 --output csv
orbix klines BTCUSDT --interval 1h --limit 100 --output ndjson
orbix funding BTCUSDT --market usdm
orbix oi BTCUSDT --market usdm
orbix stream trade BTCUSDT
```

Discover the complete command tree with:

```sh
orbix --help
orbix COMMAND --help
```

## TUI controls

| Key | Action |
| --- | --- |
| `1`–`8` | Switch views |
| `j`, `k`, arrows | Move the selection |
| `Enter` | Open the selected symbol |
| `/` | Search markets |
| `s` | Change sort order |
| `f` | Toggle favorite |
| `[` / `]` | Change page or chart interval |
| `+` / `-` | Zoom the chart |
| `a` | Toggle indicators |
| `e` | Cycle Spot exchanges |
| `q` | Cycle Spot quote assets |
| `m` | Change market |
| `t` | Change theme |
| `r` | Refresh |
| `:` / `Ctrl+P` | Open the command palette |
| `?` | Open help |
| `Ctrl+C` / `Ctrl+Q` | Quit |

See [the complete keybinding reference](docs/KEYBINDINGS.md).

## Configuration

Orbix works without a config file. Create one only when you want persistent overrides:

```sh
orbix config init
orbix config show
```

Default configuration and database locations:

| Platform | Directory |
| --- | --- |
| Windows | `%AppData%\orbix` |
| macOS | `~/Library/Application Support/orbix` |
| Linux | `${XDG_CONFIG_HOME:-~/.config}/orbix` |

Precedence is command-line flags, `ORBIX_*` environment variables, config file, then defaults. Configuration files never contain API credentials.

## Optional account access

Public market data needs no credentials. For account views, create a dedicated Binance API key with **Enable Reading only**, IP-restrict it, and disable every trading, transfer, and withdrawal permission.

```sh
orbix auth login
orbix auth status
orbix account balances --output json
orbix account orders
orbix account trades BTCUSDT
orbix account positions --market usdm
orbix account income --market usdm
```

`auth login` stores credentials in Windows Credential Manager, macOS Keychain, or Linux Secret Service. `ORBIX_API_KEY` and `ORBIX_API_SECRET` are supported as a read-only fallback. See the [security policy](docs/SECURITY.md) before enabling account access.

## Diagnostics and automation

```sh
orbix doctor --output json
orbix diagnostics --file orbix-diagnostics.zip
orbix completion bash
orbix completion zsh
orbix completion fish
orbix completion powershell
```

Command output goes to stdout and diagnostics go to stderr. Use `--no-color` and a structured output format in scripts. Diagnostic bundles exclude credentials, configuration contents, account records, and the SQLite database.

## Build from source

Requirements: Go 1.27.3 or newer. CGO is not required.

```sh
git clone https://github.com/talha-shah2031/orbix.git
cd orbix
go mod download
make build
./bin/orbix version
```

Windows PowerShell without GNU Make:

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
go mod download
go build -trimpath -o bin\orbix.exe .\cmd\orbix
.\bin\orbix.exe version
```

## Project documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Security](docs/SECURITY.md)
- [Contributing](docs/CONTRIBUTING.md)
- [Release process](docs/RELEASING.md)
- [Architecture decisions](docs/DECISIONS.md)
- [Changelog](CHANGELOG.md)

Before reporting a problem, run `orbix doctor`. Include the command output or a redacted diagnostics bundle, but never include API credentials.
