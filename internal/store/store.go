// Package store persists local preferences and cache data in an optional SQLite database.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	// The pure-Go SQLite package registers its database/sql driver here.
	_ "modernc.org/sqlite"
)

const schemaVersion = 1

type Store struct{ db *sql.DB }

type Alert struct {
	ID            int64         `json:"id"`
	Profile       string        `json:"profile"`
	Symbol        string        `json:"symbol"`
	Condition     string        `json:"condition"`
	Target        string        `json:"target"`
	Cooldown      time.Duration `json:"cooldown"`
	LastTriggered time.Time     `json:"lastTriggered,omitempty"`
	Enabled       bool          `json:"enabled"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if path != "file::memory:?cache=shared" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bnx", "bnx.sqlite"), nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	statements := []string{
		`CREATE TABLE watchlist (profile TEXT NOT NULL, symbol TEXT NOT NULL, created_at INTEGER NOT NULL, PRIMARY KEY(profile,symbol))`,
		`CREATE TABLE alerts (id INTEGER PRIMARY KEY AUTOINCREMENT, profile TEXT NOT NULL, symbol TEXT NOT NULL, condition TEXT NOT NULL, target TEXT NOT NULL, cooldown_ms INTEGER NOT NULL, last_trigger_ms INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1)`,
		`CREATE INDEX alerts_lookup ON alerts(profile,symbol,enabled)`,
		`CREATE TABLE klines_cache (product TEXT NOT NULL, symbol TEXT NOT NULL, interval TEXT NOT NULL, open_time INTEGER NOT NULL, payload BLOB NOT NULL, cached_at INTEGER NOT NULL, PRIMARY KEY(product,symbol,interval,open_time))`,
		`CREATE TABLE exchange_info_cache (product TEXT PRIMARY KEY, payload BLOB NOT NULL, cached_at INTEGER NOT NULL)`,
		`CREATE TABLE ui_state (profile TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(profile,key))`,
		`INSERT INTO schema_migrations(version,applied_at) VALUES(1,?)`,
	}
	for _, statement := range statements {
		if statement == statements[len(statements)-1] {
			_, err = tx.ExecContext(ctx, statement, time.Now().UnixMilli())
		} else {
			_, err = tx.ExecContext(ctx, statement)
		}
		if err != nil {
			return fmt.Errorf("apply database migration: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) AddWatchlist(ctx context.Context, profile, symbol string) error {
	profile, symbol = clean(profile), strings.ToUpper(clean(symbol))
	if profile == "" || symbol == "" {
		return errors.New("profile and symbol are required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO watchlist(profile,symbol,created_at) VALUES(?,?,?) ON CONFLICT(profile,symbol) DO NOTHING`, profile, symbol, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("add watchlist symbol: %w", err)
	}
	return nil
}
func (s *Store) RemoveWatchlist(ctx context.Context, profile, symbol string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM watchlist WHERE profile=? AND symbol=?`, clean(profile), strings.ToUpper(clean(symbol)))
	if err != nil {
		return fmt.Errorf("remove watchlist symbol: %w", err)
	}
	return nil
}
func (s *Store) Watchlist(ctx context.Context, profile string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT symbol FROM watchlist WHERE profile=? ORDER BY symbol`, clean(profile))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		result = append(result, symbol)
	}
	return result, rows.Err()
}

