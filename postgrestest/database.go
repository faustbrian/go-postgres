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

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
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
	Setup          func(context.Context, string) error
}

// Database owns a PostgreSQL test container and its connection string.
type Database struct {
	container      testDatabase
	native         *postgres.PostgresContainer
	dsn            string
	cleanupTimeout time.Duration
	closeMu        sync.Mutex
	closed         bool
}

// Start creates a PostgreSQL container, waits for readiness, obtains a DSN,
// and invokes the optional deterministic setup hook.
func Start(ctx context.Context, config Config) (*Database, error) {
	config = withDefaults(config)

	return startDatabase(ctx, config, startPostgreSQL)
}

type testDatabase interface {
	ConnectionString(context.Context, ...string) (string, error)
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

type startedDatabase struct {
	container testDatabase
	native    *postgres.PostgresContainer
}

type databaseStarter func(context.Context, Config) (startedDatabase, error)

func startPostgreSQL(ctx context.Context, config Config) (startedDatabase, error) {
	options := []testcontainers.ContainerCustomizer{
		postgres.WithDatabase(config.Database),
		postgres.WithUsername(config.Username),
		postgres.WithPassword(config.Password),
		postgres.BasicWaitStrategies(),
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
	container, err := postgres.Run(
		ctx,
		config.Image,
		options...,
	)

	return startedDatabase{container: container, native: container}, err
}

func startDatabase(ctx context.Context, config Config, starter databaseStarter) (*Database, error) {
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
			started.container.Terminate(cleanupCtx),
		)
	}

	database := &Database{
		container: started.container, native: started.native, dsn: dsn,
		cleanupTimeout: config.CleanupTimeout,
	}
	if config.Setup != nil {
		if err := config.Setup(ctx, dsn); err != nil {
			cleanupCtx, cancel := cleanupContext(ctx, config.CleanupTimeout)
			defer cancel()

			return nil, errors.Join(
				fmt.Errorf("postgrestest: setup database: %w", err),
				database.Close(cleanupCtx),
			)
		}
	}

	return database, nil
}

// DSN returns the complete pgx-compatible connection string. Treat it as a
// secret because it contains the configured test credentials.
func (d *Database) DSN() string {
	return d.dsn
}

// Container exposes the native testcontainers PostgreSQL container.
func (d *Database) Container() *postgres.PostgresContainer {
	return d.native
}

// Close terminates the container. Failed termination may be retried; after a
// successful termination, later calls return nil without repeating it.
func (d *Database) Close(ctx context.Context) error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()
	if d.closed {
		return nil
	}

	cleanupCtx, cancel := context.WithTimeout(ctx, d.cleanupTimeout)
	defer cancel()
	if err := d.container.Terminate(cleanupCtx); err != nil {
		return err
	}
	d.closed = true

	return nil
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
