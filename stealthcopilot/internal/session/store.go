package session

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore persists sessions and turns in sessions.db.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens sessions.db under dataDir and runs migrations.
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, dbFileName)
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("session store: open db: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			started_at INTEGER NOT NULL,
			ended_at   INTEGER,
			resume_id  TEXT,
			label      TEXT
		);
		CREATE TABLE IF NOT EXISTS turns (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id       TEXT    NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			question         TEXT    NOT NULL,
			display_question TEXT    NOT NULL,
			answer           TEXT    NOT NULL,
			created_at       INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at DESC);
		CREATE INDEX IF NOT EXISTS idx_turns_session_created ON turns(session_id, created_at ASC);
	`)
	return err
}

// Begin creates a new session or reopens an existing one for the reserved resume path.
func (s *SQLiteStore) Begin(sessionID, resumeID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session store: session id is required")
	}
	now := unixMillis()
	_, err := s.db.Exec(`
		INSERT INTO sessions(id, started_at, ended_at, resume_id, label)
		VALUES(?, ?, NULL, ?, '')
		ON CONFLICT(id) DO UPDATE SET ended_at = NULL
	`, sessionID, now, strings.TrimSpace(resumeID))
	return err
}

// End marks a session as ended. Missing IDs are treated as no-op for shutdown safety.
func (s *SQLiteStore) End(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	_, err := s.db.Exec("UPDATE sessions SET ended_at = ? WHERE id = ?", unixMillis(), sessionID)
	return err
}

// AppendTurn saves one non-empty Q&A turn.
func (s *SQLiteStore) AppendTurn(sessionID, question, displayQuestion, answer string) error {
	sessionID = strings.TrimSpace(sessionID)
	question = strings.TrimSpace(question)
	displayQuestion = strings.TrimSpace(displayQuestion)
	answer = strings.TrimSpace(answer)
	if sessionID == "" || question == "" || answer == "" {
		return nil
	}
	if displayQuestion == "" {
		displayQuestion = question
	}
	_, err := s.db.Exec(
		"INSERT INTO turns(session_id, question, display_question, answer, created_at) VALUES(?,?,?,?,?)",
		sessionID, question, displayQuestion, answer, unixMillis(),
	)
	return err
}

// GetRecentTurns returns the latest turns in chronological order.
func (s *SQLiteStore) GetRecentTurns(sessionID string, limit int) ([]Turn, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, question, display_question, answer, created_at
		FROM (
			SELECT id, session_id, question, display_question, answer, created_at
			FROM turns
			WHERE session_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		)
		ORDER BY created_at ASC, id ASC
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTurns(rows)
}

// ListSessions returns recent sessions with turn counts.
func (s *SQLiteStore) ListSessions(limit int) ([]Session, error) {
	if limit <= 0 {
		limit = defaultSessionListLimit
	}
	rows, err := s.db.Query(`
		SELECT s.id, s.started_at, s.ended_at, COALESCE(s.resume_id, ''), COALESCE(s.label, ''), COUNT(t.id) AS turn_count
		FROM sessions s
		LEFT JOIN turns t ON t.session_id = s.id
		GROUP BY s.id
		ORDER BY s.started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var item Session
		var ended sql.NullInt64
		if err := rows.Scan(&item.ID, &item.StartedAt, &ended, &item.ResumeID, &item.Label, &item.TurnCount); err != nil {
			return nil, err
		}
		if ended.Valid {
			item.EndedAt = &ended.Int64
		}
		sessions = append(sessions, item)
	}
	return sessions, rows.Err()
}

// GetTurns returns all turns for one session in chronological order.
func (s *SQLiteStore) GetTurns(sessionID string) ([]Turn, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, question, display_question, answer, created_at
		FROM turns
		WHERE session_id = ?
		ORDER BY created_at ASC, id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTurns(rows)
}

// Delete removes an ended session and its turns.
func (s *SQLiteStore) Delete(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrSessionNotFound
	}
	var ended sql.NullInt64
	err := s.db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sessionID).Scan(&ended)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if !ended.Valid {
		return ErrSessionActive
	}
	_, err = s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// CloseOrphanSessions closes active sessions older than maxAgeMillis.
func (s *SQLiteStore) CloseOrphanSessions(maxAgeMillis int64) error {
	if maxAgeMillis <= 0 {
		return nil
	}
	cutoff := unixMillis() - maxAgeMillis
	_, err := s.db.Exec(`
		UPDATE sessions
		SET ended_at = started_at + ?
		WHERE ended_at IS NULL AND started_at < ?
	`, maxAgeMillis, cutoff)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func scanTurns(rows *sql.Rows) ([]Turn, error) {
	var turns []Turn
	for rows.Next() {
		var item Turn
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Question, &item.DisplayQuestion, &item.Answer, &item.CreatedAt); err != nil {
			return nil, err
		}
		turns = append(turns, item)
	}
	return turns, rows.Err()
}

func unixMillis() int64 {
	return time.Now().UnixMilli()
}
