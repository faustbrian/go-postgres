package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewFailFastDoesNotLeakCredentials(t *testing.T) {
	t.Parallel()

	const password = "startup-secret-password"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := New(ctx, Config{
		DSN:            "postgres://app:" + password + "@127.0.0.1:1/app?sslmode=disable",
		ConnectTimeout: 50 * time.Millisecond,
		PingTimeout:    100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("New() error = nil")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("New() error leaked password: %v", err)
	}
}

func TestNewFailsBoundedlyAgainstWrongProtocolServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() { _ = listener.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	_, err = New(ctx, Config{
		DSN: fmt.Sprintf(
			"postgres://app:wrong-server-secret@%s/app?sslmode=disable",
			listener.Addr(),
		),
		ConnectTimeout: 100 * time.Millisecond,
		PingTimeout:    250 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("New() error = nil")
	}
	if strings.Contains(err.Error(), "wrong-server-secret") {
		t.Fatalf("New() error leaked password: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Fatalf("wrong-server startup took %s", elapsed)
	}

	_ = listener.Close()
	<-done
}

func TestNewLazyExposesNativePool(t *testing.T) {
	t.Parallel()

	pool, err := New(context.Background(), Config{
		DSN:           "postgres://localhost/app?sslmode=disable",
		MaxConns:      7,
		StartupPolicy: StartupLazy,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if pool.Raw() == nil {
		t.Fatal("Raw() = nil")
	}
	if got := pool.Raw().Config().MaxConns; got != 7 {
		t.Errorf("Raw().Config().MaxConns = %d, want 7", got)
	}
	if pool.acquireTimeout != 5*time.Second ||
		pool.pingTimeout != 2*time.Second ||
		pool.shutdownTimeout != 10*time.Second {
		t.Fatalf(
			"default timeouts = acquire %s ping %s shutdown %s",
			pool.acquireTimeout,
			pool.pingTimeout,
			pool.shutdownTimeout,
		)
	}
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPoolAcquireUsesConfiguredBound(t *testing.T) {
	t.Parallel()

	backend := &stubPoolBackend{
		stats: Stats{AcquiredConns: 1, MaxConns: 1},
		acquire: func(ctx context.Context) (*pgxpool.Conn, error) {
			if _, ok := ctx.Deadline(); !ok {
				return nil, errors.New("configured acquisition deadline was not applied")
			}
			<-ctx.Done()

			return nil, ctx.Err()
		},
	}
	pool := newPool(nil, backend, time.Nanosecond, time.Second, time.Second)

	_, err := pool.Acquire(context.Background())
	if !errors.Is(err, ErrAcquireTimeout) {
		t.Fatalf("Acquire() error = %v, want ErrAcquireTimeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want context deadline", err)
	}
	if !errors.Is(err, ErrPoolExhausted) || !IsPoolExhaustion(err) {
		t.Fatalf("Acquire() error = %v, want pool exhaustion", err)
	}
}

func TestPoolAcquirePreservesCallerDeadlineWithoutClaimingSaturation(t *testing.T) {
	t.Parallel()

	backend := &stubPoolBackend{
		stats: Stats{MaxConns: 4},
		acquire: func(ctx context.Context) (*pgxpool.Conn, error) {
			<-ctx.Done()

			return nil, ctx.Err()
		},
	}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.Acquire(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire() error = %v, want context cancellation", err)
	}
	if errors.Is(err, ErrAcquireTimeout) || errors.Is(err, ErrPoolExhausted) || IsPoolExhaustion(err) {
		t.Fatalf("Acquire() error = %v, incorrectly reports pool exhaustion", err)
	}
}

func TestPoolReadinessUsesBoundedPingAndReportsStats(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("database unavailable")
	backend := &stubPoolBackend{
		ping: func(context.Context) error { return sentinel },
	}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)

	health := pool.Readiness(context.Background())
	if health.Ready {
		t.Fatal("Readiness().Ready = true")
	}
	if !errors.Is(health.Err, sentinel) {
		t.Fatalf("Readiness().Err = %v, want sentinel", health.Err)
	}
	if errors.Is(health.Err, ErrHealthTimeout) {
		t.Fatalf("Readiness().Err = %v, incorrectly reports health timeout", health.Err)
	}
}

func TestPoolCloseIsBoundedAndRunsUnderlyingCloseOnce(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var closes atomic.Int32
	backend := &stubPoolBackend{
		close: func() {
			closes.Add(1)
			<-release
		},
	}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pool.Close(ctx)
	if !errors.Is(err, ErrShutdownTimeout) {
		t.Fatalf("Close() error = %v, want ErrShutdownTimeout", err)
	}
	close(release)

	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if got := closes.Load(); got != 1 {
		t.Fatalf("underlying close calls = %d, want 1", got)
	}
}

func TestPoolObservationsAreBoundedAndPanicSafe(t *testing.T) {
	t.Parallel()

	var observations []Observation
	observer := ObserverFunc(func(_ context.Context, observation Observation) {
		observations = append(observations, observation)
	})
	backend := &stubPoolBackend{}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second, observer)

	if _, err := pool.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if len(observations) != 1 || observations[0].Operation != OperationAcquire ||
		observations[0].Outcome != OutcomeSuccess || observations[0].Duration < 0 {
		t.Fatalf("observations = %#v", observations)
	}

	pool = newPool(nil, backend, time.Second, time.Second, time.Second, ObserverFunc(func(context.Context, Observation) {
		panic("telemetry failure")
	}))
	if _, err := pool.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() changed by observer panic: %v", err)
	}
}

func TestPoolLivenessDoesNotRequireDatabaseConnectivity(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	backend := &stubPoolBackend{
		ping:  func(context.Context) error { return errors.New("database unavailable") },
		close: func() { <-release },
	}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)
	if health := pool.Liveness(); !health.Ready || health.Err != nil {
		t.Fatalf("Liveness() before close = %#v", health)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = pool.Close(ctx)
	if health := pool.Liveness(); health.Ready || !errors.Is(health.Err, ErrPoolClosed) {
		t.Fatalf("Liveness() after close = %#v", health)
	}
	close(release)
}

func TestNewPreservesNativeConstructionFailureWithoutCredentials(t *testing.T) {
	t.Parallel()

	_, err := New(context.Background(), Config{
		DSN:           "postgres://app:secret@localhost/app?sslmode=disable",
		StartupPolicy: StartupLazy,
		Configure: func(config *PoolConfig) error {
			config.MaxConns = 0

			return nil
		},
	})
	if err == nil {
		t.Fatal("New() error = nil")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("New() leaked credentials: %v", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("New() error = %v, want native cause", err)
	}
}

func TestNewReturnsConfigurationFailure(t *testing.T) {
	t.Parallel()

	_, err := New(context.Background(), Config{})
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("New() error = %v, want ConfigError", err)
	}
}

func TestConnectRejectsNilAndCanceledContextsBeforeConfiguration(t *testing.T) {
	t.Parallel()

	var configureCalls atomic.Int32
	input := Config{
		DSN: "postgres://localhost/app?sslmode=disable",
		Configure: func(*PoolConfig) error {
			configureCalls.Add(1)
			return nil
		},
	}
	var nilContext context.Context
	if _, err := Connect(nilContext, input); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("Connect(nil) error = %v, want ErrContextRequired", err)
	}

	cause := errors.New("caller stopped startup")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	if _, err := Connect(ctx, input); !errors.Is(err, cause) {
		t.Fatalf("Connect(canceled) error = %v, want caller cause", err)
	}
	if got := configureCalls.Load(); got != 0 {
		t.Fatalf("Configure calls = %d, want 0", got)
	}
	if _, err := New(nilContext, input); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("New(nil) error = %v, want ErrContextRequired", err)
	}
}

func TestConnectConstructsOnceAndRollsBackFailedReadiness(t *testing.T) {
	t.Parallel()

	pingErr := errors.New("readiness failed")
	var constructs atomic.Int32
	var closes atomic.Int32
	backend := &stubPoolBackend{
		ping:  func(context.Context) error { return pingErr },
		close: func() { closes.Add(1) },
	}
	_, err := connect(context.Background(), Config{
		DSN: "postgres://localhost/app?sslmode=disable",
	}, func(context.Context, *pgxpool.Config) (*pgxpool.Pool, poolBackend, error) {
		constructs.Add(1)
		return nil, backend, nil
	})
	if !errors.Is(err, pingErr) {
		t.Fatalf("connect() error = %v, want readiness failure", err)
	}
	if got := constructs.Load(); got != 1 {
		t.Fatalf("construction calls = %d, want 1", got)
	}
	if got := closes.Load(); got != 1 {
		t.Fatalf("rollback close calls = %d, want 1", got)
	}
}

func TestPoolShutdownRejectsNilWithoutClosingAdmission(t *testing.T) {
	t.Parallel()

	var closes atomic.Int32
	backend := &stubPoolBackend{close: func() { closes.Add(1) }}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)
	var nilContext context.Context
	if err := pool.Shutdown(nilContext); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("Shutdown(nil) error = %v, want ErrContextRequired", err)
	}
	if err := pool.Close(nilContext); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("Close(nil) error = %v, want ErrContextRequired", err)
	}
	if pool.closed.Load() || closes.Load() != 0 {
		t.Fatalf("nil shutdown changed state: closed=%t closes=%d", pool.closed.Load(), closes.Load())
	}
}

