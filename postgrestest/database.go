// Package postgrestest provides real PostgreSQL containers for integration
// tests. It never substitutes a fake for PostgreSQL semantics.
package postgrestest

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	gopostgres "github.com/faustbrian/go-postgres"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	testcontainerspostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	defaultImage          = "postgres:18-alpine"
	defaultDatabase       = "postgres_test"
	defaultUsername       = "postgres_test"
	defaultPassword       = "postgres_test"
	defaultCleanupTimeout = 30 * time.Second
)

// Config controls an isolated PostgreSQL container. Setup runs exactly once
// after PostgreSQL is accepting connections and receives its complete DSN.
// Setup errors, panics, or goroutine termination trigger bounded container
// cleanup. The original setup panic is preserved even if termination also
// panics.
type Config struct {
	Image    string
	Database string
	Username string
	Password string
	// HostPort optionally binds PostgreSQL to a stable loopback port. It is
	// useful for stop/start tests whose client endpoint must not move.
	HostPort string
	// CleanupTimeout bounds container termination even when the setup context
	// has already been canceled.
	CleanupTimeout time.Duration
	// Setup runs once after startup. An error, panic, or goroutine termination
	// triggers bounded container cleanup without replacing the original cause.
	Setup func(context.Context, string) error
}

// Database owns a PostgreSQL test container and its connection string.
type Database struct {
	container      testDatabase
	native         *testcontainerspostgres.PostgresContainer
	dsn            string
	cleanupTimeout time.Duration
	shutdownOnce   sync.Once
	shutdownDone   chan struct{}
	shutdownErr    error
}

// Start delegates to Open.
//
// Deprecated: use Open to make resource acquisition explicit.
func Start(ctx context.Context, config Config) (*Database, error) {
	return Open(ctx, config)
}

// Open creates a PostgreSQL container, waits for readiness, obtains a DSN,
// and invokes the optional deterministic setup hook.
func Open(ctx context.Context, config Config) (*Database, error) {
	return openDatabase(ctx, config, startPostgreSQL)
}

func openDatabase(ctx context.Context, config Config, starter databaseStarter) (*Database, error) {
	if ctx == nil {
		return nil, gopostgres.ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, context.Cause(ctx)
	}
	config = withDefaults(config)

	return startDatabase(ctx, config, starter)
}

type testDatabase interface {
	ConnectionString(context.Context, ...string) (string, error)
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

type startedDatabase struct {
	container testDatabase
	native    *testcontainerspostgres.PostgresContainer
}

type databaseStarter func(context.Context, Config) (startedDatabase, error)

func startPostgreSQL(ctx context.Context, config Config) (startedDatabase, error) {
	options := []testcontainers.ContainerCustomizer{
		testcontainerspostgres.WithDatabase(config.Database),
		testcontainerspostgres.WithUsername(config.Username),
		testcontainerspostgres.WithPassword(config.Password),
		testcontainerspostgres.BasicWaitStrategies(),
	}
	if config.HostPort != "" {
		options = append(options, testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			hostConfig.PortBindings = network.PortMap{
				network.MustParsePort("5432/tcp"): {
					{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: config.HostPort},
				},
			}
		}))
	}
	container, err := testcontainerspostgres.Run(
		ctx,
		config.Image,
		options...,
	)

	return startedDatabase{container: container, native: container}, err
}

func startDatabase(ctx context.Context, config Config, starter databaseStarter) (*Database, error) {
	config = withDefaults(config)
	started, err := starter(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("postgrestest: start PostgreSQL: %w", err)
	}

	dsn, err := started.container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanupCtx, cancel := cleanupContext(ctx, config.CleanupTimeout)
		defer cancel()

		return nil, errors.Join(
			fmt.Errorf("postgrestest: obtain connection string: %w", err),
			terminateAfterConnectionStringError(cleanupCtx, started.container),
		)
	}

	database := &Database{
		container: started.container, native: started.native, dsn: dsn,
		cleanupTimeout: config.CleanupTimeout,
		shutdownDone:   make(chan struct{}),
	}
	if config.Setup != nil {
		if err := setupDatabase(ctx, database, config.Setup); err != nil {
			return nil, err
		}
	}

	return database, nil
}

func terminateAfterConnectionStringError(ctx context.Context, container testDatabase) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()

	return container.Terminate(ctx)
}

func setupDatabase(
	ctx context.Context,
	database *Database,
	setup func(context.Context, string) error,
) (err error) {
	completed := false
	defer func() {
		panicValue := recover()
		if panicValue == nil && completed {
			return
		}

		cleanupCtx, cancel := cleanupContext(ctx, database.cleanupTimeout)
		defer cancel()
		closeAfterTerminalSetup(cleanupCtx, database)
		if panicValue != nil {
			panic(panicValue)
		}
	}()

	setupErr := setup(ctx, database.dsn)
	completed = true
	if setupErr != nil {
		cleanupCtx, cancel := cleanupContext(ctx, database.cleanupTimeout)
		defer cancel()

		return errors.Join(
			fmt.Errorf("postgrestest: setup database: %w", setupErr),
			closeAfterSetupError(cleanupCtx, database),
		)
	}

	return nil
}

func closeAfterSetupError(ctx context.Context, database *Database) error {
	return database.Close(ctx)
}

func closeAfterTerminalSetup(ctx context.Context, database *Database) {
	defer func() {
		_ = recover()
	}()
	_ = database.Close(ctx)
}

// DSN returns the complete pgx-compatible connection string. Treat it as a
// secret because it contains the configured test credentials.
func (d *Database) DSN() string {
	return d.dsn
}

// Container exposes the native testcontainers PostgreSQL container.
func (d *Database) Container() *testcontainerspostgres.PostgresContainer {
	return d.native
}

// Close delegates to Shutdown.
//
// Deprecated: use Shutdown for complete bounded owned shutdown.
func (d *Database) Close(ctx context.Context) error {
	return d.Shutdown(ctx)
}

// Shutdown starts one caller-independent, bounded container termination and
// lets each caller wait with its own context. Repeated calls return the shared
// terminal termination result.
func (d *Database) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return gopostgres.ErrContextRequired
	}

	d.shutdownOnce.Do(func() {
		go func() {
			cleanupCtx, cancel := cleanupContext(ctx, d.cleanupTimeout)
			defer cancel()
			d.shutdownErr = terminate(cleanupCtx, d.container)
			close(d.shutdownDone)
		}()
	})

	select {
	case <-d.shutdownDone:
		return d.shutdownErr
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func terminate(ctx context.Context, database testDatabase) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("postgrestest: container termination panicked")
		}
	}()

	return database.Terminate(ctx)
}

func withDefaults(config Config) Config {
	if config.Image == "" {
		config.Image = defaultImage
	}
	if config.Database == "" {
		config.Database = defaultDatabase
	}
	if config.Username == "" {
		config.Username = defaultUsername
	}
	if config.Password == "" {
		config.Password = defaultPassword
	}
	if config.CleanupTimeout <= 0 {
		config.CleanupTimeout = defaultCleanupTimeout
	}

	return config
}

func cleanupContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}
