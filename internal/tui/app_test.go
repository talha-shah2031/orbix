package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/talha-shah2031/orbix/internal/binance/models"
	marketdata "github.com/talha-shah2031/orbix/internal/core/market"
	"github.com/talha-shah2031/orbix/internal/tui/components"
)

type fakeService struct{}

func (fakeService) Ticker(context.Context, marketdata.Product, string, string) ([]models.Ticker24h, error) {
	return nil, nil
}
func (fakeService) SpotTickers(context.Context, marketdata.SpotExchange) ([]models.Ticker24h, error) {
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

func TestTUIKeepsOnlySpotMarketTab(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 120, 40
	require.Equal(t, SpotMarket, model.view)
	require.Equal(t, marketdata.Spot, model.product)
	rendered := model.View()
	require.Contains(t, rendered, "1 Spot Market")
	require.NotContains(t, rendered, "Dashboard")
	require.NotContains(t, rendered, "Markets")
	require.NotContains(t, rendered, "Futures")
	model.view = Dashboard
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model = asModel(t, updated)
	require.Equal(t, SpotMarket, model.view, "key 1 should open the only tab")
	for _, keyRune := range []rune{'2', '3', '4', '5', '6', '7', '8', '9'} {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{keyRune}})
		model = asModel(t, updated)
		require.Equal(t, SpotMarket, model.view, "key %q must not open a removed tab", keyRune)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	model = asModel(t, updated)
	require.False(t, model.paletteOpen, "command palette must not expose removed views")
	model.selectView(Dashboard)
	require.Equal(t, SpotMarket, model.view, "selectView must reject removed views")
}

func TestViewsFitCommonTerminalSizes(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.USDM, "BTCUSDT")
	model.tickers["BTCUSDT"] = models.Ticker24h{Symbol: "BTCUSDT"}
	require.Equal(t, marketdata.Spot, model.product, "single-tab TUI should stay on Spot regardless of requested market")
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		model.width, model.height = size.width, size.height
		rendered := model.View()
		require.Contains(t, rendered, "Spot Market")
		require.NotContains(t, rendered, "SPOT MARKET  /  24H MOVERS")
		require.NotContains(t, rendered, "Dashboard")
		for _, line := range strings.Split(rendered, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), size.width, "line exceeds %d columns: %q", size.width, line)
		}
		require.LessOrEqual(t, lipgloss.Height(rendered), size.height, "view exceeds %d terminal rows", size.height)
	}
}

func TestSpotMarketCountsAllSelectedQuotePairsOutsideCurrentPage(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 160, 45
	model.view = SpotMarket
	for index := 0; index < 105; index++ {
		change := decimal.Zero
		switch index % 3 {
		case 0:
			change = decimal.NewFromInt(1)
		case 1:
			change = decimal.NewFromInt(-1)
		}
		symbol := fmt.Sprintf("C%03dUSDT", index)
		model.tickers[symbol] = models.Ticker24h{Symbol: symbol, PriceChangePercent: change, Volume: decimal.NewFromInt(int64(105 - index))}
	}
	model.tickers["BTCBTC"] = models.Ticker24h{Symbol: "BTCBTC", PriceChangePercent: decimal.NewFromInt(2)}

	rendered := model.spotMarketView(35)
	require.Contains(t, rendered, "+ 35 GAIN")
	require.Contains(t, rendered, "− 35 LOSS")
	require.Contains(t, rendered, "35 FLAT")
	require.NotContains(t, rendered, "0 35 FLAT")
	require.NotContains(t, rendered, "\n  + GAIN")

	model.spotQuote = "BTC"
	rendered = model.spotMarketView(35)
	require.Contains(t, rendered, "+ 1 GAIN")
	require.Contains(t, rendered, "− 0 LOSS")
	require.Contains(t, rendered, "0 FLAT")
}

func TestSpotMarketStatusAndControlsShareOneFooterRow(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 140, 40
	model.loading = false
	model.tickers["BTCUSDT"] = models.Ticker24h{Symbol: "BTCUSDT", PriceChangePercent: decimal.NewFromInt(1)}
	rendered := model.View()
	footerRows := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "q quote") {
			footerRows++
			require.Contains(t, line, "t theme")
			require.Contains(t, line, "ctrl+c/q quit")
			require.NotContains(t, line, "REST polling")
			require.NotContains(t, line, "SPOT")
		}
	}
	require.Equal(t, 1, footerRows, "keyboard hints should use one row below the heatmap")
}

