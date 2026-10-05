package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTelemetryTransactionCategoriesAndRecovery(t *testing.T) {
	ctx := context.Background()
	var observations []Observation
	observer := ObserverFunc(func(_ context.Context, value Observation) {
		observations = append(observations, value)
	})
	for _, code := range []string{"ordinary-state", "08note", "ABCDE", "23505"} {
		original := &pgconn.PgError{Code: code}
		tx := &stubTx{}
		err := RunTransaction(ctx, &stubBeginner{tx: tx}, TransactionOptions{Observer: observer}, func(context.Context, pgx.Tx) error {
			return original
		})
		var native *pgconn.PgError
		if !errors.Is(err, original) || !errors.As(err, &native) || native != original || tx.rollbacks != 1 || tx.commits != 0 {
			t.Fatal("telemetry changed transaction error or cleanup")
		}
		if info := Classify(err); info.SQLState != code || info.Postgres != original {
			t.Fatal("raw classification changed")
		}
		if state, ok := SQLState(err); !ok || state != code {
			t.Fatal("raw SQLState changed")
		}
	}
	tx := &stubTx{}
	if err := RunTransaction(ctx, &stubBeginner{tx: tx}, TransactionOptions{Observer: observer}, func(context.Context, pgx.Tx) error { return nil }); err != nil || tx.commits != 1 {
		t.Fatal("subsequent transaction did not recover")
	}
	wantStates := []string{"unknown", "unknown", "unknown", "23505", ""}
	wantKinds := []ErrorKind{ErrorUnknown, ErrorConnectivity, ErrorUnknown, ErrorUniqueViolation, ErrorNone}
	if len(observations) != len(wantStates) {
		t.Fatal("unexpected observation count")
	}
	for i, value := range observations {
		wantOutcome := OutcomeError
		if i == len(wantStates)-1 {
			wantOutcome = OutcomeSuccess
		}
		if value.Operation != OperationTransaction || value.Outcome != wantOutcome || value.ErrorKind != wantKinds[i] || value.SQLState != wantStates[i] {
			t.Fatal("transaction observation escaped finite categories")
		}
	}
}

func TestTelemetrySlogDirectCategories(t *testing.T) {
	var output bytes.Buffer
	observer := NewSlogObserver(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	observer.Observe(context.Background(), Observation{Operation: "ordinary-operation", Outcome: "ordinary-outcome", ErrorKind: "ordinary-kind", SQLState: "08note"})
	observer.Observe(context.Background(), Observation{Operation: OperationPing, Outcome: OutcomeSuccess})
	decoder := json.NewDecoder(&output)
	for i := range 2 {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal("log record was not valid JSON")
		}
		want := map[string]string{"operation": "unknown", "outcome": "unknown", "error.kind": "unknown", "db.response.status_code": "unknown", "level": "ERROR"}
		if i == 1 {
			want = map[string]string{"operation": "pool.ping", "outcome": "success", "error.kind": "", "db.response.status_code": "", "level": "DEBUG"}
		}
		for key, value := range want {
			if record[key] != value {
				t.Fatal("log category mismatch")
			}
		}
		if record["msg"] != "postgres operation" {
			t.Fatal("log message changed")
		}
	}
}
