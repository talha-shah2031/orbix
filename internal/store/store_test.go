package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/shopspring/decimal"
)

func TestWatchlistAlertsAndCooldownPersist(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bnx.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if addErr := s.AddWatchlist(ctx, "main", "btcusdt"); addErr != nil {
		t.Fatal(addErr)
	}
	if addErr := s.AddWatchlist(ctx, "main", "BTCUSDT"); addErr != nil {
		t.Fatal(addErr)
	}
	items, err := s.Watchlist(ctx, "main")
	if err != nil || len(items) != 1 || items[0] != "BTCUSDT" {
		t.Fatalf("watchlist=%v err=%v", items, err)
	}
	if addErr := s.AddAlert(ctx, Alert{Profile: "main", Symbol: "BTCUSDT", Condition: "above", Target: "100", Cooldown: time.Minute}); addErr != nil {
		t.Fatal(addErr)
	}
	now := time.Now().UTC()
	hit, err := s.Evaluate(ctx, "main", "BTCUSDT", "above", "101", now)
	if err != nil || len(hit) != 1 {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	hit, err = s.Evaluate(ctx, "main", "BTCUSDT", "above", "101", now.Add(30*time.Second))
	if err != nil || len(hit) != 0 {
		t.Fatalf("cooldown hit=%v err=%v", hit, err)
	}
	if closeErr := s.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	alerts, err := s.Alerts(ctx, "main")
	if err != nil || len(alerts) != 1 || alerts[0].LastTriggered.IsZero() {
		t.Fatalf("alerts=%+v err=%v", alerts, err)
	}
}

func TestKlineCacheTTLAndPrecision(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	now := time.Now().UTC()
	openTime := int64(1_700_000_000_000)
	candles := []models.Kline{{OpenTime: openTime, Open: decimal.RequireFromString("1.000000000000000001"), Close: decimal.RequireFromString("1.1")}, {OpenTime: openTime + 60_000, Open: decimal.NewFromInt(2)}}
	if putErr := s.PutKlines(ctx, "spot", "BTCUSDT", "1m", candles, now); putErr != nil {
		t.Fatal(putErr)
	}
	var got []models.Kline
	found, err := s.GetKlines(ctx, "spot", "BTCUSDT", "1m", time.Minute, now.Add(time.Second), &got)
	if err != nil || !found || len(got) != 2 {
		t.Fatalf("found=%v got=%+v err=%v", found, got, err)
	}
	if !got[0].Open.Equal(candles[0].Open) {
		t.Fatalf("precision changed: %s", got[0].Open)
	}
	found, err = s.GetKlines(ctx, "spot", "BTCUSDT", "1m", time.Second, now.Add(2*time.Second), &got)
	if err != nil || found {
		t.Fatalf("stale cache found=%v err=%v", found, err)
	}
}

func TestAddAlertRejectsUnsupportedCondition(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "alert.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if addErr := s.AddAlert(context.Background(), Alert{Profile: "main", Symbol: "BTCUSDT", Condition: "order", Target: "1"}); addErr == nil {
		t.Fatal("expected unsupported condition error")
	}
}
