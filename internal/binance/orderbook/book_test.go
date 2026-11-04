package orderbook

import (
	"errors"
	"strings"
	"testing"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSnapshotBridgeReplaysBufferedEventsAndKeepsSortedSides(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.Apply(diff(12, 13, []string{"101:4"}, []string{"102:3"})))
	require.NoError(t, book.Apply(diff(11, 11, []string{"100:5"}, []string{"103:2"})))
	require.NoError(t, book.InstallSnapshot(models.Depth{LastUpdateID: 10,
		Bids: []models.Level{level("99", "1"), level("100", "2")},
		Asks: []models.Level{level("104", "1"), level("102", "2")},
	}))
	require.True(t, book.Synced())
	require.Equal(t, int64(13), book.LastUpdateID())
	bids, asks, _, ok := book.Snapshot(10)
	require.True(t, ok)
	require.Equal(t, []string{"101", "100", "99"}, prices(bids))
	require.Equal(t, []string{"102", "103", "104"}, prices(asks))
}

func TestApplyUpdatesInsertReplaceRemoveAndIgnoreOld(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.InstallSnapshot(models.Depth{LastUpdateID: 100, Bids: []models.Level{level("99", "1")}, Asks: []models.Level{level("101", "1")}}))
	require.NoError(t, book.Apply(diff(101, 101, []string{"100:2", "99:3"}, []string{"101:0", "102:4"})))
	require.NoError(t, book.Apply(diff(100, 100, []string{"99:8"}, nil)))
	bids, asks, updateID, ok := book.Snapshot(10)
	require.True(t, ok)
	require.Equal(t, int64(101), updateID)
	require.Equal(t, []string{"100", "99"}, prices(bids))
	require.Equal(t, []string{"102"}, prices(asks))
	require.Equal(t, "3", bids[1].Quantity.String())
}

func TestGapInvalidatesBookAndRequiresResync(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.InstallSnapshot(models.Depth{LastUpdateID: 20, Bids: []models.Level{level("10", "1")}, Asks: []models.Level{level("11", "1")}}))
	err := book.Apply(diff(22, 22, []string{"10:5"}, nil))
	require.ErrorIs(t, err, ErrResyncRequired)
	require.False(t, book.Synced())
	_, _, _, ok := book.Snapshot(1)
	require.False(t, ok)
}

func TestSnapshotWithNoBridgingBufferedUpdateRequiresResync(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.Apply(diff(12, 12, []string{"10:1"}, nil)))
	err := book.InstallSnapshot(models.Depth{LastUpdateID: 10})
	require.ErrorIs(t, err, ErrResyncRequired)
	require.False(t, book.Synced())
}

func TestSnapshotDropsEventsAtOrBelowUpdateID(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.Apply(diff(10, 10, []string{"9:1"}, nil)))
	require.NoError(t, book.Apply(diff(11, 11, []string{"10:2"}, nil)))
	require.NoError(t, book.InstallSnapshot(models.Depth{LastUpdateID: 10}))
	bids, _, updateID, ok := book.Snapshot(5)
	require.True(t, ok)
	require.Equal(t, int64(11), updateID)
	require.Equal(t, []string{"10"}, prices(bids))
}

func TestBestPricesSpreadCumulativeDepthAndImbalance(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.InstallSnapshot(models.Depth{LastUpdateID: 1,
		Bids: []models.Level{level("99", "6"), level("98", "4")},
		Asks: []models.Level{level("101", "2"), level("102", "2")},
	}))
	require.NoError(t, book.Apply(diff(2, 2, nil, nil)))
	bid, ask, ok := book.BestBidAsk()
	require.True(t, ok)
	require.Equal(t, "99", bid.Price.String())
	require.Equal(t, "101", ask.Price.String())
	spread, mid, ok := book.SpreadMid()
	require.True(t, ok)
	require.Equal(t, "2", spread.String())
	require.Equal(t, "100", mid.String())
	require.Equal(t, []string{"6", "10"}, quantities(book.CumulativeDepth(true, 2)))
	imbalance, ok := book.Imbalance(2)
	require.True(t, ok)
	require.True(t, imbalance.Equal(decimal.NewFromInt(3).Div(decimal.NewFromInt(7))))
}

func TestOutOfOrderLiveEventTriggersAutomaticResyncSignal(t *testing.T) {
	book := New()
	book.BeginSync()
	require.NoError(t, book.InstallSnapshot(models.Depth{LastUpdateID: 40}))
	err := book.Apply(diff(42, 42, nil, nil))
	require.True(t, errors.Is(err, ErrResyncRequired))
	require.False(t, book.Synced())
}

func diff(first, final int64, bids, asks []string) Diff {
	return Diff{FirstUpdateID: first, FinalUpdateID: final, Bids: parseLevels(bids), Asks: parseLevels(asks)}
}

func parseLevels(values []string) []models.Level {
	levels := make([]models.Level, 0, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, ":", 2)
		levels = append(levels, level(parts[0], parts[1]))
	}
	return levels
}

func level(price, quantity string) models.Level {
	return models.Level{Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity)}
}

func prices(levels []models.Level) []string {
	values := make([]string, 0, len(levels))
	for _, item := range levels {
		values = append(values, item.Price.String())
	}
	return values
}

func quantities(levels []models.Level) []string {
	values := make([]string, 0, len(levels))
	for _, item := range levels {
		values = append(values, item.Quantity.String())
	}
	return values
}
