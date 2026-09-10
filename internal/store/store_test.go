package store

import (
	"database/sql"
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

// TestMigrateAddsMissingColumn simulates a database created by an older version
// (no idempotency_key column) and verifies Open upgrades it additively.
func TestMigrateAddsMissingColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE expenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		amount_cents INTEGER NOT NULL,
		category_key TEXT NOT NULL,
		spent_on TEXT NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	);`)
	if err != nil {
		t.Fatalf("create old table: %v", err)
	}
	db.Close()

	s, err := Open(dbPath) // should ALTER in the missing column and add the index
	if err != nil {
		t.Fatalf("Open (upgrade): %v", err)
	}
	defer s.Close()

	if _, _, err := s.CreateWithKey(Expense{AmountCents: 100, CategoryKey: "other", SpentOn: "2026-09-01"}, "k1"); err != nil {
		t.Fatalf("insert after upgrade failed (column not added?): %v", err)
	}
}

func TestIsValidCategory(t *testing.T) {
	if !IsValidCategory("groceries") {
		t.Error("groceries should be a valid category")
	}
	if IsValidCategory("not_a_category") {
		t.Error("unknown key should be invalid")
	}
	if len(Categories) != 8 {
		t.Errorf("expected 8 default categories, got %d", len(Categories))
	}
}
