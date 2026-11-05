package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultProfile = "main"

var defaultSpotURLs = []string{
	"https://api.binance.com",
	"https://api-gcp.binance.com",
	"https://api1.binance.com",
	"https://api2.binance.com",
	"https://api3.binance.com",
	"https://api4.binance.com",
	"https://data-api.binance.vision",
}

type Config struct {
	Profile        string             `mapstructure:"profile" yaml:"profile"`
	Market         string             `mapstructure:"market" yaml:"market"`
	Output         string             `mapstructure:"output" yaml:"output"`
	NoColor        bool               `mapstructure:"no_color" yaml:"no_color"`
	Timeout        time.Duration      `mapstructure:"timeout" yaml:"timeout"`
	RetryAttempts  int                `mapstructure:"retry_attempts" yaml:"retry_attempts"`
	RetryBaseDelay time.Duration      `mapstructure:"retry_base_delay" yaml:"retry_base_delay"`
	RecvWindow     time.Duration      `mapstructure:"recv_window" yaml:"recv_window"`
	Log            LogConfig          `mapstructure:"log" yaml:"log"`
	Binance        BinanceConfig      `mapstructure:"binance" yaml:"binance"`
	Profiles       map[string]Profile `mapstructure:"profiles" yaml:"profiles"`
}

type LogConfig struct {
	Level string `mapstructure:"level" yaml:"level"`
	Path  string `mapstructure:"path" yaml:"path"`
}

type BinanceConfig struct {
	SpotBaseURLs    []string `mapstructure:"spot_base_urls" yaml:"spot_base_urls"`
	USDMBaseURLs    []string `mapstructure:"usdm_base_urls" yaml:"usdm_base_urls"`
	COINMBaseURLs   []string `mapstructure:"coinm_base_urls" yaml:"coinm_base_urls"`
	OptionsBaseURLs []string `mapstructure:"options_base_urls" yaml:"options_base_urls"`
}

type Profile struct {
	DefaultMarket string   `mapstructure:"default_market" yaml:"default_market" json:"defaultMarket"`
	KeyReference  string   `mapstructure:"key_reference" yaml:"key_reference" json:"keyReference,omitempty"`
	Watchlist     []string `mapstructure:"watchlist" yaml:"watchlist" json:"watchlist,omitempty"`
}

type LoadOptions struct {
	ConfigFile string
	Profile    string
	Market     string
	Output     string
	NoColor    bool
	Timeout    time.Duration
	BaseURL    string
	LogLevel   string
}

type Result struct {
	Config Config
	Path   string
	Used   bool
}

func Load(options LoadOptions) (Result, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ORBIX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)

	path := options.ConfigFile
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Result{}, err
		}
	}
	v.SetConfigFile(path)
	used := true
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf("read config %q: %w", path, err)
		}
		used = false
	}

	applyOverrides(v, options)
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Result{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	return Result{Config: cfg, Path: path, Used: used}, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("profile", defaultProfile)
	v.SetDefault("market", "spot")
	v.SetDefault("output", "table")
	v.SetDefault("timeout", "10s")
	v.SetDefault("retry_attempts", 3)
	v.SetDefault("retry_base_delay", "200ms")
	v.SetDefault("recv_window", "5s")
	v.SetDefault("log.level", "info")
	v.SetDefault("binance.spot_base_urls", defaultSpotURLs)
	v.SetDefault("binance.usdm_base_urls", []string{"https://fapi.binance.com"})
	v.SetDefault("binance.coinm_base_urls", []string{"https://dapi.binance.com"})
	v.SetDefault("binance.options_base_urls", []string{"https://eapi.binance.com"})
	v.SetDefault("profiles.main.default_market", "spot")
}

func applyOverrides(v *viper.Viper, options LoadOptions) {
	if options.Profile != "" {
		v.Set("profile", options.Profile)
	}
	if options.Market != "" {
		v.Set("market", options.Market)
	}
	if options.Output != "" {
		v.Set("output", options.Output)
	}
	if options.NoColor {
		v.Set("no_color", true)
	}
	if options.Timeout > 0 {
		v.Set("timeout", options.Timeout)
	}
	if options.BaseURL != "" {
		market := options.Market
		if market == "" {
			market = v.GetString("market")
		}
		switch market {
		case "spot":
			v.Set("binance.spot_base_urls", []string{options.BaseURL})
		case "usdm":
			v.Set("binance.usdm_base_urls", []string{options.BaseURL})
		case "coinm":
			v.Set("binance.coinm_base_urls", []string{options.BaseURL})
		case "options":
			v.Set("binance.options_base_urls", []string{options.BaseURL})
		}
	}
	if options.LogLevel != "" {
		v.Set("log.level", options.LogLevel)
	}
}

func (c Config) Validate() error {
	markets := map[string]bool{"spot": true, "usdm": true, "coinm": true, "options": true}
	if !markets[c.Market] {
		return fmt.Errorf("invalid market %q", c.Market)
	}
	outputs := map[string]bool{"table": true, "json": true, "ndjson": true, "csv": true, "yaml": true}
	if !outputs[c.Output] {
		return fmt.Errorf("invalid output %q", c.Output)
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if c.RetryAttempts < 1 || c.RetryAttempts > 10 {
		return errors.New("retry_attempts must be between 1 and 10")
	}
	if c.RetryBaseDelay <= 0 {
		return errors.New("retry_base_delay must be positive")
	}
	if c.RecvWindow <= 0 || c.RecvWindow > 60*time.Second {
		return errors.New("recv_window must be greater than zero and at most 60s")
	}
	for market, urls := range map[string][]string{"Spot": c.Binance.SpotBaseURLs, "USD-M": c.Binance.USDMBaseURLs, "COIN-M": c.Binance.COINMBaseURLs, "Options": c.Binance.OptionsBaseURLs} {
		if len(urls) == 0 {
			return fmt.Errorf("at least one %s base URL is required", market)
		}
		for _, raw := range urls {
			parsed, err := url.Parse(raw)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
				return fmt.Errorf("base URL must be an official HTTPS Binance base URL: %q", raw)
			}
			host := strings.ToLower(parsed.Hostname())
			if strings.Contains(host, "testnet") {
				return fmt.Errorf("testnet URL is forbidden: %q", raw)
			}
			allowedHosts := map[string]map[string]bool{
				"Spot":  {"api.binance.com": true, "api-gcp.binance.com": true, "api1.binance.com": true, "api2.binance.com": true, "api3.binance.com": true, "api4.binance.com": true, "data-api.binance.vision": true},
				"USD-M": {"fapi.binance.com": true}, "COIN-M": {"dapi.binance.com": true}, "Options": {"eapi.binance.com": true},
			}
			if !allowedHosts[market][host] {
				return fmt.Errorf("unrecognized %s mainnet base URL host %q", market, host)
			}
		}
	}
	return nil
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "orbix", "config.yaml"), nil
}

func CheckPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("config permissions %04o are too broad; use 0600", info.Mode().Perm())
	}
	return nil
}
