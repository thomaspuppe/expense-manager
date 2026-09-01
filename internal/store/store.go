// Package store owns SQLite persistence for expenses.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go driver (KTD1): no cgo, one static binary
)

// Store wraps the SQLite database handle.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at dbPath and ensures the
// schema exists. Connection pragmas are passed in the DSN so every pooled
// database/sql connection inherits them (KTD1 review fix): a one-off Exec would
// configure only whichever pooled connection served it, leaving other writers to
// hit "database is locked" and foreign keys silently off. busy_timeout makes a
// reader/writer collision retry briefly instead of erroring.
func Open(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)&_pragma=journal_mode(WAL)",
		dbPath,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS expenses (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	amount_cents    INTEGER NOT NULL,
	category_key    TEXT    NOT NULL,
	spent_on        TEXT    NOT NULL,          -- YYYY-MM-DD in the owner's local zone
	note            TEXT    NOT NULL DEFAULT '',
	created_at      TEXT    NOT NULL,          -- RFC3339 timestamp
	idempotency_key TEXT                       -- optional client key; guards double-submit on retry
);
CREATE INDEX IF NOT EXISTS idx_expenses_spent_on ON expenses (spent_on);
CREATE UNIQUE INDEX IF NOT EXISTS idx_expenses_idem ON expenses (idempotency_key) WHERE idempotency_key IS NOT NULL;
`

// migrate creates the schema if it does not already exist. It is idempotent.
func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}
