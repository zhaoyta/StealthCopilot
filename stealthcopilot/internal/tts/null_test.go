package tts

import (
	"context"
	"testing"
)

func TestNullExtension_Synthesize(t *testing.T) {
	p := &NullExtension{}
	ch, err := p.Synthesize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("NullExtension.Synthesize: unexpected error: %v", err)
	}
	_, open := <-ch
	if open {
		t.Error("NullExtension channel should be closed immediately")
	}
}

func TestNullExtension_VoiceID(t *testing.T) {
	p := &NullExtension{}
	if p.VoiceID() != "" {
		t.Error("NullExtension VoiceID should be empty string")
	}
}

func TestNullExtension_Close(t *testing.T) {
	p := &NullExtension{}
	if err := p.Close(); err != nil {
		t.Errorf("NullExtension.Close: unexpected error: %v", err)
	}
}
