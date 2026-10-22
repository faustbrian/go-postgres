package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"

	postgres "github.com/faustbrian/go-postgres/v3"
)

func TestPrepareConfigRequiresExplicitResolverHosted(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("native configuration admission regression runs only in hosted CI")
	}

	// PrepareConfig must refuse before native parsing can consult ambient sources.
	config, err := postgres.PrepareConfig(context.Background(), postgres.Config{
		DSN: "host=database.example port=5432 user=example database=example sslmode=disable",
	})
	if config != nil {
		t.Fatal("missing resolver returned a configuration")
	}
	var configErr *postgres.ConfigError
	if !errors.As(err, &configErr) {
		t.Fatal("missing resolver did not return a configuration error")
	}
	if configErr.Field != "resolver" || configErr.Problem != "is required" {
		t.Fatal("missing resolver returned the wrong admission category")
	}
	if err.Error() != "postgres: invalid resolver: is required" {
		t.Fatal("missing resolver returned a nonconstant error message")
	}
	if configErr.Cause != nil || errors.Unwrap(err) != nil {
		t.Fatal("missing resolver exposed an underlying cause")
	}
}
