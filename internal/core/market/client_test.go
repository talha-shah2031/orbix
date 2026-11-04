package market

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestNewRESTServiceAndAdapterReadOnlyDispatch(t *testing.T) {
	require.Equal(t, "spot", string(toEndpointProduct(Spot)))
	require.Equal(t, "usdm", string(toEndpointProduct(USDM)))
	require.Equal(t, "coinm", string(toEndpointProduct(COINM)))
	require.Equal(t, "options", string(toEndpointProduct(Options)))
	require.Equal(t, "other", string(toEndpointProduct(Product("other"))))
	_, err := NewRESTService(RESTOptions{BaseURLs: map[Product][]string{Spot: {"https://testnet.binance.vision"}}})
	require.Error(t, err)
	var requests int
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		body := `{}`
		if request.URL.Path == "/api/v3/time" {
			body = `{"serverTime":1700000000000}`
		}
		if request.URL.Path == "/api/v3/account" {
			require.Equal(t, "key", request.Header.Get("X-MBX-APIKEY"))
			require.NotEmpty(t, request.URL.Query().Get("signature"))
			body = `{"canTrade":false,"canWithdraw":false,"balances":[]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	service, err := NewRESTService(RESTOptions{APIKey: "key", APISecret: "secret", HTTPClient: &http.Client{Transport: transport}, Timeout: time.Second})
	require.NoError(t, err)
	_, err = service.Account(context.Background(), Spot)
	require.NoError(t, err)
	require.Equal(t, 2, requests, "signed account call must first synchronize server time")
	adapter, ok := service.client.(restAdapter)
	require.True(t, ok)
	err = adapter.Do(context.Background(), Spot, "/api/v3/order", nil, &models.Depth{})
	require.Error(t, err, "unlisted trading routes must be rejected before HTTP")
	require.Equal(t, 2, requests)
}
