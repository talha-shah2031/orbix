package rest

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Usage struct {
	Used, Limit            int
	OrderCount, OrderLimit int
}

type tokenBucket struct {
	tokens, capacity float64
	updated          time.Time
}

type Limiter struct {
	mu           sync.Mutex
	buckets      map[string]*tokenBucket
	usage        map[string]Usage
	limits       map[string]int
	defaultLimit int
}

func NewLimiter(requestsPerMinute int) *Limiter {
	if requestsPerMinute < 1 {
		requestsPerMinute = 1
	}
	return &Limiter{buckets: make(map[string]*tokenBucket), usage: make(map[string]Usage), limits: make(map[string]int), defaultLimit: requestsPerMinute}
}

func NewProductLimiter(limits map[string]int) *Limiter {
	limiter := NewLimiter(6000)
	for key, limit := range limits {
		if limit > 0 {
			limiter.limits[key] = limit
		}
	}
	return limiter
}

func (l *Limiter) Wait(ctx context.Context, key string, weight float64) error {
	if weight <= 0 {
		return nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		now := time.Now()
		limit := l.limits[key]
		if limit <= 0 {
			limit = l.defaultLimit
		}
		capacity := float64(limit) * 0.8
		rate := capacity / 60
		bucket := l.buckets[key]
		if bucket == nil {
			available := capacity - float64(l.usage[key].Used)
			if available < 0 {
				available = 0
			}
			bucket = &tokenBucket{tokens: available, capacity: capacity, updated: now}
			l.buckets[key] = bucket
		}
		bucket.tokens += now.Sub(bucket.updated).Seconds() * rate
		if bucket.tokens > bucket.capacity {
			bucket.tokens = bucket.capacity
		}
		bucket.updated = now
		if bucket.tokens >= weight {
			bucket.tokens -= weight
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((weight - bucket.tokens) / rate * float64(time.Second))
		l.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (l *Limiter) Observe(key string, headers map[string][]string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for name, values := range headers {
		upper := strings.ToUpper(name)
		if len(values) == 0 {
			continue
		}
		order := strings.HasPrefix(upper, "X-MBX-ORDER-COUNT-")
		if !order && !strings.HasPrefix(upper, "X-MBX-USED-WEIGHT-") {
			continue
		}
		if !order && !strings.HasSuffix(upper, "1M") {
			continue
		}
		used, err := strconv.Atoi(values[0])
		if err != nil {
			continue
		}
		usage := l.usage[key]
		if order {
			usage.OrderCount = used
			if usage.OrderLimit == 0 {
				usage.OrderLimit = 1200
			}
		} else {
			usage.Used = used
			usage.Limit = l.limits[key]
			if usage.Limit == 0 {
				usage.Limit = l.defaultLimit
			}
			if bucket := l.buckets[key]; bucket != nil {
				remaining := float64(usage.Limit)*0.8 - float64(used)
				if remaining < 0 {
					remaining = 0
				}
				if bucket.tokens > remaining {
					bucket.tokens = remaining
				}
			}
		}
		l.usage[key] = usage
	}
}

func (l *Limiter) Usage(key string) Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.usage[key]
}
