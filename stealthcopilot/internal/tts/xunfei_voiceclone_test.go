package tts

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestXunfeiVoiceCloneConfigReady(t *testing.T) {
	if XunfeiVoiceCloneConfigReady(XunfeiVoiceCloneConfig{AppID: "app", APIKey: "key", APISecret: "secret"}) {
		t.Fatal("config without AssetID should not be ready")
	}
	if !XunfeiVoiceCloneConfigReady(XunfeiVoiceCloneConfig{AppID: "app", APIKey: "key", APISecret: "secret", AssetID: "asset"}) {
		t.Fatal("complete config should be ready")
	}
}

func TestBuildXunfeiVoiceCloneAuthURL(t *testing.T) {
	rawURL, err := buildXunfeiVoiceCloneAuthURL(XunfeiVoiceCloneConfig{
		APIKey:    "key",
		APISecret: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rawURL, xunfeiVoiceCloneWSURL) {
		t.Fatalf("url = %q", rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for _, key := range []string{"authorization", "date", "host"} {
		if query.Get(key) == "" {
			t.Fatalf("missing %s in %s", key, rawURL)
		}
	}
	authBytes, err := base64.StdEncoding.DecodeString(query.Get("authorization"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(authBytes), `api_key="key"`) {
		t.Fatalf("authorization = %q", authBytes)
	}
}

func TestBuildXunfeiVoiceCloneSynthesisRequest(t *testing.T) {
	req := buildXunfeiVoiceCloneSynthesisRequest("app", "asset", "hello")
	if req.Header.AppID != "app" || req.Header.ResID != "asset" || req.Header.Status != 2 {
		t.Fatalf("unexpected header: %#v", req.Header)
	}
	if req.Parameter.TTS.VCN != xunfeiVoiceCloneVCN {
		t.Fatalf("unexpected vcn: %s", req.Parameter.TTS.VCN)
	}
	if req.Parameter.TTS.Audio.Encoding != "raw" || req.Parameter.TTS.Audio.SampleRate != 24000 {
		t.Fatalf("unexpected audio params: %#v", req.Parameter.TTS.Audio)
	}
	decoded, err := base64.StdEncoding.DecodeString(req.Payload.Text.Text)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "hello" {
		t.Fatalf("decoded text = %q", decoded)
	}
}
