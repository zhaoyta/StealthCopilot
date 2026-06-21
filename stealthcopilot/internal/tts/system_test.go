package tts

import (
	"context"
	"reflect"
	"testing"
)

func TestSystemExtensionEmptyText(t *testing.T) {
	ch, err := NewSystemExtension().Synthesize(context.Background(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := <-ch; ok {
		t.Fatal("empty text should close channel without chunks")
	}
}

func TestSystemExtensionVoiceID(t *testing.T) {
	if got := NewSystemExtension().VoiceID(); got != "system-default" {
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