func TestSpotMarketShowsSpinnerUntilTickerDataArrives(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.width, model.height = 100, 30
	loadingView := model.View()
	require.Contains(t, loadingView, "Fetching Binance Spot market data")
	firstFrame := model.loader.View()
	updated, _ := model.Update(model.loader.Tick())
	model = asModel(t, updated)
	require.NotEqual(t, firstFrame, model.loader.View(), "spinner should animate while the request is pending")

	updated, _ = model.Update(marketDataMsg{
		product: marketdata.Spot,
		tickers: []models.Ticker24h{{Symbol: "BTCUSDT", PriceChangePercent: decimal.NewFromInt(1)}},
	})
	model = asModel(t, updated)
	require.False(t, model.loading)
	require.NotContains(t, model.View(), "Fetching Binance Spot market data")
}

func TestSpotMarketSwitchesBetweenTreemapAndSmallGridOnResize(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.view = SpotMarket
	model.loading = false
	for i := 0; i < 100; i++ {
		symbol := fmt.Sprintf("C%03dUSDT", i)
		model.tickers[symbol] = models.Ticker24h{
			Symbol:             symbol,
			QuoteVolume:        decimal.NewFromInt(int64(10000 - i*50)),
			PriceChangePercent: decimal.NewFromInt(int64(i%21 - 10)),
		}
	}
	for _, size := range []struct{ width, height int }{{120, 35}, {200, 50}, {80, 24}} {
		model.width, model.height = size.width, size.height
		model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		require.Equal(t, size.width, model.width)
		require.Equal(t, size.height, model.height)
		require.Equal(t, size.width < 120 || size.height < 35, components.UsesHeatmapGrid(model.width, model.height, model.spotMode))
		rendered := model.View()
		if size.width < 120 || size.height < 35 {
			require.Contains(t, rendered, "Terminal small: grid mode")
			require.Contains(t, rendered, "QUOTE USDT")
			require.NotContains(t, rendered, "SIZE 24H CHANGE")
		} else {
			require.Contains(t, rendered, "SIZE 24H CHANGE")
			require.Contains(t, rendered, "QUOTE USDT")
			require.NotContains(t, rendered, "Terminal small: grid mode")
		}
		require.LessOrEqual(t, lipgloss.Height(rendered), size.height)
		for _, line := range strings.Split(rendered, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), size.width)
		}
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	state := asModel(t, updated)
	require.Equal(t, components.HeatmapAuto, state.spotMode, "layout toggle was removed")

	previousTheme := state.themeIndex
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	state = asModel(t, updated)
	require.Equal(t, (previousTheme+1)%4, state.themeIndex, "theme toggle should remain available")
}

func TestSpotMarketRanksByVolumeButSizesByPercentChange(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	for i := 0; i < 100; i++ {
		symbol := fmt.Sprintf("C%03dUSDT", i)
		change := decimal.NewFromFloat(0.5)
		if i == 1 {
			change = decimal.NewFromInt(5)
		}
		model.tickers[symbol] = models.Ticker24h{
			Symbol:             symbol,
			Volume:             decimal.NewFromInt(int64(10000 - i)),
			QuoteVolume:        decimal.NewFromInt(int64(i + 1)),
			PriceChangePercent: change,
		}
	}
	tiles := model.spotMarketTiles()
	require.Len(t, tiles, 100)
	require.Equal(t, "C000", tiles[0].Name, "highest base-asset volume should rank first")
	require.Equal(t, "C001", tiles[1].Name)
	require.Greater(t, tiles[1].Weight, tiles[0].Weight, "larger 24h move should get the larger tile despite lower base-asset volume")
}

