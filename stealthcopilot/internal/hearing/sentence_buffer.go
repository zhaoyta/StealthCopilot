package hearing

import "strings"

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

func mergeHearingInterim(left, right string) string {
	left = normalizeHearingText(left)
	right = normalizeHearingDraftUpdate(right)
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	lowerLeft := strings.ToLower(left)
	lowerRight := strings.ToLower(right)
	if strings.Contains(lowerRight, lowerLeft) {
		return right
	}
	if strings.Contains(lowerLeft, lowerRight) {
		return left
	}
	if merged, ok := mergeHearingOverlap(left, right); ok {
		return compactHearingDraftText(merged)
	}
	return compactHearingDraftText(joinHearingText(left, right))
}

func mergeHearingOverlap(left, right string) (string, bool) {
	leftWords := strings.Fields(left)
	rightWords := strings.Fields(right)
	max := len(leftWords)
	if len(rightWords) < max {
		max = len(rightWords)
	}
	for n := max; n >= 2; n-- {
		if equalHearingWords(leftWords[len(leftWords)-n:], rightWords[:n]) {
			return normalizeHearingText(joinHearingText(left, strings.Join(rightWords[n:], " "))), true
		}
	}
	return "", false
}

func equalHearingWords(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !equalHearingWord(left[i], right[i]) {
			return false
		}
	}
	return true
}

func equalHearingWord(left, right string) bool {
	left = normalizeHearingWord(left)
	right = normalizeHearingWord(right)
	if left == "" || right == "" {
		return left == right
	}
	if left == right {
		return true
	}
	if len(left) >= 4 && len(right) >= 4 && (strings.HasPrefix(left, right) || strings.HasPrefix(right, left)) {
		return true
	}
	return false
}

func normalizeHearingWord(word string) string {
	word = strings.ToLower(strings.Trim(word, ".,!?;:，。！？；：，、\"'“”‘’()[]{}"))
	if strings.HasSuffix(word, "s") && len(word) > 4 {
		return strings.TrimSuffix(word, "s")
	}
	return word
}

func normalizeHearingDraftUpdate(text string) string {
	text = normalizeHearingText(text)
	text = strings.TrimLeftFunc(text, func(r rune) bool {
		return strings.ContainsRune(".!?。！？；;", r)
	})
	return normalizeHearingText(text)
}

func hearingIsCumulativeUpdate(previous, next string) bool {
	previous = strings.ToLower(normalizeHearingText(previous))
	next = strings.ToLower(normalizeHearingDraftUpdate(next))
	if previous == "" || next == "" {
		return false
	}
	return strings.Contains(next, previous) || strings.Contains(previous, next)
}

func trimSubmittedHearingPrefix(text, submitted string) string {
	text = normalizeHearingDraftUpdate(text)
	submitted = normalizeHearingText(submitted)
	if text == "" || submitted == "" {
		return text
	}
	lowerText := strings.ToLower(text)
	lowerSubmitted := strings.ToLower(submitted)
	if lowerText == lowerSubmitted {
		return ""
	}
	if strings.HasPrefix(lowerText, lowerSubmitted) {
		return normalizeHearingDraftUpdate(text[len(submitted):])
	}
	return text
}

func hearingDraftDuplicate(current, previous string) bool {
	current = strings.ToLower(normalizeHearingText(current))
	previous = strings.ToLower(normalizeHearingText(previous))
	return current != "" && current == previous
}

func compactHearingDraftText(text string) string {
	words := strings.Fields(normalizeHearingText(text))
	if len(words) < 4 {
		return strings.Join(words, " ")
	}
	for changed := true; changed; {
		changed = false
		for i := 0; i+1 < len(words); i++ {
			if equalHearingWord(words[i], words[i+1]) {
				words = append(words[:i+1], words[i+2:]...)
				changed = true
				i--
			}
		}
		for n := maxHearingRepeatWindow(words); n >= 2; n-- {
			for i := 0; i+2*n <= len(words); i++ {
				if equalHearingWords(words[i:i+n], words[i+n:i+2*n]) {
					kept := preferHearingWords(words[i:i+n], words[i+n:i+2*n])
					words = append(append(append([]string{}, words[:i]...), kept...), words[i+2*n:]...)
					changed = true
					i--
				}
			}
		}
		for n := maxHearingRepeatWindow(words); n >= 5; n-- {
			for i := 0; i+n <= len(words); i++ {
				for j := i + n + 1; j+n <= len(words) && j <= i+n+36; j++ {
					if equalHearingWords(words[i:i+n], words[j:j+n]) {
						words = append(words[:i], words[j:]...)
						changed = true
						j--
					}
				}
			}
		}
	}
	return strings.Join(words, " ")
}

func preferHearingWords(left, right []string) []string {
	leftText := strings.Join(left, " ")
	rightText := strings.Join(right, " ")
	if len(rightText) > len(leftText) {
		return append([]string{}, right...)
	}
	return append([]string{}, left...)
}

func maxHearingRepeatWindow(words []string) int {
	if len(words)/2 < 12 {
		return len(words) / 2
	}
	return 12
}
