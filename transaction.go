package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// DefaultTransactionCleanupTimeout bounds rollback after cancellation.
	DefaultTransactionCleanupTimeout = 5 * time.Second
)

var (
	// ErrNilTransactionCallback reports a missing transaction body.
	ErrNilTransactionCallback = errors.New("postgres: transaction callback is nil")
)

// Beginner is implemented by pgx.Conn, pgxpool.Pool, pgxpool.Conn, and other
// native pgx values that can begin a transaction with explicit options.
type Beginner interface {
	// BeginTx returns a transaction whose finalization is delegated to the runner.
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// TransactionOptions combines native pgx transaction modes with the bounded
// cleanup policy used when the callback returns an error, panics, or terminates
// its goroutine.
type TransactionOptions struct {
	// TxOptions passes native transaction modes through without validation.
	pgx.TxOptions
	// CleanupTimeout bounds cooperative rollback; zero selects 5s, negative
	// expires immediately.
	CleanupTimeout time.Duration
	// Observer receives the final synchronous transaction observation; nil disables it.
	Observer Observer
}

// SavepointOptions controls cleanup and observation for a nested transaction.
type SavepointOptions struct {
	// CleanupTimeout bounds cooperative rollback; zero selects 5s, negative
	// expires immediately.
	CleanupTimeout time.Duration
	// Observer receives the final synchronous savepoint observation; nil disables it.
	Observer Observer
}

// RunTransaction begins a transaction and invokes fn once after a successful
// begin. A nil callback is refused before begin. A non-nil ctx and usable
// beginner are required. The callback borrows tx for this invocation: do not
// retain it or finalize it yourself. A nil callback result requests commit; an
// error requests rollback. A returned commit error is preserved without another
// rollback attempt or a durability guarantee. It never retries fn.
//
// Callback and rollback errors are joined so callers can inspect both with
// errors.Is and errors.As. A cleanup panic cannot replace
// a returned callback error, callback panic, or goroutine termination. A panic
// during commit triggers bounded rollback before the original value propagates.
//
// Rollback uses context.WithoutCancel(ctx) plus CleanupTimeout, preserving values
// but not caller cancellation/deadlines. Begin, body and commit retain the caller
// context and native semantics; deadlines cannot preempt uncooperative code.
// External callback effects are not undone by rollback.
func RunTransaction(
	ctx context.Context,
	beginner Beginner,
	options TransactionOptions,
	fn func(context.Context, pgx.Tx) error,
) (err error) {
	if fn == nil {
		return ErrNilTransactionCallback
	}

	started := time.Now()
	completed := false
	defer func() {
		panicValue := recover()
		switch {
		case panicValue != nil:
			observation := observationFor(OperationTransaction, started, nil)
			observation.Outcome = OutcomePanic
			safeObserve(ctx, options.Observer, observation)
			panic(panicValue)
		case !completed:
			observation := observationFor(OperationTransaction, started, nil)
			observation.Outcome = OutcomeAborted
			safeObserve(ctx, options.Observer, observation)
		default:
			safeObserve(ctx, options.Observer, observationFor(OperationTransaction, started, err))
		}
	}()

	err = runInTransaction(
		ctx,
		func(ctx context.Context) (pgx.Tx, error) {
			return beginner.BeginTx(ctx, options.TxOptions)
		},
		valueOrDefault(options.CleanupTimeout, DefaultTransactionCleanupTimeout),
		"transaction",
		fn,
	)
	completed = true

	return err
}

// RunSavepoint executes fn in an observed pgx pseudo-nested transaction with
// bounded rollback cleanup. pgx implements this with SAVEPOINT, RELEASE
// SAVEPOINT, and ROLLBACK TO SAVEPOINT. The callback borrows the child
// transaction; it must not finalize or retain it. The parent stays caller-owned
// and must still be finalized independently. Context, retry and failure-window
// rules are the same as RunTransaction; releasing a savepoint is not a durable
// commit of its parent.
func RunSavepoint(
	ctx context.Context,
	parent pgx.Tx,
	options SavepointOptions,
	fn func(context.Context, pgx.Tx) error,
) (err error) {
	if fn == nil {
		return ErrNilTransactionCallback
	}

	started := time.Now()
	completed := false
	defer func() {
		panicValue := recover()
		switch {
		case panicValue != nil:
			observation := observationFor(OperationSavepoint, started, nil)
			observation.Outcome = OutcomePanic
			safeObserve(ctx, options.Observer, observation)
			panic(panicValue)
		case !completed:
			observation := observationFor(OperationSavepoint, started, nil)
			observation.Outcome = OutcomeAborted
			safeObserve(ctx, options.Observer, observation)
		default:
			safeObserve(ctx, options.Observer, observationFor(OperationSavepoint, started, err))
		}
	}()

	err = runInTransaction(
		ctx,
		parent.Begin,
		valueOrDefault(options.CleanupTimeout, DefaultTransactionCleanupTimeout),
		"savepoint",
		fn,
	)
	completed = true

	return err
}

func runInTransaction(
	ctx context.Context,
	begin func(context.Context) (pgx.Tx, error),
	cleanupTimeout time.Duration,
	operation string,
	fn func(context.Context, pgx.Tx) error,
) (err error) {
	tx, err := begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin %s: %w", operation, err)
	}

	needsRollback := true
	defer func() {
		panicValue := recover()
		switch {
		case !needsRollback:
		case panicValue != nil:
			rollbackAfterTerminalCallback(ctx, tx, cleanupTimeout)
		case err == nil:
			rollbackAfterTerminalCallback(ctx, tx, cleanupTimeout)
		default:
			err = errors.Join(err, rollbackAfterCallbackError(ctx, tx, cleanupTimeout))
		}

		if panicValue != nil {
			panic(panicValue)
		}
	}()

	err = fn(ctx, tx)
	if err != nil {
		return err
	}

	// A returned native commit result is terminal; only a commit panic leaves
	// rollback armed for the deferred cleanup path.
	commitErr := tx.Commit(ctx)
	needsRollback = false
	if commitErr != nil {
		return fmt.Errorf("postgres: commit %s: %w", operation, commitErr)
	}

	return nil
}

func rollbackAfterCallbackError(ctx context.Context, tx pgx.Tx, timeout time.Duration) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()

	return rollbackTransaction(ctx, tx, timeout)
}

func rollbackAfterTerminalCallback(ctx context.Context, tx pgx.Tx, timeout time.Duration) {
	defer func() {
		_ = recover()
	}()
	_ = rollbackTransaction(ctx, tx, timeout)
}

func rollbackTransaction(ctx context.Context, tx pgx.Tx, timeout time.Duration) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	if err := tx.Rollback(cleanupCtx); err != nil {
		return fmt.Errorf("postgres: rollback transaction: %w", err)
	}

	return nil
}
