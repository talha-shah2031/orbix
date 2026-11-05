package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/talha-shah2031/orbix/internal/config"
	"github.com/talha-shah2031/orbix/internal/secrets"
	"github.com/spf13/cobra"
)

type check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func newDoctorCommand(options *globalOptions, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check configuration, credentials, and Binance connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := loadConfig(options)
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			logger, closer, err := newLogger(result.Config)
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			defer func() { _ = closer.Close() }()
			logger.InfoContext(cmd.Context(), "doctor started", "profile", result.Config.Profile)
			checks := runChecks(cmd.Context(), result)
			failed := false
			for _, item := range checks {
				if item.Status == "fail" {
					failed = true
				}
			}
			if result.Config.Output == "json" {
				if err := json.NewEncoder(stdout).Encode(map[string]any{"ok": !failed, "checks": checks}); err != nil {
					return err
				}
			} else {
				for _, item := range checks {
					if _, err := fmt.Fprintf(stdout, "%-5s %-14s %s\n", strings.ToUpper(item.Status), item.Name, item.Detail); err != nil {
						return fmt.Errorf("write diagnostics: %w", err)
					}
				}
			}
			if failed {
				if _, err := fmt.Fprintln(stderr, "doctor found one or more failures"); err != nil {
					return fmt.Errorf("write diagnostic failure: %w", err)
				}
				return &exitError{code: 3, err: errors.New("diagnostic checks failed")}
			}
			return nil
		},
	}
}

func runChecks(ctx context.Context, result config.Result) []check {
	checks := []check{{Name: "runtime", Status: "pass", Detail: runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH}}
	if result.Used {
		if err := config.CheckPermissions(result.Path); err != nil {
			checks = append(checks, check{Name: "config", Status: "warn", Detail: err.Error()})
		} else {
			checks = append(checks, check{Name: "config", Status: "pass", Detail: result.Path})
		}
	} else {
		checks = append(checks, check{Name: "config", Status: "pass", Detail: "defaults (file not created yet)"})
	}
	if _, err := (secrets.EnvStore{}).Get(result.Config.Profile); err == nil {
		checks = append(checks, check{Name: "credentials", Status: "pass", Detail: "environment credentials available"})
	} else {
		checks = append(checks, check{Name: "credentials", Status: "warn", Detail: "no environment credentials; keyring is used by auth commands"})
	}
	checks = append(checks, connectivityCheck(ctx, result.Config))
	return checks
}

func connectivityCheck(ctx context.Context, cfg config.Config) check {
	base := cfg.Binance.SpotBaseURLs[0]
	parsed, err := url.Parse(base)
	if err != nil {
		return check{Name: "connectivity", Status: "fail", Detail: err.Error()}
	}
	if parsed.Scheme != "https" || strings.Contains(strings.ToLower(parsed.Host), "testnet") {
		return check{Name: "connectivity", Status: "fail", Detail: "only HTTPS mainnet endpoints are allowed"}
	}
	requestCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(base, "/")+"/api/v3/ping", nil)
	if err != nil {
		return check{Name: "connectivity", Status: "fail", Detail: err.Error()}
	}
	client := &http.Client{Timeout: cfg.Timeout}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return check{Name: "connectivity", Status: "fail", Detail: err.Error()}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return check{Name: "connectivity", Status: "fail", Detail: response.Status}
	}
	return check{Name: "connectivity", Status: "pass", Detail: fmt.Sprintf("%s (%s)", parsed.Host, time.Since(started).Round(time.Millisecond))}
}
