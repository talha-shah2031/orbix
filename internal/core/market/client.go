package market

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/talha-shah2031/orbix/internal/binance/endpoints"
	"github.com/talha-shah2031/orbix/internal/binance/rest"
)

type RESTOptions struct {
	BaseURLs       map[Product][]string
	HTTPClient     *http.Client
	Timeout        time.Duration
	MaxAttempts    int
	RetryBaseDelay time.Duration
	RecvWindow     time.Duration
	APIKey         string
	APISecret      string
}

func NewRESTService(options RESTOptions) (*Service, error) {
	baseURLs := make(map[endpoints.Product][]string, len(options.BaseURLs))
	for product, values := range options.BaseURLs {
		baseURLs[toEndpointProduct(product)] = values
	}
	client, err := rest.NewClient(rest.ClientOptions{BaseURLs: baseURLs, HTTPClient: options.HTTPClient, Timeout: options.Timeout, MaxAttempts: options.MaxAttempts, RetryBaseDelay: options.RetryBaseDelay, RecvWindow: options.RecvWindow})
	if err != nil {
		return nil, err
	}
	return New(restAdapter{client: client, apiKey: options.APIKey, apiSecret: options.APISecret}), nil
}

type restAdapter struct {
	client            *rest.Client
	apiKey, apiSecret string
}

func (adapter restAdapter) Do(ctx context.Context, product Product, path string, query url.Values, out any) error {
	definition, err := endpoints.Lookup(toEndpointProduct(product), "GET", path)
	if err != nil {
		return err
	}
	if definition.Signed {
		return adapter.client.DoSigned(ctx, toEndpointProduct(product), path, query, adapter.apiKey, adapter.apiSecret, out)
	}
	if definition.APIKey {
		return adapter.client.DoWithAPIKey(ctx, toEndpointProduct(product), path, query, adapter.apiKey, out)
	}
	return adapter.client.Do(ctx, toEndpointProduct(product), path, query, out)
}
func (adapter restAdapter) SyncTime(ctx context.Context) error { return adapter.client.SyncTime(ctx) }
func (adapter restAdapter) ClockOffset() time.Duration         { return adapter.client.ClockOffset() }

func toEndpointProduct(product Product) endpoints.Product {
	switch product {
	case Spot:
		return endpoints.Spot
	case USDM:
		return endpoints.USDM
	case COINM:
		return endpoints.COINM
	case Options:
		return endpoints.Options
	default:
		return endpoints.Product(product)
	}
}
