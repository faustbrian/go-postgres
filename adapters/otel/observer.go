// Package postgresotel adapts postgres observations to standard OpenTelemetry
// metrics without recording SQL, arguments, DSNs, or raw errors.
package postgresotel

import (
	"context"

	postgres "github.com/faustbrian/go-postgres"
	"github.com/faustbrian/go-postgres/internal/oteladapter"
	"go.opentelemetry.io/otel/metric"
)

const scopeName = "github.com/faustbrian/go-postgres/adapters/otel"

// Config selects the standard OpenTelemetry meter provider.
type Config struct {
	MeterProvider metric.MeterProvider
}

// Observer records bounded lifecycle and transaction metrics.
type Observer struct {
	delegate *oteladapter.Observer
}

// New constructs an OpenTelemetry observer with standard database metric
// names and a no-op provider when none is supplied.
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
