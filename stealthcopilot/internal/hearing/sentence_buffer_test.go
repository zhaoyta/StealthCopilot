package hearing

import "testing"

func TestNormalizeHearingText(t *testing.T) {
	if got := normalizeHearingText("  Tell   me\nabout  yourself  "); got != "Tell me about yourself" {
		t.Fatalf("normalizeHearingText = %q", got)
	}
}

func TestMergeHearingInterimPrefersCumulativeUpdate(t *testing.T) {
	if got := mergeHearingInterim("Tell me about", "Tell me about yourself"); got != "Tell me about yourself" {
		t.Fatalf("mergeHearingInterim cumulative = %q", got)
	}
	if got := mergeHearingInterim("Tell me about yourself", "Tell me about"); got != "Tell me about yourself" {
		t.Fatalf("mergeHearingInterim shorter update = %q", got)
	}
	if got := mergeHearingInterim("Why this", "company"); got != "Why this company" {
		t.Fatalf("mergeHearingInterim append = %q", got)
	}
	if got := mergeHearingInterim("And how did you motivate them to", "How did you motivate them to improve"); got != "And how did you motivate them to improve" {
		t.Fatalf("mergeHearingInterim overlap = %q", got)
	}
	if got := mergeHearingInterim("I once had an employee", "once had an employee who struggled"); got != "I once had an employee who struggled" {
		t.Fatalf("mergeHearingInterim shifted overlap = %q", got)
	}
	if !hearingIsCumulativeUpdate("Why this", "Why this company") {
		t.Fatal("expected cumulative update")
	}
}

func TestTrimSubmittedHearingPrefix(t *testing.T) {
	if got := trimSubmittedHearingPrefix("Tell me about yourself", "Tell me about yourself"); got != "" {
		t.Fatalf("exact submitted prefix = %q", got)
	}
	if got := trimSubmittedHearingPrefix("Tell me about yourself? Why this company", "Tell me about yourself"); got != "Why this company" {
		t.Fatalf("remaining text = %q", got)
	}
	if got := trimSubmittedHearingPrefix("Why this company", "Tell me about yourself"); got != "Why this company" {
		t.Fatalf("unrelated text = %q", got)
	}
}

func TestCompactHearingDraftTextRemovesAdjacentRepeats(t *testing.T) {
	input := "I once I once had an employee who was very careless"
	want := "I once had an employee who was very careless"
	if got := compactHearingDraftText(input); got != want {
		t.Fatalf("compactHearingDraftText = %q, want %q", got, want)
	}
}

func TestCompactHearingDraftTextRemovesNearbyRepeatedPhrase(t *testing.T) {
	input := "he had a large number of typos on every page of reports he wrote sometimes he had a large number of typos on every page of reports he wrote sometimes he sent email"
	want := "he had a large number of typos on every page of reports he wrote sometimes he sent email"
	if got := compactHearingDraftText(input); got != want {
		t.Fatalf("compactHearingDraftText = %q, want %q", got, want)
	}
}

func TestCompactHearingDraftTextKeepsLaterCompleteRollback(t *testing.T) {
	input := "What type of performance problems have you encountered in people who report to you And how did you motivate them to improve I once had an employe I once had an employee who was very careless He made tons of typos on every page of the reports he wrote and also sometimes S wrote and also sometimes sent emails emails to the , he made tons of typos on every page of the reports he wrote and also sometimes sent emails to the wrong people"
	want := "What type of performance problems have you encountered in people who report to you And how did you motivate them to improve I once had an employee who was very careless he made tons of typos on every page of the reports he wrote and also sometimes sent emails to the wrong people"
	if got := compactHearingDraftText(input); got != want {
		t.Fatalf("compactHearingDraftText = %q, want %q", got, want)
	}
}
