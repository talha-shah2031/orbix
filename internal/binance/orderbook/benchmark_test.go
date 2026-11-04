package orderbook

import (
	"fmt"
	"testing"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/shopspring/decimal"
)

func BenchmarkApplyDepthUpdate(b *testing.B) {
	book := New()
	bids, asks := make([]models.Level, 1000), make([]models.Level, 1000)
	for i := range bids {
		bids[i] = models.Level{Price: decimal.NewFromInt(int64(10000 - i)), Quantity: decimal.NewFromInt(10)}
		asks[i] = models.Level{Price: decimal.NewFromInt(int64(11000 + i)), Quantity: decimal.NewFromInt(10)}
	}
	if err := book.InstallSnapshot(models.Depth{LastUpdateID: 1000, Bids: bids, Asks: asks}); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := int64(1001 + i)
		if err := book.Apply(Diff{FirstUpdateID: id, FinalUpdateID: id, Bids: []models.Level{{Price: decimal.NewFromInt(9000 + int64(i%500)), Quantity: decimal.NewFromInt(11)}}}); err != nil {
			b.Fatal(err)
		}
	}
}

func ExampleBook_BestBidAsk() {
	book := New()
	_ = book.InstallSnapshot(models.Depth{LastUpdateID: 1, Bids: []models.Level{{Price: decimal.NewFromInt(99), Quantity: decimal.NewFromInt(2)}}, Asks: []models.Level{{Price: decimal.NewFromInt(101), Quantity: decimal.NewFromInt(3)}}})
	_ = book.Apply(Diff{FirstUpdateID: 2, FinalUpdateID: 2})
	bid, ask, _ := book.BestBidAsk()
	fmt.Println(bid.Price, ask.Price)
	// Output: 99 101
}
