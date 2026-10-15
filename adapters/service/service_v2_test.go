package postgresservice_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	postgresservice "github.com/faustbrian/go-postgres/v2/adapters/service"
	service "github.com/faustbrian/go-service/v2"
)

func TestServiceV2PreservesPoolOwnershipAndReadiness(t *testing.T) {
	for _, ownership := range []string{"borrowed", "transferred", "constructed"} {
		t.Run(ownership, func(t *testing.T) {
			var pings, closes atomic.Int32
			closeErr := errors.New("close failed")
			pool := &v2Resource{
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
			check := adapter.ReadinessV2()
			if check.Name != "database" {
				t.Fatalf("readiness name = %q", check.Name)
			}
			if err := check.Run(context.Background()); !errors.Is(err, postgresservice.ErrUnavailable) || pings.Load() != 0 {
				t.Fatalf("inactive readiness = %v, pings = %d", err, pings.Load())
			}
			runtime, err := service.New(service.Config{Components: []service.Component{adapter.ComponentV2()}})
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
					err := adapter.ComponentV2().Stop(context.Background())
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

func TestServiceV2StartupFailurePreservesCleanupCauses(t *testing.T) {
	pingErr, closeErr := errors.New("ping failed"), errors.New("cleanup failed")
	closes := 0
	adapter, err := postgresservice.New(postgresservice.Options{
		Name: "database", StartupPing: true,
		Construct: func(context.Context) (postgresservice.Resource, error) {
			return &v2Resource{
				ping:  func(context.Context) error { return pingErr },
				close: func(context.Context) error { closes++; return closeErr },
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := service.New(service.Config{Components: []service.Component{adapter.ComponentV2()}})
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

func TestServiceV1AndV2HooksShareOnePoolLifecycle(t *testing.T) {
	closes := 0
	adapter, err := postgresservice.New(postgresservice.Options{
		Name: "database", TransferOwnership: true,
		Pool: &v2Resource{ping: func(context.Context) error { return nil }, close: func(context.Context) error { closes++; return nil }},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Component().Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ReadinessV2().Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ComponentV2().Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Component().Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if closes != 1 {
		t.Fatalf("shared lifecycle closes = %d", closes)
	}
	if err := adapter.Readiness().Run(context.Background()); !errors.Is(err, postgresservice.ErrUnavailable) {
		t.Fatalf("v1 readiness = %v", err)
	}
}

type v2Resource struct {
	ping  func(context.Context) error
	close func(context.Context) error
}

func (r *v2Resource) Ping(ctx context.Context) error  { return r.ping(ctx) }
func (r *v2Resource) Close(ctx context.Context) error { return r.close(ctx) }
