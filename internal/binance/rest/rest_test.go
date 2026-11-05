package rest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/talha-shah2031/orbix/internal/binance/endpoints"
	"github.com/talha-shah2031/orbix/internal/binance/models"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func response(request *http.Request, status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: request}
}

func testClient(t *testing.T, transport http.RoundTripper, attempts int) *Client {
	t.Helper()
	client, err := NewClient(ClientOptions{
		BaseURLs:   map[endpoints.Product][]string{endpoints.Spot: {"https://api.binance.com"}},
		HTTPClient: &http.Client{Transport: transport}, MaxAttempts: attempts, RetryBaseDelay: time.Nanosecond,
	})
	require.NoError(t, err)
	return client
}

func TestSigningMatchesHMACSHA256(t *testing.T) {
	query := "symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559"
	secret := "NhqPtmdSJYdKjVHjA7PZj4Mge3R5YNiP1e3UZjInClVN65XAbvqqM6A7H5fATj0j"
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(query))
	require.NoError(t, err)
	require.Equal(t, "c8db56825ae71d6d79447849e617115f4a920fa2acdcab2b053c4b2838bd6b71", Sign(query, secret))
	require.Equal(t, hex.EncodeToString(mac.Sum(nil)), Sign(query, secret))
	require.Len(t, Sign(query, secret), 64)
}

func TestRejectTestnetAndUntrustedHosts(t *testing.T) {
	for _, base := range []string{"https://testnet.binance.vision", "http://api.binance.com", "https://evil.example"} {
		_, err := NewClient(ClientOptions{BaseURLs: map[endpoints.Product][]string{endpoints.Spot: {base}}})
		require.Error(t, err, base)
	}
}

func TestDoDecodesExactDecimalAndTracksWeight(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "/api/v3/ticker/price", request.URL.Path)
		require.Equal(t, "BTCUSDT", request.URL.Query().Get("symbol"))
		headers := make(http.Header)
		headers.Set("X-MBX-USED-WEIGHT-1M", "17")
		return response(request, http.StatusOK, `{"symbol":"BTCUSDT","price":"67123.00000001"}`, headers), nil
	})
	client := testClient(t, transport, 1)
	var value models.PriceTicker
	err := client.Do(context.Background(), endpoints.Spot, "/api/v3/ticker/price", url.Values{"symbol": {"BTCUSDT"}}, &value)
	require.NoError(t, err)
	require.Equal(t, "67123.00000001", value.Price.String())
	require.Equal(t, 17, client.RateUsage(endpoints.Spot).Used)
}

func TestAPIKeyOnlyEndpointAndGeoGuidance(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v3/historicalTrades", request.URL.Path)
		require.Equal(t, "read-only-key", request.Header.Get("X-MBX-APIKEY"))
		require.Empty(t, request.URL.Query().Get("signature"))
		return response(request, http.StatusOK, `[]`, nil), nil
	})
	client := testClient(t, transport, 1)
	var rows []models.Trade
	require.NoError(t, client.DoWithAPIKey(context.Background(), endpoints.Spot, "/api/v3/historicalTrades", url.Values{"symbol": {"BTCUSDT"}}, "read-only-key", &rows))
	require.Empty(t, rows)
	require.Contains(t, GeoBlockMessage(), "data-api.binance.vision")
}

func TestAPIKeyAndSigningRequirements(t *testing.T) {
	client := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("must reject before HTTP"); return nil, io.EOF }), 1)
	require.Error(t, client.Do(context.Background(), endpoints.Spot, "/api/v3/historicalTrades", nil, nil))
	require.Error(t, client.DoSigned(context.Background(), endpoints.Spot, "/api/v3/ping", nil, "key", "secret", nil))
	require.Equal(t, client.clockOffset(), client.ClockOffset())
	require.ErrorContains(t, client.DoSigned(context.Background(), endpoints.Spot, "/api/v3/account", nil, "", "", nil), "requires API key credentials")
}

