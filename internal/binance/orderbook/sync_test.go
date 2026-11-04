package orderbook

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/stretchr/testify/require"
)

type scriptedSource struct {
	open     func(int) (<-chan Diff, func(), error)
	snapshot func(int) (models.Depth, error)
	opens    int
	snaps    int
}

func (source *scriptedSource) OpenDiffStream(context.Context, string) (<-chan Diff, func(), error) {
	source.opens++
	return source.open(source.opens)
}
func (source *scriptedSource) DepthSnapshot(context.Context, string) (models.Depth, error) {
	source.snaps++
	return source.snapshot(source.snaps)
}

func TestSynchronizeOpensStreamBeforeSnapshotAndCancelsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Diff, 1)
	events <- diff(2, 2, []string{"99:1"}, nil)
	closed, changes := 0, 0
	source := &scriptedSource{
		open:     func(int) (<-chan Diff, func(), error) { return events, func() { closed++ }, nil },
		snapshot: func(int) (models.Depth, error) { return models.Depth{LastUpdateID: 1}, nil },
	}
	book := New()
	err := book.Synchronize(ctx, source, "BTCUSDT", func() { changes++; cancel() })
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, source.opens)
	require.Equal(t, 1, source.snaps)
	require.Equal(t, 1, closed)
	require.Equal(t, 1, changes)
	require.True(t, book.Synced())
}

func TestSynchronizeRetriesStreamAndSnapshotErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	opens, closes := 0, 0
	source := &scriptedSource{
		open: func(int) (<-chan Diff, func(), error) {
			opens++
			cancel()
			return nil, nil, errors.New("temporary stream error")
		},
	}
	book := New()
	err := book.Synchronize(ctx, source, "BTCUSDT", nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, opens)
	require.Zero(t, closes)

	ctx, cancel = context.WithCancel(context.Background())
	source = &scriptedSource{
		open:     func(int) (<-chan Diff, func(), error) { return make(chan Diff), func() { closes++; cancel() }, nil },
		snapshot: func(int) (models.Depth, error) { return models.Depth{}, errors.New("temporary snapshot error") },
	}
	err = New().Synchronize(ctx, source, "BTCUSDT", nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, source.snaps)
	require.Equal(t, 1, closes)
}

func TestSynchronizeAutomaticallyReopensOnSequenceGap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opens, closes := 0, 0
	events := make(chan Diff, 1)
	events <- diff(12, 12, nil, nil)
	source := &scriptedSource{
		open: func(call int) (<-chan Diff, func(), error) {
			opens++
			if call == 2 {
				cancel()
				return nil, nil, errors.New("stop after resync attempt")
			}
			return events, func() { closes++ }, nil
		},
		snapshot: func(int) (models.Depth, error) { return models.Depth{LastUpdateID: 10}, nil },
	}
	book := New()
	err := book.Synchronize(ctx, source, "BTCUSDT", nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 2, opens)
	require.Equal(t, 1, closes)
	require.False(t, book.Synced())
}

func TestWaitRetryCanBeInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	require.ErrorIs(t, waitRetry(ctx, time.Hour), context.Canceled)
	require.Less(t, time.Since(started), time.Second)
}
