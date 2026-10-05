package telemetry_test

import (
	"testing"

	"github.com/faustbrian/go-postgres/internal/telemetry"
)

func TestRecognizedTelemetryCategories(t *testing.T) {
	for _, domain := range []struct {
		name    string
		project func(string) string
		values  []string
	}{
		{"operation", telemetry.Operation, []string{"pool.acquire", "pool.ping", "pool.close", "transaction", "savepoint"}},
		{"outcome", telemetry.Outcome, []string{"success", "error", "panic", "aborted"}},
		{"kind", telemetry.ErrorKind, []string{"", "unknown", "unique_violation", "foreign_key_violation", "check_violation", "exclusion_violation", "serialization_failure", "deadlock", "timeout", "cancellation", "query_canceled", "lock_unavailable", "connectivity", "pool_exhaustion"}},
		{"state", telemetry.SQLState, []string{"", "23505", "23503", "23514", "23P01", "40001", "40P01", "57014", "55P03", "57P01", "57P02", "57P03", "53300", "08000", "08001", "08003", "08004", "08006", "08007", "08P01"}},
	} {
		t.Run(domain.name, func(t *testing.T) {
			for _, value := range domain.values {
				if domain.project(value) != value {
					t.Fatal("recognized telemetry category changed")
				}
			}
			for _, value := range []string{"ordinary-marker", "08note", "ABCDE"} {
				if domain.project(value) != "unknown" {
					t.Fatal("unknown telemetry category escaped")
				}
			}
			if domain.project(domain.values[0]) != domain.values[0] {
				t.Fatal("projection did not recover after unknown category")
			}
		})
	}
}
