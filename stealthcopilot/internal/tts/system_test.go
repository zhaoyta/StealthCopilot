package tts

import (
	"context"
	"reflect"
	"testing"
)

func TestSystemProviderEmptyText(t *testing.T) {
	ch, err := NewSystemProvider().Synthesize(context.Background(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := <-ch; ok {
		t.Fatal("empty text should close channel without chunks")
	}
}

func TestSystemProviderVoiceID(t *testing.T) {
	if got := NewSystemProvider().VoiceID(); got != "system-default" {
		t.Fatalf("VoiceID() = %q", got)
	}
}

func TestSystemTTSFFmpegArgs(t *testing.T) {
	got := systemTTSFFmpegArgs("/tmp/in.aiff")
	want := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-i", "/tmp/in.aiff",
		"-f", "s16le",
		"-ac", "1",
		"-ar", "24000",
		"pipe:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}
