package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// These microbenchmarks isolate wrapper overhead with native parsing or stubs;
// they are not evidence of database throughput, locking or latency.
func BenchmarkParseConfig(b *testing.B) {
	for b.Loop() {
		config, err := parseHostedConfig(Config{
			DSN:      "postgres://app:secret@localhost/app?sslmode=disable",
			MaxConns: 20,
		})
		if err != nil || config.MaxConns != 20 {
			b.Fatalf("parseHostedConfig() = %#v, %v", config, err)
		}
	}
}

// BenchmarkClassify measures policy extraction for one fixed unique-violation
// error; server query execution is excluded.
func BenchmarkClassify(b *testing.B) {
	err := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}

	for b.Loop() {
		if info := Classify(err); info.Kind != ErrorUniqueViolation {
			b.Fatalf("Classify().Kind = %q", info.Kind)
		}
	}
}

// BenchmarkPoolAcquireOverhead measures wrapper context/stats delivery around
// a stub backend, not actual native connection acquisition.
func BenchmarkPoolAcquireOverhead(b *testing.B) {
	pool := newPool(nil, &stubPoolBackend{}, time.Second, time.Second, time.Second)
	ctx := context.Background()

	for b.Loop() {
		if _, err := pool.Acquire(ctx); err != nil {
			b.Fatalf("Acquire() error = %v", err)
		}
	}
}

// BenchmarkPoolCreation includes lazy native construction and matching Close
// within each iteration, without startup connectivity or application queries.
func BenchmarkPoolCreation(b *testing.B) {
	ctx := context.Background()
	config := Config{
		ResolveDSN:    resolveHostedDSN,
		DSN:           "postgres://localhost/app?sslmode=disable",
		StartupPolicy: StartupLazy,
	}

	for b.Loop() {
		pool, err := Connect(ctx, config)
		if err != nil {
			b.Fatalf("Connect() error = %v", err)
		}
		if err := pool.Shutdown(ctx); err != nil {
			b.Fatalf("Shutdown() error = %v", err)
		}
	}
}

// BenchmarkObserverInstrumentation measures direct ObserverFunc delivery;
// it excludes package projection, panic protection and exporter work.
func BenchmarkObserverInstrumentation(b *testing.B) {
	ctx := context.Background()
	observer := ObserverFunc(func(context.Context, Observation) {})
	observation := Observation{
		Operation:    OperationAcquire,
		Outcome:      OutcomeSuccess,
		Pool:         Stats{MaxConns: 10},
		HasPoolStats: true,
	}

	for b.Loop() {
		observer.Observe(ctx, observation)
	}
}

// BenchmarkRunTransactionOverhead measures a successful stub begin/body/commit
// path with setup outside the loop; it excludes database round trips.
func BenchmarkRunTransactionOverhead(b *testing.B) {
	ctx := context.Background()
	tx := &stubTx{}
	beginner := &stubBeginner{tx: tx}
	callback := func(context.Context, pgx.Tx) error { return nil }

	for b.Loop() {
		if err := RunTransaction(ctx, beginner, TransactionOptions{}, callback); err != nil {
			b.Fatalf("RunTransaction() error = %v", err)
		}
	}
}
