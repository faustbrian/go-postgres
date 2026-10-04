package postgresotel_test

import (
	"context"
	"testing"

	postgres "github.com/faustbrian/go-postgres"
	canonical "github.com/faustbrian/go-postgres/adapters/otel"
	legacy "github.com/faustbrian/go-postgres/otelpostgres"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestTelemetryOTelVariantsDirectCategories(t *testing.T) {
	for _, variant := range []string{"canonical", "retained"} {
		t.Run(variant, func(t *testing.T) {
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			t.Cleanup(func() {
				if err := provider.Shutdown(context.Background()); err != nil {
					t.Error("provider shutdown failed")
				}
			})
			var observer postgres.Observer
			var err error
			if variant == "canonical" {
				observer, err = canonical.New(canonical.Config{MeterProvider: provider})
			} else {
				observer, err = legacy.New(legacy.Config{MeterProvider: provider})
			}
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
				wantScope := "github.com/faustbrian/go-postgres/adapters/otel"
				if variant == "retained" {
					wantScope = "github.com/faustbrian/go-postgres/otelpostgres"
				}
				if scope.Scope.Name != wantScope {
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
						want := map[string]string{"db.operation.name": "unknown", "operation.outcome": "unknown", "error.type": "unknown", "db.response.status_code": "unknown"}
						if operation.AsString() == "pool.acquire" {
							want = map[string]string{"db.operation.name": "pool.acquire", "operation.outcome": "error", "error.type": "pool_exhaustion", "db.response.status_code": "53300"}
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
		})
	}
}
