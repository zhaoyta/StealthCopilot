package tts

import (
	"context"
	"testing"
)

func TestNullTTSProvider_Synthesize(t *testing.T) {
	p := &NullTTSProvider{}
	ch, err := p.Synthesize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("NullTTSProvider.Synthesize: unexpected error: %v", err)
	}
	_, open := <-ch
	if open {
		t.Error("NullTTSProvider channel should be closed immediately")
	}
}

func TestNullTTSProvider_VoiceID(t *testing.T) {
	p := &NullTTSProvider{}
	if p.VoiceID() != "" {
		t.Error("NullTTSProvider VoiceID should be empty string")
	}
}

func TestNullTTSProvider_Close(t *testing.T) {
	p := &NullTTSProvider{}
	if err := p.Close(); err != nil {
		t.Errorf("NullTTSProvider.Close: unexpected error: %v", err)
	}
}
