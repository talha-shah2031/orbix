package market

import "testing"

func TestStreamName(t *testing.T) {
	tests := []struct {
		kind, symbol, interval, want string
		bad                          bool
	}{
		{kind: "trade", symbol: "BTCUSDT", want: "btcusdt@trade"},
		{kind: "book", symbol: "ETHUSDT", want: "ethusdt@bookTicker"},
		{kind: "depth", symbol: "BTCUSDT", want: "btcusdt@depth@100ms"},
		{kind: "kline", symbol: "BTCUSDT", interval: "1h", want: "btcusdt@kline_1h"},
		{kind: "kline", symbol: "BTCUSDT", interval: "2x", bad: true},
		{kind: "trade", symbol: "bad@stream", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.symbol, func(t *testing.T) {
			got, err := StreamName(tt.kind, tt.symbol, tt.interval)
			if tt.bad {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
