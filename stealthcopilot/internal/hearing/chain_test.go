// Package hearing 单测：验证听力链事件常量、生命周期安全性和字幕推送行为。
// 依赖外部网络的集成场景（讯飞 WebSocket 连接）不在此覆盖。
package hearing

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/translation"
)

type fakeMonitorSink struct {
	mu     sync.Mutex
	spoken []string
}

func (f *fakeMonitorSink) Speak(_ context.Context, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spoken = append(f.spoken, text)
	return nil
}

func (f *fakeMonitorSink) Close() error { return nil }

func (f *fakeMonitorSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.spoken)
}

// TestEventConstants 验证向前端推送的事件名常量未被意外修改。
func TestEventConstants(t *testing.T) {
	if EventSubtitle != "hearing:subtitle" {
		t.Errorf("EventSubtitle = %q, want %q", EventSubtitle, "hearing:subtitle")
	}
	if EventError != "hearing:error" {
		t.Errorf("EventError = %q, want %q", EventError, "hearing:error")
	}
}

// TestSubtitleEvent_JSONMarshaling 验证 SubtitleEvent 与前端约定的 JSON 字段名一致。
func TestSubtitleEvent_JSONMarshaling(t *testing.T) {
	ev := SubtitleEvent{Text: "hello world", IsFinal: true}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if m["text"] != "hello world" {
		t.Errorf("json field 'text' = %v, want 'hello world'", m["text"])
	}
	if m["isFinal"] != true {
		t.Errorf("json field 'isFinal' = %v, want true", m["isFinal"])
	}
}

// TestChain_StopBeforeStart 验证在 Start 前调用 Stop 不会 panic 或 deadlock。
func TestChain_StopBeforeStart(t *testing.T) {
	var c Chain
	// 调用两次确保幂等
	c.Stop()
	c.Stop()
}

// TestProcessLoop_NonFinalSubtitle 验证 processLoop 对 IsFinal=false 的结果立即推送
// EventSubtitle 到 emitFn，不触发意图分类或 RAG（nil classifier/retriever/generator 不会 panic）。
func TestProcessLoop_NonFinalSubtitle(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan translation.DualResult, 1)
	received := make(chan SubtitleEvent, 1)

	// emitFn 替代 Wails runtime.EventsEmit，测试时捕获事件而不依赖 Wails 上下文
	emitFn := llm.EventEmitter(func(name string, data ...any) {
		if name == EventSubtitle && len(data) > 0 {
			if ev, ok := data[0].(SubtitleEvent); ok {
				received <- ev
			}
		}
	})

	// classifier/retriever/generator 传 nil —— IsFinal=false 不会触达这些分支
	go c.processLoop(ctx, resultCh, nil, nil, nil, "test-session", "", emitFn, nil)

	resultCh <- translation.DualResult{DstText: "面试官的问题", IsFinal: false}

	select {
	case ev := <-received:
		if ev.Text != "面试官的问题" {
			t.Errorf("subtitle text = %q, want %q", ev.Text, "面试官的问题")
		}
		if ev.IsFinal {
			t.Error("IsFinal should be false for intermediate result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: emitFn never received EventSubtitle")
	}
}

// TestProcessLoop_EmptyDstText 验证 DstText 为空的结果仍推送字幕事件（由前端决定是否显示）。
func TestProcessLoop_EmptyDstText(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan translation.DualResult, 1)
	received := make(chan SubtitleEvent, 1)
	emitFn := llm.EventEmitter(func(name string, data ...any) {
		if name == EventSubtitle && len(data) > 0 {
			if ev, ok := data[0].(SubtitleEvent); ok {
				received <- ev
			}
		}
	})

	go c.processLoop(ctx, resultCh, nil, nil, nil, "test-session", "", emitFn, nil)

	resultCh <- translation.DualResult{DstText: "", IsFinal: false}

	select {
	case ev := <-received:
		if ev.Text != "" {
			t.Errorf("expected empty text, got %q", ev.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: emitFn never received EventSubtitle")
	}
}

// TestProcessLoop_ContextCancel 验证 ctx 取消时 processLoop 正常退出，不 deadlock。
func TestProcessLoop_ContextCancel(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())

	resultCh := make(chan translation.DualResult) // 无缓冲，不发送任何数据
	done := make(chan struct{})

	go func() {
		c.processLoop(ctx, resultCh, nil, nil, nil, "test-session", "", nil, nil)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// processLoop 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("processLoop did not exit after context cancel")
	}
}

// TestProcessLoop_FinalTranslationSpeaksMonitor 验证耳机监听只播报最终译文。
func TestProcessLoop_FinalTranslationSpeaksMonitor(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan translation.DualResult, 2)
	monitor := &fakeMonitorSink{}
	emitFn := llm.EventEmitter(func(string, ...any) {})

	go c.processLoop(ctx, resultCh, nil, nil, nil, "test-session", "", emitFn, monitor)

	resultCh <- translation.DualResult{DstText: "处理中", IsFinal: false}
	resultCh <- translation.DualResult{DstText: "请介绍一下项目经验", IsFinal: true}

	deadline := time.After(2 * time.Second)
	for {
		if monitor.count() == 1 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("monitor speak count = %d, want 1", monitor.count())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
