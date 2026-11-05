package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/talha-shah2031/orbix/internal/binance/endpoints"
)

var ErrEndpointNotAllowed = endpoints.ErrEndpointNotAllowed

type ClientOptions struct {
	BaseURLs       map[endpoints.Product][]string
	HTTPClient     *http.Client
	Timeout        time.Duration
	MaxAttempts    int
	RetryBaseDelay time.Duration
	RecvWindow     time.Duration
	Limiter        *Limiter
}

type Client struct {
	baseURLs    map[endpoints.Product][]string
	httpClient  *http.Client
	timeout     time.Duration
	maxAttempts int
	retryDelay  time.Duration
	recvWindow  time.Duration
	limiter     *Limiter
	mu          sync.RWMutex
	offset      time.Duration
	clockSynced bool
	rotations   map[endpoints.Product]int
	banUntil    map[endpoints.Product]time.Time
}

var defaultURLs = map[endpoints.Product][]string{
	endpoints.Spot:    {"https://api.binance.com", "https://api-gcp.binance.com", "https://api1.binance.com", "https://api2.binance.com", "https://api3.binance.com", "https://api4.binance.com", "https://data-api.binance.vision"},
	endpoints.USDM:    {"https://fapi.binance.com"},
	endpoints.COINM:   {"https://dapi.binance.com"},
	endpoints.Options: {"https://eapi.binance.com"},
}

func NewClient(options ClientOptions) (*Client, error) {
	if options.Timeout <= 0 {
		options.Timeout = 10 * time.Second
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 3
	}
	if options.RetryBaseDelay <= 0 {
		options.RetryBaseDelay = 200 * time.Millisecond
	}
	if options.RecvWindow <= 0 {
		options.RecvWindow = 5 * time.Second
	}
	if options.Limiter == nil {
		options.Limiter = NewProductLimiter(map[string]int{"spot": 6000, "usdm": 2400, "coinm": 2400, "options": 2400})
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: options.Timeout}
	}
	urls := make(map[endpoints.Product][]string, len(defaultURLs))
	for product, defaults := range defaultURLs {
		configured := options.BaseURLs[product]
		if len(configured) == 0 {
			configured = defaults
		}
		for _, raw := range configured {
			parsed, err := url.Parse(raw)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || strings.Contains(strings.ToLower(parsed.Host), "testnet") {
				return nil, fmt.Errorf("invalid mainnet base URL %q", raw)
			}
			if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || !officialHost(product, strings.ToLower(parsed.Hostname())) {
				return nil, fmt.Errorf("base URL %q is not an official Binance mainnet host for %s", raw, product)
			}
		}
		urls[product] = append([]string(nil), configured...)
	}
	return &Client{baseURLs: urls, httpClient: options.HTTPClient, timeout: options.Timeout, maxAttempts: options.MaxAttempts, retryDelay: options.RetryBaseDelay, recvWindow: options.RecvWindow, limiter: options.Limiter, rotations: make(map[endpoints.Product]int), banUntil: make(map[endpoints.Product]time.Time)}, nil
}

func officialHost(product endpoints.Product, host string) bool {
	switch product {
	case endpoints.Spot:
		switch host {
		case "api.binance.com", "api-gcp.binance.com", "api1.binance.com", "api2.binance.com", "api3.binance.com", "api4.binance.com", "data-api.binance.vision":
			return true
		}
	case endpoints.USDM:
		return host == "fapi.binance.com"
	case endpoints.COINM:
		return host == "dapi.binance.com"
	case endpoints.Options:
		return host == "eapi.binance.com"
	}
	return false
}

func (c *Client) Do(ctx context.Context, product endpoints.Product, path string, query url.Values, out any) error {
	return c.do(ctx, product, path, query, "", "", out)
}

func (c *Client) DoWithAPIKey(ctx context.Context, product endpoints.Product, path string, query url.Values, apiKey string, out any) error {
	return c.do(ctx, product, path, query, apiKey, "", out)
}

func (c *Client) DoSigned(ctx context.Context, product endpoints.Product, path string, query url.Values, apiKey, apiSecret string, out any) error {
	return c.do(ctx, product, path, query, apiKey, apiSecret, out)
}

