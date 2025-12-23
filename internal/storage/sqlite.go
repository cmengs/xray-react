package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store defines the minimal storage functions the collector needs.
type Store interface {
	SaveEvent(ctx context.Context, payload string, connectionID string) error
}

// SQLiteStore implements Store backed by sqlite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a sqlite database file and ensures the
// events table exists. Note: events table now includes connection_id column.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create table with connection_id column
	create := `CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		payload TEXT NOT NULL,
		connection_id TEXT,
		created_at DATETIME NOT NULL
	);`

	if _, err := db.Exec(create); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

// SaveEvent stores an event payload and the associated connection_id.
func (s *SQLiteStore) SaveEvent(ctx context.Context, payload string, connectionID string) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store not initialized")
	}

	stmt := `INSERT INTO events (payload, connection_id, created_at) VALUES (?, ?, ?)`
	_, err := s.db.ExecContext(ctx, stmt, payload, connectionID, time.Now().UTC())
	return err
}

// Close closes the underlying DB.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
