// Package endpoints defines the only Binance REST routes the client may call.
package endpoints

import (
	"errors"
	"net/http"
)

type Product string

const (
	Spot    Product = "spot"
	USDM    Product = "usdm"
	COINM   Product = "coinm"
	Options Product = "options"
)

type Definition struct {
	Product Product
	Method  string
	Path    string
	Weight  float64
	APIKey  bool
	Signed  bool
}

var ErrEndpointNotAllowed = errors.New("endpoint is not on the read-only allowlist")

var definitions = buildRegistry()

func Lookup(product Product, method, path string) (Definition, error) {
	if method != http.MethodGet {
		return Definition{}, ErrEndpointNotAllowed
	}
	definition, ok := definitions[routeKey(product, path)]
	if !ok || definition.Method != method {
		return Definition{}, ErrEndpointNotAllowed
	}
	return definition, nil
}

func Allowed() []Definition {
	result := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, definition)
	}
	return result
}

func routeKey(product Product, path string) string { return string(product) + " " + path }

func buildRegistry() map[string]Definition {
	registry := make(map[string]Definition)
	add := func(product Product, weight float64, apiKey bool, paths ...string) {
		for _, path := range paths {
			registry[routeKey(product, path)] = Definition{Product: product, Method: http.MethodGet, Path: path, Weight: weight, APIKey: apiKey}
		}
	}
	signed := func(product Product, paths ...string) {
		for _, path := range paths {
			definition := registry[routeKey(product, path)]
			definition.Signed = true
			registry[routeKey(product, path)] = definition
		}
	}

	add(Spot, 1, false, "/api/v3/ping", "/api/v3/time", "/api/v3/exchangeInfo", "/api/v3/avgPrice", "/api/v3/ticker/price", "/api/v3/ticker/bookTicker", "/api/v3/ticker/tradingDay", "/api/v3/ticker", "/api/v3/ticker/24hr", "/api/v3/depth", "/api/v3/trades", "/api/v3/aggTrades", "/api/v3/klines", "/api/v3/uiKlines")
	add(Spot, 200, true, "/api/v3/historicalTrades")
	add(Spot, 80, false, "/api/v3/ticker")
	add(Spot, 80, false, "/api/v3/ticker/24hr", "/api/v3/ticker/tradingDay")
	add(Spot, 20, false, "/api/v3/ticker/bookTicker", "/api/v3/ticker/price")
	add(Spot, 20, false, "/api/v3/depth")
	add(Spot, 25, false, "/api/v3/klines", "/api/v3/uiKlines")
	add(Spot, 25, false, "/api/v3/aggTrades")
	add(Spot, 5, false, "/api/v3/trades")
	add(Spot, 20, false, "/api/v3/account", "/api/v3/openOrders", "/api/v3/allOrders", "/api/v3/myTrades", "/api/v3/rateLimit/order")
	signed(Spot, "/api/v3/account", "/api/v3/openOrders", "/api/v3/allOrders", "/api/v3/myTrades", "/api/v3/rateLimit/order")

	add(USDM, 1, false, "/fapi/v1/ping", "/fapi/v1/time", "/fapi/v1/exchangeInfo", "/fapi/v1/depth", "/fapi/v1/trades", "/fapi/v1/aggTrades", "/fapi/v1/klines", "/fapi/v1/continuousKlines", "/fapi/v1/indexPriceKlines", "/fapi/v1/markPriceKlines", "/fapi/v1/premiumIndex", "/fapi/v1/fundingRate", "/fapi/v1/fundingInfo", "/fapi/v1/ticker/24hr", "/fapi/v1/ticker/bookTicker", "/fapi/v1/openInterest", "/futures/data/openInterestHist", "/futures/data/topLongShortAccountRatio", "/futures/data/topLongShortPositionRatio", "/futures/data/globalLongShortAccountRatio", "/futures/data/takerlongshortRatio")
	add(USDM, 1, false, "/fapi/v2/ticker/price")
	add(USDM, 200, true, "/fapi/v1/historicalTrades")
	add(USDM, 40, false, "/fapi/v1/ticker/24hr", "/fapi/v1/ticker/bookTicker")
	add(USDM, 10, false, "/fapi/v1/premiumIndex")
	add(USDM, 1, false, "/futures/data/openInterestHist", "/futures/data/topLongShortAccountRatio", "/futures/data/topLongShortPositionRatio", "/futures/data/globalLongShortAccountRatio", "/futures/data/takerlongshortRatio")
	add(USDM, 5, false, "/fapi/v1/trades")
	add(USDM, 20, false, "/fapi/v1/depth", "/fapi/v1/klines", "/fapi/v1/continuousKlines", "/fapi/v1/indexPriceKlines", "/fapi/v1/markPriceKlines", "/fapi/v1/aggTrades")
	add(USDM, 20, false, "/fapi/v3/account", "/fapi/v3/balance", "/fapi/v3/positionRisk", "/fapi/v1/income", "/fapi/v1/userTrades", "/fapi/v1/openOrders", "/fapi/v1/allOrders")
	signed(USDM, "/fapi/v3/account", "/fapi/v3/balance", "/fapi/v3/positionRisk", "/fapi/v1/income", "/fapi/v1/userTrades", "/fapi/v1/openOrders", "/fapi/v1/allOrders")

	add(COINM, 1, false, "/dapi/v1/ping", "/dapi/v1/time", "/dapi/v1/exchangeInfo", "/dapi/v1/depth", "/dapi/v1/trades", "/dapi/v1/aggTrades", "/dapi/v1/klines", "/dapi/v1/continuousKlines", "/dapi/v1/indexPriceKlines", "/dapi/v1/markPriceKlines", "/dapi/v1/premiumIndex", "/dapi/v1/fundingRate", "/dapi/v1/fundingInfo", "/dapi/v1/ticker/24hr", "/dapi/v1/ticker/price", "/dapi/v1/ticker/bookTicker", "/dapi/v1/openInterest", "/futures/data/openInterestHist", "/futures/data/topLongShortAccountRatio", "/futures/data/topLongShortPositionRatio", "/futures/data/globalLongShortAccountRatio", "/futures/data/takerBuySellVol")
	add(COINM, 200, true, "/dapi/v1/historicalTrades")
	add(COINM, 40, false, "/dapi/v1/ticker/24hr", "/dapi/v1/ticker/bookTicker")
	add(COINM, 10, false, "/dapi/v1/premiumIndex")
	add(COINM, 1, false, "/futures/data/openInterestHist", "/futures/data/topLongShortAccountRatio", "/futures/data/topLongShortPositionRatio", "/futures/data/globalLongShortAccountRatio", "/futures/data/takerBuySellVol")
	add(COINM, 5, false, "/dapi/v1/trades")
	add(COINM, 20, false, "/dapi/v1/depth", "/dapi/v1/klines", "/dapi/v1/continuousKlines", "/dapi/v1/indexPriceKlines", "/dapi/v1/markPriceKlines", "/dapi/v1/aggTrades")
	add(COINM, 20, false, "/dapi/v1/account", "/dapi/v1/balance", "/dapi/v1/positionRisk", "/dapi/v1/income", "/dapi/v1/userTrades", "/dapi/v1/openOrders", "/dapi/v1/allOrders")
	signed(COINM, "/dapi/v1/account", "/dapi/v1/balance", "/dapi/v1/positionRisk", "/dapi/v1/income", "/dapi/v1/userTrades", "/dapi/v1/openOrders", "/dapi/v1/allOrders")

	add(Options, 1, false, "/eapi/v1/ping", "/eapi/v1/time", "/eapi/v1/exchangeInfo", "/eapi/v1/depth", "/eapi/v1/trades", "/eapi/v1/klines", "/eapi/v1/mark", "/eapi/v1/ticker", "/eapi/v1/index")
	add(Options, 5, false, "/eapi/v1/mark", "/eapi/v1/trades")
	add(Options, 40, false, "/eapi/v1/ticker")
	add(Options, 10, false, "/eapi/v1/depth", "/eapi/v1/klines")
	add(Options, 5, false, "/eapi/v1/trades", "/eapi/v1/mark")
	return registry
}
