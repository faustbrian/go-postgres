package main

import (
	"context"
	"strings"
	"testing"

	migrations "github.com/faustbrian/go-migrations/v2"
)

func TestEmbeddedMigrationSourceLoadsCanonicalHistory(t *testing.T) {
	source, err := migrations.NewFSSource(embeddedSourceFileSystem{files: migrationFiles}, "schema")
	if err != nil {
		t.Fatalf("NewFSSource() error = %v", err)
	}

	loaded, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("Load() count = %d, want 2", len(loaded))
	}

	checks := []struct {
		version migrations.Version
		name    string
		mode    migrations.TransactionMode
		upSQL   string
	}{
		{1, "create_widgets", migrations.TransactionModeDefault, "CREATE TABLE widgets"},
		{2, "index_widgets", migrations.TransactionModeNone, "CREATE INDEX CONCURRENTLY widgets_name_idx"},
	}
	for index, check := range checks {
		migration := loaded[index]
		if migration.Version() != check.version || migration.Name() != check.name {
			t.Errorf("Load()[%d] identity = %d_%s, want %d_%s", index, migration.Version(), migration.Name(), check.version, check.name)
		}
		if migration.TransactionMode() != check.mode {
			t.Errorf("Load()[%d] transaction mode = %d, want %d", index, migration.TransactionMode(), check.mode)
		}
		if !strings.Contains(migration.UpSQL(), check.upSQL) {
			t.Errorf("Load()[%d] is missing the expected forward SQL", index)
		}
	}
}
