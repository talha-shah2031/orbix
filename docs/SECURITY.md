# Security

## Threat model

`orbix` assumes the local user account and operating system are trusted. Primary risks are API credential disclosure, accidental use of privileged endpoints, sensitive data in logs, insecure configuration permissions, dependency compromise, and malicious or malformed remote responses.

The application treats Binance, WEEX, and MEXC market responses as untrusted, uses bounded resources, and never implements order placement, modification, cancellation, withdrawal, transfer, or borrowing. Binance requests pass through the explicit read-only endpoint allowlist. WEEX and MEXC are contacted only through fixed public ticker GET URLs; no API credentials are sent to them.

## API key setup

Create a dedicated Binance API key with **Enable Reading only**. Disable Spot & Margin Trading, futures trading, withdrawals, transfers, and every other write permission. Restrict the key to trusted IP addresses wherever practical. Do not reuse a trading key.

Credentials belong in the OS keychain. `ORBIX_API_KEY` and `ORBIX_API_SECRET` are supported only as an environment fallback and may be visible to processes with sufficient local privileges. Secrets must never be passed as command-line flags or stored in YAML.

## Controls

- Mainnet HTTPS endpoints only; testnet URLs are rejected.
- JSON file logs pass through a redaction handler.
- Config permission checks warn when POSIX files are broader than `0600`.
- CI verifies modules, runs race-enabled tests and strict linting, and executes `govulncheck`.
- The application has no analytics or telemetry service. The Spot heatmap contacts only the selected public ticker endpoint for Binance, WEEX, or MEXC; other market and account services contact Binance.

Phase 1 adds the central endpoint allowlist and rejection tests that make write endpoints unreachable at the transport boundary.
