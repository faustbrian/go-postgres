package postgrestest

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	gopostgres "github.com/faustbrian/go-postgres"
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

func TestOpenRejectsNilAndCanceledContextsBeforeStarting(t *testing.T) {
	t.Parallel()

	var starts atomic.Int32
	starter := func(context.Context, Config) (startedDatabase, error) {
		starts.Add(1)
		return startedDatabase{}, nil
	}
	var nilContext context.Context
	if _, err := openDatabase(nilContext, Config{}, starter); !errors.Is(err, gopostgres.ErrContextRequired) {
		t.Fatalf("openDatabase(nil) error = %v, want ErrContextRequired", err)
	}
	cause := errors.New("startup canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	if _, err := openDatabase(ctx, Config{}, starter); !errors.Is(err, cause) {
		t.Fatalf("openDatabase(canceled) error = %v, want caller cause", err)
	}
	if starts.Load() != 0 {
		t.Fatalf("starter calls = %d, want 0", starts.Load())
	}
	if _, err := Start(nilContext, Config{}); !errors.Is(err, gopostgres.ErrContextRequired) {
		t.Fatalf("Start(nil) error = %v, want ErrContextRequired", err)
	}
}

func TestDatabaseShutdownUsesOneTerminationAndIndependentCallerBounds(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	container := &stubDatabase{dsn: "postgres://test", terminate: func(context.Context) error {
		close(started)
		<-release
		return nil
	}}
	database, err := startDatabase(context.Background(), Config{CleanupTimeout: time.Second}, stubStarter(container))
	if err != nil {
		t.Fatalf("startDatabase() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := database.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown(canceled) error = %v, want cancellation", err)
	}
	<-started
	done := make(chan error, 1)
	go func() { done <- database.Close(context.Background()) }()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := database.Shutdown(context.Background()); err != nil {
		t.Fatalf("repeated Shutdown() error = %v", err)
	}
	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
	}
}

func TestDatabaseShutdownRejectsNilWithoutTermination(t *testing.T) {
	t.Parallel()

	container := &stubDatabase{dsn: "postgres://test"}
	database, err := startDatabase(context.Background(), Config{}, stubStarter(container))
	if err != nil {
		t.Fatalf("startDatabase() error = %v", err)
	}
	var nilContext context.Context
	if err := database.Shutdown(nilContext); !errors.Is(err, gopostgres.ErrContextRequired) {
		t.Fatalf("Shutdown(nil) error = %v, want ErrContextRequired", err)
	}
	if err := database.Close(nilContext); !errors.Is(err, gopostgres.ErrContextRequired) {
		t.Fatalf("Close(nil) error = %v, want ErrContextRequired", err)
	}
	if container.terminations != 0 {
		t.Fatalf("termination calls = %d, want 0", container.terminations)
	}
	if err := database.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestDatabaseShutdownSharesTerminationFailure(t *testing.T) {
	t.Parallel()

	terminationErr := errors.New("termination failed")
	container := &stubDatabase{dsn: "postgres://test", terminateErr: terminationErr}
	database, err := startDatabase(context.Background(), Config{}, stubStarter(container))
	if err != nil {
		t.Fatalf("startDatabase() error = %v", err)
	}
	for range 2 {
		if err := database.Shutdown(context.Background()); !errors.Is(err, terminationErr) {
			t.Fatalf("Shutdown() error = %v, want termination failure", err)
		}
	}
	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
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

func TestStartDatabasePreservesConnectionStringErrorWhenCleanupPanics(t *testing.T) {
	t.Parallel()

	connectionErr := errors.New("connection string failed")
	container := &stubDatabase{
		connectionErr: connectionErr,
		terminate: func(context.Context) error {
			panic("cleanup panic")
		},
	}
	_, err := startDatabase(context.Background(), Config{}, stubStarter(container))
	if !errors.Is(err, connectionErr) {
		t.Fatalf("startDatabase() error = %v, want connection string error", err)
	}
	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
	}
}

func TestStartDatabaseRunsSetupAndSharesFailedShutdown(t *testing.T) {
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
	if !errors.Is(firstErr, closeErr) || !errors.Is(secondErr, closeErr) || !errors.Is(thirdErr, closeErr) {
		t.Fatalf("Close() errors = (%v, %v, %v), want shared failure", firstErr, secondErr, thirdErr)
	}
	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
	}
}

func TestStartDatabaseCleansUpSetupPanic(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cleanupContextCanceled := false
	container := &stubDatabase{
		dsn: "postgres://test",
		terminate: func(ctx context.Context) error {
			cleanupContextCanceled = ctx.Err() != nil

			return nil
		},
	}
	const panicValue = "setup panic"
	defer func() {
		if recovered := recover(); recovered != panicValue {
			t.Fatalf("recovered panic = %v", recovered)
		}
		if container.terminations != 1 {
			t.Fatalf("termination calls = %d, want 1", container.terminations)
		}
		if cleanupContextCanceled {
			t.Fatal("cleanup inherited canceled setup context")
		}
	}()

	_, _ = startDatabase(ctx, Config{
		CleanupTimeout: time.Second,
		Setup: func(context.Context, string) error {
			cancel()
			panic(panicValue)
		},
	}, stubStarter(container))
}

func TestStartDatabasePreservesSetupPanicWhenCleanupPanics(t *testing.T) {
	t.Parallel()

	container := &stubDatabase{
		dsn: "postgres://test",
		terminate: func(context.Context) error {
			panic("cleanup panic")
		},
	}
	const panicValue = "setup panic"
	defer func() {
		if recovered := recover(); recovered != panicValue {
			t.Fatalf("recovered panic = %v, want %q", recovered, panicValue)
		}
		if container.terminations != 1 {
			t.Fatalf("termination calls = %d, want 1", container.terminations)
		}
	}()

	_, _ = startDatabase(context.Background(), Config{
		CleanupTimeout: time.Second,
		Setup: func(context.Context, string) error {
			panic(panicValue)
		},
	}, stubStarter(container))
}

func TestStartDatabasePreservesSetupErrorWhenCleanupPanics(t *testing.T) {
	t.Parallel()

	setupErr := errors.New("setup failed")
	container := &stubDatabase{
		dsn: "postgres://test",
		terminate: func(context.Context) error {
			panic("cleanup panic")
		},
	}
	_, err := startDatabase(context.Background(), Config{
		CleanupTimeout: time.Second,
		Setup:          func(context.Context, string) error { return setupErr },
	}, stubStarter(container))
	if !errors.Is(err, setupErr) {
		t.Fatalf("startDatabase() error = %v, want setup error", err)
	}
	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
	}
}

func TestStartDatabaseCleansUpSetupGoexit(t *testing.T) {
	t.Parallel()

	container := &stubDatabase{dsn: "postgres://test"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = startDatabase(context.Background(), Config{
			CleanupTimeout: time.Second,
			Setup: func(context.Context, string) error {
				runtime.Goexit()

				return nil
			},
		}, stubStarter(container))
	}()
	<-done

	if container.terminations != 1 {
		t.Fatalf("termination calls = %d, want 1", container.terminations)
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
