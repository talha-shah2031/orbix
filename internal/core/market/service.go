package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bubblenexus/bnx/internal/binance/models"
)

type Product string

const (
	Spot    Product = "spot"
	USDM    Product = "usdm"
	COINM   Product = "coinm"
	Options Product = "options"
)

type REST interface {
	Do(context.Context, Product, string, url.Values, any) error
	SyncTime(context.Context) error
	ClockOffset() time.Duration
}

type Service struct{ client REST }

func New(client REST) *Service { return &Service{client: client} }

type TimeReport struct {
	ServerTime    int64 `json:"serverTime"`
	ClockOffsetMS int64 `json:"clockOffsetMs"`
}

func (s *Service) Time(ctx context.Context, product Product) (TimeReport, error) {
	if err := s.client.SyncTime(ctx); err != nil {
		return TimeReport{}, err
	}
	path := marketPath(product, "/api/v3/time", "/fapi/v1/time", "/dapi/v1/time", "/eapi/v1/time")
	var result models.ServerTime
	if err := s.client.Do(ctx, product, path, nil, &result); err != nil {
		return TimeReport{}, err
	}
	return TimeReport{ServerTime: result.ServerTime, ClockOffsetMS: s.client.ClockOffset().Milliseconds()}, nil
}

func (s *Service) Price(ctx context.Context, product Product, symbols []string) ([]models.PriceTicker, error) {
	path := marketPath(product, "/api/v3/ticker/price", "/fapi/v2/ticker/price", "/dapi/v1/ticker/price", "/eapi/v1/ticker")
	if product == Options {
		if len(symbols) > 1 {
			return nil, fmt.Errorf("options price accepts one contract symbol at a time")
		}
		tickers, err := s.Ticker(ctx, product, "", "")
		if len(symbols) > 0 {
			tickers, err = s.Ticker(ctx, product, symbols[0], "")
		}
		if err != nil {
			return nil, err
		}
		result := make([]models.PriceTicker, 0, len(tickers))
		for _, item := range tickers {
			result = append(result, models.PriceTicker{Symbol: item.Symbol, Price: item.LastPrice})
		}
		return result, nil
	}
	if len(symbols) == 0 {
		var result []models.PriceTicker
		if err := s.client.Do(ctx, product, path, nil, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	result := make([]models.PriceTicker, 0, len(symbols))
	for _, symbol := range symbols {
		query := url.Values{"symbol": {symbol}}
		if product == Options {
			var tickers []models.Ticker24h
			if err := s.client.Do(ctx, product, path, query, &tickers); err != nil {
				return nil, err
			}
			if len(tickers) > 0 {
				result = append(result, models.PriceTicker{Symbol: tickers[0].Symbol, Price: tickers[0].LastPrice})
			}
			continue
		}
		var value models.PriceTicker
		if err := s.client.Do(ctx, product, path, query, &value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (s *Service) Ticker(ctx context.Context, product Product, symbol, window string) ([]models.Ticker24h, error) {
	path := marketPath(product, "/api/v3/ticker/24hr", "/fapi/v1/ticker/24hr", "/dapi/v1/ticker/24hr", "/eapi/v1/ticker")
	query := url.Values{}
	if symbol != "" {
		query.Set("symbol", symbol)
	}
	if window != "" && product == Spot {
		path = "/api/v3/ticker"
		query.Set("windowSize", window)
	}
	var raw json.RawMessage
	if err := s.client.Do(ctx, product, path, query, &raw); err != nil {
		return nil, err
	}
	var result []models.Ticker24h
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
	} else {
		var single models.Ticker24h
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, err
		}
		result = []models.Ticker24h{single}
	}
	return result, nil
}

func (s *Service) Book(ctx context.Context, product Product, symbol string, depth int) (models.Depth, error) {
	if depth < 1 || depth > 1000 {
		return models.Depth{}, fmt.Errorf("depth must be between 1 and 1000")
	}
	query := url.Values{"symbol": {symbol}, "limit": {strconv.Itoa(depth)}}
	var result models.Depth
	if err := s.client.Do(ctx, product, marketPath(product, "/api/v3/depth", "/fapi/v1/depth", "/dapi/v1/depth", "/eapi/v1/depth"), query, &result); err != nil {
		return models.Depth{}, err
	}
	return result, nil
}

// BookTickers returns the best bid and ask for one symbol, or for all symbols
// when symbol is empty.
func (s *Service) BookTickers(ctx context.Context, product Product) ([]models.BookTicker, error) {
	path := marketPath(product, "/api/v3/ticker/bookTicker", "/fapi/v1/ticker/bookTicker", "/dapi/v1/ticker/bookTicker", "")
	if path == "" {
		return nil, fmt.Errorf("book ticker is not available for %s", product)
	}
	var raw json.RawMessage
	if err := s.client.Do(ctx, product, path, nil, &raw); err != nil {
		return nil, err
	}
	if len(raw) > 0 && raw[0] == '[' {
		var result []models.BookTicker
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("decode book tickers: %w", err)
		}
		return result, nil
	}
	var single models.BookTicker
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, fmt.Errorf("decode book ticker: %w", err)
	}
	return []models.BookTicker{single}, nil
}

func (s *Service) Trades(ctx context.Context, product Product, symbol string, limit int) ([]models.Trade, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("trade limit must be between 1 and 1000")
	}
	query := url.Values{"symbol": {symbol}, "limit": {strconv.Itoa(limit)}}
	var result []models.Trade
	if err := s.client.Do(ctx, product, marketPath(product, "/api/v3/trades", "/fapi/v1/trades", "/dapi/v1/trades", "/eapi/v1/trades"), query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Klines(ctx context.Context, product Product, symbol, interval string, limit int, start, end int64) ([]models.Kline, error) {
	if limit < 1 || limit > 1500 {
		return nil, fmt.Errorf("kline limit must be between 1 and 1500")
	}
	query := url.Values{"symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit)}}
	if start > 0 {
		query.Set("startTime", strconv.FormatInt(start, 10))
	}
	if end > 0 {
		query.Set("endTime", strconv.FormatInt(end, 10))
	}
	var result []models.Kline
	if err := s.client.Do(ctx, product, marketPath(product, "/api/v3/klines", "/fapi/v1/klines", "/dapi/v1/klines", "/eapi/v1/klines"), query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ExchangeInfo(ctx context.Context, product Product, symbol string) (models.ExchangeInfo, error) {
	query := url.Values{}
	if symbol != "" && product != Options {
		query.Set("symbol", symbol)
	}
	var result models.ExchangeInfo
	if err := s.client.Do(ctx, product, marketPath(product, "/api/v3/exchangeInfo", "/fapi/v1/exchangeInfo", "/dapi/v1/exchangeInfo", "/eapi/v1/exchangeInfo"), query, &result); err != nil {
		return models.ExchangeInfo{}, err
	}
	if symbol != "" && product == Options {
		filtered := result.OptionSymbols[:0]
		for _, item := range result.OptionSymbols {
			if item.Symbol == symbol {
				filtered = append(filtered, item)
			}
		}
		result.OptionSymbols = filtered
	}
	return result, nil
}

func (s *Service) Funding(ctx context.Context, product Product, symbol string, history bool) ([]models.FundingRate, error) {
	if product != USDM && product != COINM {
		return nil, fmt.Errorf("funding is available for usdm and coinm markets")
	}
	query := url.Values{}
	if symbol != "" {
		query.Set("symbol", symbol)
	}
	if history {
		query.Set("limit", "100")
		var result []models.FundingRate
		if err := s.client.Do(ctx, product, marketPath(product, "", "/fapi/v1/fundingRate", "/dapi/v1/fundingRate", ""), query, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	var raw json.RawMessage
	if err := s.client.Do(ctx, product, marketPath(product, "", "/fapi/v1/premiumIndex", "/dapi/v1/premiumIndex", ""), query, &raw); err != nil {
		return nil, err
	}
	var values []models.PremiumIndex
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
	} else {
		var value models.PremiumIndex
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		values = []models.PremiumIndex{value}
	}
	result := make([]models.FundingRate, 0, len(values))
	for _, value := range values {
		result = append(result, models.FundingRate{Symbol: value.Symbol, FundingRate: value.LastFundingRate, FundingTime: value.NextFundingTime, MarkPrice: value.MarkPrice})
	}
	return result, nil
}

func (s *Service) OpenInterest(ctx context.Context, product Product, symbol string) (models.OpenInterest, error) {
	if product != USDM && product != COINM {
		return models.OpenInterest{}, fmt.Errorf("open interest is available for usdm and coinm markets")
	}
	query := url.Values{"symbol": {symbol}}
	var result models.OpenInterest
	if err := s.client.Do(ctx, product, marketPath(product, "", "/fapi/v1/openInterest", "/dapi/v1/openInterest", ""), query, &result); err != nil {
		return models.OpenInterest{}, err
	}
	return result, nil
}

// FuturesData reads the documented public futures analytics endpoints.
func (s *Service) FuturesData(ctx context.Context, product Product, symbol, kind, period string, limit int) ([]models.FuturesData, error) {
	if product != USDM && product != COINM {
		return nil, fmt.Errorf("futures analytics is available for usdm and coinm markets")
	}
	if symbol == "" {
		return nil, fmt.Errorf("symbol or pair is required")
	}
	if period == "" {
		period = "1h"
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("futures data limit must be between 1 and 500")
	}
	paths := map[string][2]string{
		"oi-history":     {"/futures/data/openInterestHist", "/futures/data/openInterestHist"},
		"top-account":    {"/futures/data/topLongShortAccountRatio", "/futures/data/topLongShortAccountRatio"},
		"top-position":   {"/futures/data/topLongShortPositionRatio", "/futures/data/topLongShortPositionRatio"},
		"global-account": {"/futures/data/globalLongShortAccountRatio", "/futures/data/globalLongShortAccountRatio"},
		"taker":          {"/futures/data/takerlongshortRatio", "/futures/data/takerBuySellVol"},
	}
	productPaths, ok := paths[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported futures data kind %q", kind)
	}
	path := productPaths[0]
	if product == COINM {
		path = productPaths[1]
	}
	query := url.Values{"period": {period}, "limit": {strconv.Itoa(limit)}}
	if product == COINM {
		query.Set("pair", strings.ToUpper(symbol))
	} else {
		query.Set("symbol", strings.ToUpper(symbol))
	}
	var result []models.FuturesData
	if err := s.client.Do(ctx, product, path, query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

type AccountSnapshot struct {
	CanTrade              bool                     `json:"canTrade"`
	CanWithdraw           bool                     `json:"canWithdraw"`
	CanDeposit            bool                     `json:"canDeposit"`
	Balances              []models.AccountBalance  `json:"balances"`
	Positions             []models.FuturesPosition `json:"positions,omitempty"`
	TotalWalletBalance    models.Decimal           `json:"totalWalletBalance,omitempty"`
	TotalUnrealizedProfit models.Decimal           `json:"totalUnrealizedProfit,omitempty"`
	AvailableBalance      models.Decimal           `json:"availableBalance,omitempty"`
}

func (s *Service) Account(ctx context.Context, product Product) (AccountSnapshot, error) {
	switch product {
	case Spot:
		var account models.SpotAccount
		if err := s.client.Do(ctx, product, "/api/v3/account", nil, &account); err != nil {
			return AccountSnapshot{}, err
		}
		return AccountSnapshot{CanTrade: account.CanTrade, CanWithdraw: account.CanWithdraw, CanDeposit: account.CanDeposit, Balances: account.Balances}, nil
	case USDM, COINM:
		path := marketPath(product, "", "/fapi/v3/account", "/dapi/v1/account", "")
		var account models.FuturesAccount
		if err := s.client.Do(ctx, product, path, nil, &account); err != nil {
			return AccountSnapshot{}, err
		}
		return AccountSnapshot{CanTrade: account.CanTrade, CanWithdraw: account.CanWithdraw, CanDeposit: account.CanDeposit, Balances: account.Assets, Positions: account.Positions, TotalWalletBalance: account.TotalWalletBalance, TotalUnrealizedProfit: account.TotalUnrealizedProfit, AvailableBalance: account.AvailableBalance}, nil
	case Options:
		return AccountSnapshot{}, fmt.Errorf("account balances are not available for %s", product)
	default:
		return AccountSnapshot{}, fmt.Errorf("account balances are not available for %s", product)
	}
}

func (s *Service) OpenOrders(ctx context.Context, product Product, symbol string) ([]models.AccountOrder, error) {
	path := marketPath(product, "/api/v3/openOrders", "/fapi/v1/openOrders", "/dapi/v1/openOrders", "")
	if path == "" {
		return nil, fmt.Errorf("open orders are not available for %s", product)
	}
	query := url.Values{}
	if symbol != "" {
		query.Set("symbol", strings.ToUpper(symbol))
	}
	var result []models.AccountOrder
	if err := s.client.Do(ctx, product, path, query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) OrderHistory(ctx context.Context, product Product, symbol string, limit int) ([]models.AccountOrder, error) {
	if symbol == "" {
		return nil, fmt.Errorf("a symbol is required for order history")
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("order history limit must be between 1 and 1000")
	}
	path := marketPath(product, "/api/v3/allOrders", "/fapi/v1/allOrders", "/dapi/v1/allOrders", "")
	if path == "" {
		return nil, fmt.Errorf("order history is not available for %s", product)
	}
	query := url.Values{"symbol": {strings.ToUpper(symbol)}, "limit": {strconv.Itoa(limit)}}
	var result []models.AccountOrder
	if err := s.client.Do(ctx, product, path, query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) AccountTrades(ctx context.Context, product Product, symbol string, limit int) ([]models.AccountTrade, error) {
	if symbol == "" {
		return nil, fmt.Errorf("a symbol is required for account trade history")
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("account trade limit must be between 1 and 1000")
	}
	path := marketPath(product, "/api/v3/myTrades", "/fapi/v1/userTrades", "/dapi/v1/userTrades", "")
	if path == "" {
		return nil, fmt.Errorf("account trades are not available for %s", product)
	}
	query := url.Values{"symbol": {strings.ToUpper(symbol)}, "limit": {strconv.Itoa(limit)}}
	var result []models.AccountTrade
	if err := s.client.Do(ctx, product, path, query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Positions(ctx context.Context, product Product) ([]models.FuturesPosition, error) {
	path := marketPath(product, "", "/fapi/v3/positionRisk", "/dapi/v1/positionRisk", "")
	if path == "" {
		return nil, fmt.Errorf("positions are available only for futures markets")
	}
	var result []models.FuturesPosition
	if err := s.client.Do(ctx, product, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) IncomeHistory(ctx context.Context, product Product, symbol string, limit int) ([]models.IncomeRecord, error) {
	if product != USDM && product != COINM {
		return nil, fmt.Errorf("income history is available only for futures markets")
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("income limit must be between 1 and 1000")
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}}
	if symbol != "" {
		query.Set("symbol", strings.ToUpper(symbol))
	}
	path := marketPath(product, "", "/fapi/v1/income", "/dapi/v1/income", "")
	var result []models.IncomeRecord
	if err := s.client.Do(ctx, product, path, query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func marketPath(product Product, spot, usdm, coinm, options string) string {
	switch product {
	case Spot:
		return spot
	case USDM:
		return usdm
	case COINM:
		return coinm
	case Options:
		return options
	default:
		return ""
	}
}
