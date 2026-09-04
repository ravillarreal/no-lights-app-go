// Package metrics exposes the deterministic per-request query counter and
// latency headers used by the performance harness (Google SRE RED method).
package metrics

import (
	"context"
	"sync/atomic"
)

type counterKey struct{}

// WithCounter returns a context carrying a fresh query counter. The HTTP
// middleware calls this at the start of each request; the store bumps it on
// every database round-trip.
func WithCounter(ctx context.Context) context.Context {
	return context.WithValue(ctx, counterKey{}, new(int64))
}

// BumpQuery increments the query counter held in ctx (no-op if absent).
func BumpQuery(ctx context.Context) {
	if c, ok := ctx.Value(counterKey{}).(*int64); ok {
		atomic.AddInt64(c, 1)
	}
}

// QueryCount returns the number of database round-trips recorded in ctx.
func QueryCount(ctx context.Context) int64 {
	if c, ok := ctx.Value(counterKey{}).(*int64); ok {
		return atomic.LoadInt64(c)
	}
	return 0
}
