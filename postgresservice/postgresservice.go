// Package postgresservice preserves the released service-adapter path. New
// code should import github.com/faustbrian/go-postgres/v2/adapters/service.
// New delegates the ordered lifecycle while retaining released type and error
// identities. Existing pools stay shared unless ownership is transferred;
// constructors own failed acquisition cleanup and successful results transfer.
package postgresservice

import (
	"context"
	"errors"
	"fmt"

	canonical "github.com/faustbrian/go-postgres/v2/adapters/service"
	"github.com/faustbrian/go-service"
)

var (
	// ErrInvalidOptions identifies invalid adapter construction.
	//
	// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
	ErrInvalidOptions = errors.New("invalid postgres service options")
	// ErrUnavailable identifies a pool that has not started or is stopping.
	//
	// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
	ErrUnavailable = errors.New("postgres service pool unavailable")
)

// Resource is the released adapter resource contract.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
type Resource interface {
	// Ping checks connectivity using the service-supplied operation context.
	Ping(context.Context) error
	// Close releases owned resources; implementations must honor their context
	// cooperatively.
	Close(context.Context) error
}

// Constructor acquires a pool whose ownership transfers after a successful
// return. On an error the adapter does not close a partially returned resource;
// the constructor must clean up its own failed acquisition.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
type Constructor func(context.Context) (Resource, error)

// Options configure one PostgreSQL lifecycle adapter.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
type Options struct {
	// Name is the secret-safe component and readiness-check name.
	Name string
	// Construct acquires an adapter-owned pool during component startup.
	Construct Constructor
	// Pool supplies an existing pool; ownership stays with the caller unless
	// transferred.
	Pool Resource
	// TransferOwnership closes Pool during shutdown and failed startup.
	TransferOwnership bool
	// StartupPing validates the acquired pool before startup succeeds.
	StartupPing bool
}

// OptionsError identifies a rejected option.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
type OptionsError struct {
	// Field names the rejected option.
	Field string
	// Reason describes the option refusal without echoing the supplied pool or name.
	Reason string
}

// Error returns a secret-safe construction diagnostic.
func (err *OptionsError) Error() string {
	return fmt.Sprintf("%s: %s: %v", err.Field, err.Reason, ErrInvalidOptions)
}

// Unwrap exposes the stable option classification.
func (err *OptionsError) Unwrap() error { return ErrInvalidOptions }

// StartupError preserves validation and cleanup failures.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
type StartupError struct {
	// Validation is the original failed startup ping; inspect under your
	// privacy policy.
	Validation error
	// Cleanup is an owned resource close failure; nil does not add a second cause.
	Cleanup error
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
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
type Adapter struct {
	delegate *canonical.Adapter
}

// New delegates to the canonical service adapter.
//
// Deprecated: import github.com/faustbrian/go-postgres/v2/adapters/service.
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
		var optionsErr *canonical.OptionsError
		if errors.As(err, &optionsErr) {
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

// Pool returns a borrowed resource and whether startup succeeded without stopping.
// The bool is a lifecycle snapshot, not a lease preventing concurrent shutdown;
// the resource may remain non-nil when inactive. Do not close an adapter-owned
// resource independently.
func (adapter *Adapter) Pool() (Resource, bool) {
	resource, active := adapter.delegate.Pool()

	return resource, active
}

// Readiness returns an opt-in dependency check for the active pool. It must be
// added to the service plan separately; it does not enable startup validation.
// Inactive resources return ErrUnavailable without Ping. Active resources retain
// their own context bounds and may race with shutdown after the state snapshot.
func (adapter *Adapter) Readiness() service.ReadinessCheck {
	check := adapter.delegate.Readiness()
	run := check.Run
	check.Run = func(ctx context.Context) error {
		return compatibilityError(run(ctx))
	}

	return check
}

func compatibilityError(err error) error {
	if errors.Is(err, canonical.ErrUnavailable) {
		return ErrUnavailable
	}
	var startupErr *canonical.StartupError
	if errors.As(err, &startupErr) {
		return &StartupError{
			Validation: startupErr.Validation,
			Cleanup:    startupErr.Cleanup,
		}
	}

	return err
}
