// Package components provides small terminal-native market visualizations.
package components

import (
	"strings"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
)

// Sparkline renders close prices as a compact, scaled block graph.
func Sparkline(values []models.Kline, width int, color lipgloss.AdaptiveColor) string {
	if width < 1 {
		return ""
	}
	if len(values) == 0 {
		return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("·", width))
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	minimum, maximum := values[0].Close, values[0].Close
	for _, value := range values[1:] {
		if value.Close.LessThan(minimum) {
			minimum = value.Close
		}
		if value.Close.GreaterThan(maximum) {
			maximum = value.Close
		}
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	span := maximum.Sub(minimum)
	var line strings.Builder
	for _, value := range values {
		level := 0
		if !span.IsZero() {
			level = int(value.Close.Sub(minimum).Div(span).Mul(decimal.NewFromInt(int64(len(blocks) - 1))).IntPart())
		}
		line.WriteRune(blocks[level])
	}
	return lipgloss.NewStyle().Foreground(color).Render(line.String())
}

// Candles renders compact OHLC candles as half-block glyphs, scaled to the data range.
func Candles(values []models.Kline, width, height int) string {
	return CandlesWithColors(values, width, height,
		lipgloss.AdaptiveColor{Light: "#16803C", Dark: "#5CE08A"},
		lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"},
	)
}

func CandlesWithColors(values []models.Kline, width, height int, upColor, downColor lipgloss.AdaptiveColor) string {
	if width < 1 || height < 2 || len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	minimum, maximum := values[0].Low, values[0].High
	for _, candle := range values[1:] {
		if candle.Low.LessThan(minimum) {
			minimum = candle.Low
		}
		if candle.High.GreaterThan(maximum) {
			maximum = candle.High
		}
	}
	span := maximum.Sub(minimum)
	if span.IsZero() {
		span = minimum.Abs().Mul(decimal.NewFromInt(1).Div(decimal.NewFromInt(100)))
		if span.IsZero() {
			span = decimal.NewFromInt(1)
		}
		minimum = minimum.Sub(span)
		maximum = maximum.Add(span)
		span = maximum.Sub(minimum)
	}
	rows := make([]strings.Builder, height)
	for _, candle := range values {
		high := scale(candle.High, minimum, span, height*2-1)
		low := scale(candle.Low, minimum, span, height*2-1)
		open := scale(candle.Open, minimum, span, height*2-1)
		closeValue := scale(candle.Close, minimum, span, height*2-1)
		up := candle.Close.GreaterThanOrEqual(candle.Open)
		style := lipgloss.NewStyle().Foreground(upColor)
		glyph := "▀"
		if !up {
			style = lipgloss.NewStyle().Foreground(downColor)
			glyph = "▄"
		}
		for row := height - 1; row >= 0; row-- {
			upper, lower := (height-row-1)*2+1, (height-row-1)*2
			body := (open >= lower && open <= upper) || (closeValue >= lower && closeValue <= upper)
			wick := high >= lower && low <= upper
			cell := " "
			if wick {
				cell = "│"
			}
			if body {
				cell = glyph
			}
			rows[row].WriteString(style.Render(cell))
		}
	}
	lines := make([]string, height)
	for index := range rows {
		lines[index] = rows[index].String()
	}
	return strings.Join(lines, "\n")
}

func scale(value, minimum, span models.Decimal, height int) int {
	return int(value.Sub(minimum).Div(span).Mul(decimal.NewFromInt(int64(height))).IntPart())
}
