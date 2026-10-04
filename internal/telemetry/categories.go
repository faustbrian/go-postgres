// Package telemetry owns the finite categorical projection shared by observers.
package telemetry

// Operation returns an owned built-in operation or the fixed unknown category.
func Operation(value string) string {
	return project(value, []string{"pool.acquire", "pool.ping", "pool.close", "transaction", "savepoint"})
}

// Outcome returns an owned built-in outcome or the fixed unknown category.
func Outcome(value string) string {
	return project(value, []string{"success", "error", "panic", "aborted"})
}

// ErrorKind preserves the empty no-error category and recognized policy kinds.
func ErrorKind(value string) string {
	return project(value, []string{
		"", "unknown", "unique_violation", "foreign_key_violation", "check_violation",
		"exclusion_violation", "serialization_failure", "deadlock", "timeout", "cancellation",
		"query_canceled", "lock_unavailable", "connectivity", "pool_exhaustion",
	})
}

// SQLState preserves no-state and exact recognized states, never arbitrary
// suffixes of the connection class accepted by raw error classification.
func SQLState(value string) string {
	return project(value, []string{
		"", "23505", "23503", "23514", "23P01", "40001", "40P01", "57014", "55P03",
		"57P01", "57P02", "57P03", "53300",
		"08000", "08001", "08003", "08004", "08006", "08007", "08P01",
	})
}

func project(value string, categories []string) string {
	for _, category := range categories {
		if value == category {
			// Return the fixed category, not caller-backed string storage. The
			// finite comparisons reject unequal lengths before examining bytes.
			return category
		}
	}

	return "unknown"
}
