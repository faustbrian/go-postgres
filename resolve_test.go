package postgres

import (
	"context"
	"errors"
	"testing"
)

func TestResolveNativeDSNParsesWithPGX(t *testing.T) {
	t.Parallel()

	config, err := ResolveNativeDSN(context.Background(), "postgres://app@db.example:5433/orders?sslmode=disable")
	if err != nil {
		t.Fatalf("ResolveNativeDSN() error = %v", err)
	}
	if config.ConnConfig.Host != "db.example" || config.ConnConfig.Port != 5433 ||
		config.ConnConfig.Database != "orders" || config.ConnConfig.User != "app" {
		t.Fatalf("ResolveNativeDSN() = %+v", config.ConnConfig)
	}
	if _, err := ResolveNativeDSN(context.Background(), "postgres://%zz"); err == nil {
		t.Fatal("ResolveNativeDSN(malformed) error = nil")
	}
}

func TestResolveNativeDSNHonorsContextBeforeParsing(t *testing.T) {
	t.Parallel()

	var nilContext context.Context
	if config, err := ResolveNativeDSN(nilContext, "postgres://app@db.example/orders"); config != nil || !errors.Is(err, ErrContextRequired) {
		t.Fatalf("ResolveNativeDSN(nil) = %v, %v", config, err)
	}
	cause := errors.New("resolution canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	if config, err := ResolveNativeDSN(ctx, "postgres://app@db.example/orders"); config != nil || !errors.Is(err, cause) {
		t.Fatalf("ResolveNativeDSN(canceled) = %v, %v; want %v", config, err, cause)
	}
}

func TestResolveNativeDSNPreparesThroughConfig(t *testing.T) {
	t.Parallel()

	config, err := PrepareConfig(context.Background(), Config{
		DSN: "postgres://app@db.example/orders", ResolveDSN: ResolveNativeDSN,
	})
	if err != nil || config.ConnConfig.Database != "orders" {
		t.Fatalf("PrepareConfig(ResolveNativeDSN) = %v, %v", config, err)
	}
}