func TestPoolShutdownUsesOneCleanupAndIndependentCallerBounds(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	var closes atomic.Int32
	backend := &stubPoolBackend{close: func() {
		closes.Add(1)
		close(started)
		<-release
	}}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)

	short, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pool.Shutdown(short); !errors.Is(err, ErrShutdownTimeout) || !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown(canceled) error = %v, want timeout and cancellation", err)
	}
	<-started

	done := make(chan error, 1)
	go func() { done <- pool.Shutdown(context.Background()) }()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("concurrent Shutdown() error = %v", err)
	}
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("repeated Close() error = %v", err)
	}
	if got := closes.Load(); got != 1 {
		t.Fatalf("underlying close calls = %d, want 1", got)
	}
}

func TestPoolRejectsUseAfterShutdownBegins(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var acquires atomic.Int32
	var pings atomic.Int32
	backend := &stubPoolBackend{
		acquire: func(context.Context) (*pgxpool.Conn, error) {
			acquires.Add(1)
			return nil, nil
		},
		ping: func(context.Context) error {
			pings.Add(1)
			return nil
		},
		close: func() { <-release },
	}
	pool := newPool(nil, backend, time.Second, time.Second, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = pool.Shutdown(ctx)
	if _, err := pool.Acquire(context.Background()); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("Acquire() error = %v, want ErrPoolClosed", err)
	}
	if err := pool.Ping(context.Background()); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("Ping() error = %v, want ErrPoolClosed", err)
	}
	if health := pool.Readiness(context.Background()); health.Ready || !errors.Is(health.Err, ErrPoolClosed) {
		t.Fatalf("Readiness() = %#v, want ErrPoolClosed", health)
	}
	if acquires.Load() != 0 || pings.Load() != 0 {
		t.Fatalf("use after shutdown reached backend: acquire=%d ping=%d", acquires.Load(), pings.Load())
	}
	close(release)
	if err := pool.Shutdown(context.Background()); err != nil {
		t.Fatalf("final Shutdown() error = %v", err)
	}
}

func TestBoundedContextSupportsNoAdditionalTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := boundedContext(context.Background(), 0)
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("context error = %v", ctx.Err())
	}
}

func TestBoundedContextWithCauseSupportsNoAdditionalTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := boundedContextWithCause(context.Background(), 0, ErrAcquireTimeout)
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) || errors.Is(context.Cause(ctx), ErrAcquireTimeout) {
		t.Fatalf("context error and cause = (%v, %v)", ctx.Err(), context.Cause(ctx))
	}
}

func TestPoolSaturationAccountsForConstructingConnections(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		stats Stats
		want  bool
	}{
		"unbounded": {
			stats: Stats{AcquiredConns: 1},
		},
		"available": {
			stats: Stats{AcquiredConns: 1, MaxConns: 2},
		},
		"constructing reaches capacity": {
			stats: Stats{AcquiredConns: 1, ConstructingConns: 1, MaxConns: 2},
			want:  true,
		},
	} {
		if got := poolIsSaturated(test.stats); got != test.want {
			t.Fatalf("poolIsSaturated(%s) = %t, want %t", name, got, test.want)
		}
	}
}

func TestBoundedContextWithCauseAppliesPositiveTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := boundedContextWithCause(context.Background(), time.Hour, ErrAcquireTimeout)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("bounded context has no deadline")
	}
}

type stubPoolBackend struct {
	acquire func(context.Context) (*pgxpool.Conn, error)
	ping    func(context.Context) error
	close   func()
	stats   Stats
}

func (s *stubPoolBackend) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	if s.acquire == nil {
		return nil, nil
	}

	return s.acquire(ctx)
}

func (s *stubPoolBackend) Ping(ctx context.Context) error {
	if s.ping == nil {
		return nil
	}

	return s.ping(ctx)
}

func (s *stubPoolBackend) Stats() Stats {
	return s.stats
}

func (s *stubPoolBackend) Close() {
	if s.close != nil {
		s.close()
	}
}
