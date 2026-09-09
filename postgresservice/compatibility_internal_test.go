package postgresservice

import (
	"errors"
	"testing"

	canonical "github.com/faustbrian/go-postgres/adapters/service"
)

func TestNewAdapterPreservesUnexpectedCanonicalFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("canonical failure")
	_, err := newAdapter(Options{}, func(canonical.Options) (*canonical.Adapter, error) {
		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("newAdapter() error = %v, want canonical failure", err)
	}
}
