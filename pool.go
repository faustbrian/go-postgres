package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrContextRequired identifies an operation that received a nil context.
	ErrContextRequired = errors.New("postgres: context required")
	// ErrAcquireTimeout identifies the configured acquisition deadline.
	ErrAcquireTimeout = errors.New("postgres: acquire timeout")
	// ErrPoolExhausted identifies an acquisition timeout observed while all
	// configured connection slots were acquired or constructing.
	ErrPoolExhausted = errors.New("postgres: pool exhausted")
	// ErrHealthTimeout identifies a configured or caller health-check deadline.
	ErrHealthTimeout = errors.New("postgres: health check timeout")
	// ErrPoolClosed identifies a pool whose shutdown has begun.
	ErrPoolClosed = errors.New("postgres: pool closed")
	// ErrShutdownTimeout identifies a shutdown that outlasted its context.
	ErrShutdownTimeout = errors.New("postgres: shutdown timeout")
)

type poolBackend interface {
	Acquire(context.Context) (*pgxpool.Conn, error)
	Ping(context.Context) error
	Stats() Stats
	Close()
}

type nativePoolBackend struct {
	pool *pgxpool.Pool
}

func (b *nativePoolBackend) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	return b.pool.Acquire(ctx)
}

func (b *nativePoolBackend) Ping(ctx context.Context) error {
	return b.pool.Ping(ctx)
}

func (b *nativePoolBackend) Stats() Stats {
	return snapshotStats(b.pool.Stat())
}

func (b *nativePoolBackend) Close() {
	b.pool.Close()
}

// Pool owns a native pgxpool and adds bounded lifecycle waits. Use Connect;
// a zero Pool is not usable. Native operations and observer callbacks retain
// their own cooperation and concurrency requirements. Wrapper admission and
// shutdown state are synchronized; concurrent operations require safe native
// hooks and observers. Do not copy a Pool after use.
type Pool struct {
	raw             *pgxpool.Pool
	backend         poolBackend
	acquireTimeout  time.Duration
	pingTimeout     time.Duration
	shutdownTimeout time.Duration
	observer        Observer
	closeOnce       sync.Once
	closeDone       chan struct{}
	closed          atomic.Bool
}

// Stats is a detached value snapshot of native pgxpool statistics.
type Stats struct {
	// AcquireCount is the native count of successful acquisitions.
	AcquireCount int64
	// AcquiredConns is the number of connections currently borrowed.
	AcquiredConns int32
	// CanceledAcquireCount is the native count of canceled acquisitions.
	CanceledAcquireCount int64
	// ConstructingConns is the number of connections being constructed.
	ConstructingConns int32
	// EmptyAcquireCount counts successful acquisitions that waited for an empty pool.
	EmptyAcquireCount int64
	// EmptyAcquireWaitTime is cumulative wait time for successful acquisitions
	// from an empty pool; canceled waits are not included.
	EmptyAcquireWaitTime time.Duration
	// IdleConns is the number of currently idle connections.
	IdleConns int32
	// MaxConns is the configured native maximum connection count.
	MaxConns int32
	// NewConnsCount is the native cumulative count of created connections.
	NewConnsCount int64
	// TotalConns includes idle, acquired and constructing connections.
	TotalConns int32
}

// Health is the result of a readiness probe and its contemporaneous stats.
type Health struct {
	// Ready reports success of this probe, not a persistent availability guarantee.
	Ready bool
	// Err preserves the probe error; nil means the probe succeeded.
	Err error
	// Stats is sampled separately from the readiness ping or liveness check.
	Stats Stats
}

// Connect constructs a native pgxpool without a startup connection by default.
// StartupPing explicitly requests a bounded connectivity check. A nil
// or already-canceled context is rejected before configuration or I/O. The
// caller owns the successful pool and must Shutdown it. A failed startup ping
// closes the constructed backend before returning; native Close and application
// callbacks are not forcibly interrupted by the caller deadline.
func Connect(ctx context.Context, input Config) (*Pool, error) {
	return connect(ctx, input, openNativePool)
}

