package metrics

import (
	"context"
	"testing"
)

func TestCounterIncrements(t *testing.T) {
	ctx := WithCounter(context.Background())

	if got := QueryCount(ctx); got != 0 {
		t.Fatalf("initial count = %d, want 0", got)
	}

	BumpQuery(ctx)
	BumpQuery(ctx)
	BumpQuery(ctx)

	if got := QueryCount(ctx); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
}

func TestBumpWithoutCounterIsNoop(t *testing.T) {
	BumpQuery(context.Background())
	if got := QueryCount(context.Background()); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
}
