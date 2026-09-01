package store

import (
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenCreatesSchema(t *testing.T) {
	s := tempStore(t)

	var name string
	err := s.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='expenses'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("expenses table not found after Open: %v", err)
	}
	if name != "expenses" {
		t.Fatalf("got table %q, want expenses", name)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.Close()

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open on existing db: %v", err)
	}
	s2.Close()
}

func TestIsValidCategory(t *testing.T) {
	if !IsValidCategory("groceries") {
		t.Error("groceries should be a valid category")
	}
	if IsValidCategory("not_a_category") {
		t.Error("unknown key should be invalid")
	}
	if len(Categories) != 10 {
		t.Errorf("expected 10 default categories, got %d", len(Categories))
	}
}
