package hearing

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const hearingSentenceMaxRunes = 220

type hearingSentenceBuffer struct {
	text       string
	lastOutput string
}

func (b *hearingSentenceBuffer) Add(text string, force bool) []string {
	text = normalizeHearingText(text)
	if text == "" {
		return nil
	}
	b.text = normalizeHearingText(joinHearingText(b.text, text))
	var out []string
	for {
		idx := hearingSentenceBoundary(b.text)
		if idx <= 0 {
			break
		}
		out = appendHearingSentence(out, strings.TrimSpace(b.text[:idx]), &b.lastOutput)
		b.text = strings.TrimSpace(b.text[idx:])
	}
	if force {
		if utf8.RuneCountInString(b.text) > hearingSentenceMaxRunes {
			for _, sentence := range splitLongHearingText(b.text, hearingSentenceMaxRunes) {
				out = appendHearingSentence(out, sentence, &b.lastOutput)
			}
			b.text = ""
		} else if hearingLooksCompleteEnough(b.text) {
			out = appendHearingSentence(out, b.text, &b.lastOutput)
			b.text = ""
		}
	}
	return out
}

// HasPending 报告 sentence buffer 里是否还有未发出的文本（非空且非纯空白）。
func (b *hearingSentenceBuffer) HasPending() bool {
	return strings.TrimSpace(b.text) != ""
}

func (b *hearingSentenceBuffer) Flush() []string {
	if strings.TrimSpace(b.text) == "" {
		return nil
	}
	text := b.text
	b.text = ""
	var out []string
	out = appendHearingSentence(out, text, &b.lastOutput)
	return out
}

func appendHearingSentence(out []string, sentence string, lastOutput *string) []string {
	sentence = normalizeHearingText(sentence)
	if sentence == "" || hearingSentenceDuplicate(sentence, *lastOutput) {
		return out
	}
	// 跳过纯标点片段（讯飞把前一句的结束标点放在下一段 interim 开头，flush 后会产生单独的 "." "?" 等）
	if hearingIsPunctuationOnly(sentence) {
		return out
	}
	*lastOutput = sentence
	return append(out, sentence)
}

// hearingStartsWithBoundary 判断文本是否以句子边界标点开头。
// 讯飞把前一句的结束标点放在下一段 interim 的开头，这是"前一句已结束"的可靠信号。
func hearingStartsWithBoundary(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		return strings.ContainsRune(".!?。！？；;", r)
	}
	return false
}

// hearingIsPunctuationOnly 判断字符串是否只由标点和空格组成（无实质词语内容）。
func hearingIsPunctuationOnly(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune(".!?。！？；;,，、：: ", r) {
			return false
		}
	}
	return true
}

func hearingSentenceDuplicate(current, previous string) bool {
	current = strings.ToLower(normalizeHearingText(current))
	previous = strings.ToLower(normalizeHearingText(previous))
	if current == "" || previous == "" {
		return false
	}
	return current == previous || strings.Contains(previous, current) || strings.Contains(current, previous)
}

func hearingSentenceBoundary(text string) int {
	last := 0
	for i, r := range text {
		if strings.ContainsRune(".!?。！？；;", r) {
			last = i + len(string(r))
			break
		}
	}
	return last
}

func hearingLooksCompleteEnough(text string) bool {
	if text == "" {
		return false
	}
	count := utf8.RuneCountInString(text)
	if count >= 36 {
		return true
	}
	fields := strings.Fields(text)
	return len(fields) >= 8
}

func splitLongHearingText(text string, maxRunes int) []string {
	var out []string
	for utf8.RuneCountInString(text) > maxRunes {
		cut := hearingSoftCut(text, maxRunes)
		out = append(out, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

func hearingSoftCut(text string, maxRunes int) int {
	count := 0
	lastSpace := -1
	lastComma := -1
	for i, r := range text {
		if unicode.IsSpace(r) {
			lastSpace = i
		}
		if strings.ContainsRune(",，、:", r) {
			lastComma = i + len(string(r))
		}
		count++
		if count >= maxRunes {
			if lastComma > 0 {
				return lastComma
			}
			if lastSpace > 0 {
				return lastSpace
			}
			return i + len(string(r))
		}
	}
	return len(text)
}

func normalizeHearingText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func joinHearingText(left, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	if strings.HasSuffix(left, " ") || strings.HasPrefix(right, " ") {
		return left + right
	}
	return left + " " + right
}
