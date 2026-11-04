package models

import (
	"encoding/json"
	"testing"
)

var tickerFixture = []byte(`{"symbol":"BTCUSDT","priceChange":"123.45678901","priceChangePercent":"1.2345","weightedAvgPrice":"99876.54321098","prevClosePrice":"99999","lastPrice":"100123.45678901","lastQty":"0.001","bidPrice":"100123.45","bidQty":"4.5","askPrice":"100123.46","askQty":"3.5","openPrice":"99999","highPrice":"101000","lowPrice":"99000","volume":"1234.56","quoteVolume":"123456789.12345678","openTime":1,"closeTime":2,"firstId":1,"lastId":2,"count":2}`)

func BenchmarkTickerDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var ticker Ticker24h
		if err := json.Unmarshal(tickerFixture, &ticker); err != nil {
			b.Fatal(err)
		}
	}
}
