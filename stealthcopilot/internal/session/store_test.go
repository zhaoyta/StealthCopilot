package session

import (
	"errors"
	"testing"
	"time"
)

func TestSQLiteStore_BeginEndAppendListTurns(t *testing.T) {
	store := newTestStore(t)

	if err := store.Begin("sess-1", "resume-1"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := store.AppendTurn("sess-1", "Q1", "问题1", "A1"); err != nil {
		t.Fatalf("AppendTurn 1: %v", err)
	}
	if err := store.AppendTurn("sess-1", "Q2", "", "A2"); err != nil {
		t.Fatalf("AppendTurn 2: %v", err)
	}
	if err := store.End("sess-1"); err != nil {
		t.Fatalf("End: %v", err)
	}

	sessions, err := store.ListSessions(10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions len = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "sess-1" || sessions[0].ResumeID != "resume-1" || sessions[0].TurnCount != 2 {
		t.Fatalf("session summary mismatch: %+v", sessions[0])
	}
	if sessions[0].EndedAt == nil {
		t.Fatalf("EndedAt is nil")
	}

	turns, err := store.GetTurns("sess-1")
	if err != nil {
		t.Fatalf("GetTurns: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("GetTurns len = %d, want 2", len(turns))
	}
	if turns[0].DisplayQuestion != "问题1" {
		t.Fatalf("first display_question = %q", turns[0].DisplayQuestion)
	}
	if turns[1].DisplayQuestion != "Q2" {
		t.Fatalf("fallback display_question = %q, want Q2", turns[1].DisplayQuestion)
	}
}

func TestSQLiteStore_GetRecentTurnsIsolation(t *testing.T) {
	store := newTestStore(t)
	must(t, store.Begin("a", ""))
	must(t, store.Begin("b", ""))

	for _, q := range []string{"Q1", "Q2", "Q3"} {
		must(t, store.AppendTurn("a", q, q, "A-"+q))
		time.Sleep(time.Millisecond)
	}
	must(t, store.AppendTurn("b", "QB", "QB", "AB"))

	turns, err := store.GetRecentTurns("a", 2)
	if err != nil {
		t.Fatalf("GetRecentTurns: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("len = %d, want 2", len(turns))
	}
	if turns[0].Question != "Q2" || turns[1].Question != "Q3" {
		t.Fatalf("recent order mismatch: %+v", turns)
	}
	for _, turn := range turns {
		if turn.SessionID != "a" {
			t.Fatalf("cross-session turn leaked: %+v", turn)
		}
	}
}

func TestSQLiteStore_DeleteEndedOnly(t *testing.T) {
	store := newTestStore(t)
	must(t, store.Begin("active", ""))
	must(t, store.AppendTurn("active", "Q", "Q", "A"))

	if err := store.Delete("active"); !errors.Is(err, ErrSessionActive) {
		t.Fatalf("Delete active err = %v, want ErrSessionActive", err)
	}
	must(t, store.End("active"))
	if err := store.Delete("active"); err != nil {
		t.Fatalf("Delete ended: %v", err)
	}
	turns, err := store.GetTurns("active")
	if err != nil {
		t.Fatalf("GetTurns after delete: %v", err)
	}
	if len(turns) != 0 {
		t.Fatalf("turns remain after cascade delete: %d", len(turns))
	}
	if err := store.Delete("missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Delete missing err = %v, want ErrSessionNotFound", err)
	}
}

func TestSQLiteStore_CloseOrphanSessions(t *testing.T) {
	store := newTestStore(t)
	must(t, store.Begin("old", ""))
	must(t, store.Begin("fresh", ""))

	oldStart := time.Now().Add(-48 * time.Hour).UnixMilli()
	_, err := store.db.Exec("UPDATE sessions SET started_at = ? WHERE id = ?", oldStart, "old")
	if err != nil {
		t.Fatalf("seed old session: %v", err)
	}

	must(t, store.CloseOrphanSessions(int64(24*time.Hour/time.Millisecond)))

	sessions, err := store.ListSessions(10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	byID := map[string]Session{}
	for _, item := range sessions {
		byID[item.ID] = item
	}
	if byID["old"].EndedAt == nil {
		t.Fatalf("old session was not closed")
	}
	if byID["fresh"].EndedAt != nil {
		t.Fatalf("fresh session was unexpectedly closed")
	}
}

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
