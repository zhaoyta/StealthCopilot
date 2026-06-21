package trans

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseXunfeiTextTransResponse(t *testing.T) {
	decoded, _ := json.Marshal(map[string]any{
		"trans_result": map[string]string{
			"src": "hello",
			"dst": "你好",
		},
		"from": "en",
		"to":   "cn",
	})
	raw, _ := json.Marshal(map[string]any{
		"header": map[string]any{"code": 0, "message": "success"},
		"payload": map[string]any{
			"result": map[string]string{
				"text": base64.StdEncoding.EncodeToString(decoded),
			},
		},
	})
	got, err := parseXunfeiTextTransResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got != "你好" {
		t.Fatalf("translated = %q", got)
	}
}

func TestParseXunfeiTextTransResponseError(t *testing.T) {
	raw := []byte(`{"header":{"code":11200,"message":"bad request"}}`)
	if _, err := parseXunfeiTextTransResponse(raw); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildXunfeiTextTransURL(t *testing.T) {
	endpoint := buildXunfeiTextTransURL(XunfeiTextTransConfig{
		AppID:      "app",
		APIKey:     "key",
		APISecret:  "secret",
		SourceLang: "en",
		TargetLang: "zh",
	}, time.Date(2025, 9, 4, 7, 38, 7, 0, time.UTC))
	if endpoint == "" || endpoint[:8] != "https://" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestXunfeiTextTransConfigReady(t *testing.T) {
	if !XunfeiTextTransConfigReady(XunfeiTextTransConfig{
		AppID:      "app",
		APIKey:     "key",
		APISecret:  "secret",
		SourceLang: "en",
		TargetLang: "zh",
	}) {
		t.Fatal("expected config ready")
	}
	if XunfeiTextTransConfigReady(XunfeiTextTransConfig{AppID: "app"}) {
		t.Fatal("expected incomplete config")
	}
}
