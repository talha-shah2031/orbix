# Architecture decisions

## 2026-11-02: repository and module identity

- The project lives in the `bnx/` subdirectory of the existing checkout and uses module path `github.com/bubblenexus/bnx` until a dedicated upstream repository is assigned.
- The current module directive and CI toolchain are Go 1.27.3, which includes the standard-library security fixes found by the vulnerability scan.

## 2026-11-02: mainnet-only boundary

- Mainnet is an invariant checked during configuration validation. Base URLs must use HTTPS and any URL containing `testnet` is rejected before network access.
- The production Spot REST URL order matches Binance's current official REST documentation. `data-api.binance.vision` is retained last and is public-market-data-only.
- Phase 0's doctor calls only `GET /api/v3/ping`. Endpoint allowlisting and the complete REST transport are Phase 1 deliverables; doctor performs its own strict HTTPS/mainnet validation meanwhile.

Official source: <https://developers.binance.com/en/docs/products/spot/rest-api> (checked 2026-11-02).

## 2026-11-03: REST routes, transport, and market-data shapes

- The REST registry admits exact GET method/path pairs. Account history routes are marked signed and historical trades routes requiring a key header are marked separately; no write or stream-management route is registered.
- Configurable REST hosts are restricted to documented Binance production hosts for each product. Public Spot failover may end at `data-api.binance.vision`; authenticated calls exclude that public-data-only host.
- Spot, USDⓈ-M, COIN-M, and Options market-data paths follow their product REST references. Options uses `/eapi/v1/ticker` for 24-hour statistics/last-price data, while USDⓈ-M's current price route remains the versioned `/fapi/v2/ticker/price` specified by Binance.
- The Options documentation confirms 12-value kline tuples, matching the typed decoder used by Spot and both Futures products.

Official references checked 2026-11-03:

- <https://developers.binance.com/en/docs/products/spot/rest-api>
- <https://developers.binance.com/docs/derivatives/coin-margined-futures/market-data/rest-api/Get-Funding-Info>
- <https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-options/api/rest-api/market-data>
- <https://developers.binance.com/en/docs/products/spot/faqs/spot_glossary>

## 2026-11-02: configuration and secrets

- Viper owns defaults, YAML, environment loading, and explicit flag overrides. CLI parsing remains in Cobra.
- Secrets never enter Viper and have no CLI flags. The OS keyring is the writable store; environment variables are a read-only fallback.
- The initial config file need not exist. When introduced by `config init`, it will be created with mode `0600`; startup permission checks warn on broader POSIX modes. Windows ACLs are not faithfully represented by Go mode bits, so the Windows check does not claim equivalent ACL validation.

## 2026-11-02: logging and diagnostics

- Application logs use `slog` JSON output in the user cache directory unless configured otherwise. The redacting handler sanitizes both messages and structured attributes.
- Diagnostic data goes to stdout and human diagnostic errors to stderr. JSON output is kept free of human prose.
- Lack of account credentials is a warning, not a doctor failure. Failure to reach the configured mainnet endpoint is a failure.

## 2026-11-03: first interactive market UI

- Bubble Tea, Bubbles, and Lip Gloss provide the TUI runtime and styling. The documented Go minimum follows the module directive.
- The TUI gets data only through `internal/core/market`; CLI wiring injects a runner so no Bubble Tea imports leak outside `internal/tui`.
- The initial Markets and Symbol screens poll Binance REST every five seconds. Symbol depth is an ordinary REST snapshot and must not be treated as a synchronized local book until Phase 2 implements the documented diff-stream/snapshot procedure.
- The Dashboard ranks only active stable-quoted symbols for Spot/USDⓈ-M movers and volume, and focuses its watch panel on major contracts; COIN-M uses active contracts. Sparkline history is fetched as 24 hourly klines at launch/market switch. Best bid/ask comes from the dedicated book-ticker endpoint, not fields that may be absent from bulk 24-hour ticker responses.
- Favorites are session-only in this phase. Theme selection is in-memory and cycles among dark, light, high-contrast, and colorblind-safe palettes.

Official dependency release metadata was checked 2026-11-03; Binance REST endpoints continue to use the Phase 1 official references above.

## 2026-11-03: public WebSocket streams and local depth synchronization

- Public stream connections use Binance production-only endpoints. Spot tries `stream.binance.com:9443`, `stream.binance.com:443`, then the market-data-only `data-stream.binance.vision`; USDⓈ-M, COIN-M, and Options use their documented production stream base paths.
- Binance combined streams wrap events as `{stream,data}`. The manager reconnects and restores subscriptions, proactively rotates before the documented 24-hour connection age, shards at 1024 streams, and exposes bounded subscriber queues. Ticker consumers drop oldest; order-book consumers request a resync signal on overflow.
- The local order book opens depth streaming before taking the REST snapshot, buffers and sorts events, discards stale `u` IDs, finds the bridge event, then enforces the project brief's strict `U == previous_u + 1` continuity requirement. Sequence gaps invalidate the book and trigger a fresh stream/snapshot cycle.
- `bnx stream` writes combined stream events as NDJSON and uses context cancellation for Ctrl+C. Local verification includes a TLS fake-stream server. The 30-minute external forced-disconnect soak remains an operational acceptance test and was not run in this environment.

