package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a simple SQLite-backed event store that uses prepared statements.
type SQLiteStore struct {
	db         *sql.DB
	insertStmt *sql.Stmt
	listStmt   *sql.Stmt
}

// Event represents a stored event.
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// NewSQLiteStore opens the database, creates tables if necessary, and prepares statements.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// set pragmas similar to DSN params used previously
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")

	// Create table if not exists
	schema := `CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	payload TEXT,
	created_at DATETIME DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	ins, err := db.Prepare("INSERT INTO events(type,payload) VALUES(?, ?)")
	if err != nil {
		db.Close()
		return nil, err
	}

	// listStmt supports an optional type filter: (? = '' OR type = ?)
	listQ := `SELECT id, type, payload, created_at FROM events
	WHERE (? = '' OR type = ?)
	ORDER BY id DESC
	LIMIT ? OFFSET ?;`
	list, err := db.Prepare(listQ)
	if err != nil {
		ins.Close()
		db.Close()
		return nil, err
	}

	// assemble store
	s := &SQLiteStore{
		db:         db,
		insertStmt: ins,
		listStmt:   list,
	}
	return s, nil
}

// Close closes prepared statements and the underlying DB.
func (s *SQLiteStore) Close() error {
	if s.insertStmt != nil {
		s.insertStmt.Close()
	}
	if s.listStmt != nil {
		s.listStmt.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// InsertEvent inserts a new event using a prepared statement.
func (s *SQLiteStore) InsertEvent(ctx context.Context, typ, payload string) (int64, error) {
	res, err := s.insertStmt.ExecContext(ctx, typ, payload)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListEvents returns events with optional type filtering and pagination (limit, offset).
func (s *SQLiteStore) ListEvents(ctx context.Context, typ string, limit, offset int) ([]Event, error) {
	if typ == "" {
		// for prepared statement, pass empty string to match all
	}
	rows, err := s.listStmt.QueryContext(ctx, typ, typ, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		var createdStr string
		if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &createdStr); err != nil {
			return nil, err
		}
		// try to parse createdStr; if parse fails, leave zero value
		if t, err := time.Parse("2006-01-02T15:04:05.999Z", createdStr); err == nil {
			e.CreatedAt = t
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
