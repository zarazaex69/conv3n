package api_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zarazaex69/conv3n/internal/storage"
)

var testCtx = context.Background()

func newTestStorage(t *testing.T) storage.Storage {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}
