package translation

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestBuildXunfeiMTRequest(t *testing.T) {
	req := buildXunfeiMTRequest("app", "你好", "cn", "en")
	if req.Header.AppID != "app" || req.Header.Status != 3 {
		t.Fatalf("unexpected header: %#v", req.Header)
	}
	if req.Parameter.ITS.From != "cn" || req.Parameter.ITS.To != "en" {
		t.Fatalf("unexpected language params: %#v", req.Parameter.ITS)
	}
	decoded, err := base64.StdEncoding.DecodeString(req.Payload.InputData.Text)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "你好" {
		t.Fatalf("decoded text = %q", decoded)
	}
}

func TestXunfeiTextTranslatorBuildAuthURL(t *testing.T) {
	translator := NewXunfeiTextTranslator(XunfeiMachineTranslationConfig{
		AppID:     "app",
		APIKey:    "key",
		APISecret: "secret",
	})
	rawURL, err := translator.buildAuthURL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rawURL, xunfeiMTNewEndpoint) {
		t.Fatalf("url = %q", rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for _, param := range []string{"authorization", "date", "host"} {
		if query.Get(param) == "" {
			t.Fatalf("missing %s in %s", param, rawURL)
		}
	}
	authBytes, err := base64.StdEncoding.DecodeString(query.Get("authorization"))
	if err != nil {
		t.Fatal(err)
	}
	auth := string(authBytes)
	if !strings.Contains(auth, `api_key="key",algorithm="hmac-sha256",headers="host date request-line",signature="`) {
		t.Fatalf("authorization = %q", auth)
	}
}

func TestBuildXunfeiMTLegacyRequest(t *testing.T) {
	req := buildXunfeiMTLegacyRequest("app", "你好", "cn", "en")
	if req.Common.AppID != "app" {
		t.Fatalf("unexpected app id: %#v", req.Common)
	}
	if req.Business.From != "cn" || req.Business.To != "en" {
		t.Fatalf("unexpected language params: %#v", req.Business)
	}
	decoded, err := base64.StdEncoding.DecodeString(req.Data.Text)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "你好" {
		t.Fatalf("decoded text = %q", decoded)
	}
}

func TestParseXunfeiMTResponse(t *testing.T) {
	inner, _ := json.Marshal(xunfeiMTText{
		TransResult: struct {
			Dst string `json:"dst"`
			Src string `json:"src"`
		}{Dst: "hello", Src: "你好"},
		From: "cn",
		To:   "en",
	})
	resp := xunfeiMTResponse{}
	resp.Header.Code = 0
	resp.Header.Message = "success"
	resp.Payload.Result.Text = base64.StdEncoding.EncodeToString(inner)
	data, _ := json.Marshal(resp)

	got, err := parseXunfeiMTResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("translated text = %q", got)
	}
}

func TestParseXunfeiMTLegacyResponse(t *testing.T) {
	resp := xunfeiMTLegacyResponse{}
	resp.Code = 0
	resp.Message = "success"
	resp.Data.Result.TransResult.Dst = "hello"
	resp.Data.Result.TransResult.Src = "你好"
	resp.Data.Result.From = "cn"
	resp.Data.Result.To = "en"
	data, _ := json.Marshal(resp)

	got, err := parseXunfeiMTLegacyResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("translated text = %q", got)
	}
}

func TestIsXunfeiMTNotFound(t *testing.T) {
	if !isXunfeiMTNotFound(errors.New(`xunfei_mt: status 403: {"message":"not found"}`)) {
		t.Fatal("expected status 403 not found to be detected")
	}
	if isXunfeiMTNotFound(errors.New(`xunfei_mt: status 401: {"message":"apikey not found"}`)) {
		t.Fatal("apikey not found should not trigger legacy fallback")
	}
}

func TestXunfeiMachineTranslationConfigReady(t *testing.T) {
	if XunfeiMachineTranslationConfigReady(XunfeiMachineTranslationConfig{AppID: "app", APIKey: "key"}) {
		t.Fatal("config without APISecret should not be ready")
	}
	if !XunfeiMachineTranslationConfigReady(XunfeiMachineTranslationConfig{AppID: "app", APIKey: "key", APISecret: "secret"}) {
		t.Fatal("complete machine translation config should be ready")
	}
}
