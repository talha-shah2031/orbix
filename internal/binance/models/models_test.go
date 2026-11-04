package models

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDepthPreservesDecimalPrecision(t *testing.T) {
	var depth Depth
	err := json.Unmarshal([]byte(`{"lastUpdateId":10,"bids":[["67123.123456789","0.00000001"]],"asks":[]}`), &depth)
	require.NoError(t, err)
	require.Equal(t, "67123.123456789", depth.Bids[0].Price.String())
	require.Equal(t, "0.00000001", depth.Bids[0].Quantity.String())
	encoded, err := json.Marshal(depth)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `67123.123456789`)
}

func TestKlineDecodesDocumentedArrayShape(t *testing.T) {
	var rows []Kline
	err := json.Unmarshal([]byte(`[[1700000000000,"1.10","2.20","0.90","1.80","4.00000001",1700000000999,"7.20",12,"2.25","4.05","0"]]`), &rows)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "1.1", rows[0].Open.String())
	require.Equal(t, "4.00000001", rows[0].Volume.String())
	require.Equal(t, int64(12), rows[0].Trades)
}

func TestMalformedDepthLevelRejected(t *testing.T) {
	var depth Depth
	err := json.Unmarshal([]byte(`{"lastUpdateId":1,"bids":[["1"]],"asks":[]}`), &depth)
	require.Error(t, err)
}

func TestExchangeFilterRoundTripKeepsOnlyDocumentedFields(t *testing.T) {
	input := `{"timezone":"UTC","serverTime":1,"symbols":[{"symbol":"ABCUSDT","filters":[{"filterType":"PRICE_FILTER","minPrice":"0.01","maxPrice":"100","tickSize":"0.01"}]}]}`
	var info ExchangeInfo
	require.NoError(t, json.Unmarshal([]byte(input), &info))
	require.Equal(t, "0.01", info.Symbols[0].Filters[0].TickSize.String())
	encoded, err := json.Marshal(info)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	symbols, ok := decoded["symbols"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, symbols)
	symbol, ok := symbols[0].(map[string]any)
	require.True(t, ok)
	filters, ok := symbol["filters"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, filters)
	filter, ok := filters[0].(map[string]any)
	require.True(t, ok)
	require.Len(t, filter, 4)
	require.NotContains(t, filter, "minQty")
}

func TestAccountModelsParsePermissionAndPositionFields(t *testing.T) {
	var account SpotAccount
	err := json.Unmarshal([]byte(`{"canTrade":false,"canWithdraw":false,"canDeposit":true,"balances":[{"asset":"USDT","free":"12.34000000","locked":"0.00000001"}]}`), &account)
	require.NoError(t, err)
	require.False(t, account.CanTrade)
	require.False(t, account.CanWithdraw)
	require.Equal(t, "12.34", account.Balances[0].Free.String())
	require.Equal(t, "0.00000001", account.Balances[0].Locked.String())
	var positions []FuturesPosition
	err = json.Unmarshal([]byte(`[{"symbol":"BTCUSDT","positionAmt":"0.010","entryPrice":"65000.10","markPrice":"65100.20","liquidationPrice":"42000.00","unRealizedProfit":"1.001","leverage":5,"marginType":"cross"}]`), &positions)
	require.NoError(t, err)
	require.Equal(t, "0.01", positions[0].PositionAmt.String())
	require.Equal(t, "1.001", positions[0].UnrealizedProfit.String())
	require.Equal(t, 5, positions[0].Leverage)
}

func TestCustomModelMarshalAndDecodeFailures(t *testing.T) {
	candle := Kline{OpenTime: 1, Open: decimal.RequireFromString("1.2300"), High: decimal.RequireFromString("2"), Low: decimal.RequireFromString("1"), Close: decimal.RequireFromString("1.5"), Volume: decimal.RequireFromString("3"), CloseTime: 2, QuoteVolume: decimal.RequireFromString("4"), Trades: 5, TakerBuyBase: decimal.RequireFromString("1"), TakerBuyQuote: decimal.RequireFromString("1.5")}
	encoded, err := json.Marshal(candle)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"1.23"`)
	var decoded Kline
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, candle.Close, decoded.Close)

	filter := SymbolFilter{Type: "PRICE_FILTER", TickSize: decimal.RequireFromString("0.01")}
	encoded, err = json.Marshal(filter)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"filterType":"PRICE_FILTER"`)
	var preserved SymbolFilter
	require.NoError(t, json.Unmarshal([]byte(`{"filterType":"CUSTOM_FILTER","customField":"keep-me"}`), &preserved))
	encoded, err = json.Marshal(preserved)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"customField":"keep-me"`)

	for _, raw := range []string{`{}`, `[]`, `["bad","1"]`, `["1","bad"]`} {
		var level Level
		require.Error(t, json.Unmarshal([]byte(raw), &level), raw)
	}
	var candleError Kline
	require.Error(t, json.Unmarshal([]byte(`[]`), &candleError))
	require.Error(t, json.Unmarshal([]byte(`[[1]]`), &[]Kline{}))
}
