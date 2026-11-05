package cli

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/talha-shah2031/orbix/internal/binance/models"
	"github.com/talha-shah2031/orbix/internal/config"
	"github.com/talha-shah2031/orbix/internal/logging"
	"github.com/spf13/cobra"
)

func newExportCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "export", Short: "Export public market data as CSV"}
	klines := &cobra.Command{Use: "klines SYMBOL", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, _, err := newMarketService(options)
		if err != nil {
			return err
		}
		interval, _ := cmd.Flags().GetString("interval")
		limit, _ := cmd.Flags().GetInt("limit")
		start, _ := cmd.Flags().GetInt64("start")
		end, _ := cmd.Flags().GetInt64("end")
		rows, err := service.Klines(cmd.Context(), product, strings.ToUpper(args[0]), interval, limit, start, end)
		if err != nil {
			return err
		}
		return writeOutput(stdout, "csv", klineExportRows(rows))
	}}
	klines.Flags().String("interval", "1h", "candlestick interval")
	klines.Flags().Int("limit", 500, "row limit")
	klines.Flags().Int64("start", 0, "start time in Unix milliseconds")
	klines.Flags().Int64("end", 0, "end time in Unix milliseconds")
	trades := &cobra.Command{Use: "trades SYMBOL", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, _, err := newMarketService(options)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		rows, err := service.Trades(cmd.Context(), product, strings.ToUpper(args[0]), limit)
		if err != nil {
			return err
		}
		return writeOutput(stdout, "csv", rows)
	}}
	trades.Flags().Int("limit", 1000, "row limit")
	command.AddCommand(klines, trades)
	return command
}

func klineExportRows(values []models.Kline) []map[string]any {
	rows := make([]map[string]any, len(values))
	for i, candle := range values {
		rows[i] = map[string]any{
			"openTime": candle.OpenTime, "open": candle.Open, "high": candle.High, "low": candle.Low,
			"close": candle.Close, "volume": candle.Volume, "closeTime": candle.CloseTime,
			"quoteVolume": candle.QuoteVolume, "trades": candle.Trades,
			"takerBuyBase": candle.TakerBuyBase, "takerBuyQuote": candle.TakerBuyQuote,
		}
	}
	return rows
}

func newDiagnosticsCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "diagnostics", Short: "Create a redacted diagnostics bundle", Args: cobra.NoArgs}
	command.Flags().String("file", "orbix-diagnostics.zip", "output ZIP path")
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		result, err := loadConfig(options)
		if err != nil {
			return err
		}
		checks := runChecks(cmd.Context(), result)
		path, _ := cmd.Flags().GetString("file")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("create diagnostics bundle: %w", err)
		}
		if chmodErr := file.Chmod(0o600); chmodErr != nil {
			_ = file.Close()
			return fmt.Errorf("set diagnostics bundle permissions: %w", chmodErr)
		}
		archive := zip.NewWriter(file)
		manifest := map[string]any{"version": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "profile": result.Config.Profile, "market": result.Config.Market, "checks": checks}
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err == nil {
			err = writeZipFile(archive, "diagnostics.json", encoded)
		}
		if err == nil && result.Config.Log.Path != "" {
			if logFile, openErr := os.Open(result.Config.Log.Path); openErr == nil {
				defer func() { _ = logFile.Close() }()
				if info, statErr := logFile.Stat(); statErr == nil {
					const maxLogBytes = 256 * 1024
					if info.Size() > maxLogBytes {
						_, _ = logFile.Seek(-maxLogBytes, io.SeekEnd)
					}
					if logData, readErr := io.ReadAll(io.LimitReader(logFile, maxLogBytes)); readErr == nil {
						lines := strings.Split(string(logData), "\n")
						start := max(0, len(lines)-1000)
						redacted := logging.Redact(strings.Join(lines[start:], "\n"))
						err = writeZipFile(archive, "redacted-log-tail.jsonl", []byte(redacted))
					}
				}
			}
		}
		closeArchiveErr := archive.Close()
		closeFileErr := file.Close()
		if err != nil {
			return err
		}
		if closeArchiveErr != nil {
			return closeArchiveErr
		}
		if closeFileErr != nil {
			return closeFileErr
		}
		if result.Config.Output == "json" {
			return emit(stdout, "json", map[string]any{"file": path, "checks": checks}, nil)
		}
		_, err = fmt.Fprintf(stdout, "Wrote redacted diagnostics bundle: %s\n", path)
		return err
	}
	return command
}

