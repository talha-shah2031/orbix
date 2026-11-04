package market

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeREST struct {
	raw    map[string]string
	calls  []string
	offset time.Duration
}

func (f *fakeREST) Do(_ context.Context, product Product, path string, query url.Values, out any) error {
	f.calls = append(f.calls, string(product)+" "+path+"?"+query.Encode())
	return json.Unmarshal([]byte(f.raw[path]), out)
}
func (f *fakeREST) SyncTime(context.Context) error {
	f.calls = append(f.calls, "sync")
	f.offset = -42 * time.Millisecond
	return nil
}
func (f *fakeREST) ClockOffset() time.Duration { return f.offset }

func TestTickerUsesSingleObjectAndRollingWindow(t *testing.T) {
	client := &fakeREST{raw: map[string]string{"/api/v3/ticker": `{"symbol":"BTCUSDT","lastPrice":"100.10","priceChangePercent":"1.2"}`}}
	values, err := New(client).Ticker(context.Background(), Spot, "BTCUSDT", "1h")
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, "100.1", values[0].LastPrice.String())
	require.Contains(t, client.calls[0], "windowSize=1h")
}

func TestBookTickersDecodesAllSymbols(t *testing.T) {
	client := &fakeREST{raw: map[string]string{"/api/v3/ticker/bookTicker": `[{"symbol":"BTCUSDT","bidPrice":"10.10","bidQty":"2","askPrice":"10.20","askQty":"3"}]`}}
	values, err := New(client).BookTickers(context.Background(), Spot)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, "BTCUSDT", values[0].Symbol)
	require.Equal(t, "10.1", values[0].BidPrice.String())
	require.Equal(t, "10.2", values[0].AskPrice.String())
	_, err = New(client).BookTickers(context.Background(), Options)
	require.Error(t, err)
}

func TestTimeSynchronizesBeforeReportingOffset(t *testing.T) {
	client := &fakeREST{raw: map[string]string{"/api/v3/time": `{"serverTime":1700000000000}`}}
	report, err := New(client).Time(context.Background(), Spot)
	require.NoError(t, err)
	require.Equal(t, int64(1700000000000), report.ServerTime)
	require.Equal(t, int64(-42), report.ClockOffsetMS)
	require.Equal(t, []string{"sync", "spot /api/v3/time?"}, client.calls)
}

func TestOptionExchangeInfoFiltersLocally(t *testing.T) {
	client := &fakeREST{raw: map[string]string{"/eapi/v1/exchangeInfo": `{"optionSymbols":[{"symbol":"BTC-271231-100000-C","strikePrice":"100000"},{"symbol":"BTC-271231-90000-C","strikePrice":"90000"}]}`}}
	value, err := New(client).ExchangeInfo(context.Background(), Options, "BTC-271231-100000-C")
	require.NoError(t, err)
	require.Len(t, value.OptionSymbols, 1)
	require.Empty(t, client.calls[0][strings.Index(client.calls[0], "?")+1:])
}

func TestFuturesDataUsesVersionedProductParameters(t *testing.T) {
	client := &fakeREST{raw: map[string]string{"/futures/data/globalLongShortAccountRatio": `[{"symbol":"BTCUSDT","longShortRatio":"1.25","longAccount":"0.55","shortAccount":"0.45","timestamp":1700000000000}]`}}
	values, err := New(client).FuturesData(context.Background(), USDM, "BTCUSDT", "global-account", "1h", 30)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, "1.25", values[0].LongShortRatio.String())
	require.Contains(t, client.calls[0], "symbol=BTCUSDT")
	require.Contains(t, client.calls[0], "period=1h")
}

