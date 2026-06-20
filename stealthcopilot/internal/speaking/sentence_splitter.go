package speaking

import (
	"strings"
	"unicode"
)

const (
	defaultTTSSentenceMaxRunes = 180
)

func splitForTTS(text string) []string {
	return splitForTTSWithLimit(text, defaultTTSSentenceMaxRunes)
}

func splitForTTSWithLimit(text string, maxRunes int) []string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return nil
	}
	if maxRunes <= 0 {
		maxRunes = defaultTTSSentenceMaxRunes
	}
	var sentences []string
	var b strings.Builder
	currentRunes := 0
	for _, r := range text {
		b.WriteRune(r)
		currentRunes++
		if isSentenceTerminator(r) {
			sentences = appendSentence(sentences, b.String())
			b.Reset()
			currentRunes = 0
			continue
		}
		if currentRunes >= maxRunes {
			left, right := splitLongSentence(b.String())
			sentences = appendSentence(sentences, left)
			b.Reset()
			b.WriteString(right)
			currentRunes = len([]rune(right))
		}
	}
	sentences = appendSentence(sentences, b.String())
	return sentences
}

func appendSentence(sentences []string, text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return sentences
	}
	return append(sentences, text)
}

func isSentenceTerminator(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '。', '！', '？', '；':
		return true
	default:
		return false
	}
}

func splitLongSentence(text string) (string, string) {
	runes := []rune(text)
	if len(runes) <= 1 {
		return text, ""
	}
	splitAt := len(runes)
	for i := len(runes) - 1; i > len(runes)/2; i-- {
		if isSoftBreak(runes[i]) {
			splitAt = i + 1
			break
		}
	}
	left := strings.TrimSpace(string(runes[:splitAt]))
	right := strings.TrimSpace(string(runes[splitAt:]))
	return left, right
}

func isSoftBreak(r rune) bool {
	switch r {
	case ',', '，', '、', ':', '：':
		return true
	default:
		return unicode.IsSpace(r)
	}
}
