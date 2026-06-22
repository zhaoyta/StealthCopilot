// Package session stores interview sessions and their Q&A turns locally.
package session

import "errors"

const (
	dbFileName              = "sessions.db"
	defaultSessionListLimit = 100
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionActive   = errors.New("cannot delete active session")
)

// Session describes one hearing-chain interview session.
type Session struct {
	ID        string `json:"id"`
	StartedAt int64  `json:"started_at"`
	EndedAt   *int64 `json:"ended_at,omitempty"`
	ResumeID  string `json:"resume_id,omitempty"`
	Label     string `json:"label,omitempty"`
	TurnCount int    `json:"turn_count"`
}

// SessionSummary is the frontend view for one session row.
type SessionSummary struct {
	Session
	ResumeName string `json:"resume_name,omitempty"`
}

// Turn describes one saved question and answer in a session.
type Turn struct {
	ID              int64  `json:"id"`
	SessionID       string `json:"session_id"`
	Question        string `json:"question"`
	DisplayQuestion string `json:"display_question"`
	Answer          string `json:"answer"`
	CreatedAt       int64  `json:"created_at"`
}

// Store defines session persistence used by hearing and answer generation.
type Store interface {
	Begin(sessionID, resumeID string) error
	End(sessionID string) error
	AppendTurn(sessionID, question, displayQuestion, answer string) error
	GetRecentTurns(sessionID string, limit int) ([]Turn, error)
	ListSessions(limit int) ([]Session, error)
	GetTurns(sessionID string) ([]Turn, error)
	Delete(sessionID string) error
	CloseOrphanSessions(maxAgeMillis int64) error
	Close() error
}
