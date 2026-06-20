package asr

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseXunfeiSimultRecognitionIATShape(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"bg":0,"ed":0,"ls":true,"pgs":"apd","sn":1,"ws":[{"cw":[{"w":"测试"}]}]}`))
	data, _ := json.Marshal(map[string]any{
		"header": map[string]any{"code": 0, "message": "success", "sid": "sid", "status": 1},
		"payload": map[string]any{
			"recognition_results": map[string]any{
				"encoding": "utf8",
				"format":   "json",
				"status":   2,
				"text":     encoded,
			},
		},
	})
	result, ok := parseXunfeiSimultResponse(data)
	if !ok {
		t.Fatal("should parse recognition result")
	}
	if result.SrcText != "测试" || result.DstText != "" {
		t.Fatalf("result = %#v", result)
	}
	if !result.IsFinal {
		t.Fatal("expected final result")
	}
}

func TestXunfeiSimultEmptySuccess(t *testing.T) {
	data := []byte(`{"header":{"code":0,"message":"success","sid":"sid","status":0}}`)
	if !isXunfeiSimultEmptySuccess(data) {
		t.Fatal("header-only success response should be treated as empty success")
	}
	if _, ok := parseXunfeiSimultResponse(data); ok {
		t.Fatal("header-only success response should not produce a result")
	}
}

func TestXunfeiBlankASRData(t *testing.T) {
	data := []byte(`{"bg":0,"ed":0,"ls":true,"ws":[{"cw":[{"w":""}]}]}`)
	if !isXunfeiBlankASRData(data) {
		t.Fatal("empty word ASR payload should be treated as blank")
	}
	nonBlank := []byte(`{"bg":0,"ed":0,"ls":true,"ws":[{"cw":[{"w":"你好"}]}]}`)
	if isXunfeiBlankASRData(nonBlank) {
		t.Fatal("non-empty word ASR payload should not be treated as blank")
	}
}

func TestXunfeiSimultNeedsTranslation(t *testing.T) {
	if !xunfeiSimultNeedsTranslation(XunfeiSimultConfig{SourceLang: "cn", TargetLang: "en"}) {
		t.Fatal("cn->en should need translation")
	}
	if xunfeiSimultNeedsTranslation(XunfeiSimultConfig{SourceLang: "zh-CN", TargetLang: "cn"}) {
		t.Fatal("zh-CN->cn should not need translation")
	}
}

func TestXunfeiSimultLangPairSupported(t *testing.T) {
	if !XunfeiSimultLangPairSupported("zh-CN", "en") {
		t.Fatal("zh-CN->en should be supported")
	}
	if XunfeiSimultLangPairSupported("en", "zh-CN") {
		t.Fatal("en->zh-CN should not be sent to iFlytek simultaneous interpretation")
	}
}

func TestXunfeiSimultSpeakTimeoutScalesWithAudio(t *testing.T) {
	short := make([]byte, 32000)
	if got := xunfeiSimultSpeakTimeout(short); got != 20*time.Second {
		t.Fatalf("short timeout = %s, want 20s", got)
	}
	long := make([]byte, 16000*2*30)
	if got := xunfeiSimultSpeakTimeout(long); got < 41*time.Second || got > 43*time.Second {
		t.Fatalf("long timeout = %s, want about 42s", got)
	}
}
