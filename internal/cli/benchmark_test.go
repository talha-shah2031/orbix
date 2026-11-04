package cli

import (
	"bytes"
	"fmt"
	"testing"
)

func BenchmarkTableRendering(b *testing.B) {
	rows := make([]map[string]any, 2000)
	for i := range rows {
		rows[i] = map[string]any{"symbol": fmt.Sprintf("COIN%04dUSDT", i), "price": "12345.67890123", "change": "1.23", "volume": "123456789.123"}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		if err := writeOutput(&output, "table", rows); err != nil {
			b.Fatal(err)
		}
	}
}