func TestSpotMarketQuoteFilteringAndPaging(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.view = SpotMarket
	model.symbols = []models.SymbolInfo{
		{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING"},
		{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "TRADING"},
		{Symbol: "ETHBTC", BaseAsset: "ETH", QuoteAsset: "BTC", Status: "TRADING"},
		{Symbol: "ADAUSDC", BaseAsset: "ADA", QuoteAsset: "USDC", Status: "TRADING"},
		{Symbol: "BNBETH", BaseAsset: "BNB", QuoteAsset: "ETH", Status: "TRADING"},
		{Symbol: "SOLFDUSD", BaseAsset: "SOL", QuoteAsset: "FDUSD", Status: "TRADING"},
		{Symbol: "HALTEDUSDT", BaseAsset: "HALTED", QuoteAsset: "USDT", Status: "BREAK"},
	}
	for i := 0; i < 205; i++ {
		symbol := fmt.Sprintf("C%03dUSDT", i)
		model.tickers[symbol] = models.Ticker24h{Symbol: symbol, Volume: decimal.NewFromInt(int64(1000 - i)), QuoteVolume: decimal.NewFromInt(int64(i + 1))}
		model.symbols = append(model.symbols, models.SymbolInfo{Symbol: symbol, BaseAsset: fmt.Sprintf("C%03d", i), QuoteAsset: "USDT", Status: "TRADING"})
	}
	model.tickers["BTCUSDT"] = models.Ticker24h{Symbol: "BTCUSDT", Volume: decimal.NewFromInt(2000), QuoteVolume: decimal.NewFromInt(1)}
	model.tickers["ETHUSDT"] = models.Ticker24h{Symbol: "ETHUSDT", Volume: decimal.NewFromInt(1500), QuoteVolume: decimal.NewFromInt(2)}
	model.tickers["ETHBTC"] = models.Ticker24h{Symbol: "ETHBTC", Volume: decimal.NewFromInt(500), QuoteVolume: decimal.NewFromInt(3)}
	model.tickers["ADAUSDC"] = models.Ticker24h{Symbol: "ADAUSDC", Volume: decimal.NewFromInt(600), QuoteVolume: decimal.NewFromInt(4)}
	model.tickers["BNBETH"] = models.Ticker24h{Symbol: "BNBETH", Volume: decimal.NewFromInt(700), QuoteVolume: decimal.NewFromInt(5)}
	model.tickers["SOLFDUSD"] = models.Ticker24h{Symbol: "SOLFDUSD", Volume: decimal.NewFromInt(800), QuoteVolume: decimal.NewFromInt(6)}
	model.tickers["HALTEDUSDT"] = models.Ticker24h{Symbol: "HALTEDUSDT", Volume: decimal.NewFromInt(100), QuoteVolume: decimal.NewFromInt(7000)}

	require.Equal(t, []string{"USDT", "BTC", "USDC", "ETH"}, model.spotQuoteOptions())
	ranked, total := model.rankedSpotMarketTiles()
	require.Equal(t, 208, total, "count every USDT-suffixed ticker even if exchange metadata marks it inactive")
	haltedIncluded := false
	for _, coin := range ranked {
		if coin.base == "HALTED" {
			haltedIncluded = true
			break
		}
	}
	require.True(t, haltedIncluded, "USDT suffix match should not be removed by exchange status metadata")
	firstPage := model.spotMarketTiles()
	require.Len(t, firstPage, 100)
	require.Equal(t, "BTC", firstPage[0].Name)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	model = asModel(t, updated)
	require.Equal(t, 1, model.spotPage)
	require.Len(t, model.spotMarketTiles(), 100)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = asModel(t, updated)
	require.Nil(t, cmd, "q should change quote instead of quitting in Spot Market")
	require.Equal(t, "BTC", model.spotQuote)
	require.Equal(t, 0, model.spotPage, "changing quote token returns to first range")
	require.Equal(t, []string{"ETH"}, []string{model.spotMarketTiles()[0].Name})

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = asModel(t, updated)
	require.Equal(t, "USDC", model.spotQuote)
	require.Equal(t, []string{"ADA"}, []string{model.spotMarketTiles()[0].Name})
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = asModel(t, updated)
	require.Equal(t, "ETH", model.spotQuote)
	require.Equal(t, []string{"BNB"}, []string{model.spotMarketTiles()[0].Name})
}

func TestLowercaseQStillQuitsOutsideSpotMarket(t *testing.T) {
	model := New(context.Background(), fakeService{}, marketdata.Spot, "")
	model.view = Dashboard
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd)
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
