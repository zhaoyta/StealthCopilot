package hearing

import "testing"

func TestHearingSentenceBufferWaitsForCompleteSentence(t *testing.T) {
	var b hearingSentenceBuffer
	if got := b.Add("This is only", false); len(got) != 0 {
		t.Fatalf("unexpected sentences: %#v", got)
	}
	got := b.Add("part of a sentence.", false)
	if len(got) != 1 || got[0] != "This is only part of a sentence." {
		t.Fatalf("sentences = %#v", got)
	}
}

func TestHearingSentenceBufferFinalCanFlushCompleteEnoughText(t *testing.T) {
	var b hearingSentenceBuffer
	got := b.Add("Can you describe your most challenging distributed systems project", true)
	if len(got) != 1 {
		t.Fatalf("sentences = %#v", got)
	}
}

func TestHearingSentenceBufferSplitsLongText(t *testing.T) {
	var b hearingSentenceBuffer
	long := "This is a very long interview question with many clauses and it keeps going because the interviewer is describing constraints, expectations, tradeoffs, and a few examples before asking the actual question at the end"
	got := b.Add(long, true)
	if len(got) == 0 {
		t.Fatal("expected long final text to flush")
	}
	for _, sentence := range got {
		if sentence == "" {
			t.Fatalf("empty sentence in %#v", got)
		}
	}
}

// TestHearingSentenceBufferShortFinalFlushesViaFlush 验证短句（不满足 hearingLooksCompleteEnough）
// 在 IsFinal=true 时通过额外调用 Flush() 强制发出，不卡在 buffer 里。
// 这是 processLoop 对 IsFinal 结果的实际处理路径。
func TestHearingSentenceBufferShortFinalFlushesViaFlush(t *testing.T) {
	var b hearingSentenceBuffer
	// "Tell me about yourself" = 4 词 / 22 字符，不满足 hearingLooksCompleteEnough
	got := b.Add("Tell me about yourself", true)
	if len(got) != 0 {
		// Add 本身不应该 flush（测试前置条件）
		t.Fatalf("Add should not flush short text, got %#v", got)
	}
	// processLoop 对 IsFinal=true 结果额外调用 Flush()
	flushed := b.Flush()
	if len(flushed) != 1 || flushed[0] != "Tell me about yourself" {
		t.Fatalf("Flush should emit short IsFinal sentence, got %#v", flushed)
	}
}

// TestHearingIsPunctuationOnly 验证纯标点过滤器正确识别孤立标点（讯飞把句尾标点放在下一段开头时产生的碎片）。
func TestHearingIsPunctuationOnly(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{".", true},
		{"?", true},
		{". ", true},
		{"! ?", true},
		{"Nice to meet you.", false},
		{"Where are you from ?", false},
		{"OK", false},
	}
	for _, tc := range cases {
		if got := hearingIsPunctuationOnly(tc.input); got != tc.want {
			t.Errorf("hearingIsPunctuationOnly(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestHearingSentenceBufferIdleFlushShortSentence 模拟 idle-timer 之后调用 Add+Flush 的完整路径：
// 短句不满足 hearingLooksCompleteEnough 时，Flush() 应强制发出。
func TestHearingSentenceBufferIdleFlushShortSentence(t *testing.T) {
	var b hearingSentenceBuffer
	// idle-timer 触发前，短句留在 pendingInterim，Add 无法发出
	got := b.Add("Nice to meet you", true)
	if len(got) != 0 {
		t.Fatalf("Add should not emit short sentence via hearingLooksCompleteEnough, got %#v", got)
	}
	// idle-timer 路径额外调用 Flush()
	flushed := b.Flush()
	if len(flushed) != 1 || flushed[0] != "Nice to meet you" {
		t.Fatalf("Flush should emit short sentence, got %#v", flushed)
	}
}

// TestHearingSentenceBufferPunctuationOnlyFiltered 验证 flush 之后下一句 interim 产生的孤立标点不进入翻译队列。
func TestHearingSentenceBufferPunctuationOnlyFiltered(t *testing.T) {
	var b hearingSentenceBuffer
	// buffer 已清空（flush 之后）
	got := b.Add(". Where are you from", false)
	// "." 应被 hearingIsPunctuationOnly 过滤掉，只剩 "Where are you from" 留在 buffer 里等下一个信号
	if len(got) != 0 {
		t.Fatalf("punctuation-only fragment should be filtered, got %#v", got)
	}
}

func TestHearingSentenceBufferSkipsDuplicateOutput(t *testing.T) {
	var b hearingSentenceBuffer
	first := b.Add("Where are you from?", true)
	if len(first) != 1 {
		t.Fatalf("first = %#v", first)
	}
	second := b.Add("Where are you from?", true)
	if len(second) != 0 {
		t.Fatalf("duplicate should be skipped, got %#v", second)
	}
}
