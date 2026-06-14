// Package translation 单测：验证纯函数逻辑（帧构建、响应解析、URL 鉴权格式）。
// 讯飞 WebSocket 连接属于集成测试，不在此覆盖。
package translation

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildContFrame 验证音频中间帧的 base64 编码和状态字段。
func TestBuildContFrame(t *testing.T) {
	pcm := []byte{0x01, 0x02, 0x03}
	msg := buildContFrame(pcm)

	if msg.Data.Status != frameStatusCont {
		t.Errorf("status = %d, want %d (frameStatusCont)", msg.Data.Status, frameStatusCont)
	}
	want := base64.StdEncoding.EncodeToString(pcm)
	if msg.Data.Audio != want {
		t.Errorf("audio = %q, want %q", msg.Data.Audio, want)
	}
	if msg.Data.Encoding != "raw" {
		t.Errorf("encoding = %q, want %q", msg.Data.Encoding, "raw")
	}
}

// TestBuildLastFrame 验证最后帧的状态和 audio 为空。
func TestBuildLastFrame(t *testing.T) {
	msg := buildLastFrame()
	if msg.Data.Status != frameStatusLast {
		t.Errorf("status = %d, want %d (frameStatusLast)", msg.Data.Status, frameStatusLast)
	}
	if msg.Data.Audio != "" {
		t.Errorf("last frame audio should be empty, got %q", msg.Data.Audio)
	}
}

// TestParseXunfeiResponse_Valid 验证标准讯飞响应正确解析为 DualResult。
func TestParseXunfeiResponse_Valid(t *testing.T) {
	resp := xunfeiResponse{
		Code: 0,
		Data: struct {
			Status int `json:"status"`
			Result struct {
				Src   string `json:"src"`
				Dst   string `json:"dst"`
				IsEnd int    `json:"is_end"`
			} `json:"result"`
		}{
			Result: struct {
				Src   string `json:"src"`
				Dst   string `json:"dst"`
				IsEnd int    `json:"is_end"`
			}{Src: "hello", Dst: "你好", IsEnd: 1},
		},
	}
	data, _ := json.Marshal(resp)
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
		t.Error("IsFinal should be true when is_end=1")
	}
}

// TestParseXunfeiResponse_NonZeroCode 验证 code != 0 时返回 false（API 错误）。
func TestParseXunfeiResponse_NonZeroCode(t *testing.T) {
	data := []byte(`{"code":10001,"message":"invalid appid"}`)
	_, ok := parseXunfeiResponse(data)
	if ok {
		t.Error("should return false for non-zero code")
	}
}

// TestParseXunfeiResponse_EmptyContent 验证 src 和 dst 均为空时返回 false（心跳帧）。
func TestParseXunfeiResponse_EmptyContent(t *testing.T) {
	data := []byte(`{"code":0,"data":{"status":1,"result":{"src":"","dst":"","is_end":0}}}`)
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

// TestParseXunfeiResponse_NotFinal 验证 is_end=0 时 IsFinal 为 false。
func TestParseXunfeiResponse_NotFinal(t *testing.T) {
	data := []byte(`{"code":0,"data":{"result":{"src":"hi","dst":"嗨","is_end":0}}}`)
	result, ok := parseXunfeiResponse(data)
	if !ok {
		t.Fatal("should parse successfully")
	}
	if result.IsFinal {
		t.Error("IsFinal should be false when is_end=0")
	}
}

// TestBuildAuthURL_Format 验证生成的鉴权 URL 包含所有必需参数且格式正确。
func TestBuildAuthURL_Format(t *testing.T) {
	p := NewXunfeiProvider(XunfeiConfig{
		AppID:     "test-app",
		APIKey:    "test-key",
		APISecret: "test-secret",
	})
	rawURL, err := p.buildAuthURL()
	if err != nil {
		t.Fatalf("buildAuthURL error: %v", err)
	}
	if !strings.HasPrefix(rawURL, xunfeiWSS) {
		t.Errorf("URL should start with %q, got %q", xunfeiWSS, rawURL)
	}
	for _, param := range []string{"authorization=", "date=", "host="} {
		if !strings.Contains(rawURL, param) {
			t.Errorf("URL missing required parameter %q", param)
		}
	}
	// 验证 authorization 是有效 base64
	parts := strings.SplitN(rawURL, "?", 2)
	if len(parts) < 2 {
		t.Fatal("URL missing query string")
	}
	for _, kv := range strings.Split(parts[1], "&") {
		if strings.HasPrefix(kv, "authorization=") {
			b64 := strings.TrimPrefix(kv, "authorization=")
			if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
				t.Errorf("authorization is not valid base64: %v", err)
			}
		}
	}
}
