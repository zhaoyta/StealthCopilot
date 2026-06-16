// Package translation 单测：验证纯函数逻辑（响应解析、RTASR URL 鉴权格式）。
// 讯飞 WebSocket 连接属于集成测试，不在此覆盖。
package translation

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestXunfeiConfigReady(t *testing.T) {
	if XunfeiConfigReady(XunfeiConfig{}) {
		t.Fatal("empty Xunfei config should not be ready")
	}
	if !XunfeiConfigReady(XunfeiConfig{
		AppID:      "test-app",
		APIKey:     "test-key",
		SourceLang: "zh",
		TargetLang: "en",
	}) {
		t.Fatal("complete Xunfei config should be ready")
	}
}

func TestBuildXunfeiSigna_DocExample(t *testing.T) {
	got := buildXunfeiSigna(
		"595f23df",
		"d9f4aa7ea6d94faca62cd88a28fd5234",
		"1512041814",
	)
	if got != "IrrzsJeOFk1NGfJHW6SkHUoN9CU=" {
		t.Fatalf("signa = %q", got)
	}
}

func TestNormalizeXunfeiLang(t *testing.T) {
	for _, lang := range []string{"zh", "zh-CN", "chinese"} {
		if got := normalizeXunfeiLang(lang); got != "cn" {
			t.Fatalf("normalizeXunfeiLang(%q) = %q, want cn", lang, got)
		}
	}
	if got := normalizeXunfeiLang("en"); got != "en" {
		t.Fatalf("normalizeXunfeiLang(en) = %q", got)
	}
}

// TestParseXunfeiResponse_Translate 验证 RTASR 开启翻译后的 data 字符串正确解析为 DualResult。
func TestParseXunfeiResponse_Translate(t *testing.T) {
	data := marshalXunfeiOuter(`{"biz":"trans","src":"hello","dst":"你好","isEnd":true,"type":0}`)
	result, ok := parseXunfeiResponse(data)
	if !ok {
		t.Fatal("parseXunfeiResponse returned false for valid response")
	}
	if result.SrcText != "hello" {
		t.Errorf("SrcText = %q, want %q", result.SrcText, "hello")
	}
	if result.DstText != "你好" {
		t.Errorf("DstText = %q, want %q", result.DstText, "你好")
	}
	if !result.IsFinal {
		t.Error("IsFinal should be true when isEnd=true")
	}
}

// TestParseXunfeiResponse_NonZeroCode 验证 code != 0 时返回 false（API 错误）。
func TestParseXunfeiResponse_NonZeroCode(t *testing.T) {
	data := []byte(`{"action":"error","code":"10110","desc":"invalid authorization"}`)
	_, ok := parseXunfeiResponse(data)
	if ok {
		t.Error("should return false for non-zero code")
	}
}

// TestParseXunfeiResponse_EmptyContent 验证 src 和 dst 均为空时返回 false（心跳帧）。
func TestParseXunfeiResponse_EmptyContent(t *testing.T) {
	data := marshalXunfeiOuter(`{"biz":"trans","src":"","dst":"","isEnd":false}`)
	_, ok := parseXunfeiResponse(data)
	if ok {
		t.Error("should return false when both src and dst are empty")
	}
}

// TestParseXunfeiResponse_InvalidJSON 验证非法 JSON 不 panic 且返回 false。
func TestParseXunfeiResponse_InvalidJSON(t *testing.T) {
	_, ok := parseXunfeiResponse([]byte("not-json"))
	if ok {
		t.Error("should return false for invalid JSON")
	}
}

// TestParseXunfeiResponse_NotFinal 验证 isEnd=false 时 IsFinal 为 false。
func TestParseXunfeiResponse_NotFinal(t *testing.T) {
	data := marshalXunfeiOuter(`{"biz":"trans","src":"hi","dst":"嗨","isEnd":false}`)
	result, ok := parseXunfeiResponse(data)
	if !ok {
		t.Fatal("should parse successfully")
	}
	if result.IsFinal {
		t.Error("IsFinal should be false when is_end=0")
	}
}

func TestParseXunfeiResponse_ASR(t *testing.T) {
	data := marshalXunfeiOuter(`{"cn":{"st":{"type":"0","rt":[{"ws":[{"cw":[{"w":"你"}]},{"cw":[{"w":"好"}]}]}]}},"seg_id":1}`)
	result, ok := parseXunfeiResponse(data)
	if !ok {
		t.Fatal("should parse ASR result")
	}
	if result.SrcText != "你好" || result.DstText != "你好" {
		t.Fatalf("result = %#v, want source and target fallback text", result)
	}
	if !result.IsFinal {
		t.Fatal("ASR type=0 should be final")
	}
}

// TestBuildAuthURL_Format 验证生成的 RTASR 鉴权 URL 只包含转写参数，不启用昂贵实时翻译。
func TestBuildAuthURL_Format(t *testing.T) {
	p := NewXunfeiProvider(XunfeiConfig{
		AppID:      "test-app",
		APIKey:     "test-key",
		SourceLang: "cn",
		TargetLang: "en",
	})
	rawURL, err := p.buildAuthURL()
	if err != nil {
		t.Fatalf("buildAuthURL error: %v", err)
	}
	if !strings.HasPrefix(rawURL, xunfeiWSS) {
		t.Errorf("URL should start with %q, got %q", xunfeiWSS, rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for _, param := range []string{"appid", "ts", "signa", "lang"} {
		if query.Get(param) == "" {
			t.Errorf("URL missing required parameter %q", param)
		}
	}
	for _, param := range []string{"transType", "transStrategy", "targetLang"} {
		if query.Get(param) != "" {
			t.Errorf("RTASR URL should not include realtime translation param %q", param)
		}
	}
	if _, err := base64.StdEncoding.DecodeString(query.Get("signa")); err != nil {
		t.Errorf("signa is not valid base64: %v", err)
	}
	if query.Get("lang") != "cn" {
		t.Errorf("unexpected lang params: %s", query.Encode())
	}
}

func marshalXunfeiOuter(inner string) []byte {
	data, _ := json.Marshal(xunfeiResponse{
		Action: "result",
		Code:   json.RawMessage(`"0"`),
		Data:   inner,
		Desc:   "success",
	})
	return data
}