func (s *Store) AddAlert(ctx context.Context, a Alert) error {
	a.Profile = clean(a.Profile)
	a.Symbol = strings.ToUpper(clean(a.Symbol))
	a.Condition = clean(a.Condition)
	a.Target = clean(a.Target)
	if a.Profile == "" || a.Symbol == "" || a.Target == "" {
		return errors.New("profile, symbol, condition, and target are required")
	}
	if !validCondition(a.Condition) {
		return fmt.Errorf("unsupported alert condition %q", a.Condition)
	}
	if _, err := decimal.NewFromString(a.Target); err != nil {
		return fmt.Errorf("invalid alert target: %w", err)
	}
	if a.Cooldown < 0 {
		return errors.New("alert cooldown cannot be negative")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO alerts(profile,symbol,condition,target,cooldown_ms,enabled) VALUES(?,?,?,?,?,1)`, a.Profile, a.Symbol, a.Condition, a.Target, a.Cooldown.Milliseconds())
	if err != nil {
		return fmt.Errorf("add alert: %w", err)
	}
	return nil
}
func (s *Store) RemoveAlert(ctx context.Context, profile string, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alerts WHERE profile=? AND id=?`, clean(profile), id)
	if err != nil {
		return fmt.Errorf("remove alert: %w", err)
	}
	return nil
}
func (s *Store) Alerts(ctx context.Context, profile string) ([]Alert, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,profile,symbol,condition,target,cooldown_ms,last_trigger_ms,enabled FROM alerts WHERE profile=? ORDER BY id`, clean(profile))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []Alert{}
	for rows.Next() {
		var a Alert
		var cooldown, last int64
		var enabled int
		if err := rows.Scan(&a.ID, &a.Profile, &a.Symbol, &a.Condition, &a.Target, &cooldown, &last, &enabled); err != nil {
			return nil, err
		}
		a.Cooldown = time.Duration(cooldown) * time.Millisecond
		a.Enabled = enabled != 0
		if last > 0 {
			a.LastTriggered = time.UnixMilli(last)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// Evaluate triggers matching alerts and atomically persists their cooldown timestamps.
func (s *Store) Evaluate(ctx context.Context, profile, symbol, condition string, value string, now time.Time) ([]Alert, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,profile,symbol,condition,target,cooldown_ms,last_trigger_ms,enabled FROM alerts WHERE profile=? AND symbol=? AND condition=? AND enabled=1`, clean(profile), strings.ToUpper(clean(symbol)), clean(condition))
	if err != nil {
		return nil, err
	}
	var candidates []Alert
	for rows.Next() {
		var a Alert
		var cooldown, last int64
		var enabled int
		if err := rows.Scan(&a.ID, &a.Profile, &a.Symbol, &a.Condition, &a.Target, &cooldown, &last, &enabled); err != nil {
			return nil, err
		}
		a.Cooldown = time.Duration(cooldown) * time.Millisecond
		if last > 0 {
			a.LastTriggered = time.UnixMilli(last)
		}
		candidates = append(candidates, a)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var triggered []Alert
	for _, a := range candidates {
		if !a.LastTriggered.IsZero() && now.Sub(a.LastTriggered) < a.Cooldown {
			continue
		}
		hit, err := thresholdHit(a.Condition, a.Target, value)
		if err != nil {
			return nil, err
		}
		if !hit {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE alerts SET last_trigger_ms=? WHERE id=?`, now.UnixMilli(), a.ID); err != nil {
			return nil, err
		}
		a.LastTriggered = now
		triggered = append(triggered, a)
	}
	return triggered, nil
}

func (s *Store) PutKlines(ctx context.Context, product, symbol, interval string, values any, now time.Time) error {
	payload, err := json.Marshal(values)
	if err != nil {
		return err
	}
	var items []json.RawMessage
	if decodeErr := json.Unmarshal(payload, &items); decodeErr != nil {
		return decodeErr
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range items {
		var candle struct {
			OpenTime int64 `json:"openTime"`
		}
		if err := json.Unmarshal(item, &candle); err != nil || candle.OpenTime == 0 {
			var tuple []json.RawMessage
			if decodeErr := json.Unmarshal(item, &tuple); decodeErr != nil || len(tuple) == 0 {
				if err != nil {
					return err
				}
				return errors.New("cached kline has no open time")
			}
			if err := json.Unmarshal(tuple[0], &candle.OpenTime); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO klines_cache(product,symbol,interval,open_time,payload,cached_at) VALUES(?,?,?,?,?,?) ON CONFLICT(product,symbol,interval,open_time) DO UPDATE SET payload=excluded.payload,cached_at=excluded.cached_at`, product, strings.ToUpper(symbol), interval, candle.OpenTime, item, now.UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) GetKlines(ctx context.Context, product, symbol, interval string, ttl time.Duration, now time.Time, out any) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload,cached_at FROM klines_cache WHERE product=? AND symbol=? AND interval=? ORDER BY open_time`, product, strings.ToUpper(symbol), interval)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	var payloads []json.RawMessage
	var newest int64
	var previousOpen int64
	for rows.Next() {
		var payload []byte
		var cached int64
		if scanErr := rows.Scan(&payload, &cached); scanErr != nil {
			return false, scanErr
		}
		var candle struct {
			OpenTime int64 `json:"openTime"`
		}
		if decodeErr := json.Unmarshal(payload, &candle); decodeErr != nil || candle.OpenTime == 0 {
			var tuple []json.RawMessage
			if tupleErr := json.Unmarshal(payload, &tuple); tupleErr != nil || len(tuple) == 0 {
				return false, errors.New("invalid cached kline")
			}
			if decodeErr = json.Unmarshal(tuple[0], &candle.OpenTime); decodeErr != nil {
				return false, decodeErr
			}
		}
		if len(payloads) > 0 {
			step, ok := intervalStep(interval, previousOpen)
			if ok && candle.OpenTime-previousOpen != step {
				return false, nil
			}
		}
		previousOpen = candle.OpenTime
		payloads = append(payloads, payload)
		if cached > newest {
			newest = cached
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return false, rowsErr
	}
	if len(payloads) == 0 || now.Sub(time.UnixMilli(newest)) > ttl {
		return false, nil
	}
	combined, err := json.Marshal(payloads)
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(combined, out)
}

func (s *Store) InvalidateKlines(ctx context.Context, product, symbol, interval string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM klines_cache WHERE product=? AND symbol=? AND interval=?`, product, strings.ToUpper(symbol), interval)
	return err
}

