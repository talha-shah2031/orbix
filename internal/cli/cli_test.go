package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/talha-shah2031/orbix/internal/binance/models"
	marketdata "github.com/talha-shah2031/orbix/internal/core/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := NewRootCommand(BuildInfo{Version: "1.2.3", Commit: "abc", Date: "today"}, &stdout, &stderr)
	command.SetArgs([]string{"version"})
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.Contains(t, stdout.String(), "1.2.3")
}

func TestTUICommandUsesInjectedRunner(t *testing.T) {
	var stdout, stderr bytes.Buffer
	called := false
	runner := func(_ context.Context, service *marketdata.Service, product marketdata.Product, symbol, profile string) error {
		called = true
		require.NotNil(t, service)
		require.Equal(t, marketdata.Spot, product)
		require.Equal(t, "BTCUSDT", symbol)
		require.Equal(t, "main", profile)
		return nil
	}
	command := NewRootCommand(BuildInfo{}, &stdout, &stderr, runner)
	command.SetArgs([]string{"tui", "--symbol", "BTCUSDT"})
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.True(t, called)
	require.Empty(t, stderr.String())
}

func TestLocalWatchlistAndAlertsCommandsPersist(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	var stdout, stderr bytes.Buffer
	command := NewRootCommand(BuildInfo{}, &stdout, &stderr)
	command.SetArgs([]string{"--output", "json", "watchlist", "add", "btcusdt"})
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.Contains(t, stdout.String(), `"symbol":"BTCUSDT"`)
	stdout.Reset()
	command = NewRootCommand(BuildInfo{}, &stdout, &stderr)
	command.SetArgs([]string{"--output", "json", "watchlist", "list"})
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.Contains(t, stdout.String(), `"BTCUSDT"`)
	stdout.Reset()
	command = NewRootCommand(BuildInfo{}, &stdout, &stderr)
	command.SetArgs([]string{"--output", "json", "alerts", "add", "BTCUSDT", "above", "70000"})
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.Contains(t, stdout.String(), `"status":"added"`)
}

func TestAuthStatusNeverPrintsSecrets(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("APPDATA", configDir)
	t.Setenv("LOCALAPPDATA", configDir)
	t.Setenv("ORBIX_API_KEY", "public-key-test-value")
	t.Setenv("ORBIX_API_SECRET", "private-secret-test-value")
	var stdout, stderr bytes.Buffer
	command := NewRootCommand(BuildInfo{}, &stdout, &stderr)
	command.SetArgs([]string{"--output", "json", "auth", "status"})
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.Contains(t, stdout.String(), `"available":true`)
	require.NotContains(t, stdout.String(), "public-key-test-value")
	require.NotContains(t, stdout.String(), "private-secret-test-value")
	require.Empty(t, stderr.String())
}

func TestDoctorJSON(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", configDir)
	t.Setenv("APPDATA", configDir)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/v3/ping", request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{}"))
	}))
	defer server.Close()

	// Doctor intentionally uses the default transport. Trust the local TLS server for this test.
	original := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = original })

	var stdout, stderr bytes.Buffer
	command := NewRootCommand(BuildInfo{}, &stdout, &stderr)
	command.SetArgs([]string{"doctor", "--base-url", "https://api.binance.com", "--output", "json", "--config", filepath.Join(t.TempDir(), "missing.yaml")})
	http.DefaultTransport = rewriteTransport{target: server.URL, next: server.Client().Transport}
	t.Cleanup(func() { http.DefaultTransport = original })
	require.NoError(t, command.ExecuteContext(context.Background()))
	require.Contains(t, stdout.String(), `"ok":true`)
	require.Empty(t, stderr.String())
}

func TestConfigInitAndShowDoNotWriteSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	var output bytes.Buffer
	options := &globalOptions{configFile: path, output: "json"}
	command := newConfigCommand(options, &output)
	command.SetArgs([]string{"init"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), `"created":true`)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(contents), "secret")
	output.Reset()
	command = newConfigCommand(options, &output)
	command.SetArgs([]string{"show"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), `"profile":"main"`)
}

func TestCompletionCommandGeneratesFishScript(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "test"}, io.Discard, io.Discard)
	var output bytes.Buffer
	command := newCompletionCommand(root, &output)
	command.SetArgs([]string{"fish"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "complete -c orbix")
}

func TestKlineCSVExportUsesNamedColumnsAndDecimalStrings(t *testing.T) {
	var output bytes.Buffer
	rows := klineExportRows([]models.Kline{{OpenTime: 123, Open: decimal.RequireFromString("1.23000001"), Close: decimal.RequireFromString("1.24"), Trades: 7}})
	require.NoError(t, writeOutput(&output, "csv", rows))
	require.Contains(t, output.String(), "openTime")
	require.Contains(t, output.String(), "1.23000001")
	require.Contains(t, output.String(), "1.24")
}

type rewriteTransport struct {
	target string
	next   http.RoundTripper
}

func (r rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	target, err := url.Parse(r.target)
	if err != nil {
		return nil, err
	}
	cloned.URL.Scheme = target.Scheme
	cloned.URL.Host = target.Host
	cloned.Host = target.Host
	return r.next.RoundTrip(cloned)
}
