package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/talha-shah2031/orbix/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newAccountCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "account", Short: "Read-only account and position monitoring"}
	command.AddCommand(&cobra.Command{Use: "balances", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		service, product, result, err := newAuthenticatedMarketService(options)
		if err != nil {
			return err
		}
		value, err := service.Account(cmd.Context(), product)
		if err != nil {
			return err
		}
		if value.CanTrade || value.CanWithdraw {
			if _, writeErr := fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: this API key has trading or withdrawal permission. Use a key with Enable Reading only, IP restricted, and trading/withdrawals disabled."); writeErr != nil {
				return writeErr
			}
		}
		return emit(stdout, result.Config.Output, value, nil)
	}})
	orders := &cobra.Command{Use: "orders [SYMBOL]", Short: "Show open orders or order history", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newAuthenticatedMarketService(options)
		if err != nil {
			return err
		}
		history, _ := cmd.Flags().GetBool("history")
		symbol := ""
		if len(args) > 0 {
			symbol = strings.ToUpper(args[0])
		}
		if history {
			limit, _ := cmd.Flags().GetInt("limit")
			values, historyErr := service.OrderHistory(cmd.Context(), product, symbol, limit)
			return emit(stdout, result.Config.Output, values, historyErr)
		}
		values, err := service.OpenOrders(cmd.Context(), product, symbol)
		return emit(stdout, result.Config.Output, values, err)
	}}
	orders.Flags().Bool("history", false, "show completed and historical orders (requires SYMBOL)")
	orders.Flags().Int("limit", 100, "history row limit (1-1000)")
	command.AddCommand(orders)
	trades := &cobra.Command{Use: "trades SYMBOL", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newAuthenticatedMarketService(options)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		values, err := service.AccountTrades(cmd.Context(), product, strings.ToUpper(args[0]), limit)
		return emit(stdout, result.Config.Output, values, err)
	}}
	trades.Flags().Int("limit", 100, "number of account trades (1-1000)")
	command.AddCommand(trades)
	command.AddCommand(&cobra.Command{Use: "positions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		service, product, result, err := newAuthenticatedMarketService(options)
		if err != nil {
			return err
		}
		values, err := service.Positions(cmd.Context(), product)
		return emit(stdout, result.Config.Output, values, err)
	}})
	income := &cobra.Command{Use: "income [SYMBOL]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newAuthenticatedMarketService(options)
		if err != nil {
			return err
		}
		symbol := ""
		if len(args) > 0 {
			symbol = strings.ToUpper(args[0])
		}
		limit, _ := cmd.Flags().GetInt("limit")
		values, err := service.IncomeHistory(cmd.Context(), product, symbol, limit)
		return emit(stdout, result.Config.Output, values, err)
	}}
	income.Flags().Int("limit", 100, "number of income rows (1-1000)")
	command.AddCommand(income)
	return command
}

func isTerminalInput() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
func readHidden(prompt string, writer io.Writer) (string, error) {
	if _, err := io.WriteString(writer, prompt); err != nil {
		return "", err
	}
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	_, _ = io.WriteString(writer, "\n")
	if err != nil {
		return "", fmt.Errorf("read hidden credential: %w", err)
	}
	return strings.TrimSpace(string(value)), nil
}
func getEnv(name string) string { return os.Getenv(name) }

func newAuthCommand(options *globalOptions, stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "auth", Short: "Manage read-only Binance API credentials"}
	command.AddCommand(&cobra.Command{Use: "login", Short: "Securely store API credentials in the OS keychain", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if !isTerminalInput() {
			return fmt.Errorf("auth login requires an interactive terminal; never pass API secrets as CLI flags")
		}
		if _, err := fmt.Fprintln(stderr, "Security: use an API key with Enable Reading only, restrict it by IP, and disable trading and withdrawals."); err != nil {
			return err
		}
		apiKey, err := readHidden("API key: ", stderr)
		if err != nil {
			return err
		}
		apiSecret, err := readHidden("API secret: ", stderr)
		if err != nil {
			return err
		}
		cfg, err := loadConfig(options)
		if err != nil {
			return err
		}
		err = (secrets.KeyringStore{}).Set(cfg.Config.Profile, secrets.Credentials{APIKey: apiKey, APISecret: apiSecret})
		if err != nil {
			return fmt.Errorf("store credentials in OS keychain: %w", err)
		}
		return emit(stdout, cfg.Config.Output, map[string]any{"profile": cfg.Config.Profile, "stored": true}, nil)
	}})
	command.AddCommand(&cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := loadConfig(options)
		if err != nil {
			return err
		}
		credentials, err := (secrets.FallbackStore{Primary: secrets.KeyringStore{}, Fallback: secrets.EnvStore{}}).Get(cfg.Config.Profile)
		if err != nil {
			return emit(stdout, cfg.Config.Output, map[string]any{"profile": cfg.Config.Profile, "available": false, "setup": "orbix auth login"}, nil)
		}
		source := "keychain"
		if credentials.APIKey == getEnv("ORBIX_API_KEY") {
			source = "environment"
		}
		return emit(stdout, cfg.Config.Output, map[string]any{"profile": cfg.Config.Profile, "available": true, "source": source, "secretMasked": true}, nil)
	}})
	command.AddCommand(&cobra.Command{Use: "logout", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := loadConfig(options)
		if err != nil {
			return err
		}
		if err := (secrets.KeyringStore{}).Delete(cfg.Config.Profile); err != nil {
			return err
		}
		return emit(stdout, cfg.Config.Output, map[string]any{"profile": cfg.Config.Profile, "loggedOut": true, "environmentFallbackMayRemain": true}, nil)
	}})
	return command
}