func TestPublicAndAccountServiceRoutes(t *testing.T) {
	ctx := context.Background()
	client := &fakeREST{raw: map[string]string{
		"/api/v3/ticker/price":          `[{"symbol":"BTCUSDT","price":"100.25"}]`,
		"/fapi/v2/ticker/price":         `{"symbol":"BTCUSDT","price":"100.25"}`,
		"/api/v3/ticker/24hr":           `{"symbol":"BTCUSDT","lastPrice":"100.25"}`,
		"/api/v3/depth":                 `{"lastUpdateId":7,"bids":[["100","2"]],"asks":[["101","3"]]}`,
		"/api/v3/trades":                `[{"id":1,"price":"100.25","qty":"0.1","time":1}]`,
		"/api/v3/klines":                `[[1,"100","102","99","101","10",2,"1000",5,"4","400"]]`,
		"/api/v3/exchangeInfo":          `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING"}]}`,
		"/fapi/v1/premiumIndex":         `{"symbol":"BTCUSDT","markPrice":"100.2","lastFundingRate":"0.001","nextFundingTime":9}`,
		"/fapi/v1/fundingRate":          `[]`,
		"/fapi/v1/openInterest":         `{"symbol":"BTCUSDT","openInterest":"12.5","time":3}`,
		"/futures/data/takerBuySellVol": `[]`,
		"/api/v3/account":               `{"canTrade":false,"canWithdraw":false,"balances":[{"asset":"USDT","free":"10.123456789"}]}`,
		"/api/v3/openOrders":            `[]`,
		"/api/v3/allOrders":             `[]`,
		"/api/v3/myTrades":              `[]`,
		"/fapi/v3/account":              `{"canTrade":false,"assets":[],"positions":[],"totalWalletBalance":"100"}`,
		"/fapi/v3/positionRisk":         `[]`,
		"/fapi/v1/income":               `[]`,
		"/fapi/v1/userTrades":           `[]`,
		"/fapi/v1/openOrders":           `[]`,
		"/fapi/v1/allOrders":            `[]`,
		"/api/v3/ticker/bookTicker":     `[]`,
		"/fapi/v1/ticker/24hr":          `{"symbol":"BTCUSDT","lastPrice":"100.25"}`,
		"/dapi/v1/premiumIndex":         `[]`,
		"/dapi/v1/fundingRate":          `[]`,
		"/eapi/v1/ticker":               `[]`,
	}}
	service := New(client)
	prices, err := service.Price(ctx, Spot, nil)
	require.NoError(t, err)
	require.Equal(t, "100.25", prices[0].Price.String())
	prices, err = service.Price(ctx, USDM, []string{"BTCUSDT"})
	require.NoError(t, err)
	require.Len(t, prices, 1)
	tickers, err := service.Ticker(ctx, Spot, "BTCUSDT", "")
	require.NoError(t, err)
	require.Equal(t, "100.25", tickers[0].LastPrice.String())
	book, err := service.Book(ctx, Spot, "BTCUSDT", 10)
	require.NoError(t, err)
	require.Equal(t, int64(7), book.LastUpdateID)
	trades, err := service.Trades(ctx, Spot, "BTCUSDT", 10)
	require.NoError(t, err)
	require.Len(t, trades, 1)
	klines, err := service.Klines(ctx, Spot, "BTCUSDT", "1h", 10, 1, 2)
	require.NoError(t, err)
	require.Equal(t, "101", klines[0].Close.String())
	info, err := service.ExchangeInfo(ctx, Spot, "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", info.Symbols[0].Symbol)
	funding, err := service.Funding(ctx, USDM, "BTCUSDT", false)
	require.NoError(t, err)
	require.Equal(t, "0.001", funding[0].FundingRate.String())
	_, err = service.Funding(ctx, USDM, "BTCUSDT", true)
	require.NoError(t, err)
	_, err = service.OpenInterest(ctx, USDM, "BTCUSDT")
	require.NoError(t, err)
	_, err = service.FuturesData(ctx, COINM, "BTCUSD", "taker", "", 1)
	require.NoError(t, err)
	spot, err := service.Account(ctx, Spot)
	require.NoError(t, err)
	require.Equal(t, "10.123456789", spot.Balances[0].Free.String())
	_, err = service.OpenOrders(ctx, Spot, "BTCUSDT")
	require.NoError(t, err)
	_, err = service.OrderHistory(ctx, Spot, "BTCUSDT", 10)
	require.NoError(t, err)
	_, err = service.AccountTrades(ctx, Spot, "BTCUSDT", 10)
	require.NoError(t, err)
	futures, err := service.Account(ctx, USDM)
	require.NoError(t, err)
	require.Equal(t, "100", futures.TotalWalletBalance.String())
	_, err = service.Positions(ctx, USDM)
	require.NoError(t, err)
	_, err = service.IncomeHistory(ctx, USDM, "BTCUSDT", 10)
	require.NoError(t, err)
	_, err = service.AccountTrades(ctx, USDM, "BTCUSDT", 10)
	require.NoError(t, err)
	_, err = service.OpenOrders(ctx, USDM, "")
	require.NoError(t, err)
	_, err = service.OrderHistory(ctx, USDM, "BTCUSDT", 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(client.calls), 20)
}

