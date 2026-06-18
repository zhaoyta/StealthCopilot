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

func TestXunfeiVoiceTokenRetCodeHint(t *testing.T) {
	hint := xunfeiVoiceTokenRetCodeHint("000007")
	if !strings.Contains(hint, "签名校验失败") || !strings.Contains(hint, "官方 demo") {
		t.Fatalf("hint = %q", hint)
	}
}

func TestBuildXunfeiVoiceTokenBodyMatchesOfficialPythonDemo(t *testing.T) {
	body := buildXunfeiVoiceTokenBody("app", "123456")
	want := `{"base":{"appid":"app","version":"v1","timestamp":"123456"},"model":"remote"}`
	if body != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}

func TestXunfeiVoiceTokenSignMatchesOfficialPythonDemo(t *testing.T) {
	timestamp := "1710000000000"
	body := buildXunfeiVoiceTokenBody("appid123", timestamp)
	wantBody := `{"base":{"appid":"appid123","version":"v1","timestamp":"1710000000000"},"model":"remote"}`
	if body != wantBody {
		t.Fatalf("body = %s, want %s", body, wantBody)
	}
	sign := xunfeiVoiceTokenSign("key123", timestamp, body)
	if sign != "923410ed6c83f264f54a1f0549e576d5" {
		t.Fatalf("sign = %s", sign)
	}
}

func TestXunfeiVoiceSignKeyCandidates(t *testing.T) {
	candidates := xunfeiVoiceSignKeyCandidates(XunfeiVoiceCloneConfig{
		APIKey:    " key ",
		APISecret: "c2VjcmV0",
	})
	got := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		got = append(got, candidate.value)
	}
	want := []string{"key", "c2VjcmV0", "secret"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("candidates = %#v, want %#v", got, want)
	}
}

func TestXunfeiVoiceTokenDebugInfoDoesNotExposeSecrets(t *testing.T) {
	info := xunfeiVoiceTokenDebugInfo(XunfeiVoiceCloneConfig{
		AppID:     "appid123",
		APIKey:    "secret-api-key",
		APISecret: "secret-api-secret",
	}, `{"body":true}`, []string{"api_key:000007"})
	for _, secret := range []string{"appid123", "secret-api-key", "secret-api-secret", `{"body":true}`} {
		if strings.Contains(info, secret) {
			t.Fatalf("debug info exposes secret %q in %q", secret, info)
		}
	}
	for _, part := range []string{"app_id_len=8", "api_key_len=14", "api_secret_len=17", "token_body_sha256=", "api_key:000007"} {
		if !strings.Contains(info, part) {
			t.Fatalf("debug info missing %q in %q", part, info)
		}
	}
}