func TestRecordedSpotFixturesDecode(t *testing.T) {
	fixtures := []struct {
		file   string
		target any
	}{
		{"spot_price.json", &models.PriceTicker{}},
		{"spot_depth.json", &models.Depth{}},
		{"spot_klines.json", &[]models.Kline{}},
		{"spot_exchange_info.json", &models.ExchangeInfo{}},
	}
	for _, fixture := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", fixture.file))
		require.NoError(t, err)
		require.NoError(t, decodeJSON(data, fixture.target), fixture.file)
	}
}

func TestDoRejectsNonAllowlistedBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, nil }), 1)
	err := client.Do(context.Background(), endpoints.Spot, "/api/v3/order", nil, nil)
	require.ErrorIs(t, err, ErrEndpointNotAllowed)
	require.Zero(t, calls.Load())
}

func TestRetries5xxAndUsesFailover(t *testing.T) {
	var calls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return response(request, http.StatusServiceUnavailable, `{"code":-1000,"msg":"temporary"}`, nil), nil
		}
		return response(request, http.StatusOK, `{}`, nil), nil
	})
	client := testClient(t, transport, 2)
	var value map[string]any
	require.NoError(t, client.Do(context.Background(), endpoints.Spot, "/api/v3/ping", nil, &value))
	require.Equal(t, int32(2), calls.Load())
}

func Test418BanPreventsFollowupRequest(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		headers := make(http.Header)
		headers.Set("Retry-After", "2")
		return response(request, http.StatusTeapot, `{"code":-1003,"msg":"IP banned"}`, headers), nil
	}), 1)
	err := client.Do(context.Background(), endpoints.Spot, "/api/v3/ping", nil, nil)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 2*time.Second, apiErr.RetryAfter)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, client.Do(ctx, endpoints.Spot, "/api/v3/ping", nil, nil), context.Canceled)
	require.Equal(t, int32(1), calls.Load())
}

func TestTypedErrors(t *testing.T) {
	for _, test := range []struct {
		status, code int
		message      string
		check        func(*APIError) bool
	}{
		{429, 0, "slow", (*APIError).IsRateLimit},
		{418, 0, "banned", (*APIError).IsRateLimit},
		{400, -1021, "timestamp outside recvWindow", (*APIError).IsTimestamp},
		{401, -2015, "bad key", (*APIError).IsAuth},
		{451, 0, "restricted location", (*APIError).IsGeoBlocked},
	} {
		payload := `{"code":` + strconv.Itoa(test.code) + `,"msg":"` + test.message + `"}`
		apiErr := parseAPIError(test.status, "2", []byte(payload))
		require.True(t, test.check(apiErr), apiErr.Error())
		if test.status == 429 {
			require.Equal(t, 2*time.Second, apiErr.RetryAfter)
		}
	}
}

func TestSignedReadOnlyCallAndKeyRedaction(t *testing.T) {
	var calls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v3/time":
			return response(request, http.StatusOK, `{"serverTime":1700000000000}`, nil), nil
		case "/api/v3/account":
			calls.Add(1)
			require.Equal(t, "read-key", request.Header.Get("X-MBX-APIKEY"))
			require.NotEmpty(t, request.URL.Query().Get("signature"))
			return response(request, http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected endpoint %s", request.URL.Path)
			return nil, io.EOF
		}
	})
	client := testClient(t, transport, 1)
	err := client.DoSigned(context.Background(), endpoints.Spot, "/api/v3/account", nil, "read-key", "secret-value", &map[string]any{})
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
}

func TestPeriodicTimeSyncStopsWithContext(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(request, http.StatusOK, `{"serverTime":1700000000000}`, nil), nil
	}), 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := client.StartTimeSync(ctx, time.Hour, nil)
	deadline := time.After(time.Second)
	for calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("time sync did not make its initial request")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("time sync goroutine did not stop")
	}
}

func TestAPIErrorRedactsSignature(t *testing.T) {
	err := parseAPIError(400, "", []byte(`{"code":-1,"msg":"signature=ultra-secret&bad"}`))
	require.NotContains(t, err.Error(), "ultra-secret")
	require.Contains(t, err.Error(), "[REDACTED]")
}
