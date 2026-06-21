package asr

import (
	"encoding/json"
	"strings"
)

type xunfeiASRData struct {
	CN struct {
		ST struct {
			Type string `json:"type"`
			RT   []struct {
				WS []struct {
					CW []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"rt"`
		} `json:"st"`
	} `json:"cn"`
	LS bool `json:"ls"`
	WS []struct {
		CW []struct {
			W string `json:"w"`
		} `json:"cw"`
	} `json:"ws"`
}

func parseXunfeiASRData(data []byte) (Result, bool) {
	var asr xunfeiASRData
	if err := json.Unmarshal(data, &asr); err != nil {
		text := extractXunfeiASRWords(data)
		if text == "" {
			return Result{}, false
		}
		return Result{SrcText: text, DstText: text, IsFinal: true}, true
	}
	var b strings.Builder
	appendWords := func(words []struct {
		CW []struct {
			W string `json:"w"`
		} `json:"cw"`
	}) {
		for _, ws := range words {
			if len(ws.CW) > 0 {
				b.WriteString(ws.CW[0].W)
			}
		}
	}
	appendWords(asr.WS)
	for _, rt := range asr.CN.ST.RT {
		appendWords(rt.WS)
	}
	text := b.String()
	if text == "" {
		text = extractXunfeiASRWords(data)
	}
	if text == "" {
		return Result{}, false
	}
	return Result{SrcText: text, DstText: text, IsFinal: asr.CN.ST.Type == "0" || asr.LS}, true
}

func extractXunfeiASRWords(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			if word, ok := typed["w"].(string); ok {
				b.WriteString(word)
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return b.String()
}
