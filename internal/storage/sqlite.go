package storage

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

type ConnectionEvent struct {
	ID         int64                  `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	SrcIP      string                 `json:"src_ip"`
	SrcPort    int                    `json:"src_port"`
	DstIP      string                 `json:"dst_ip"`
	DstPort    int                    `json:"dst_port"`
	Protocol   string                 `json:"protocol"`
	User       string                 `json:"user,omitempty"`
	BytesUp    int64                  `json:"bytes_up,omitempty"`
	BytesDown  int64                  `json:"bytes_down,omitempty"`
	Status     string                 `json:"status,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

type Store interface {
	Insert(ConnectionEvent) error
	ListRecent(limit int) ([]ConnectionEvent, error)
	Close() error
}

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// set pragmas similar to DSN params used previously
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")

	s := &SQLiteStore{db: db}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) init() error {
	schema := `
CREATE TABLE IF NOT EXISTS connection_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME,
	src_ip TEXT,
	src_port INTEGER,
	dst_ip TEXT,
	dst_port INTEGER,
	protocol TEXT,
	user TEXT,
	bytes_up INTEGER,
	bytes_down INTEGER,
	status TEXT,
	reason TEXT,
	duration_ms INTEGER,
	meta TEXT
);
CREATE INDEX IF NOT EXISTS idx_ts ON connection_events(timestamp);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Insert(ev ConnectionEvent) error {
	metaBytes, _ := json.Marshal(ev.Meta)
	_, err := s.db.Exec(
		`INSERT INTO connection_events(timestamp, src_ip, src_port, dst_ip, dst_port, protocol, user, bytes_up, bytes_down, status, reason, duration_ms, meta)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ev.Timestamp, ev.SrcIP, ev.SrcPort, ev.DstIP, ev.DstPort, ev.Protocol, ev.User, ev.BytesUp, ev.BytesDown, ev.Status, ev.Reason, ev.DurationMs, string(metaBytes),
	)
	return err
}

func (s *SQLiteStore) ListRecent(limit int) ([]ConnectionEvent, error) {
	rows, err := s.db.Query(`SELECT id, timestamp, src_ip, src_port, dst_ip, dst_port, protocol, user, bytes_up, bytes_down, status, reason, duration_ms, meta
		FROM connection_events ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConnectionEvent
	for rows.Next() {
		var ev ConnectionEvent
		var metaText sql.NullString
		var ts time.Time
		if err := rows.Scan(&ev.ID, &ts, &ev.SrcIP, &ev.SrcPort, &ev.DstIP, &ev.DstPort, &ev.Protocol, &ev.User, &ev.BytesUp, &ev.BytesDown, &ev.Status, &ev.Reason, &ev.DurationMs, &metaText); err != nil {
			return nil, err
		}
		ev.Timestamp = ts
		if metaText.Valid && metaText.String != "" {
			var m map[string]interface{}
			_ = json.Unmarshal([]byte(metaText.String), &m)
			ev.Meta = m
		}
		out = append(out, ev)
	}
	return out, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
