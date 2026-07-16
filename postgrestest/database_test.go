package postgrestest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

func TestWithDefaultsProvidesDeterministicPostgreSQLConfiguration(t *testing.T) {
	t.Parallel()

	config := withDefaults(Config{})
	if config.Image != defaultImage || config.Database != defaultDatabase ||
		config.Username != defaultUsername || config.Password != defaultPassword ||
		config.CleanupTimeout != defaultCleanupTimeout {
		t.Fatalf("withDefaults() = %#v", config)
	}

	explicit := Config{
		Image: "image", Database: "db", Username: "user", Password: "pass",
		CleanupTimeout: time.Second,
	}
	if got := withDefaults(explicit); got.Image != explicit.Image || got.Database != explicit.Database ||
		got.Username != explicit.Username || got.Password != explicit.Password ||
		got.CleanupTimeout != explicit.CleanupTimeout {
		t.Fatalf("withDefaults(explicit) = %#v", got)
	}
}

func TestStartDatabaseBoundsFailureCleanupAfterCallerCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	container := &stubDatabase{
		connectionErr: errors.New("connection string failed"),
		terminate: func(ctx context.Context) error {
			<-ctx.Done()

			return ctx.Err()
		},
	}
	started := time.Now()
	_, err := startDatabase(ctx, Config{CleanupTimeout: time.Millisecond}, stubStarter(container))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startDatabase() error = %v, want cleanup deadline", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("failure cleanup took %s", elapsed)
	}
}

func TestStartDatabasePreservesStartupFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("startup failed")
	_, err := startDatabase(context.Background(), Config{}, func(context.Context, Config) (startedDatabase, error) {
		return startedDatabase{}, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("startDatabase() error = %v, want sentinel", err)
	}
}

func TestStartDatabaseCleansUpConnectionStringFailure(t *testing.T) {
	t.Parallel()

	connectionErr := errors.New("connection string failed")
	terminateErr := errors.New("terminate failed")
	container := &stubDatabase{connectionErr: connectionErr, terminateErr: terminateErr}
	_, err := startDatabase(context.Background(), Config{}, stubStarter(container))
	if !errors.Is(err, connectionErr) || !errors.Is(err, terminateErr) {
		t.Fatalf("startDatabase() error = %v, want both failures", err)
	}
	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
	}
}

func TestStartDatabaseRunsSetupAndRetriesFailedClose(t *testing.T) {
	t.Parallel()

	setupErr := errors.New("setup failed")
	container := &stubDatabase{dsn: "postgres://test"}
	_, err := startDatabase(context.Background(), Config{
		Setup: func(_ context.Context, dsn string) error {
			if dsn != container.dsn {
				t.Fatalf("setup DSN = %q", dsn)
			}

			return setupErr
		},
	}, stubStarter(container))
	if !errors.Is(err, setupErr) || container.terminations != 1 {
		t.Fatalf("setup error = %v, terminations = %d", err, container.terminations)
	}

	closeErr := errors.New("close failed")
	container = &stubDatabase{dsn: "postgres://test", terminateErrors: []error{closeErr, nil}}
	database, err := startDatabase(context.Background(), Config{
		Setup: func(context.Context, string) error { return nil },
	}, stubStarter(container))
	if err != nil {
		t.Fatalf("startDatabase() error = %v", err)
	}
	if database.DSN() != container.dsn || database.Container() != nil {
		t.Fatalf("database = %#v", database)
	}
	firstErr := database.Close(context.Background())
	secondErr := database.Close(context.Background())
	thirdErr := database.Close(context.Background())
	if !errors.Is(firstErr, closeErr) || secondErr != nil || thirdErr != nil {
		t.Fatalf("Close() errors = (%v, %v)", firstErr, secondErr)
	}
	if container.terminations != 2 {
		t.Fatalf("termination calls = %d, want 2", container.terminations)
	}
}

func stubStarter(container testDatabase) databaseStarter {
	return func(context.Context, Config) (startedDatabase, error) {
		return startedDatabase{container: container}, nil
	}
}

type stubDatabase struct {
	dsn             string
	connectionErr   error
	terminateErr    error
	terminateErrors []error
	terminate       func(context.Context) error
	terminations    int
}

func (s *stubDatabase) ConnectionString(context.Context, ...string) (string, error) {
	return s.dsn, s.connectionErr
}

func (s *stubDatabase) Terminate(ctx context.Context, _ ...testcontainers.TerminateOption) error {
	s.terminations++
	if s.terminate != nil {
		return s.terminate(ctx)
	}
	if len(s.terminateErrors) > 0 {
		err := s.terminateErrors[0]
		s.terminateErrors = s.terminateErrors[1:]

		return err
	}

	return s.terminateErr
}
