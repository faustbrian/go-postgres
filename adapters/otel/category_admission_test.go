package postgresotel

import (
	"context"
	"testing"

	postgres "github.com/faustbrian/go-postgres/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestObserverCollapsesUnknownCategories(t *testing.T) {
	t.Parallel()

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error("provider shutdown failed")
		}
	})
	observer, err := New(Config{MeterProvider: provider})
	if err != nil {
		t.Fatal("observer construction failed")
	}
	observer.Observe(context.Background(), postgres.Observation{Operation: "ordinary-operation", Outcome: "ordinary-outcome", ErrorKind: "ordinary-kind", SQLState: "08note"})
	observer.Observe(context.Background(), postgres.Observation{Operation: postgres.OperationAcquire, Outcome: postgres.OutcomeError, ErrorKind: postgres.ErrorPoolExhaustion, SQLState: "53300"})
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal("metric collection failed")
	}
	points := 0
	for _, scope := range collected.ScopeMetrics {
		if scope.Scope.Name != scopeName {
			t.Fatal("instrumentation scope changed")
		}
		for _, instrument := range scope.Metrics {
			if instrument.Name != "db.client.operation.count" {
				continue
			}
			sum, ok := instrument.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatal("operation count type changed")
			}
			for _, point := range sum.DataPoints {
				points++
				operation, _ := point.Attributes.Value("db.operation.name")
				want := map[string]string{"db.system.name": "postgresql", "db.operation.name": "unknown", "operation.outcome": "unknown", "error.type": "unknown", "db.response.status_code": "unknown"}
				if operation.AsString() == "pool.acquire" {
					want = map[string]string{"db.system.name": "postgresql", "db.operation.name": "pool.acquire", "operation.outcome": "error", "error.type": "pool_exhaustion", "db.response.status_code": "53300"}
				}
				for key, value := range want {
					actual, ok := point.Attributes.Value(attribute.Key(key))
					if !ok || actual.AsString() != value {
						t.Fatal("metric category mismatch")
					}
				}
				if point.Value != 1 {
					t.Fatal("operation count changed")
				}
			}
		}
	}
	if points != 2 {
		t.Fatal("unexpected metric category count")
	}
}
