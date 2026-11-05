package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newTUICommand(options *globalOptions, runner TUIRunner) *cobra.Command {
	command := &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive terminal interface",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			symbol, _ := cmd.Flags().GetString("symbol")
			return launchTUI(cmd.Context(), options, runner, symbol)
		},
	}
	command.Flags().String("symbol", "", "filter Spot Market to this symbol at launch")
	return command
}

func launchTUI(ctx context.Context, options *globalOptions, runner TUIRunner, symbol string) error {
	service, product, result, err := newMarketService(options)
	if err != nil {
		return err
	}
	return runner(ctx, service, product, symbol, result.Config.Profile)
}
