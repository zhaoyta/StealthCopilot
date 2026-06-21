package speaking

import "testing"

func TestSplitForTTSSplitsSentences(t *testing.T) {
	got := splitForTTSWithLimit("First answer is ready. Second answer is also ready! Third?", 200)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %#v", len(got), got)
	}
	if got[0] != "First answer is ready." {
		t.Fatalf("first = %q", got[0])
	}
}

func TestSplitForTTSSplitsLongText(t *testing.T) {
	got := splitForTTSWithLimit("This is a long clause, with a useful soft break, and the rest should continue after the split", 42)
	if len(got) < 2 {
		t.Fatalf("expected split, got %#v", got)
	}
	if got[0] == "" || got[1] == "" {
		t.Fatalf("empty sentence in %#v", got)
	}
}

func TestSplitForTTSNormalizesWhitespace(t *testing.T) {
	got := splitForTTS("  hello   world  ")
	if len(got) != 1 || got[0] != "hello world" {
		t.Fatalf("got %#v", got)
	}
}
