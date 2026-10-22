// Package postgresservice adapts PostgreSQL pools to the Service v2 lifecycle.
// Constructor results are owned by the adapter. Existing pools remain shared
// unless ownership is explicitly transferred. The adapter performs no retries,
// closes active owned pools once per ordered service lifecycle, and never closes
// shared pools. Startup validation failure attempts owned cleanup separately;
// custom Resource callbacks retain their own context and concurrency contracts.
package postgresservice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	service "github.com/faustbrian/go-service/v2"
)

var (
	// ErrInvalidOptions identifies invalid adapter construction.
	ErrInvalidOptions = errors.New("invalid postgres service options")
	// ErrUnavailable identifies a pool that has not started or is stopping.
	ErrUnavailable = errors.New("postgres service pool unavailable")
)

// Resource is the lifecycle surface required from a PostgreSQL pool.
type Resource interface {
	// Ping checks connectivity using the service-supplied operation context.
	Ping(context.Context) error
	// Shutdown releases owned resources; implementations must honor their
	// context cooperatively.
	Shutdown(context.Context) error
}

// Constructor acquires a pool whose ownership transfers after a successful
// return. On an error the adapter does not close a partially returned resource;
// the constructor must clean up its own failed acquisition.
type Constructor func(context.Context) (Resource, error)

// Options configure one PostgreSQL lifecycle adapter.
type Options struct {
	// Name is the secret-safe component and readiness-check name.
	Name string
	// Construct acquires an adapter-owned pool during component startup.
	Construct Constructor
	// Pool supplies an existing pool. The caller retains ownership unless
	// TransferOwnership is true.
	Pool Resource
	// TransferOwnership closes Pool during shutdown and failed startup.
	TransferOwnership bool
	// StartupPing validates the acquired pool before startup succeeds.
	StartupPing bool
}

// OptionsError identifies one rejected option.
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

// StartupError preserves validation and partial-cleanup failures.
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

// Adapter owns the lifecycle state for one ordered PostgreSQL pool lifecycle.
// Construct it with New and do not copy it after use. Pool snapshots are
// synchronized; native resource operations must tolerate service probes and
// shutdown. Start must not be concurrent or repeated as a restart mechanism.
type Adapter struct {
	name      string
	construct Constructor
	existing  Resource
	owned     bool
	ping      bool

	mu       sync.RWMutex
	resource Resource
	active   bool
	stopOnce sync.Once
	stopErr  error
}

// New validates exactly one Pool or Construct and constructs an inert adapter.
// Resource acquisition is deferred to Component.Start. Use one ordered service
// lifecycle; this adapter is not a restartable pool factory.
func New(options Options) (*Adapter, error) {
	if strings.TrimSpace(options.Name) == "" {
		return nil, &OptionsError{Field: "Name", Reason: "must not be blank"}
	}
	hasPool := !nilResource(options.Pool)
	hasConstructor := options.Construct != nil
	if hasConstructor == hasPool {
		return nil, &OptionsError{Field: "Pool", Reason: "configure exactly one pool or constructor"}
	}
	return &Adapter{
		name: options.Name, construct: options.Construct, existing: options.Pool,
		owned: options.Construct != nil || options.TransferOwnership,
		ping:  options.StartupPing,
	}, nil
}

// Component returns the callbacks for one ordered startup/shutdown cycle.
// Construct and optional Ping run synchronously; Stop marks the adapter inactive
// and attempts owned Shutdown once, retaining any close error for later Stop calls.
func (adapter *Adapter) Component() service.Component {
	return service.Component{Name: adapter.name, Start: adapter.start, Stop: adapter.stop}
}

// Pool returns a borrowed resource and whether startup succeeded without stopping.
// The bool is a lifecycle snapshot, not a lease preventing concurrent shutdown;
// the resource may remain non-nil when inactive. Do not close an adapter-owned
// resource independently.
func (adapter *Adapter) Pool() (Resource, bool) {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	return adapter.resource, adapter.active
}

// Readiness returns an opt-in dependency check for the active pool. It must be
// added to the service plan separately; it does not enable startup validation.
// Inactive resources return ErrUnavailable without Ping. Active resources retain
// their own context bounds and may race with shutdown after the state snapshot.
func (adapter *Adapter) Readiness() service.ReadinessCheck {
	return service.ReadinessCheck{Name: adapter.name, Run: func(ctx context.Context) error {
		resource, ok := adapter.Pool()
		if !ok {
			return ErrUnavailable
		}
		return resource.Ping(ctx)
	}}
}

func (adapter *Adapter) start(ctx context.Context) error {
	resource := adapter.existing
	if adapter.construct != nil {
		var err error
		resource, err = adapter.construct(ctx)
		if err != nil {
			return err
		}
	}
	if nilResource(resource) {
		return ErrUnavailable
	}
	if adapter.ping {
		if err := resource.Ping(ctx); err != nil {
			var cleanup error
			if adapter.owned {
				cleanup = resource.Shutdown(ctx)
			}
			return &StartupError{Validation: err, Cleanup: cleanup}
		}
	}
	adapter.mu.Lock()
	adapter.resource = resource
	adapter.active = true
	adapter.mu.Unlock()
	return nil
}

func (adapter *Adapter) stop(ctx context.Context) error {
	// Closing admission does not revoke a resource already borrowed by a reader.
	adapter.stopOnce.Do(func() {
		adapter.mu.Lock()
		resource := adapter.resource
		active := adapter.active
		adapter.active = false
		adapter.mu.Unlock()
		if active && adapter.owned {
			adapter.stopErr = resource.Shutdown(ctx)
		}
	})
	return adapter.stopErr
}

func nilResource(resource Resource) bool {
	if resource == nil {
		return true
	}
	value := reflect.ValueOf(resource)
	return value.Kind() == reflect.Pointer && value.IsNil()
}