Official references checked 2026-11-03:

- <https://developers.binance.com/en/docs/products/spot/spot-apis/websocket-streams>
- <https://developers.binance.com/en/docs/products/derivatives-trading-usds-futures/websocket-market-streams/Connect>
- <https://developers.binance.com/en/docs/products/derivatives-trading-coin-futures/websocket-market-streams/Connect>
- <https://developers.binance.com/zh-CN/docs/products/derivatives-trading-coin-futures/change-log> (Options production stream path and keepalive behavior)

## 2026-11-03: local persistence, analytics, and indicators

- SQLite uses `modernc.org/sqlite`, keeping the binary free of CGO. Migrations are tracked in `schema_migrations`; watchlists and alerts are profile-scoped. Cache data is optional and may be ignored when unavailable or stale.
- Kline cache rows are stored per open time, preserve the documented positional response tuple and decimal precision, detect gaps for supported intervals, and expire through TTL checks. Exchange metadata has a separate TTL cache. The app continues with live REST data if the local database cannot be opened.
- Alert thresholds support `above`, `below`, `change_above`, `change_below`, `funding_above`, and `funding_below`; cooldown timestamps persist between runs. Price and funding alerts render a TUI toast and terminal bell.
- Indicator calculations use decimal arithmetic: SMA, SMA-seeded EMA, population-standard-deviation Bollinger bands, and Wilder-smoothed RSI. They are summaries shown below the chart and are toggled with `a`.
- Futures analytics use the public `/futures/data/*` routes in the exact GET allowlist. USDⓈ-M uses `symbol` and `takerlongshortRatio`; COIN-M uses `pair` and `takerBuySellVol`, matching the product documentation.

Official reference checked 2026-11-03: <https://developers.binance.com/docs/derivatives/coin-margined-futures/market-data/rest-api/Get-Funding-Info>.

## 2026-11-03: authenticated read-only account monitoring

- Account REST calls use signed GET routes registered in the existing endpoint allowlist. Spot account flags expose `canTrade` and `canWithdraw`; the CLI warns on stderr when either is enabled. Credentials are prompted without echo and saved to the OS keychain; `BNX_API_KEY` / `BNX_API_SECRET` remain the documented fallback.
- Futures account events use the current USDⓈ-M and COIN-M WebSocket API `userDataStream.start` and `.ping` methods. The client does not call a REST listen-key lifecycle or any account mutation endpoint.
- Binance's current Spot user-data WebSocket API subscription documentation requires an Ed25519 session login, while this application's supported credential scheme is the HMAC API-key/secret pair. Spot therefore uses bounded five-second account REST refreshes instead of a misleading or deprecated listen-key subscription. Spot balances, open orders, order history, and trade history remain available via signed GET.
- Options have public market data only; no account routes are exposed.

Official references checked 2026-11-03:

- <https://developers.binance.com/en/docs/catalog/core-trading-spot-trading/api/ws-api/user-data-stream>
- <https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-api/user-data-streams>
- <https://developers.binance.com/en/docs/products/derivatives-trading-coin-futures/user-data-streams>

## 2026-11-03: Phase 6 scope and release checks

- The command palette searches view names and current market symbols; choosing a match opens that view/symbol. Shell completion is generated from the actual Cobra command tree.
- CSV exports stream public klines or recent trades to stdout. Historical bulk archive downloads are not included because their file formats/ranges and separate data host need an explicit size, resume, and integrity policy; no unbounded downloader is exposed.
- Diagnostics bundles include runtime metadata, doctor checks, and at most the last 1000 redacted log lines. They omit config contents, key material, account data, and the local database. Bundle files are created with restrictive permissions.
- `config init` writes a secret-free mainnet defaults file with restrictive permissions, and `config show` emits a stable, secret-free summary. The initializer is deliberately noninteractive so it can be used in scripts.
- The Spot Market heatmap remains Binance-only and derives its rows from `GET /api/v3/ticker/24hr` alone: symbols ending in `USDT` are ranked by `quoteVolume` to select the top 100. Tile area uses absolute `priceChangePercent` (minimum 0.25% before normalization, with each normalized tile capped at 15% of total area); quote volume does not size tiles. Tile color uses the signed change clamped to ±10%. The ticker response does not provide global coin market-cap rankings.
- The local Windows environment has no CGO compiler, so `go test -race` cannot execute here; the CI matrix remains the race-test authority. A multi-hour WebSocket soak and live authenticated account checks require a separately authorized operational environment and were not performed.
- Releases are intentionally kept minimal: `.github/workflows/release.yml` builds one Windows amd64 executable and attaches only `bnx.exe` to the GitHub Release for a pushed `v*` tag.
- Coverage was rechecked after adding service, config, REST, model, order-book synchronization, and WebSocket tests: aggregate `internal/binance` coverage is 81.1%, `internal/core/market` is 82.0%, and `internal/config` is 89.8%. The WebSocket subpackage remains 77.0% individually, while the aggregate Binance target is met.
