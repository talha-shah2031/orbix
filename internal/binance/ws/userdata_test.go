package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"go.uber.org/goleak"
)

func TestUserDataManagerStartsReadOnlySubscriptionAndDeliversEvents(t *testing.T) {
	defer goleak.VerifyNone(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer func() { _ = connection.Close(websocket.StatusNormalClosure, "test complete") }()
		_, payload, err := connection.Read(request.Context())
		if err != nil {
			return
		}
		var start struct {
			Method string `json:"method"`
			Params struct {
				APIKey string `json:"apiKey"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &start); err != nil {
			t.Errorf("decode start: %v", err)
			return
		}
		if start.Method != "userDataStream.start" || start.Params.APIKey != "test-key" {
			t.Errorf("unexpected start request: %+v", start)
		}
		if err := connection.Write(request.Context(), websocket.MessageText, []byte(`{"id":"bnx-start","status":200,"result":{"listenKey":"opaque"}}`)); err != nil {
			return
		}
		_ = connection.Write(request.Context(), websocket.MessageText, []byte(`{"e":"ACCOUNT_UPDATE","E":123}`))
		_, _, _ = connection.Read(request.Context())
	}))
	defer server.Close()
	manager, err := NewUserDataManager(UserDataOptions{Product: "usdm", APIKey: "test-key", HTTPClient: server.Client(), endpoint: "wss" + strings.TrimPrefix(server.URL, "https"), RotateAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan json.RawMessage, 1)
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx, events) }()
	select {
	case event := <-events:
		var decoded map[string]any
		if err := json.Unmarshal(event, &decoded); err != nil || decoded["e"] != "ACCOUNT_UPDATE" {
			t.Fatalf("unexpected event %s err=%v", event, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for user-data event")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected context cancellation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("user-data manager did not stop")
	}
}

func TestUserDataManagerRejectsSpotAndMissingKey(t *testing.T) {
	defer goleak.VerifyNone(t)
	if _, err := NewUserDataManager(UserDataOptions{Product: "spot", APIKey: "key"}); err == nil {
		t.Fatal("expected unsupported Spot HMAC stream error")
	}
	if _, err := NewUserDataManager(UserDataOptions{Product: "usdm"}); err == nil {
		t.Fatal("expected missing API key error")
	}
	if _, err := NewUserDataManager(UserDataOptions{Product: "usdm", APIKey: "key", endpoint: "wss://example.com"}); err == nil {
		t.Fatal("expected non-loopback test URL rejection")
	}
}

func TestUserDataProductNormalizationAndInitialError(t *testing.T) {
	manager, err := NewUserDataManager(UserDataOptions{Product: "usdm", APIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	if manager.LastError() != nil {
		t.Fatalf("unexpected initial error: %v", manager.LastError())
	}
	if got := UserDataProduct(" USDm "); got != "usdm" {
		t.Fatalf("unexpected normalized product %q", got)
	}
}
