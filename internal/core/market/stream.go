package market

import (
	"fmt"
	"strings"

	binancews "github.com/talha-shah2031/orbix/internal/binance/ws"
)

type StreamManager = binancews.Manager
type StreamSubscription = binancews.Subscription
type StreamEvent = binancews.Event
type StreamPolicy = binancews.Policy
type UserDataManager = binancews.UserDataManager

const (
	StreamDropOldest       = binancews.DropOldest
	StreamResyncOnOverflow = binancews.ResyncOnOverflow
)

func NewStreamManager(product Product) (*StreamManager, error) {
	var urls []string
	switch product {
	case Spot:
		urls = append(urls, "wss://stream.binance.com:9443", "wss://stream.binance.com:443", "wss://data-stream.binance.vision")
	case USDM:
		urls = []string{"wss://fstream.binance.com"}
	case COINM:
		urls = []string{"wss://dstream.binance.com"}
	case Options:
		urls = []string{"wss://nbstream.binance.com/eoptions"}
	default:
		return nil, fmt.Errorf("unsupported market %q", product)
	}
	return binancews.New(binancews.Options{URLs: urls})
}

func NewUserDataManager(product Product, apiKey string) (*UserDataManager, error) {
	return binancews.NewUserDataManager(binancews.UserDataOptions{Product: string(product), APIKey: apiKey})
}

func StreamName(kind, symbol, interval string) (string, error) {
	symbol = strings.ToLower(strings.TrimSpace(symbol))
	if symbol == "" || strings.ContainsAny(symbol, " /?#@") {
		return "", fmt.Errorf("invalid symbol %q", symbol)
	}
	switch kind {
	case "trade", "ticker", "book":
		name := map[string]string{"trade": "trade", "ticker": "ticker", "book": "bookTicker"}[kind]
		return symbol + "@" + name, nil
	case "depth":
		return symbol + "@depth@100ms", nil
	case "kline":
		if !validInterval(interval) {
			return "", fmt.Errorf("invalid kline interval %q", interval)
		}
		return symbol + "@kline_" + interval, nil
	default:
		return "", fmt.Errorf("unsupported stream type %q", kind)
	}
}

func validInterval(value string) bool {
	allowed := map[string]bool{"1s": true, "1m": true, "3m": true, "5m": true, "15m": true, "30m": true, "1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true, "1d": true, "3d": true, "1w": true, "1M": true}
	return allowed[value]
}
