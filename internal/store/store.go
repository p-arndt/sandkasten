package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Session struct {
	ID           string
	Image        string
	ContainerID  string
	Status       string
	Cwd          string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	LastActivity time.Time
}

type Store struct {
	db *sql.DB
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS sessions (
	id            TEXT PRIMARY KEY,
	image         TEXT NOT NULL,
	container_id  TEXT NOT NULL,
	status        TEXT NOT NULL DEFAULT 'running',
	cwd           TEXT NOT NULL DEFAULT '/workspace',
	created_at    DATETIME NOT NULL,
	expires_at    DATETIME NOT NULL,
	last_activity DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
`

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting journal mode: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateSession(sess *Session) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, image, container_id, status, cwd, created_at, expires_at, last_activity)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Image, sess.ContainerID, sess.Status, sess.Cwd,
		sess.CreatedAt.UTC(), sess.ExpiresAt.UTC(), sess.LastActivity.UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}
	return nil
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, image, container_id, status, cwd, created_at, expires_at, last_activity
		 FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions() ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, image, container_id, status, cwd, created_at, expires_at, last_activity
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *Store) UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error {
	result, err := s.db.Exec(
		`UPDATE sessions SET cwd = ?, last_activity = ?, expires_at = ? WHERE id = ?`,
		cwd, time.Now().UTC(), expiresAt.UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("updating session activity: %w", err)
	}
	return checkRowAffected(result, id)
}

func (s *Store) UpdateSessionStatus(id string, status string) error {
	result, err := s.db.Exec(
		`UPDATE sessions SET status = ? WHERE id = ?`, status, id,
	)
	if err != nil {
		return fmt.Errorf("updating session status: %w", err)
	}
	return checkRowAffected(result, id)
}

func (s *Store) ListExpiredSessions() ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, image, container_id, status, cwd, created_at, expires_at, last_activity
		 FROM sessions WHERE status = 'running' AND expires_at <= ?`,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("listing expired sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *Store) ListRunningSessions() ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, image, container_id, status, cwd, created_at, expires_at, last_activity
		 FROM sessions WHERE status = 'running'`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing running sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *Store) DeleteSession(id string) error {
	result, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return checkRowAffected(result, id)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanSession(row scannable) (*Session, error) {
	var sess Session
	err := row.Scan(
		&sess.ID, &sess.Image, &sess.ContainerID, &sess.Status, &sess.Cwd,
		&sess.CreatedAt, &sess.ExpiresAt, &sess.LastActivity,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning session: %w", err)
	}
	return &sess, nil
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var sessions []*Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sessions: %w", err)
	}
	return sessions, nil
}

func checkRowAffected(result sql.Result, id string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session not found: %s", id)
	}
	return nil
}
