package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/bubblenexus/bnx/internal/config"
	marketdata "github.com/bubblenexus/bnx/internal/core/market"
	"github.com/bubblenexus/bnx/internal/secrets"
	"github.com/spf13/cobra"
)

func newMarketCommands(options *globalOptions, stdout io.Writer) []*cobra.Command {
	return []*cobra.Command{
		newPriceCommand(options, stdout),
		newTickerCommand(options, stdout),
		newBookCommand(options, stdout),
		newTradesCommand(options, stdout),
		newKlinesCommand(options, stdout),
		newInfoCommand(options, stdout),
		newTimeCommand(options, stdout),
		newFundingCommand(options, stdout),
		newOpenInterestCommand(options, stdout),
	}
}

func newPriceCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "price [SYMBOL...]", Short: "Show latest prices", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		values, err := service.Price(cmd.Context(), product, upperSymbols(args))
		return emit(stdout, result.Config.Output, values, err)
	}}
}

func newTickerCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "ticker SYMBOL", Short: "Show 24-hour or rolling-window ticker statistics", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		window, _ := cmd.Flags().GetString("window")
		values, err := service.Ticker(cmd.Context(), product, strings.ToUpper(args[0]), window)
		return emit(stdout, result.Config.Output, values, err)
	}}
	command.Flags().String("window", "", "rolling window (for example 1h or 1d; Spot only)")
	return command
}

func newBookCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "book SYMBOL", Short: "Show the current order book", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		depth, _ := cmd.Flags().GetInt("depth")
		value, err := service.Book(cmd.Context(), product, strings.ToUpper(args[0]), depth)
		return emit(stdout, result.Config.Output, value, err)
	}}
	command.Flags().Int("depth", 20, "number of price levels (1-1000)")
	return command
}

func newTradesCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "trades SYMBOL", Short: "Show recent market trades", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		values, err := service.Trades(cmd.Context(), product, strings.ToUpper(args[0]), limit)
		return emit(stdout, result.Config.Output, values, err)
	}}
	command.Flags().Int("limit", 50, "number of trades (1-1000)")
	return command
}

func newKlinesCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "klines SYMBOL", Short: "Show candlesticks", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		interval, _ := cmd.Flags().GetString("interval")
		limit, _ := cmd.Flags().GetInt("limit")
		startRaw, _ := cmd.Flags().GetInt64("start")
		endRaw, _ := cmd.Flags().GetInt64("end")
		values, err := service.Klines(cmd.Context(), product, strings.ToUpper(args[0]), interval, limit, startRaw, endRaw)
		return emit(stdout, result.Config.Output, values, err)
	}}
	command.Flags().String("interval", "1h", "interval (1m, 5m, 15m, 1h, 4h, 1d, etc.)")
	command.Flags().Int("limit", 500, "number of candles (1-1500)")
	command.Flags().Int64("start", 0, "start time in Unix milliseconds")
	command.Flags().Int64("end", 0, "end time in Unix milliseconds")
	return command
}

func newInfoCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "info SYMBOL", Short: "Show exchange rules and symbol precision", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		value, err := service.ExchangeInfo(cmd.Context(), product, strings.ToUpper(args[0]))
		return emit(stdout, result.Config.Output, value, err)
	}}
}

func newTimeCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "time", Short: "Show Binance server time and local clock offset", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		value, err := service.Time(cmd.Context(), product)
		return emit(stdout, result.Config.Output, value, err)
	}}
}

func newFundingCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "funding [SYMBOL]", Short: "Show futures funding rates", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		symbol := ""
		if len(args) > 0 {
			symbol = strings.ToUpper(args[0])
		}
		history, _ := cmd.Flags().GetBool("history")
		values, err := service.Funding(cmd.Context(), product, symbol, history)
		return emit(stdout, result.Config.Output, values, err)
	}}
	command.Flags().Bool("history", false, "show historical funding rates")
	return command
}

func newOpenInterestCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "oi SYMBOL", Short: "Show current futures open interest", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		value, err := service.OpenInterest(cmd.Context(), product, strings.ToUpper(args[0]))
		return emit(stdout, result.Config.Output, value, err)
	}}
}

func newMarketService(options *globalOptions) (*marketdata.Service, marketdata.Product, config.Result, error) {
	result, err := loadConfig(options)
	if err != nil {
		return nil, "", config.Result{}, &exitError{code: 2, err: err}
	}
	return newMarketServiceFromConfig(result)
}

func newMarketServiceFromConfigWithCredentials(result config.Result, apiKey, apiSecret string) (*marketdata.Service, marketdata.Product, config.Result, error) {
	product, err := marketProduct(result.Config.Market)
	if err != nil {
		return nil, "", config.Result{}, err
	}
	service, err := marketdata.NewRESTService(marketdata.RESTOptions{Timeout: result.Config.Timeout, MaxAttempts: result.Config.RetryAttempts, RetryBaseDelay: result.Config.RetryBaseDelay, RecvWindow: result.Config.RecvWindow, APIKey: apiKey, APISecret: apiSecret, BaseURLs: map[marketdata.Product][]string{
		marketdata.Spot: result.Config.Binance.SpotBaseURLs, marketdata.USDM: result.Config.Binance.USDMBaseURLs, marketdata.COINM: result.Config.Binance.COINMBaseURLs, marketdata.Options: result.Config.Binance.OptionsBaseURLs,
	}})
	if err != nil {
		return nil, "", config.Result{}, fmt.Errorf("initialize Binance client: %w", err)
	}
	return service, product, result, nil
}

func newAuthenticatedMarketService(options *globalOptions) (*marketdata.Service, marketdata.Product, config.Result, error) {
	result, err := loadConfig(options)
	if err != nil {
		return nil, "", config.Result{}, &exitError{code: 2, err: err}
	}
	credentials, err := (secrets.FallbackStore{Primary: secrets.KeyringStore{}, Fallback: secrets.EnvStore{}}).Get(result.Config.Profile)
	if err != nil {
		return nil, "", config.Result{}, fmt.Errorf("no API credentials for profile %q; run `bnx auth login` or set BNX_API_KEY and BNX_API_SECRET", result.Config.Profile)
	}
	service, product, loaded, err := newMarketServiceFromConfigWithCredentials(result, credentials.APIKey, credentials.APISecret)
	return service, product, loaded, err
}

func newMarketServiceFromConfig(result config.Result) (*marketdata.Service, marketdata.Product, config.Result, error) {
	return newMarketServiceFromConfigWithCredentials(result, "", "")
}

func marketProduct(market string) (marketdata.Product, error) {
	switch market {
	case "spot":
		return marketdata.Spot, nil
	case "usdm":
		return marketdata.USDM, nil
	case "coinm":
		return marketdata.COINM, nil
	case "options":
		return marketdata.Options, nil
	default:
		return "", fmt.Errorf("unsupported market %q", market)
	}
}

func emit(stdout io.Writer, format string, value any, err error) error {
	if err != nil {
		return err
	}
	if err := writeOutput(stdout, format, value); err != nil {
		return fmt.Errorf("format command output: %w", err)
	}
	return nil
}

func upperSymbols(symbols []string) []string {
	result := make([]string, len(symbols))
	for index, symbol := range symbols {
		result[index] = strings.ToUpper(symbol)
	}
	return result
}
