package asr

import "testing"

func TestParseXunfeiASRData(t *testing.T) {
	data := []byte(`{"cn":{"st":{"type":"0","rt":[{"ws":[{"cw":[{"w":"你"}]},{"cw":[{"w":"好"}]}]}]}},"seg_id":1}`)
	result, ok := parseXunfeiASRData(data)
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

func TestParseXunfeiASRDataIATShape(t *testing.T) {
	data := []byte(`{"bg":0,"ed":0,"ls":true,"pgs":"apd","sn":1,"ws":[{"cw":[{"w":"你"}]},{"cw":[{"w":"好"}]}]}`)
	result, ok := parseXunfeiASRData(data)
	if !ok {
		t.Fatal("should parse IAT-shaped ASR result")
	}
	if result.SrcText != "你好" || result.DstText != "你好" {
		t.Fatalf("result = %#v, want source and target fallback text", result)
	}
	if !result.IsFinal {
		t.Fatal("ls=true should be final")
	}
}

func TestParseXunfeiASRDataFallbackExtractsWords(t *testing.T) {
	data := []byte(`{"bg":0,"ed":0,"ls":true,"ws":[{"cw":{"w":"测"}},{"nested":[{"w":"试"}]}]}`)
	result, ok := parseXunfeiASRData(data)
	if !ok {
		t.Fatal("should parse ASR result with fallback word extraction")
	}
	if result.SrcText != "测试" || result.DstText != "测试" {
		t.Fatalf("result = %#v", result)
	}
}

func TestParseXunfeiASRDataEmpty(t *testing.T) {
	if _, ok := parseXunfeiASRData([]byte(`{"cn":{"st":{"type":"1","rt":[]}}}`)); ok {
		t.Fatal("empty ASR text should be ignored")
	}
}