func intervalStep(interval string, openTime int64) (int64, bool) {
	if interval == "1M" {
		start := time.UnixMilli(openTime).UTC()
		return start.AddDate(0, 1, 0).UnixMilli() - openTime, true
	}
	units := map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour}
	if len(interval) < 2 {
		return 0, false
	}
	unit, ok := units[interval[len(interval)-1:]]
	if !ok {
		return 0, false
	}
	count, err := strconv.Atoi(interval[:len(interval)-1])
	if err != nil || count <= 0 {
		return 0, false
	}
	return int64(unit * time.Duration(count) / time.Millisecond), true
}

func (s *Store) PutExchangeInfo(ctx context.Context, product string, value any, now time.Time) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO exchange_info_cache(product,payload,cached_at) VALUES(?,?,?) ON CONFLICT(product) DO UPDATE SET payload=excluded.payload,cached_at=excluded.cached_at`, product, payload, now.UnixMilli())
	return err
}

func (s *Store) GetExchangeInfo(ctx context.Context, product string, ttl time.Duration, now time.Time, out any) (bool, error) {
	var payload []byte
	var cached int64
	err := s.db.QueryRowContext(ctx, `SELECT payload,cached_at FROM exchange_info_cache WHERE product=?`, product).Scan(&payload, &cached)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if now.Sub(time.UnixMilli(cached)) > ttl {
		return false, nil
	}
	return true, json.Unmarshal(payload, out)
}

func validCondition(c string) bool {
	switch c {
	case "above", "below", "change_above", "change_below", "funding_above", "funding_below":
		return true
	}
	return false
}
func thresholdHit(condition, target, value string) (bool, error) {
	t, err := parseDecimal(target)
	if err != nil {
		return false, fmt.Errorf("invalid alert target: %w", err)
	}
	v, err := parseDecimal(value)
	if err != nil {
		return false, fmt.Errorf("invalid observed value: %w", err)
	}
	switch condition {
	case "above", "change_above", "funding_above":
		return v.GreaterThanOrEqual(t), nil
	case "below", "change_below", "funding_below":
		return v.LessThanOrEqual(t), nil
	default:
		return false, fmt.Errorf("unsupported alert condition %q", condition)
	}
}
func parseDecimal(raw string) (decimal.Decimal, error) { return decimal.NewFromString(raw) }
func clean(value string) string                        { return strings.TrimSpace(value) }
