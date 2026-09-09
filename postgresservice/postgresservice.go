// Package postgresservice preserves the released service-adapter path. New
// code should import github.com/faustbrian/go-postgres/adapters/service.
package postgresservice

import (
	"context"
	"errors"
	"fmt"

	canonical "github.com/faustbrian/go-postgres/adapters/service"
	"github.com/faustbrian/go-service"
)

var (
	// ErrInvalidOptions identifies invalid adapter construction.
	// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
	ErrInvalidOptions = errors.New("invalid postgres service options")
	// ErrUnavailable identifies a pool that has not started or is stopping.
	// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
	ErrUnavailable = errors.New("postgres service pool unavailable")
)

// Resource is the released adapter resource contract.
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
type Resource interface {
	Ping(context.Context) error
	Close(context.Context) error
}

// Constructor acquires a pool whose ownership transfers after a successful
// return.
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
type Constructor func(context.Context) (Resource, error)

// Options configure one PostgreSQL lifecycle adapter.
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
type Options struct {
	// Name is the secret-safe component and readiness-check name.
	Name string
	// Construct acquires an adapter-owned pool during component startup.
	Construct Constructor
	// Pool supplies an existing pool.
	Pool Resource
	// TransferOwnership closes Pool during shutdown and failed startup.
	TransferOwnership bool
	// StartupPing validates the acquired pool before startup succeeds.
	StartupPing bool
}

// OptionsError identifies a rejected option.
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
type OptionsError struct {
	Field  string
	Reason string
}

// Error returns a secret-safe construction diagnostic.
func (err *OptionsError) Error() string {
	return fmt.Sprintf("%s: %s: %v", err.Field, err.Reason, ErrInvalidOptions)
}

// Unwrap exposes the stable option classification.
func (err *OptionsError) Unwrap() error { return ErrInvalidOptions }

// StartupError preserves validation and cleanup failures.
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
type StartupError struct {
	Validation error
	Cleanup    error
}

// Error returns a secret-safe startup diagnostic.
func (err *StartupError) Error() string {
	if err.Cleanup != nil {
		return "postgres service startup validation and cleanup failed"
	}

	return "postgres service startup validation failed"
}

// Unwrap preserves both failures for errors.Is and errors.As.
func (err *StartupError) Unwrap() []error {
	causes := []error{err.Validation}
	if err.Cleanup != nil {
		causes = append(causes, err.Cleanup)
	}

	return causes
}

// Adapter preserves released type identity while delegating behavior to the
// canonical service adapter.
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
type Adapter struct {
	delegate *canonical.Adapter
}

// New delegates to the canonical service adapter.
//
// Deprecated: import github.com/faustbrian/go-postgres/adapters/service.
func New(options Options) (*Adapter, error) {
	return newAdapter(options, canonical.New)
}

type adapterFactory func(canonical.Options) (*canonical.Adapter, error)

func newAdapter(options Options, factory adapterFactory) (*Adapter, error) {
	var construct canonical.Constructor
	if options.Construct != nil {
		construct = func(ctx context.Context) (canonical.Resource, error) {
			return options.Construct(ctx)
		}
	}
	delegate, err := factory(canonical.Options{
		Name:              options.Name,
		Construct:         construct,
		Pool:              options.Pool,
		TransferOwnership: options.TransferOwnership,
		StartupPing:       options.StartupPing,
	})
	if err != nil {
		if optionsErr, ok := err.(*canonical.OptionsError); ok {
			return nil, &OptionsError{Field: optionsErr.Field, Reason: optionsErr.Reason}
		}

		return nil, err
	}

	return &Adapter{delegate: delegate}, nil
}

// Component returns the ordered service lifecycle component.
func (adapter *Adapter) Component() service.Component {
	component := adapter.delegate.Component()
	start := component.Start
	component.Start = func(ctx context.Context) error {
		return compatibilityError(start(ctx))
	}

	return component
}

// Pool returns the current pool after successful component startup.
func (adapter *Adapter) Pool() (Resource, bool) {
	resource, active := adapter.delegate.Pool()

	return resource, active
}

// Readiness returns an opt-in dependency check for the active pool.
func (adapter *Adapter) Readiness() service.ReadinessCheck {
	check := adapter.delegate.Readiness()
	run := check.Run
	check.Run = func(ctx context.Context) error {
		return compatibilityError(run(ctx))
	}

	return check
}

func compatibilityError(err error) error {
	if err == canonical.ErrUnavailable {
		return ErrUnavailable
	}
	if startupErr, ok := err.(*canonical.StartupError); ok {
		return &StartupError{
			Validation: startupErr.Validation,
			Cleanup:    startupErr.Cleanup,
		}
	}

	return err
}