func (c *Client) do(ctx context.Context, product endpoints.Product, path string, query url.Values, apiKey, apiSecret string, out any) error {
	definition, err := endpoints.Lookup(product, http.MethodGet, path)
	if err != nil {
		return ErrEndpointNotAllowed
	}
	if definition.APIKey && apiKey == "" {
		return errors.New("this public endpoint requires an API key header")
	}
	if definition.Signed && (apiKey == "" || apiSecret == "") {
		return errors.New("this read-only endpoint requires API key credentials")
	}
	if !definition.Signed && apiSecret != "" {
		return errors.New("refusing to sign an endpoint not marked as signed in the registry")
	}
	if definition.Signed && !c.isClockSynced() {
		if err := c.SyncTime(ctx); err != nil {
			return err
		}
	}
	if err := c.waitForBan(ctx, product); err != nil {
		return err
	}
	if apiSecret != "" {
		query = cloneValues(query)
		query.Set("recvWindow", fmt.Sprintf("%d", c.recvWindow.Milliseconds()))
		query.Set("timestamp", fmt.Sprintf("%d", time.Now().Add(c.clockOffset()).UnixMilli()))
		query.Set("signature", Sign(query.Encode(), apiSecret))
	}
	if err := c.limiter.Wait(ctx, string(product), definition.Weight); err != nil {
		return err
	}

	urls := c.baseURLs[product]
	if (definition.APIKey || definition.Signed) && product == endpoints.Spot {
		filtered := make([]string, 0, len(urls))
		for _, base := range urls {
			parsed, parseErr := url.Parse(base)
			if parseErr == nil && parsed.Hostname() != "data-api.binance.vision" {
				filtered = append(filtered, base)
			}
		}
		urls = filtered
	}
	if len(urls) == 0 {
		return fmt.Errorf("no mainnet base URL configured for %s", product)
	}
	start := c.rotation(product) % len(urls)
	var lastErr error
	timestampRetried := false
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		base := urls[(start+attempt)%len(urls)]
		requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
		request, requestErr := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(base, "/")+path+queryString(query), nil)
		if requestErr != nil {
			cancel()
			return fmt.Errorf("create Binance request: %w", requestErr)
		}
		if apiKey != "" {
			request.Header.Set("X-MBX-APIKEY", apiKey)
		}
		request.Header.Set("Accept", "application/json")
		response, requestErr := c.httpClient.Do(request)
		if requestErr != nil {
			cancel()
			lastErr = fmt.Errorf("request %s: %s", path, redact(requestErr.Error()))
			if !isRetryableNetworkError(requestErr) || attempt+1 == c.maxAttempts {
				break
			}
			if err := c.waitRetry(ctx, attempt, 0); err != nil {
				return err
			}
			continue
		}
		c.limiter.Observe(string(product), response.Header)
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 32<<20))
		closeErr := response.Body.Close()
		cancel()
		if readErr != nil {
			return fmt.Errorf("read Binance response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close Binance response: %w", closeErr)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			apiErr := parseAPIError(response.StatusCode, response.Header.Get("Retry-After"), body)
			if apiErr.IsGeoBlocked() {
				apiErr.Message = GeoBlockMessage()
			}
			lastErr = apiErr
			if apiErr.StatusCode == http.StatusTeapot {
				c.mu.Lock()
				c.banUntil[product] = time.Now().Add(apiErr.RetryAfter)
				c.mu.Unlock()
				return apiErr
			}
			if apiErr.IsTimestamp() && apiSecret != "" && !timestampRetried {
				timestampRetried = true
				if syncErr := c.SyncTime(ctx); syncErr == nil && attempt+1 < c.maxAttempts {
					query.Set("timestamp", fmt.Sprintf("%d", time.Now().Add(c.clockOffset()).UnixMilli()))
					query.Set("signature", Sign(queryWithoutSignature(query), apiSecret))
					continue
				}
			}
			if response.StatusCode == http.StatusTooManyRequests && attempt+1 < c.maxAttempts {
				if err := c.waitRetry(ctx, attempt, apiErr.RetryAfter); err != nil {
					return err
				}
				continue
			}
			if response.StatusCode >= 500 && attempt+1 < c.maxAttempts {
				if err := c.waitRetry(ctx, attempt, 0); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if out == nil || len(body) == 0 {
			return nil
		}
		if err := decodeJSON(body, out); err != nil {
			return fmt.Errorf("decode Binance %s response: %w", path, err)
		}
		c.setRotation(product, 0)
		return nil
	}
	return lastErr
}

func (c *Client) SyncTime(ctx context.Context) error {
	var result struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := c.Do(ctx, endpoints.Spot, "/api/v3/time", nil, &result); err != nil {
		return fmt.Errorf("sync Binance clock: %w", err)
	}
	c.mu.Lock()
	c.offset = time.Until(time.UnixMilli(result.ServerTime))
	c.clockSynced = true
	c.mu.Unlock()
	return nil
}

func (c *Client) ClockOffset() time.Duration                { return c.clockOffset() }
func (c *Client) RateUsage(product endpoints.Product) Usage { return c.limiter.Usage(string(product)) }
func (c *Client) clockOffset() time.Duration                { c.mu.RLock(); defer c.mu.RUnlock(); return c.offset }
func (c *Client) isClockSynced() bool                       { c.mu.RLock(); defer c.mu.RUnlock(); return c.clockSynced }

// StartTimeSync synchronizes immediately and then at the requested interval until ctx ends.
func (c *Client) StartTimeSync(ctx context.Context, interval time.Duration, onError func(error)) <-chan struct{} {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := c.SyncTime(ctx); err != nil && onError != nil {
			onError(err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.SyncTime(ctx); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
	return done
}

func (c *Client) rotation(product endpoints.Product) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rotations[product]
}
func (c *Client) setRotation(product endpoints.Product, value int) {
	c.mu.Lock()
	c.rotations[product] = value
	c.mu.Unlock()
}

func (c *Client) waitForBan(ctx context.Context, product endpoints.Product) error {
	c.mu.RLock()
	until := c.banUntil[product]
	c.mu.RUnlock()
	delay := time.Until(until)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) waitRetry(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delay := c.retryDelay * time.Duration(1<<attempt)
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	delay += time.Duration(rand.Int64N(int64(delay/2 + 1)))
	if retryAfter > delay {
		delay = retryAfter
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded)
}

func queryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

func cloneValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, entries := range values {
		clone[key] = append([]string(nil), entries...)
	}
	return clone
}

func queryWithoutSignature(query url.Values) string {
	values := cloneValues(query)
	values.Del("signature")
	return values.Encode()
}

func decodeJSON(body []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON data")
		}
		return err
	}
	return nil
}