func writeZipFile(archive *zip.Writer, name string, content []byte) error {
	file, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	return err
}

func newCompletionCommand(root *cobra.Command, stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "completion bash|zsh|fish|powershell", Short: "Generate shell completion scripts", Args: cobra.ExactArgs(1), ValidArgs: []string{"bash", "zsh", "fish", "powershell"}, RunE: func(_ *cobra.Command, args []string) error {
		var err error
		switch args[0] {
		case "bash":
			err = root.GenBashCompletionV2(stdout, true)
		case "zsh":
			err = root.GenZshCompletion(stdout)
		case "fish":
			err = root.GenFishCompletion(stdout, true)
		case "powershell":
			err = root.GenPowerShellCompletionWithDesc(stdout)
		default:
			return fmt.Errorf("unsupported shell %q", args[0])
		}
		return err
	}}
}

func newConfigCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Initialize or inspect local configuration"}
	command.AddCommand(&cobra.Command{Use: "show", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		result, err := loadConfig(options)
		if err != nil {
			return err
		}
		cfg := result.Config
		return emit(stdout, cfg.Output, map[string]any{
			"profile": cfg.Profile, "market": cfg.Market, "output": cfg.Output, "noColor": cfg.NoColor,
			"timeout": cfg.Timeout.String(), "retryAttempts": cfg.RetryAttempts, "retryBaseDelay": cfg.RetryBaseDelay.String(), "recvWindow": cfg.RecvWindow.String(),
			"log":      map[string]string{"level": cfg.Log.Level, "path": cfg.Log.Path},
			"binance":  map[string]any{"spotBaseURLs": cfg.Binance.SpotBaseURLs, "usdmBaseURLs": cfg.Binance.USDMBaseURLs, "coinmBaseURLs": cfg.Binance.COINMBaseURLs, "optionsBaseURLs": cfg.Binance.OptionsBaseURLs},
			"profiles": cfg.Profiles,
		}, nil)
	}})
	command.AddCommand(&cobra.Command{Use: "init", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		path := options.configFile
		if path == "" {
			var err error
			path, err = config.DefaultPath()
			if err != nil {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
		encoded := []byte("profile: main\nmarket: spot\noutput: table\ntimeout: 10s\nretry_attempts: 3\nretry_base_delay: 200ms\nrecv_window: 5s\nlog:\n  level: info\nbinance:\n  spot_base_urls:\n    - https://api.binance.com\n    - https://api-gcp.binance.com\n    - https://api1.binance.com\n    - https://api2.binance.com\n    - https://api3.binance.com\n    - https://api4.binance.com\n    - https://data-api.binance.vision\n  usdm_base_urls: [https://fapi.binance.com]\n  coinm_base_urls: [https://dapi.binance.com]\n  options_base_urls: [https://eapi.binance.com]\nprofiles:\n  main:\n    default_market: spot\n")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("create config file %q: %w", path, err)
		}
		if _, writeErr := file.Write(encoded); writeErr != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return writeErr
		}
		if closeErr := file.Close(); closeErr != nil {
			return closeErr
		}
		if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
			return fmt.Errorf("set config permissions: %w", chmodErr)
		}
		result, err := loadConfig(options)
		if err != nil {
			return err
		}
		if result.Config.Output == "json" {
			return emit(stdout, "json", map[string]any{"path": path, "created": true}, nil)
		}
		_, err = fmt.Fprintf(stdout, "Created %s with mode 0600. Edit it to customize ORBIX; credentials are stored separately.\n", path)
		return err
	}})
	return command
}
