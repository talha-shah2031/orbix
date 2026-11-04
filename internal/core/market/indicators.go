package market

import (
	"errors"

	"github.com/shopspring/decimal"
)

var ErrInvalidIndicatorPeriod = errors.New("indicator period must be positive and no greater than the series length")

type Bands struct{ Middle, Upper, Lower decimal.Decimal }

func SMA(values []decimal.Decimal, period int) ([]decimal.Decimal, error) {
	if period < 1 || period > len(values) {
		return nil, ErrInvalidIndicatorPeriod
	}
	result := make([]decimal.Decimal, len(values))
	sum := decimal.Zero
	for i, value := range values {
		sum = sum.Add(value)
		if i >= period {
			sum = sum.Sub(values[i-period])
		}
		if i >= period-1 {
			result[i] = sum.Div(decimal.NewFromInt(int64(period)))
		}
	}
	return result, nil
}

func EMA(values []decimal.Decimal, period int) ([]decimal.Decimal, error) {
	if period < 1 || period > len(values) {
		return nil, ErrInvalidIndicatorPeriod
	}
	result := make([]decimal.Decimal, len(values))
	initial, err := SMA(values[:period], period)
	if err != nil {
		return nil, err
	}
	result[period-1] = initial[period-1]
	alpha := decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(period + 1)))
	oneMinus := decimal.NewFromInt(1).Sub(alpha)
	for i := period; i < len(values); i++ {
		result[i] = values[i].Mul(alpha).Add(result[i-1].Mul(oneMinus))
	}
	return result, nil
}

func Bollinger(values []decimal.Decimal, period int, deviations decimal.Decimal) ([]Bands, error) {
	means, err := SMA(values, period)
	if err != nil {
		return nil, err
	}
	if deviations.IsNegative() {
		return nil, errors.New("standard deviation multiplier cannot be negative")
	}
	result := make([]Bands, len(values))
	denom := decimal.NewFromInt(int64(period))
	for i := period - 1; i < len(values); i++ {
		variance := decimal.Zero
		mean := means[i]
		for _, value := range values[i-period+1 : i+1] {
			delta := value.Sub(mean)
			variance = variance.Add(delta.Mul(delta))
		}
		std := decimalSqrt(variance.Div(denom))
		offset := std.Mul(deviations)
		result[i] = Bands{Middle: mean, Upper: mean.Add(offset), Lower: mean.Sub(offset)}
	}
	return result, nil
}

func decimalSqrt(value decimal.Decimal) decimal.Decimal {
	if !value.IsPositive() {
		return decimal.Zero
	}
	guess := value
	if guess.LessThan(decimal.NewFromInt(1)) {
		guess = decimal.NewFromInt(1)
	} else {
		guess = guess.Div(decimal.NewFromInt(2))
	}
	two := decimal.NewFromInt(2)
	for iteration := 0; iteration < 64; iteration++ {
		next := guess.Add(value.Div(guess)).Div(two)
		if next.Equal(guess) {
			return next
		}
		guess = next
	}
	return guess
}

// RSI uses Wilder's smoothed average gains and losses, with a neutral 50 for a flat series.
func RSI(values []decimal.Decimal, period int) ([]decimal.Decimal, error) {
	if period < 1 || period >= len(values) {
		return nil, ErrInvalidIndicatorPeriod
	}
	result := make([]decimal.Decimal, len(values))
	gain, loss := decimal.Zero, decimal.Zero
	for i := 1; i <= period; i++ {
		delta := values[i].Sub(values[i-1])
		if delta.IsPositive() {
			gain = gain.Add(delta)
		} else {
			loss = loss.Sub(delta)
		}
	}
	periodD := decimal.NewFromInt(int64(period))
	gain = gain.Div(periodD)
	loss = loss.Div(periodD)
	result[period] = rsiValue(gain, loss)
	for i := period + 1; i < len(values); i++ {
		delta := values[i].Sub(values[i-1])
		up, down := decimal.Zero, decimal.Zero
		if delta.IsPositive() {
			up = delta
		} else {
			down = delta.Abs()
		}
		gain = gain.Mul(periodD.Sub(decimal.NewFromInt(1))).Add(up).Div(periodD)
		loss = loss.Mul(periodD.Sub(decimal.NewFromInt(1))).Add(down).Div(periodD)
		result[i] = rsiValue(gain, loss)
	}
	return result, nil
}

func rsiValue(gain, loss decimal.Decimal) decimal.Decimal {
	if gain.IsZero() && loss.IsZero() {
		return decimal.NewFromInt(50)
	}
	if loss.IsZero() {
		return decimal.NewFromInt(100)
	}
	relative := gain.Div(loss)
	return decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(relative)))
}
