package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestManagerReceivesCombinedEvent(t *testing.T) {
	defer goleak.VerifyNone(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()
		if !strings.Contains(r.URL.RawQuery, "btcusdt%40trade") && !strings.Contains(r.URL.RawQuery, "btcusdt@trade") {
			t.Errorf("subscription missing from URL: %s", r.URL.RawQuery)
		}
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"stream":"btcusdt@trade","data":{"e":"trade","p":"123.45"}}`))
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	manager, err := New(Options{URLs: []string{"wss" + strings.TrimPrefix(server.URL, "https")}, HTTPClient: server.Client(), MinBackoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := manager.Subscribe("btcusdt@trade", 2, DropOldest)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	select {
	case event := <-sub.Messages:
		if event.Stream != "btcusdt@trade" || string(event.Data) != `{"e":"trade","p":"123.45"}` {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stream event")
	}
	if got := manager.Stats(); got.Messages != 1 || got.State != Live {
		t.Fatalf("unexpected stats: %+v", got)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("manager exited without context cancellation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("manager leaked after cancellation")
	}
}

func TestManagerRejectsNonMainnetURL(t *testing.T) {
	defer goleak.VerifyNone(t)
	if _, err := New(Options{URLs: []string{"wss://stream.testnet.binance.vision"}}); err == nil {
		t.Fatal("expected testnet URL rejection")
	}
	if _, err := New(Options{URLs: []string{"ws://localhost:9000"}}); err == nil {
		t.Fatal("expected non-TLS URL rejection")
	}
}

func TestFanoutSlowConsumerPolicies(t *testing.T) {
	manager, err := New(Options{URLs: []string{"wss://stream.binance.com:9443"}})
	if err != nil {
		t.Fatal(err)
	}
	ticker, err := manager.Subscribe("btcusdt@trade", 1, DropOldest)
	if err != nil {
		t.Fatal(err)
	}
	defer ticker.Close()
	book, err := manager.Subscribe("btcusdt@depth", 1, ResyncOnOverflow)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	ticker.subscriber.ch <- Event{Stream: "btcusdt@trade", Data: []byte("old")}
	book.subscriber.ch <- Event{Stream: "btcusdt@depth", Data: []byte("old")}
	manager.fanout(Event{Stream: "btcusdt@trade", Data: []byte("new")})
	manager.fanout(Event{Stream: "btcusdt@depth", Data: []byte("new")})
	require.Equal(t, "new", string((<-ticker.Messages).Data))
	select {
	case <-book.Resync:
	default:
		t.Fatal("depth overflow must request a resync")
	}
}

func TestRunWaitsForSubscriptionsAndCancels(t *testing.T) {
	manager, err := New(Options{URLs: []string{"wss://stream.binance.com:9443"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("manager did not stop")
	}
	require.GreaterOrEqual(t, jitter(time.Second), time.Second/2)
	require.LessOrEqual(t, jitter(time.Second), time.Second)
	require.NoError(t, sleep(context.Background(), time.Millisecond))
	canceled, stop := context.WithCancel(context.Background())
	stop()
	require.ErrorIs(t, sleep(canceled, time.Hour), context.Canceled)
}

func TestWriteControlEncodesDynamicSubscription(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer func() { _ = connection.Close(websocket.StatusNormalClosure, "done") }()
		_, payload, err := connection.Read(request.Context())
		if err == nil {
			received <- payload
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connection, response, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(server.URL, "https"), &websocket.DialOptions{HTTPClient: server.Client()})
	if response != nil && response.Body != nil {
		defer func() { _ = response.Body.Close() }()
	}
	require.NoError(t, err)
	defer func() { _ = connection.Close(websocket.StatusNormalClosure, "done") }()
	require.NoError(t, writeControl(ctx, connection, "SUBSCRIBE", []string{"btcusdt@trade"}))
	select {
	case payload := <-received:
		require.JSONEq(t, `{"method":"SUBSCRIBE","params":["btcusdt@trade"],"id":1}`, string(payload))
	case <-ctx.Done():
		t.Fatal("timed out waiting for subscription control message")
	}
}
