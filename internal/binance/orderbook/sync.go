package orderbook

import (
	"context"
	"errors"
	"time"

	"github.com/bubblenexus/bnx/internal/binance/models"
)

type Source interface {
	OpenDiffStream(context.Context, string) (<-chan Diff, func(), error)
	DepthSnapshot(context.Context, string) (models.Depth, error)
}

// Synchronize runs the documented stream-before-snapshot procedure and
// automatically opens a fresh stream and snapshot on any sequence gap.
func (book *Book) Synchronize(ctx context.Context, source Source, symbol string, onChange func()) error {
	backoff := 100 * time.Millisecond
	for ctx.Err() == nil {
		book.BeginSync()
		events, closeStream, err := source.OpenDiffStream(ctx, symbol)
		if err != nil {
			if err := waitRetry(ctx, backoff); err != nil {
				return err
			}
			backoff = min(backoff*2, 5*time.Second)
			continue
		}
		snapshot, snapshotErr := source.DepthSnapshot(ctx, symbol)
		if snapshotErr != nil {
			closeStream()
			if err := waitRetry(ctx, backoff); err != nil {
				return err
			}
			backoff = min(backoff*2, 5*time.Second)
			continue
		}
		if installErr := book.InstallSnapshot(snapshot); installErr != nil {
			closeStream()
			if errors.Is(installErr, ErrResyncRequired) {
				if err := waitRetry(ctx, backoff); err != nil {
					return err
				}
				backoff = min(backoff*2, 5*time.Second)
				continue
			}
			return installErr
		}
		if book.Synced() && onChange != nil {
			onChange()
		}
		backoff = 100 * time.Millisecond
		resync := false
		for !resync {
			select {
			case <-ctx.Done():
				closeStream()
				return ctx.Err()
			case update, ok := <-events:
				if !ok {
					resync = true
					continue
				}
				if err := book.Apply(update); err != nil {
					if !errors.Is(err, ErrResyncRequired) && !errors.Is(err, ErrBufferOverflow) {
						closeStream()
						return err
					}
					resync = true
					continue
				}
				if book.Synced() && onChange != nil {
					onChange()
				}
			}
		}
		closeStream()
		if err := waitRetry(ctx, backoff); err != nil {
			return err
		}
		backoff = min(backoff*2, 5*time.Second)
	}
	return ctx.Err()
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
