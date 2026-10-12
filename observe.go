package postgres

import (
	"context"
	"time"

	"github.com/faustbrian/go-postgres/v2/internal/telemetry"
)

// Operation is a fixed low-cardinality operation name.
type Operation string

const (
	// OperationAcquire identifies pool connection acquisition.
	OperationAcquire Operation = "pool.acquire"
	// OperationPing identifies pool health checks.
	OperationPing Operation = "pool.ping"
	// OperationClose identifies bounded pool shutdown waits.
	OperationClose Operation = "pool.close"
	// OperationTransaction identifies top-level transactions.
	OperationTransaction Operation = "transaction"
	// OperationSavepoint identifies explicit nested savepoints.
	OperationSavepoint Operation = "savepoint"
)

// Outcome is a fixed low-cardinality operation result.
type Outcome string

const (
	// OutcomeSuccess indicates a nil operation error.
	OutcomeSuccess Outcome = "success"
	// OutcomeError indicates a classified operation error.
	OutcomeError Outcome = "error"
	// OutcomePanic indicates a transaction or savepoint panic.
	OutcomePanic Outcome = "panic"
	// OutcomeAborted indicates goroutine termination before the runner completed.
	OutcomeAborted Outcome = "aborted"
)

// Observation contains bounded metadata only. It intentionally excludes SQL,
// query arguments, DSNs, database error text, and arbitrary caller labels.
// Package-produced observations and built-in observers project unrecognized
// categories to "unknown"; Classify and SQLState retain native diagnostic data.
type Observation struct {
	// Operation identifies a wrapper lifecycle operation, not application SQL.
	Operation Operation
	// Outcome identifies completion, error, panic or goroutine termination.
	Outcome Outcome
	// Duration is sampled before observer delivery and excludes its own callback time.
	Duration time.Duration
	// ErrorKind is the finite projection of diagnostic classification.
	ErrorKind ErrorKind
	// SQLState is an exact recognized state, empty, or the fixed unknown category.
	SQLState string
	// Pool contains a separate native statistics snapshot when HasPoolStats is true.
	Pool Stats
	// HasPoolStats distinguishes an actual zero-valued snapshot from an
	// operation, such as a transaction, that did not sample pool state.
	HasPoolStats bool
}

// Observer consumes bounded lifecycle and transaction observations. Package
// delivery is synchronous and may be concurrent across operations. Keep callbacks
// prompt and concurrency-safe: ordinary panics are recovered, but blocking and
// goroutine termination are not isolated. The operation context may be canceled.
// Avoid recursively invoking observed operations from their own observer;
// delivery has no recursion guard and may still hold acquired resources.
type Observer interface {
	// Observe consumes metadata synchronously; concurrent calls must be safe.
	Observe(context.Context, Observation)
}

// ObserverFunc adapts a non-nil function to Observer without synchronization or
// recovery of direct calls. Package delivery supplies its own panic boundary.
type ObserverFunc func(context.Context, Observation)

// Observe implements Observer.
func (f ObserverFunc) Observe(ctx context.Context, observation Observation) {
	f(ctx, observation)
}

func safeObserve(ctx context.Context, observer Observer, observation Observation) {
	if observer == nil {
		return
	}

	defer func() {
		_ = recover()
	}()
	observer.Observe(ctx, boundedObservation(observation))
}

func boundedObservation(observation Observation) Observation {
	observation.Operation = Operation(telemetry.Operation(string(observation.Operation)))
	observation.Outcome = Outcome(telemetry.Outcome(string(observation.Outcome)))
	observation.ErrorKind = ErrorKind(telemetry.ErrorKind(string(observation.ErrorKind)))
	observation.SQLState = telemetry.SQLState(observation.SQLState)

	return observation
}

func observationFor(operation Operation, started time.Time, err error) Observation {
	info := Classify(err)
	outcome := OutcomeSuccess
	if err != nil {
		outcome = OutcomeError
	}

	return Observation{
		Operation: operation,
		Outcome:   outcome,
		Duration:  time.Since(started),
		ErrorKind: info.Kind,
		SQLState:  info.SQLState,
	}
}
