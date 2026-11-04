package market

import (
	"testing"

	"github.com/shopspring/decimal"
)

func decimals(values ...int64) []decimal.Decimal {
	out := make([]decimal.Decimal, len(values))
	for i, v := range values {
		out[i] = decimal.NewFromInt(v)
	}
	return out
}

func TestIndicatorsReferenceValues(t *testing.T) {
	values := decimals(1, 2, 3, 4, 5)
	sma, err := SMA(values, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !sma[2].Equal(decimal.NewFromInt(2)) || !sma[4].Equal(decimal.NewFromInt(4)) {
		t.Fatalf("SMA=%v", sma)
	}
	ema, err := EMA(values, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !ema[2].Equal(decimal.NewFromInt(2)) || !ema[3].Equal(decimal.RequireFromString("3")) || !ema[4].Equal(decimal.RequireFromString("4")) {
		t.Fatalf("EMA=%v", ema)
	}
	rsi, err := RSI(values, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !rsi[3].Equal(decimal.NewFromInt(100)) || !rsi[4].Equal(decimal.NewFromInt(100)) {
		t.Fatalf("RSI=%v", rsi)
	}
	flat, err := RSI(decimals(5, 5, 5, 5), 2)
	if err != nil || !flat[2].Equal(decimal.NewFromInt(50)) {
		t.Fatalf("flat RSI=%v err=%v", flat, err)
	}
	bands, err := Bollinger(decimals(1, 2, 3), 3, decimal.NewFromInt(2))
	if err != nil {
		t.Fatal(err)
	}
	if !bands[2].Middle.Equal(decimal.NewFromInt(2)) || bands[2].Upper.LessThan(decimal.NewFromInt(3)) || bands[2].Lower.GreaterThan(decimal.NewFromInt(1)) {
		t.Fatalf("bands=%+v", bands[2])
	}
}

func TestIndicatorsRejectInvalidPeriods(t *testing.T) {
	if _, err := SMA(decimals(1, 2), 0); err == nil {
		t.Fatal("expected period error")
	}
	if _, err := RSI(decimals(1, 2), 2); err == nil {
		t.Fatal("expected RSI period error")
	}
}

func TestRSIClassicReferenceSeries(t *testing.T) {
	prices := []decimal.Decimal{}
	for _, raw := range []string{"44.34", "44.09", "44.15", "43.61", "44.33", "44.83", "45.10", "45.42", "45.84", "46.08", "45.89", "46.03", "45.61", "46.28", "46.28", "46.00", "46.03", "46.41"} {
		prices = append(prices, decimal.RequireFromString(raw))
	}
	values, err := RSI(prices, 14)
	if err != nil {
		t.Fatal(err)
	}
	if !values[14].Round(2).Equal(decimal.RequireFromString("70.46")) {
		t.Fatalf("classic RSI reference = %s, want 70.46", values[14])
	}
}
