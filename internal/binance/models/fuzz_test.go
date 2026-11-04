package models

import (
	"encoding/json"
	"testing"
)

func FuzzTickerDecoder(f *testing.F) {
	f.Add([]byte(`{"symbol":"BTCUSDT","lastPrice":"12.3400"}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"lastPrice":1.23}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			t.Skip()
		}
		var ticker Ticker24h
		_ = json.Unmarshal(data, &ticker)
	})
}
