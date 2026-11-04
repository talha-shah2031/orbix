package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	marketdata "github.com/bubblenexus/bnx/internal/core/market"
	"github.com/spf13/cobra"
)

func newStreamCommand(options *globalOptions, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "stream trade|kline|depth|ticker|book SYMBOL...", Short: "Stream public market events as NDJSON", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		result, err := loadConfig(options)
		if err != nil {
			return &exitError{code: 2, err: err}
		}
		product, err := marketProduct(result.Config.Market)
		if err != nil {
			return err
		}
		kind := strings.ToLower(args[0])
		interval, _ := cmd.Flags().GetString("interval")
		manager, err := marketdata.NewStreamManager(product)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		encoder := json.NewEncoder(stdout)
		var subscriptions []*marketdata.StreamSubscription
		for _, symbol := range upperSymbols(args[1:]) {
			name, e := marketdata.StreamName(kind, symbol, interval)
			if e != nil {
				return e
			}
			sub, e := manager.Subscribe(name, 256, marketdata.StreamDropOldest)
			if e != nil {
				return e
			}
			subscriptions = append(subscriptions, sub)
		}
		defer func() {
			for _, sub := range subscriptions {
				sub.Close()
			}
		}()
		events := make(chan marketdata.StreamEvent, 256)
		for _, sub := range subscriptions {
			go func(messages <-chan marketdata.StreamEvent) {
				for {
					select {
					case <-ctx.Done():
						return
					case event, ok := <-messages:
						if !ok {
							return
						}
						select {
						case events <- event:
						case <-ctx.Done():
							return
						}
					}
				}
			}(sub.Messages)
		}
		done := make(chan error, 1)
		go func() { done <- manager.Run(ctx) }()
		for {
			select {
			case <-ctx.Done():
				if runErr := <-done; runErr != nil && ctx.Err() == nil {
					return fmt.Errorf("stop market stream: %w", runErr)
				}
				return nil
			case err := <-done:
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("market stream stopped: %w", err)
			case event := <-events:
				if err := encoder.Encode(event); err != nil {
					cancel()
					if runErr := <-done; runErr != nil && ctx.Err() == nil {
						return fmt.Errorf("stop market stream: %w", runErr)
					}
					return fmt.Errorf("write stream event: %w", err)
				}
			}
		}
	}}
	command.Flags().String("interval", "1m", "kline interval, when streaming klines")
	return command
}
