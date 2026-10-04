package main

import (
	"context"
	"testing"
	"time"

	postgres "github.com/faustbrian/go-postgres"
	postgresotel "github.com/faustbrian/go-postgres/adapters/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// This consumer uses the published PostgreSQL dependency, not the root source.
func TestPublishedTelemetrySDKConsumer(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader), metric.WithResource(resource.Empty()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			t.Error("metric provider cleanup failed")
		}
	})
	observer, err := postgresotel.New(postgresotel.Config{MeterProvider: provider})
	if err != nil {
		t.Fatal("published observer construction failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	observer.Observe(ctx, postgres.Observation{
		Operation: postgres.OperationAcquire,
		Outcome:   postgres.OutcomeError,
		ErrorKind: postgres.ErrorPoolExhaustion,
		SQLState:  "53300",
		Duration:  time.Millisecond,
	})
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatal("metric collection failed")
	}
	points := 0
	for _, scope := range collected.ScopeMetrics {
		if scope.Scope.Name != "github.com/faustbrian/go-postgres/adapters/otel" {
			t.Fatal("published instrumentation scope changed")
		}
		for _, instrument := range scope.Metrics {
			if instrument.Name == "db.client.connection.count" {
				t.Fatal("observation without a pool snapshot emitted gauges")
			}
			if instrument.Name != "db.client.operation.count" {
				continue
			}
			sum, ok := instrument.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatal("operation count type changed")
			}
			for _, point := range sum.DataPoints {
				points++
				if point.Value != 1 {
					t.Fatal("operation count changed")
				}
				for _, expected := range []struct{ key, value string }{
					{"db.operation.name", "pool.acquire"},
					{"operation.outcome", "error"},
					{"error.type", "pool_exhaustion"},
					{"db.response.status_code", "53300"},
				} {
					found := false
					for _, actual := range point.Attributes.ToSlice() {
						if string(actual.Key) == expected.key && actual.Value.AsString() == expected.value {
							found = true
							break
						}
					}
					if !found {
						t.Fatal("published metric category changed")
					}
				}
			}
		}
	}
	if points != 1 {
		t.Fatal("operation count was not recorded exactly once")
	}
}
