package components

import (
	"strings"
	"testing"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCandlesUsesRequestedSizeAndPriceDirection(t *testing.T) {
	values := []models.Kline{
		{Open: decimal.NewFromInt(10), High: decimal.NewFromInt(14), Low: decimal.NewFromInt(8), Close: decimal.NewFromInt(12)},
		{Open: decimal.NewFromInt(12), High: decimal.NewFromInt(15), Low: decimal.NewFromInt(9), Close: decimal.NewFromInt(10)},
	}
	chart := Candles(values, 2, 4)
	require.Len(t, strings.Split(chart, "\n"), 4)
	require.Contains(t, chart, "▀")
	require.Contains(t, chart, "▄")
	require.Empty(t, Candles(nil, 10, 4))
}

func TestSparklineScalesTheCloseSeries(t *testing.T) {
	values := []models.Kline{
		{Close: decimal.NewFromInt(10)},
		{Close: decimal.NewFromInt(12)},
		{Close: decimal.NewFromInt(11)},
		{Close: decimal.NewFromInt(14)},
	}
	line := Sparkline(values, 4, lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})
	require.Equal(t, 4, lipgloss.Width(line))
	require.NotEqual(t, "····", line)
	require.Equal(t, "····", Sparkline(nil, 4, lipgloss.AdaptiveColor{}))
}
