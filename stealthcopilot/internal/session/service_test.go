package session

import "testing"

func TestService_ListAndGetTurns(t *testing.T) {
	store := newTestStore(t)
	must(t, store.Begin("sess", "resume-1"))
	must(t, store.AppendTurn("sess", "Q", "显示问题", "A"))
	must(t, store.End("sess"))

	svc := NewService(store, func(resumeID string) string {
		if resumeID == "resume-1" {
			return "backend.pdf"
		}
		return ""
	})

	sessions := svc.ListSessions(10)
	if len(sessions) != 1 {
		t.Fatalf("ListSessions len = %d, want 1", len(sessions))
	}
	if sessions[0].ResumeName != "backend.pdf" {
		t.Fatalf("ResumeName = %q", sessions[0].ResumeName)
	}

	turns := svc.GetSessionTurns("sess")
	if len(turns) != 1 || turns[0].DisplayQuestion != "显示问题" {
		t.Fatalf("turns mismatch: %+v", turns)
	}
}

func TestService_DeleteSession(t *testing.T) {
	store := newTestStore(t)
	must(t, store.Begin("active", ""))
	svc := NewService(store, nil)

	if got := svc.DeleteSession("active"); got == "" {
		t.Fatalf("DeleteSession active returned empty error")
	}
	must(t, store.End("active"))
	if got := svc.DeleteSession("active"); got != "" {
		t.Fatalf("DeleteSession ended = %q", got)
	}
	if got := svc.DeleteSession("missing"); got == "" {
		t.Fatalf("DeleteSession missing returned empty error")
	}
}
