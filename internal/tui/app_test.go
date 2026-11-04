package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/bubblenexus/bnx/internal/binance/models"
	marketdata "github.com/bubblenexus/bnx/internal/core/market"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type fakeService struct{}

func (fakeService) Ticker(context.Context, marketdata.Product, string, string) ([]models.Ticker24h, error) {
	return nil, nil
}
func (fakeService) BookTickers(context.Context, marketdata.Product) ([]models.BookTicker, error) {
	return nil, nil
}
func (fakeService) ExchangeInfo(context.Context, marketdata.Product, string) (models.ExchangeInfo, error) {
	return models.ExchangeInfo{}, nil
}
func (fakeService) Klines(context.Context, marketdata.Product, string, string, int, int64, int64) ([]models.Kline, error) {
	return nil, nil
}
func (fakeService) Book(context.Context, marketdata.Product, string, int) (models.Depth, error) {
	return models.Depth{}, nil
}
func (fakeService) Trades(context.Context, marketdata.Product, string, int) ([]models.Trade, error) {
	return nil, nil
}

func TestMarketDataFiltersAndSortsSymbols(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 100, 24
	updated, _ := model.Update(marketDataMsg{
		product: marketdata.Spot,
		symbols: []models.SymbolInfo{
			{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT"},
			{Symbol: "ETHBTC", BaseAsset: "ETH", QuoteAsset: "BTC"},
		},
		tickers: []models.Ticker24h{
			{Symbol: "BTCUSDT", PriceChangePercent: decimal.NewFromInt(3), QuoteVolume: decimal.NewFromInt(100)},
			{Symbol: "ETHBTC", PriceChangePercent: decimal.NewFromInt(1), QuoteVolume: decimal.NewFromInt(500)},
		},
	})
	state := asModel(t, updated)
	require.Len(t, state.rows, 2)
	require.Equal(t, "BTCUSDT", state.rows[0].Symbol)
	state.quoteIndex = 1
	state.filterRows()
	require.Equal(t, []string{"BTCUSDT"}, []string{state.rows[0].Symbol})
	state.quoteIndex = 0
	state.sortBy = 2
	state.filterRows()
	require.Equal(t, "ETHBTC", state.rows[0].Symbol)
	state.showOnly = true
	state.favorites["BTCUSDT"] = true
	state.filterRows()
	require.Equal(t, []string{"BTCUSDT"}, []string{state.rows[0].Symbol})
}

func TestSearchKeyboardAndResponsiveView(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 80, 24
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	state := asModel(t, updated)
	require.True(t, state.searching)
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b', 't', 'c'}})
	state = asModel(t, updated)
	require.Equal(t, "BTC", state.search)
	updated, _ = state.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	state = asModel(t, updated)
	require.Equal(t, 120, state.width)
	require.Equal(t, 40, state.height)
	require.NotEmpty(t, state.View())
	_, cmd := state.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	require.False(t, state.searching)
}

func TestCommandPaletteFiltersAndOpensSymbol(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 80, 24
	model.rows = []models.SymbolInfo{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	state := asModel(t, updated)
	require.True(t, state.paletteOpen)
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("btc")})
	state = asModel(t, updated)
	require.Contains(t, state.paletteView(), "BTCUSDT")
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = asModel(t, updated)
	require.False(t, state.paletteOpen)
	require.Equal(t, Symbol, state.view)
	require.Equal(t, "BTCUSDT", state.symbol)
	require.LessOrEqual(t, lipgloss.Height(state.View()), state.height)
}

func TestViewsFitCommonTerminalSizes(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "BTCUSDT")
	model.tickers["BTCUSDT"] = models.Ticker24h{Symbol: "BTCUSDT"}
	model.book = models.Depth{}
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		model.width, model.height = size.width, size.height
		for view := Dashboard; view <= Help; view++ {
			model.view = view
			rendered := model.View()
			require.NotEmpty(t, rendered, "view %d at %dx%d", view, size.width, size.height)
			for _, line := range strings.Split(rendered, "\n") {
				require.LessOrEqual(t, lipgloss.Width(line), size.width, "view %d line exceeds %d columns: %q", view, size.width, line)
			}
			require.LessOrEqual(t, lipgloss.Height(rendered), size.height, "view %d exceeds %d terminal rows", view, size.height)
		}
	}
}

func TestDashboardUsesMajorSymbolsAndBookTickerPrices(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 140, 40
	model.symbols = []models.SymbolInfo{
		{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING"},
		{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "TRADING"},
		{Symbol: "ODDIDR", BaseAsset: "ODD", QuoteAsset: "IDR", Status: "TRADING"},
		{Symbol: "OLDUSDT", BaseAsset: "OLD", QuoteAsset: "USDT", Status: "BREAK"},
	}
	model.tickers = map[string]models.Ticker24h{
		"BTCUSDT": {Symbol: "BTCUSDT", LastPrice: decimal.RequireFromString("67123.45"), PriceChangePercent: decimal.NewFromInt(2), QuoteVolume: decimal.NewFromInt(1_000_000)},
		"ETHUSDT": {Symbol: "ETHUSDT", LastPrice: decimal.RequireFromString("3210.1"), PriceChangePercent: decimal.NewFromInt(-1), QuoteVolume: decimal.NewFromInt(500_000)},
		"ODDIDR":  {Symbol: "ODDIDR", LastPrice: decimal.NewFromInt(100), QuoteVolume: decimal.NewFromInt(100_000_000)},
		"OLDUSDT": {Symbol: "OLDUSDT", LastPrice: decimal.NewFromInt(1), QuoteVolume: decimal.NewFromInt(100_000_000)},
	}
	model.bookTickers = map[string]models.BookTicker{
		"BTCUSDT": {Symbol: "BTCUSDT", BidPrice: decimal.RequireFromString("67123.40"), AskPrice: decimal.RequireFromString("67123.45")},
	}
	model.sparklines["BTCUSDT"] = []models.Kline{{Close: decimal.NewFromInt(1)}, {Close: decimal.NewFromInt(2)}, {Close: decimal.NewFromInt(3)}}
	rendered := model.dashboard(34)
	require.Contains(t, rendered, "MARKET SUMMARY")
	require.Contains(t, rendered, "BTCUSDT")
	require.Contains(t, rendered, "67,123.45")
	require.Contains(t, rendered, "67,123.4 / 67,123.45")
	require.NotContains(t, rendered, "ODDIDR")
	require.NotContains(t, rendered, "OLDUSDT")
	require.Contains(t, rendered, "TOP 24H MOVERS")
	require.Contains(t, rendered, "HIGHEST QUOTE VOLUME")
}

func TestPriceAndVolumeFormatting(t *testing.T) {
	require.Equal(t, "0.00001235", displayPrice(decimal.RequireFromString("0.000012345")))
	require.Equal(t, "1,234,567.89", displayPrice(decimal.RequireFromString("1234567.89")))
	require.Equal(t, "1.23B", compactDecimal(decimal.RequireFromString("1234567890")))
	require.Equal(t, "+2.50%", signedPercent(decimal.RequireFromString("2.5")))
}

func asModel(t *testing.T, value tea.Model) *Model {
	t.Helper()
	model, ok := value.(*Model)
	require.True(t, ok)
	return model
}