type poolFactory func(context.Context, *pgxpool.Config) (*pgxpool.Pool, poolBackend, error)

func openNativePool(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, poolBackend, error) {
	raw, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, err
	}

	return raw, &nativePoolBackend{pool: raw}, nil
}

func connect(ctx context.Context, input Config, factory poolFactory) (*Pool, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, context.Cause(ctx)
	}
	config, err := PrepareConfig(ctx, input)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}

	raw, backend, err := factory(ctx, config)
	if err != nil {
		return nil, &startupError{cause: err}
	}

	pool := newPool(
		raw,
		backend,
		valueOrDefault(input.AcquireTimeout, DefaultAcquireTimeout),
		valueOrDefault(input.PingTimeout, DefaultPingTimeout),
		valueOrDefault(input.ShutdownTimeout, DefaultShutdownTimeout),
		input.Observer,
	)
	if input.StartupPolicy == StartupLazy {
		return pool, nil
	}

	if err := pool.Ping(ctx); err != nil {
		backend.Close()

		return nil, &startupError{cause: err}
	}

	return pool, nil
}

func newPool(
	raw *pgxpool.Pool,
	backend poolBackend,
	acquireTimeout time.Duration,
	pingTimeout time.Duration,
	shutdownTimeout time.Duration,
	observers ...Observer,
) *Pool {
	var observer Observer
	if len(observers) > 0 {
		observer = observers[0]
	}

	return &Pool{
		raw:             raw,
		backend:         backend,
		acquireTimeout:  acquireTimeout,
		pingTimeout:     pingTimeout,
		shutdownTimeout: shutdownTimeout,
		observer:        observer,
		closeDone:       make(chan struct{}),
	}
}

// Raw returns a borrowed native pool without copying it or transferring ownership.
// Operations through Raw bypass wrapper acquisition bounds, shutdown admission
// checks and observations. Do not close it independently of its owning Pool.
func (p *Pool) Raw() *pgxpool.Pool {
	return p.raw
}

// Acquire obtains a connection while honoring the earlier of the caller's
// deadline and the configured acquisition timeout. A non-nil ctx is required.
// On success the caller must Release the connection; the timeout does not bound
// how long it is held or any subsequent query. Saturation is inferred from the
// sampled statistics only when the configured timeout is the context cause.
// Observer delivery runs before returning the connection and can extend the
// overall call duration.
func (p *Pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	started := time.Now()
	if p.closed.Load() {
		err := ErrPoolClosed
		observation := observationFor(OperationAcquire, started, err)
		observation.Pool = p.Stats()
		observation.HasPoolStats = true
		safeObserve(ctx, p.observer, observation)

		return nil, err
	}
	ctx, cancel := boundedContextWithCause(ctx, p.acquireTimeout, ErrAcquireTimeout)
	defer cancel()

	conn, err := p.backend.Acquire(ctx)
	stats := p.Stats()
	if err != nil && errors.Is(context.Cause(ctx), ErrAcquireTimeout) {
		err = errors.Join(ErrAcquireTimeout, err)
		if poolIsSaturated(stats) {
			err = errors.Join(ErrPoolExhausted, err)
		}
	}
	observation := observationFor(OperationAcquire, started, err)
	observation.Pool = stats
	observation.HasPoolStats = true
	safeObserve(ctx, p.observer, observation)

	return conn, err
}

// Ping checks PostgreSQL connectivity using the earlier caller or configured
// deadline. A non-nil ctx is required. The observer runs synchronously after
// the native probe and can extend the overall call duration.
func (p *Pool) Ping(ctx context.Context) error {
	started := time.Now()
	if p.closed.Load() {
		err := ErrPoolClosed
		observation := observationFor(OperationPing, started, err)
		observation.Pool = p.Stats()
		observation.HasPoolStats = true
		safeObserve(ctx, p.observer, observation)

		return err
	}
	ctx, cancel := boundedContext(ctx, p.pingTimeout)
	defer cancel()

	err := p.backend.Ping(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		err = errors.Join(ErrHealthTimeout, err)
	}
	observation := observationFor(OperationPing, started, err)
	observation.Pool = p.Stats()
	observation.HasPoolStats = true
	safeObserve(ctx, p.observer, observation)

	return err
}

