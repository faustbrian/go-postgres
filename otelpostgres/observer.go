// Package otelpostgres preserves the released OpenTelemetry adapter path.
// New code should import github.com/faustbrian/go-postgres/v2/adapters/otel.
package otelpostgres

import (
	"context"

	postgres "github.com/faustbrian/go-postgres/v2"
	"github.com/faustbrian/go-postgres/v2/internal/oteladapter"
	"go.opentelemetry.io/otel/metric"
)

const scopeName = "github.com/faustbrian/go-postgres/otelpostgres"

// Config selects the standard OpenTelemetry meter provider.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/otel.
type Config struct {
	MeterProvider metric.MeterProvider
}

// Observer records bounded lifecycle and transaction metrics.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/otel.
type Observer struct {
	delegate *oteladapter.Observer
}

// New constructs an OpenTelemetry observer with the released instrumentation
// scope and standard database metric names.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/otel.
func New(config Config) (*Observer, error) {
	delegate, err := oteladapter.New(scopeName, config.MeterProvider)
	if err != nil {
		return nil, err
	}

	return &Observer{delegate: delegate}, nil
}

// Observe implements postgres.Observer.
func (o *Observer) Observe(ctx context.Context, observation postgres.Observation) {
	o.delegate.Observe(ctx, observation)
}

var _ postgres.Observer = (*Observer)(nil)