func TestServiceValidationAndProductBranches(t *testing.T) {
	ctx := context.Background()
	client := &fakeREST{raw: map[string]string{
		"/api/v3/ticker/price":      `{"symbol":"BTCUSDT","price":"1"}`,
		"/eapi/v1/ticker":           `[]`,
		"/api/v3/ticker/bookTicker": `{"symbol":"BTCUSDT","bidPrice":"1","askPrice":"2"}`,
		"/fapi/v1/ticker/24hr":      `{"symbol":"BTCUSDT","lastPrice":"1"}`,
		"/dapi/v1/premiumIndex":     `[]`,
		"/dapi/v1/fundingRate":      `[]`,
	}}
	service := New(client)
	_, err := service.Price(ctx, Spot, []string{"BTCUSDT"})
	require.NoError(t, err)
	_, err = service.Price(ctx, Options, []string{"A", "B"})
	require.Error(t, err)
	_, err = service.Price(ctx, Options, nil)
	require.NoError(t, err)
	_, err = service.Price(ctx, Options, []string{"BTC-271231-100000-C"})
	require.NoError(t, err)
	_, err = service.Ticker(ctx, USDM, "BTCUSDT", "1h")
	require.NoError(t, err)
	_, err = service.Book(ctx, Spot, "BTCUSDT", 0)
	require.Error(t, err)
	_, err = service.BookTickers(ctx, Options)
	require.Error(t, err)
	tickers, err := service.BookTickers(ctx, Spot)
	require.NoError(t, err)
	require.Len(t, tickers, 1)
	_, err = service.Trades(ctx, Spot, "BTCUSDT", 0)
	require.Error(t, err)
	_, err = service.Klines(ctx, Spot, "BTCUSDT", "1m", 0, 0, 0)
	require.Error(t, err)
	_, err = service.Funding(ctx, Spot, "BTCUSDT", false)
	require.Error(t, err)
	_, err = service.Funding(ctx, COINM, "BTCUSD_PERP", false)
	require.NoError(t, err)
	_, err = service.Funding(ctx, COINM, "BTCUSD_PERP", true)
	require.NoError(t, err)
	_, err = service.OpenInterest(ctx, Spot, "BTCUSDT")
	require.Error(t, err)
	_, err = service.FuturesData(ctx, Spot, "BTCUSDT", "taker", "1h", 10)
	require.Error(t, err)
	_, err = service.FuturesData(ctx, USDM, "", "taker", "1h", 10)
	require.Error(t, err)
	_, err = service.FuturesData(ctx, USDM, "BTCUSDT", "taker", "1h", 0)
	require.Error(t, err)
	_, err = service.Account(ctx, Options)
	require.Error(t, err)
	_, err = service.OpenOrders(ctx, Options, "")
	require.Error(t, err)
	_, err = service.OrderHistory(ctx, Spot, "", 10)
	require.Error(t, err)
	_, err = service.AccountTrades(ctx, Spot, "", 10)
	require.Error(t, err)
	_, err = service.IncomeHistory(ctx, Spot, "BTCUSDT", 10)
	require.Error(t, err)
}
