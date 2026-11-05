package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/talha-shah2031/orbix/internal/binance/models"
)

// SpotExchange identifies a public spot ticker source used by the Spot Market
// heatmap.
type SpotExchange string

const (
	SpotExchangeBinance SpotExchange = "binance"
	SpotExchangeWEEX    SpotExchange = "weex"
	SpotExchangeMEXC    SpotExchange = "mexc"
)

func (exchange SpotExchange) String() string { return strings.ToUpper(string(exchange)) }

// SpotTickers returns all 24-hour spot tickers for the selected exchange.
// Binance values are already expressed in percentage points. WEEX and MEXC
// return priceChangePercent as a ratio, so those values are converted to
// percentage points before being passed to the shared heatmap.
func (s *Service) SpotTickers(ctx context.Context, exchange SpotExchange) ([]models.Ticker24h, error) {
	if exchange == SpotExchangeBinance {
		return s.Ticker(ctx, Spot, "", "")
	}
	endpoint := ""
	switch exchange {
	case SpotExchangeWEEX:
		endpoint = "https://api-spot.weex.com/api/v3/market/ticker/24hr"
	case SpotExchangeMEXC:
		// MEXC uses 24hr, not the 24h path supplied in the request.
		endpoint = "https://api.mexc.com/api/v3/ticker/24hr"
	default:
		return nil, fmt.Errorf("unsupported spot exchange %q", exchange)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create %s ticker request: %w", exchange, err)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s spot tickers: %w", exchange, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("fetch %s spot tickers: HTTP %s", exchange, response.Status)
	}

	var tickers []models.Ticker24h
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&tickers); err != nil {
		return nil, fmt.Errorf("decode %s spot tickers: %w", exchange, err)
	}
	for index := range tickers {
		tickers[index].PriceChangePercent = tickers[index].PriceChangePercent.Mul(decimal.NewFromInt(100))
	}
	return tickers, nil
}
