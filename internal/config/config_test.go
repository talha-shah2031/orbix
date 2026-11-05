package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ORBIX_MARKET", "")
	result, err := Load(LoadOptions{ConfigFile: filepath.Join(t.TempDir(), "missing.yaml")})
	require.NoError(t, err)
	require.Equal(t, "spot", result.Config.Market)
	require.Equal(t, 10*time.Second, result.Config.Timeout)
	require.False(t, result.Used)
}

func TestPrecedenceFlagEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("market: coinm\ntimeout: 3s\n"), 0o600))
	t.Setenv("ORBIX_MARKET", "usdm")
	result, err := Load(LoadOptions{ConfigFile: path, Market: "options"})
	require.NoError(t, err)
	require.Equal(t, "options", result.Config.Market)
	require.Equal(t, 3*time.Second, result.Config.Timeout)
}

func TestRejectsTestnet(t *testing.T) {
	cfg := Config{Market: "spot", Output: "table", Timeout: time.Second, RetryAttempts: 3, RetryBaseDelay: time.Millisecond, RecvWindow: 5 * time.Second, Binance: BinanceConfig{SpotBaseURLs: []string{"https://testnet.binance.vision"}, USDMBaseURLs: []string{"https://fapi.binance.com"}, COINMBaseURLs: []string{"https://dapi.binance.com"}, OptionsBaseURLs: []string{"https://eapi.binance.com"}}}
	require.ErrorContains(t, cfg.Validate(), "testnet")
}

func TestFlagOverridesBaseURLAndConfigSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orbix.yaml")
	require.NoError(t, os.WriteFile(path, []byte("market: spot\noutput: table\n"), 0o600))
	result, err := Load(LoadOptions{ConfigFile: path, Profile: "sub1", Market: "usdm", Output: "json", NoColor: true, Timeout: 3 * time.Second, BaseURL: "https://fapi.binance.com", LogLevel: "debug"})
	require.NoError(t, err)
	require.Equal(t, "sub1", result.Config.Profile)
	require.Equal(t, "usdm", result.Config.Market)
	require.Equal(t, "json", result.Config.Output)
	require.True(t, result.Config.NoColor)
	require.Equal(t, 3*time.Second, result.Config.Timeout)
	require.Equal(t, "debug", result.Config.Log.Level)
	require.Equal(t, []string{"https://fapi.binance.com"}, result.Config.Binance.USDMBaseURLs)
	require.Equal(t, "https://api.binance.com", result.Config.Binance.SpotBaseURLs[0])
}

func TestConfigValidationAndFilesystemHelpers(t *testing.T) {
	base, err := Load(LoadOptions{})
	require.NoError(t, err)
	invalidCases := []struct {
		name string
		edit func(*Config)
	}{
		{"market", func(cfg *Config) { cfg.Market = "margin" }},
		{"output", func(cfg *Config) { cfg.Output = "xml" }},
		{"timeout", func(cfg *Config) { cfg.Timeout = 0 }},
		{"attempts", func(cfg *Config) { cfg.RetryAttempts = 0 }},
		{"delay", func(cfg *Config) { cfg.RetryBaseDelay = 0 }},
		{"receive window", func(cfg *Config) { cfg.RecvWindow = 61 * time.Second }},
		{"missing spot URL", func(cfg *Config) { cfg.Binance.SpotBaseURLs = nil }},
		{"unsupported product host", func(cfg *Config) { cfg.Binance.USDMBaseURLs = []string{"https://api.binance.com"} }},
		{"URL path", func(cfg *Config) { cfg.Binance.SpotBaseURLs = []string{"https://api.binance.com/api"} }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := base.Config
			testCase.edit(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
	defaultPath, err := DefaultPath()
	require.NoError(t, err)
	require.Contains(t, defaultPath, "orbix")
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	require.Error(t, CheckPermissions(missing))
	file := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte("market: spot"), 0o600))
	require.NoError(t, CheckPermissions(file))
	if runtime.GOOS != "windows" {
		wide := filepath.Join(t.TempDir(), "wide.yaml")
		require.NoError(t, os.WriteFile(wide, []byte("market: spot"), 0o644))
		require.Error(t, CheckPermissions(wide))
	}
}
