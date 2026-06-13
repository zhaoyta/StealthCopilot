package ui

import "testing"

func TestApplyStealthToHandleWithEmptyHandle(t *testing.T) {
	status, err := ApplyStealthToHandle(0)
	if err != nil {
		t.Fatalf("ApplyStealthToHandle(0) error = %v", err)
	}
	if status != StealthStatusUnavailable {
		t.Fatalf("ApplyStealthToHandle(0) status = %q, want %q", status, StealthStatusUnavailable)
	}
}
