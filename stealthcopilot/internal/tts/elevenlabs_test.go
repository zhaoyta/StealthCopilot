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
	// channel 应立即关闭（无音频输出）
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

func TestElevenLabsProvider_SynthesizeWithoutConfig(t *testing.T) {
	// APIKey 为空时应返回错误，而不是 panic
	p := NewElevenLabsProvider(ElevenLabsConfig{})
	_, err := p.Synthesize(context.Background(), "hello")
	if err == nil {
		t.Error("expected error when APIKey is empty")
	}
}

func TestElevenLabsProvider_VoiceID(t *testing.T) {
	p := NewElevenLabsProvider(ElevenLabsConfig{VoiceID: "test-voice-id"})
	if p.VoiceID() != "test-voice-id" {
		t.Errorf("VoiceID: expected 'test-voice-id', got '%s'", p.VoiceID())
	}
}

func TestElevenLabsProvider_DefaultModelID(t *testing.T) {
	p := NewElevenLabsProvider(ElevenLabsConfig{})
	if p.cfg.ModelID != "eleven_multilingual_v2" {
		t.Errorf("default ModelID: expected 'eleven_multilingual_v2', got '%s'", p.cfg.ModelID)
	}
}
