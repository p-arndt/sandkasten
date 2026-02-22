package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Sentinel errors
var (
	ErrNotFound = errors.New("not found")
)

// isBusyLock reports whether err indicates SQLite database lock (SQLITE_BUSY).
// Handles wrapped errors from database/sql.
func isBusyLock(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "database is locked") || strings.Contains(s, "SQLITE_BUSY")
}

// retryOnBusy runs fn and retries on SQLITE_BUSY with exponential backoff.
func retryOnBusy(fn func() error) error {
	const maxAttempts = 4
	backoff := 25 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil || !isBusyLock(lastErr) {
			return lastErr
		}
		if attempt < maxAttempts-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return lastErr
}

// StatusPoolIdle indicates a session is idle in the pre-warmed pool.
// Reaper must not reap these sessions.
const StatusPoolIdle = "pool_idle"

type Session struct {
	ID           string    `json:"id"`
	Image        string    `json:"image"`
	InitPID      int       `json:"init_pid"`
	CgroupPath   string    `json:"cgroup_path"`
	Status       string    `json:"status"`
	Cwd          string    `json:"cwd"`
	WorkspaceID  string    `json:"workspace_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastActivity time.Time `json:"last_activity,omitempty"`
}

type Store struct {
	db *sql.DB
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS sessions (
	id            TEXT PRIMARY KEY,
	image         TEXT NOT NULL,
	init_pid      INTEGER NOT NULL DEFAULT 0,
	cgroup_path   TEXT NOT NULL DEFAULT '',
	status        TEXT NOT NULL DEFAULT 'running',
	cwd           TEXT NOT NULL DEFAULT '/workspace',
	workspace_id  TEXT,
	created_at    DATETIME NOT NULL,
	expires_at    DATETIME NOT NULL,
	last_activity DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_workspace_id ON sessions(workspace_id);
`

const migrateAddRuntimeFieldsSQL = `
ALTER TABLE sessions ADD COLUMN init_pid INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN cgroup_path TEXT NOT NULL DEFAULT '';
`

// DefaultMaxOpenConns is the default connection pool size for concurrent reads.
// WAL mode allows multiple readers + 1 writer; more conns improve read throughput.
const DefaultMaxOpenConns = 4

// dsnWithPragmas returns a connection string with WAL, busy_timeout, and perf
// pragmas applied to every new connection. Critical for parallel session creation:
// PRAGMAs in DSN are applied per-connection by the driver.
func dsnWithPragmas(dbPath string) string {
	// busy_timeout: 15s wait on lock (pool refill + API + reaper overlap)
	// journal_mode=WAL: concurrent reads during writes
	// synchronous=NORMAL: safe in WAL, ~50x faster writes than FULL
	// cache_size=-64000: 64MB page cache
	// temp_store=MEMORY: temp tables in RAM
	return dbPath + "?_pragma=busy_timeout(15000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=cache_size(-64000)" +
		"&_pragma=temp_store(MEMORY)"
}

// New opens the store. maxOpenConns controls the connection pool size (0 = default 4).
// For high scale: 4–8 allows concurrent reads while writers serialize; SQLite remains
// single-writer. For very high write throughput, consider PostgreSQL.
func New(dbPath string, maxOpenConns int) (*Store, error) {
	dsn := dsnWithPragmas(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if maxOpenConns <= 0 {
		maxOpenConns = DefaultMaxOpenConns
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	// Run migration for runtime fields (idempotent)
	db.Exec(migrateAddRuntimeFieldsSQL) // Ignore error if columns exist

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateSession(sess *Session) error {
	err := retryOnBusy(func() error {
		_, e := s.db.Exec(
			`INSERT INTO sessions (id, image, init_pid, cgroup_path, status, cwd, workspace_id, created_at, expires_at, last_activity)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sess.ID, sess.Image, sess.InitPID, sess.CgroupPath, sess.Status, sess.Cwd, sess.WorkspaceID,
			sess.CreatedAt.UTC(), sess.ExpiresAt.UTC(), sess.LastActivity.UTC(),
		)
		return e
	})
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}
	return nil
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, image, init_pid, cgroup_path, status, cwd, workspace_id, created_at, expires_at, last_activity
		 FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions() ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, image, init_pid, cgroup_path, status, cwd, workspace_id, created_at, expires_at, last_activity
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *Store) UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error {
	var result sql.Result
	err := retryOnBusy(func() error {
		var e error
		result, e = s.db.Exec(
			`UPDATE sessions SET cwd = ?, last_activity = ?, expires_at = ? WHERE id = ?`,
			cwd, time.Now().UTC(), expiresAt.UTC(), id,
		)
		return e
	})
	if err != nil {
		return fmt.Errorf("updating session activity: %w", err)
	}
	return checkRowAffected(result, id)
}

func (s *Store) UpdateSessionStatus(id string, status string) error {
	var result sql.Result
	err := retryOnBusy(func() error {
		var e error
		result, e = s.db.Exec(
			`UPDATE sessions SET status = ? WHERE id = ?`, status, id,
		)
		return e
	})
	if err != nil {
		return fmt.Errorf("updating session status: %w", err)
	}
	return checkRowAffected(result, id)
}

func (s *Store) UpdateSessionWorkspace(id string, workspaceID string) error {
	var result sql.Result
	err := retryOnBusy(func() error {
		var e error
		result, e = s.db.Exec(
			`UPDATE sessions SET workspace_id = ? WHERE id = ?`, workspaceID, id,
		)
		return e
	})
	if err != nil {
		return fmt.Errorf("updating session workspace: %w", err)
	}
	return checkRowAffected(result, id)
}

func (s *Store) ListExpiredSessions() ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, image, init_pid, cgroup_path, status, cwd, workspace_id, created_at, expires_at, last_activity
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
		`SELECT id, image, init_pid, cgroup_path, status, cwd, workspace_id, created_at, expires_at, last_activity
		 FROM sessions WHERE status = 'running'`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing running sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *Store) DeleteSession(id string) error {
	var result sql.Result
	err := retryOnBusy(func() error {
		var e error
		result, e = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
		return e
	})
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
	var workspaceID sql.NullString
	err := row.Scan(
		&sess.ID, &sess.Image, &sess.InitPID, &sess.CgroupPath, &sess.Status, &sess.Cwd,
		&workspaceID, &sess.CreatedAt, &sess.ExpiresAt, &sess.LastActivity,
	)
	if workspaceID.Valid {
		sess.WorkspaceID = workspaceID.String
	}
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
