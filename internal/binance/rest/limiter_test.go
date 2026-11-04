package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLimiterCapsAtEightyPercent(t *testing.T) {
	limiter := NewProductLimiter(map[string]int{"spot": 100})
	for range 80 {
		require.NoError(t, limiter.Wait(context.Background(), "spot", 1))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, limiter.Wait(ctx, "spot", 1), context.Canceled)
}

func TestLimiterHandlesInvalidHeadersWeightsAndCancellation(t *testing.T) {
	limiter := NewLimiter(0)
	require.NoError(t, limiter.Wait(context.Background(), "spot", 0))
	require.NoError(t, limiter.Wait(context.Background(), "spot", 0.5))
	limiter.Observe("spot", http.Header{
		"X-MBX-USED-WEIGHT-5M": {"20"},
		"X-MBX-USED-WEIGHT-1M": {"not-a-number"},
		"X-MBX-ORDER-COUNT-1M": {"7"},
		"Unrelated":            {"9"},
	})
	require.Equal(t, 7, limiter.Usage("spot").OrderCount)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_ = limiter.Wait(context.Background(), "tiny", 0.8)
	require.ErrorIs(t, limiter.Wait(ctx, "tiny", 1), context.DeadlineExceeded)
}

func TestLimiterAccountsForServerReportedWeight(t *testing.T) {
	limiter := NewProductLimiter(map[string]int{"spot": 100})
	limiter.Observe("spot", http.Header{"X-Mbx-Used-Weight-1m": {"85"}})
	require.Equal(t, Usage{Used: 85, Limit: 100}, limiter.Usage("spot"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, limiter.Wait(ctx, "spot", 1), context.Canceled)
}
