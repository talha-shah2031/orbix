package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"

	"github.com/talha-shah2031/orbix/internal/cli"
	marketdata "github.com/talha-shah2031/orbix/internal/core/market"
	"github.com/talha-shah2031/orbix/internal/secrets"
	"github.com/talha-shah2031/orbix/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runTUI := func(ctx context.Context, service *marketdata.Service, product marketdata.Product, symbol, profile string) error {
		runCtx, cancel := context.WithCancel(ctx)
		var events chan json.RawMessage
		var streamDone chan struct{}
		if product == marketdata.USDM || product == marketdata.COINM {
			credentials, credentialErr := (secrets.FallbackStore{Primary: secrets.KeyringStore{}, Fallback: secrets.EnvStore{}}).Get(profile)
			if credentialErr == nil {
				manager, managerErr := marketdata.NewUserDataManager(product, credentials.APIKey)
				if managerErr == nil {
					events = make(chan json.RawMessage, 128)
					streamDone = make(chan struct{})
					go func() { defer close(streamDone); defer close(events); _ = manager.Run(runCtx, events) }()
				}
			}
		}
		err := tui.RunProfileWithUserData(runCtx, service, product, symbol, profile, events, os.Stdin, os.Stdout)
		cancel()
		if streamDone != nil {
			<-streamDone
		}
		return err
	}
	if err := cli.ExecuteWithTUI(ctx, cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, runTUI); err != nil {
		return cli.ExitCode(err)
	}
	return 0
}
