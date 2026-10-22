package postgres

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestBoundedCloseDoesNotLeakGoroutines(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	release := make(chan struct{})
	backend := &stubPoolBackend{close: func() { <-release }}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = pool.Shutdown(ctx)
	close(release)
	if err := pool.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