// Readiness performs a bounded ping and returns a pool-statistics snapshot.
func (p *Pool) Readiness(ctx context.Context) Health {
	err := p.Ping(ctx)

	return Health{
		Ready: err == nil,
		Err:   err,
		Stats: p.Stats(),
	}
}

// Liveness reports whether pool shutdown has started without contacting the
// database. Database availability belongs to Readiness rather than liveness.
func (p *Pool) Liveness() Health {
	err := error(nil)
	if p.closed.Load() {
		err = ErrPoolClosed
	}

	return Health{
		Ready: err == nil,
		Err:   err,
		Stats: p.Stats(),
	}
}

// Stats returns a detached value copy of native pgxpool counters and gauges.
func (p *Pool) Stats() Stats {
	return p.backend.Stats()
}

func snapshotStats(stats *pgxpool.Stat) Stats {
	return Stats{
		AcquireCount:         stats.AcquireCount(),
		AcquiredConns:        stats.AcquiredConns(),
		CanceledAcquireCount: stats.CanceledAcquireCount(),
		ConstructingConns:    stats.ConstructingConns(),
		EmptyAcquireCount:    stats.EmptyAcquireCount(),
		EmptyAcquireWaitTime: stats.EmptyAcquireWaitTime(),
		IdleConns:            stats.IdleConns(),
		MaxConns:             stats.MaxConns(),
		NewConnsCount:        stats.NewConnsCount(),
		TotalConns:           stats.TotalConns(),
	}
}

// Shutdown closes admission and starts native pool shutdown exactly once. Each
// caller waits independently until its own or the configured shutdown bound.
// Native cleanup continues after an individual caller stops waiting, so a
// timeout is not proof of resource release. Release borrowed connections to
// allow cleanup to finish. Even an already-canceled non-nil ctx starts shutdown;
// nil ctx returns ErrContextRequired without changing admission. In-flight
// calls that passed the admission check retain native pgx behavior.
func (p *Pool) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return ErrContextRequired
	}

	started := time.Now()
	p.closed.Store(true)
	// Only the cleanup goroutine closes closeDone; caller deadlines never own it.
	p.closeOnce.Do(func() {
		go func() {
			p.backend.Close()
			close(p.closeDone)
		}()
	})

	ctx, cancel := boundedContext(ctx, p.shutdownTimeout)
	defer cancel()

	select {
	case <-p.closeDone:
		observation := observationFor(OperationClose, started, nil)
		observation.Pool = p.Stats()
		observation.HasPoolStats = true
		safeObserve(ctx, p.observer, observation)
		return nil
	case <-ctx.Done():
		err := errors.Join(ErrShutdownTimeout, ctx.Err())
		observation := observationFor(OperationClose, started, err)
		observation.Pool = p.Stats()
		observation.HasPoolStats = true
		safeObserve(ctx, p.observer, observation)

		return err
	}
}

func poolIsSaturated(stats Stats) bool {
	return stats.MaxConns > 0 &&
		stats.AcquiredConns+stats.ConstructingConns >= stats.MaxConns
}

func boundedContextWithCause(
	parent context.Context,
	timeout time.Duration,
	cause error,
) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}

	return context.WithTimeoutCause(parent, timeout, cause)
}

type startupError struct {
	cause error
}

func (e *startupError) Error() string {
	return "postgres: startup connectivity check failed"
}

func (e *startupError) Unwrap() error {
	return e.cause
}

func boundedContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}

	return context.WithTimeout(parent, timeout)
}
