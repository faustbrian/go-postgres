package postgresservice_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	postgres "github.com/faustbrian/go-postgres/v3"
	postgresservice "github.com/faustbrian/go-postgres/v3/adapters/service"
	service "github.com/faustbrian/go-service/v3"
)

func TestServicePreservesPoolOwnershipAndReadiness(t *testing.T) {
	for _, ownership := range []string{"borrowed", "transferred", "constructed"} {
		t.Run(ownership, func(t *testing.T) {
			var pings, closes atomic.Int32
			closeErr := errors.New("close failed")
			pool := &lifecycleResource{
				ping:  func(ctx context.Context) error { pings.Add(1); return ctx.Err() },
				close: func(context.Context) error { closes.Add(1); return closeErr },
			}
			options := postgresservice.Options{Name: "database", Pool: pool, StartupPing: true}
			if ownership == "transferred" {
				options.TransferOwnership = true
			}
			if ownership == "constructed" {
				options.Pool = nil
				options.Construct = func(ctx context.Context) (postgresservice.Resource, error) {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					return pool, nil
				}
			}
			adapter, err := postgresservice.New(options)
			if err != nil {
				t.Fatal(err)
			}
			check := adapter.Readiness()
			if check.Name != "database" {
				t.Fatalf("readiness name = %q", check.Name)
			}
			if err := check.Run(context.Background()); !errors.Is(err, postgresservice.ErrUnavailable) || pings.Load() != 0 {
				t.Fatalf("inactive readiness = %v, pings = %d", err, pings.Load())
			}
			runtime, err := service.New(service.Config{Components: []service.Component{adapter.Component()}})
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			if current, active := adapter.Pool(); current != pool || !active {
				t.Fatal("started pool was not exposed")
			}
			if pings.Load() != 1 {
				t.Fatalf("startup pings = %d", pings.Load())
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := check.Run(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled readiness = %v", err)
			}
			if err := check.Run(context.Background()); err != nil {
				t.Fatalf("recovered readiness = %v", err)
			}
			shutdownErr := runtime.Shutdown(context.Background())
			owned := ownership != "borrowed"
			if (owned && !errors.Is(shutdownErr, closeErr)) || (!owned && shutdownErr != nil) {
				t.Fatalf("shutdown error = %v, owned = %t", shutdownErr, owned)
			}
			var callers sync.WaitGroup
			for range 8 {
				callers.Go(func() {
					err := adapter.Component().Stop(context.Background())
					if (owned && !errors.Is(err, closeErr)) || (!owned && err != nil) {
						t.Errorf("repeated stop = %v", err)
					}
				})
			}
			callers.Wait()
			wantCloses := int32(0)
			if owned {
				wantCloses = 1
			}
			if closes.Load() != wantCloses {
				t.Fatalf("closes = %d, want %d", closes.Load(), wantCloses)
			}
			before := pings.Load()
			if err := check.Run(context.Background()); !errors.Is(err, postgresservice.ErrUnavailable) || pings.Load() != before {
				t.Fatalf("stopped readiness = %v, pings = %d", err, pings.Load())
			}
		})
	}
}

func TestServiceStartupFailurePreservesCleanupCauses(t *testing.T) {
	pingErr, closeErr := errors.New("ping failed"), errors.New("cleanup failed")
	closes := 0
	adapter, err := postgresservice.New(postgresservice.Options{
		Name: "database", StartupPing: true,
		Construct: func(context.Context) (postgresservice.Resource, error) {
			return &lifecycleResource{
				ping:  func(context.Context) error { return pingErr },
				close: func(context.Context) error { closes++; return closeErr },
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := service.New(service.Config{Components: []service.Component{adapter.Component()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Start(context.Background()); !errors.Is(err, pingErr) || !errors.Is(err, closeErr) {
		t.Fatalf("startup error = %v, want ping and cleanup causes", err)
	}
	if _, active := adapter.Pool(); active || closes != 1 {
		t.Fatalf("failed startup active = %t, closes = %d", active, closes)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if closes != 1 {
		t.Fatalf("cleanup repeated after failed start: %d", closes)
	}
}

type lifecycleResource struct {
	ping  func(context.Context) error
	close func(context.Context) error
}

func (r *lifecycleResource) Ping(ctx context.Context) error     { return r.ping(ctx) }
func (r *lifecycleResource) Shutdown(ctx context.Context) error { return r.close(ctx) }

func TestServiceStopPassesServiceContextToOwnedShutdown(t *testing.T) {
	adapter, err := postgresservice.New(postgresservice.Options{
		Name: "database", TransferOwnership: true,
		Pool: &lifecycleResource{
			ping:  func(context.Context) error { return nil },
			close: func(ctx context.Context) error { return ctx.Err() },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	component := adapter.Component()
	if err := component.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := component.Stop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("stop error = %v, want the service context cancellation", err)
	}
}

func TestServiceOwnsOnlyTransferredNativePools(t *testing.T) {
	for _, transfer := range []bool{false, true} {
		pool, err := postgres.Connect(context.Background(), postgres.Config{
			DSN:           "postgres://localhost/app?sslmode=disable",
			StartupPolicy: postgres.StartupLazy,
			ResolveDSN:    postgres.ResolveNativeDSN,
		})
		if err != nil {
			t.Fatal(err)
		}
		adapter, err := postgresservice.New(postgresservice.Options{Name: "database", Pool: pool, TransferOwnership: transfer})
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := service.New(service.Config{Components: []service.Component{adapter.Component()}})
		if err != nil {
			t.Fatal(err)
		}
		if err := runtime.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Fatalf("transfer=%t shutdown = %v", transfer, err)
		}
		if err := adapter.Component().Stop(context.Background()); err != nil {
			t.Fatalf("transfer=%t repeated stop = %v", transfer, err)
		}
		liveness := pool.Liveness()
		if transfer {
			if liveness.Ready || !errors.Is(liveness.Err, postgres.ErrPoolClosed) {
				t.Fatalf("transferred pool liveness = %+v, want closed", liveness)
			}
			if err := pool.Ping(context.Background()); !errors.Is(err, postgres.ErrPoolClosed) {
				t.Fatalf("transferred pool ping = %v, want ErrPoolClosed", err)
			}
			continue
		}
		if !liveness.Ready {
			t.Fatalf("borrowed pool liveness = %+v, want open", liveness)
		}
		if err := pool.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
