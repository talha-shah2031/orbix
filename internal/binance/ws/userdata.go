package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

var ErrUserDataConsumerSlow = errors.New("user-data consumer is too slow; refresh the account snapshot")

type UserDataOptions struct {
	Product     string
	APIKey      string
	HTTPClient  *http.Client
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	RotateAfter time.Duration
	Keepalive   time.Duration
	endpoint    string
}

type UserDataManager struct {
	options  UserDataOptions
	endpoint string
	lastErr  error
	mu       sync.RWMutex
}

func NewUserDataManager(options UserDataOptions) (*UserDataManager, error) {
	if options.APIKey == "" {
		return nil, errors.New("API key is required for the futures user-data stream")
	}
	manager := &UserDataManager{options: options}
	switch options.Product {
	case "usdm":
		manager.endpoint = "wss://ws-fapi.binance.com/ws-fapi/v1"
	case "coinm":
		manager.endpoint = "wss://ws-dapi.binance.com/ws-dapi/v1"
	default:
		return nil, fmt.Errorf("user-data WebSocket API is supported for usdm and coinm, not %q", options.Product)
	}
	if options.endpoint != "" {
		parsed, parseErr := url.Parse(options.endpoint)
		if parseErr != nil || parsed.Scheme != "wss" || parsed.Host == "" || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
			return nil, errors.New("test user-data endpoint must use secure loopback only")
		}
		manager.endpoint = options.endpoint
	}
	if options.MinBackoff <= 0 {
		manager.options.MinBackoff = 250 * time.Millisecond
	}
	if options.MaxBackoff <= 0 {
		manager.options.MaxBackoff = 15 * time.Second
	}
	if options.RotateAfter <= 0 || options.RotateAfter >= 24*time.Hour {
		manager.options.RotateAfter = 23*time.Hour + 50*time.Minute
	}
	if options.Keepalive <= 0 || options.Keepalive >= 60*time.Minute {
		manager.options.Keepalive = 45 * time.Minute
	}
	return manager, nil
}

// Run reconnects and starts a read-only user-data subscription; slow consumers force resnapshot and reconnect.
func (m *UserDataManager) Run(ctx context.Context, events chan<- json.RawMessage) error {
	backoff := m.options.MinBackoff
	for ctx.Err() == nil {
		err := m.runConnection(ctx, events)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			err = errors.New("user-data connection closed")
		}
		m.mu.Lock()
		m.lastErr = err
		m.mu.Unlock()
		if sleepErr := sleep(ctx, jitter(backoff)); sleepErr != nil {
			return sleepErr
		}
		backoff = min(backoff*2, m.options.MaxBackoff)
	}
	return ctx.Err()
}

// LastError reports the most recent disconnect or reconnect error, if any.
func (m *UserDataManager) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr
}

func (m *UserDataManager) runConnection(ctx context.Context, events chan<- json.RawMessage) error {
	conn, response, err := websocket.Dial(ctx, m.endpoint, &websocket.DialOptions{HTTPClient: m.options.HTTPClient})
	if response != nil && response.Body != nil {
		defer func() { _ = response.Body.Close() }()
	}
	if err != nil {
		return fmt.Errorf("connect futures user-data WebSocket: %w", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "reconnect") }()
	start := map[string]any{"id": "bnx-start", "method": "userDataStream.start", "params": map[string]string{"apiKey": m.options.APIKey}}
	if err := writeJSON(ctx, conn, start); err != nil {
		return err
	}
	for {
		_, frame, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var response struct {
			ID     string `json:"id"`
			Status int    `json:"status"`
		}
		if err := json.Unmarshal(frame, &response); err != nil {
			return fmt.Errorf("decode user-data start response: %w", err)
		}
		if response.ID != "bnx-start" {
			continue
		}
		if response.Status != 200 {
			return fmt.Errorf("binance rejected user-data stream start with status %d", response.Status)
		}
		break
	}
	rotation := time.NewTimer(m.options.RotateAfter)
	defer rotation.Stop()
	keepalive := time.NewTicker(m.options.Keepalive)
	defer keepalive.Stop()
	readCtx, cancel := context.WithCancel(ctx)
	var readerWG sync.WaitGroup
	frames := make(chan []byte, 64)
	readErr := make(chan error, 1)
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		defer close(frames)
		for {
			_, frame, readFailure := conn.Read(readCtx)
			if readFailure != nil {
				select {
				case readErr <- readFailure:
				default:
				}
				return
			}
			select {
			case frames <- frame:
			case <-readCtx.Done():
				return
			}
		}
	}()
	defer func() { cancel(); readerWG.Wait() }()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rotation.C:
			return errors.New("planned user-data WebSocket rotation")
		case err := <-readErr:
			return err
		case <-keepalive.C:
			ping := map[string]any{"id": "bnx-ping", "method": "userDataStream.ping", "params": map[string]string{"apiKey": m.options.APIKey}}
			if err := writeJSON(ctx, conn, ping); err != nil {
				return err
			}
		case frame, ok := <-frames:
			if !ok {
				return errors.New("user-data WebSocket reader stopped")
			}
			var control struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(frame, &control) == nil && control.ID != "" {
				continue
			}
			select {
			case events <- append(json.RawMessage(nil), frame...):
			default:
				return ErrUserDataConsumerSlow
			}
		}
	}
}

func writeJSON(ctx context.Context, conn *websocket.Conn, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, payload)
}

func UserDataProduct(product string) string { return strings.ToLower(strings.TrimSpace(product)) }
