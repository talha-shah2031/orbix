package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/bubblenexus/bnx/internal/config"
	"github.com/bubblenexus/bnx/internal/store"
	"github.com/spf13/cobra"
)

func newWatchlistCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "watchlist", Short: "Manage the local profile watchlist"}
	command.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		db, cfg, err := openLocalStore(cmd.Context(), options)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		values, err := db.Watchlist(cmd.Context(), cfg.Config.Profile)
		return emit(stdout, cfg.Config.Output, values, err)
	}})
	command.AddCommand(&cobra.Command{Use: "add SYMBOL", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		db, cfg, err := openLocalStore(cmd.Context(), options)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if err := db.AddWatchlist(cmd.Context(), cfg.Config.Profile, args[0]); err != nil {
			return err
		}
		return emit(stdout, cfg.Config.Output, map[string]string{"symbol": strings.ToUpper(args[0]), "status": "added"}, nil)
	}})
	command.AddCommand(&cobra.Command{Use: "rm SYMBOL", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		db, cfg, err := openLocalStore(cmd.Context(), options)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if err := db.RemoveWatchlist(cmd.Context(), cfg.Config.Profile, args[0]); err != nil {
			return err
		}
		return emit(stdout, cfg.Config.Output, map[string]string{"symbol": strings.ToUpper(args[0]), "status": "removed"}, nil)
	}})
	return command
}

func newAlertsCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "alerts", Short: "Manage local price and funding alerts"}
	command.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		db, cfg, err := openLocalStore(cmd.Context(), options)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		values, err := db.Alerts(cmd.Context(), cfg.Config.Profile)
		return emit(stdout, cfg.Config.Output, values, err)
	}})
	add := &cobra.Command{Use: "add SYMBOL CONDITION TARGET", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		db, cfg, err := openLocalStore(cmd.Context(), options)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		cooldown, _ := cmd.Flags().GetDuration("cooldown")
		alert := store.Alert{Profile: cfg.Config.Profile, Symbol: args[0], Condition: args[1], Target: args[2], Cooldown: cooldown}
		if err := db.AddAlert(cmd.Context(), alert); err != nil {
			return err
		}
		return emit(stdout, cfg.Config.Output, map[string]string{"symbol": strings.ToUpper(args[0]), "condition": args[1], "target": args[2], "status": "added"}, nil)
	}}
	add.Flags().Duration("cooldown", 5*time.Minute, "minimum time between repeated triggers")
	command.AddCommand(add)
	command.AddCommand(&cobra.Command{Use: "rm ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid alert ID: %w", err)
		}
		db, cfg, err := openLocalStore(cmd.Context(), options)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if err := db.RemoveAlert(cmd.Context(), cfg.Config.Profile, id); err != nil {
			return err
		}
		return emit(stdout, cfg.Config.Output, map[string]any{"id": id, "status": "removed"}, nil)
	}})
	return command
}

func newFuturesDataCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "futures-data SYMBOL", Short: "Show futures positioning and open-interest history", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, product, result, err := newMarketService(options)
		if err != nil {
			return err
		}
		kind, _ := cmd.Flags().GetString("kind")
		period, _ := cmd.Flags().GetString("period")
		limit, _ := cmd.Flags().GetInt("limit")
		data, err := service.FuturesData(cmd.Context(), product, strings.ToUpper(args[0]), kind, period, limit)
		return emit(stdout, result.Config.Output, data, err)
	}}
	command.Flags().String("kind", "global-account", "oi-history, top-account, top-position, global-account, taker")
	command.Flags().String("period", "1h", "analytics period: 5m, 15m, 30m, 1h, 2h, 4h, 6h, 12h, 1d")
	command.Flags().Int("limit", 30, "number of records (1-500)")
	return command
}

func openLocalStore(ctx context.Context, options *globalOptions) (*store.Store, config.Result, error) {
	result, err := loadConfig(options)
	if err != nil {
		return nil, config.Result{}, &exitError{code: 2, err: err}
	}
	path, err := store.DefaultPath()
	if err != nil {
		return nil, config.Result{}, err
	}
	db, err := store.Open(path)
	if err != nil {
		return nil, config.Result{}, fmt.Errorf("open local database: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = db.Close()
		return nil, config.Result{}, err
	}
	return db, result, nil
}
