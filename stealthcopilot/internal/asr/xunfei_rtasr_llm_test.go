package asr

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseXunfeiRTASRLLMResponse(t *testing.T) {
	raw := []byte(`{
		"msg_type":"result",
		"res_type":"asr",
		"data":{
			"ls":true,
			"cn":{"st":{"type":"0","rt":[{"ws":[{"cw":[{"w":"hello","wp":"n"}]},{"cw":[{"w":" world","wp":"n"}]}]}]}}
		}
	}`)
	result, ok := parseXunfeiRTASRLLMResponse(raw)
	if !ok {
		t.Fatal("expected ASR result")
	}
	if result.SrcText != "hello world" {
		t.Fatalf("SrcText = %q", result.SrcText)
	}
	if !result.IsFinal {
		t.Fatal("expected final result")
	}
	if !result.Stable {
		t.Fatal("expected stable result")
	}
	if result.DstText != "" {
		t.Fatalf("DstText = %q, want empty", result.DstText)
	}
}

func TestParseXunfeiRTASRLLMStableType(t *testing.T) {
	raw := []byte(`{
		"msg_type":"result",
		"res_type":"asr",
		"data":{
			"ls":false,
			"cn":{"st":{"type":"0","rt":[{"ws":[{"cw":[{"w":"Tell me about yourself","wp":"n"}]}]}]}}
		}
	}`)
	result, ok := parseXunfeiRTASRLLMResponse(raw)
	if !ok {
		t.Fatal("expected ASR result")
	}
	if result.IsFinal {
		t.Fatal("did not expect final result")
	}
	if !result.Stable {
		t.Fatal("expected st.type=0 to mark stable")
	}
}

func TestSignXunfeiRTASRLLMStable(t *testing.T) {
	params := map[string]string{
		"appId":       "app",
		"accessKeyId": "key",
		"utc":         "2025-09-04T15:38:07+0800",
	}
	first := signXunfeiRTASRLLM(params, "secret")
	second := signXunfeiRTASRLLM(map[string]string{
		"utc":         "2025-09-04T15:38:07+0800",
		"accessKeyId": "key",
		"appId":       "app",
	}, "secret")
	if first == "" || first != second {
		t.Fatalf("signature should be stable, first=%q second=%q", first, second)
	}
}

func TestBuildXunfeiRTASRLLMURL(t *testing.T) {
	endpoint := buildXunfeiRTASRLLMURL(XunfeiRTASRLLMConfig{
		AppID:      "app",
		APIKey:     "key",
		APISecret:  "secret",
		SourceLang: "en",
	}, "session", time.Date(2025, 9, 4, 15, 38, 7, 0, time.FixedZone("CST", 8*3600)))
	if endpoint == "" || endpoint[:6] != "wss://" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestParseXunfeiRTASRLLMError(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"msg_type": "error",
		"code":     "35020",
		"desc":     "unsupported language",
	})
	if err := parseXunfeiRTASRLLMError(raw); err == nil {
		t.Fatal("expected error")
	}
}
