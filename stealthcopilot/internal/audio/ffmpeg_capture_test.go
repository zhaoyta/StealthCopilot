package audio

import "testing"

func TestNewSystemCaptureProviderChecked_MissingFFmpegReportsError(t *testing.T) {
	t.Setenv("PATH", "")
	provider, msg := NewSystemCaptureProviderChecked()
	if msg == "" {
		t.Fatal("expected missing ffmpeg error")
	}
	if _, ok := provider.(*NullCaptureProvider); !ok {
		t.Fatalf("provider = %T, want *NullCaptureProvider", provider)
	}
}

func TestNewSystemMicProviderChecked_MissingFFmpegReportsError(t *testing.T) {
	t.Setenv("PATH", "")
	provider, msg := NewSystemMicProviderChecked()
	if msg == "" {
		t.Fatal("expected missing ffmpeg error")
	}
	if provider == nil {
		t.Fatal("provider should not be nil")
	}
}
