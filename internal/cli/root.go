package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/talha-shah2031/orbix/internal/config"
	marketdata "github.com/talha-shah2031/orbix/internal/core/market"
	"github.com/talha-shah2031/orbix/internal/logging"
	"github.com/spf13/cobra"
)

type BuildInfo struct{ Version, Commit, Date string }

type TUIRunner func(context.Context, *marketdata.Service, marketdata.Product, string, string) error

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func ExitCode(err error) int {
	var target *exitError
	if errors.As(err, &target) {
		return target.code
	}
	return 1
}

func Execute(ctx context.Context, build BuildInfo) error {
	return execute(ctx, build, nil)
}

func ExecuteWithTUI(ctx context.Context, build BuildInfo, runner TUIRunner) error {
	return execute(ctx, build, runner)
}

func execute(ctx context.Context, build BuildInfo, runner TUIRunner) error {
	command := NewRootCommand(build, os.Stdout, os.Stderr, runner)
	command.SetArgs(os.Args[1:])
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, logging.Redact(err.Error()))
		return err
	}
	return nil
}

type globalOptions struct {
	configFile string
	profile    string
	market     string
	output     string
	noColor    bool
	logLevel   string
	timeout    time.Duration
	baseURL    string
}

func NewRootCommand(build BuildInfo, stdout, stderr io.Writer, tuiRunner ...TUIRunner) *cobra.Command {
	options := &globalOptions{}
	var runTUI TUIRunner
	if len(tuiRunner) > 0 {
		runTUI = tuiRunner[0]
	}
	root := &cobra.Command{
		Use:           "orbix",
		Short:         "Read-only Binance terminal",
		Long:          "orbix is a mainnet-only, read-only Binance terminal and command-line client.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runTUI != nil {
				return launchTUI(cmd.Context(), options, runTUI, "")
			}
			return cmd.Help()
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	flags := root.PersistentFlags()
	flags.StringVar(&options.market, "market", "", "market: spot, usdm, coinm, or options")
	flags.StringVar(&options.profile, "profile", "", "configuration profile")
	flags.StringVarP(&options.output, "output", "o", "", "output: table, json, ndjson, csv, or yaml")
	flags.BoolVar(&options.noColor, "no-color", false, "disable color output")
	flags.StringVar(&options.logLevel, "log-level", "", "log level")
	flags.StringVar(&options.configFile, "config", "", "configuration file")
	flags.DurationVar(&options.timeout, "timeout", 0, "request timeout")
	flags.StringVar(&options.baseURL, "base-url", "", "override the mainnet base URL")

	root.AddCommand(newVersionCommand(build, stdout))
	root.AddCommand(newDoctorCommand(options, stdout, stderr))
	if runTUI != nil {
		root.AddCommand(newTUICommand(options, runTUI))
	}
	for _, command := range newMarketCommands(options, stdout) {
		root.AddCommand(command)
	}
	root.AddCommand(newStreamCommand(options, stdout))
	root.AddCommand(newWatchlistCommand(options, stdout))
	root.AddCommand(newAlertsCommand(options, stdout))
	root.AddCommand(newFuturesDataCommand(options, stdout))
	root.AddCommand(newAccountCommand(options, stdout))
	root.AddCommand(newAuthCommand(options, stdout, stderr))
	root.AddCommand(newExportCommand(options, stdout))
	root.AddCommand(newDiagnosticsCommand(options, stdout))
	root.AddCommand(newCompletionCommand(root, stdout))
	root.AddCommand(newConfigCommand(options, stdout))
	return root
}

func loadConfig(options *globalOptions) (config.Result, error) {
	return config.Load(config.LoadOptions{
		ConfigFile: options.configFile, Profile: options.profile, Market: options.market,
		Output: options.output, NoColor: options.noColor, Timeout: options.timeout,
		BaseURL: options.baseURL, LogLevel: options.logLevel,
	})
}

func newLogger(cfg config.Config) (*slog.Logger, io.Closer, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		return nil, nil, fmt.Errorf("invalid log level: %w", err)
	}
	return logging.New(cfg.Log.Path, level)
}
