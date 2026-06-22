// Package hearing 单测：验证听力链事件常量、生命周期安全性和字幕推送行为。
// 依赖外部网络的集成场景（讯飞 WebSocket 连接）不在此覆盖。
package hearing

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/asr"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/session"
	"github.com/zhaoyta/stealthcopilot/internal/trans"
)

type fakeMonitorSink struct {
	mu     sync.Mutex
	spoken []string
	pcm    [][]byte
}

func (f *fakeMonitorSink) Speak(_ context.Context, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spoken = append(f.spoken, text)
	return nil
}

func (f *fakeMonitorSink) PlayPCM(_ context.Context, pcm []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pcm = append(f.pcm, append([]byte(nil), pcm...))
	return nil
}

func (f *fakeMonitorSink) Close() error { return nil }

func (f *fakeMonitorSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.spoken)
}

func (f *fakeMonitorSink) pcmCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pcm)
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

func TestChain_StartWithCaptureDeviceRequiresRealCapture(t *testing.T) {
	t.Setenv("PATH", "")
	var c Chain
	result := c.Start(context.Background(), ChainConfig{VirtualMicDevice: "1"})
	if result == "" {
		c.Stop()
		t.Fatal("expected startup error when capture device is configured but ffmpeg is unavailable")
	}
}

func TestChain_StartRequiresSimultConfigWhenNoInjectedTranslator(t *testing.T) {
	var c Chain
	result := c.Start(context.Background(), ChainConfig{})
	if result == "" {
		c.Stop()
		t.Fatal("expected startup error for missing Xunfei config")
	}
}

func TestChain_StartStopSessionLifecycle(t *testing.T) {
	var c Chain
	store := &fakeSessionStore{}
	result := c.Start(context.Background(), ChainConfig{
		ASRExtension: blockingStreamingExtension{},
		MonitorSink:  &fakeMonitorSink{},
		SessionStore: store,
		ResumeID:     "resume-1",
	})
	if result != "" {
		t.Fatalf("Start = %q", result)
	}
	c.Stop()

	if len(store.begun) != 1 {
		t.Fatalf("begin count = %d, want 1", len(store.begun))
	}
	if store.begun[0].sessionID == "" || store.begun[0].resumeID != "resume-1" {
		t.Fatalf("begin args mismatch: %+v", store.begun[0])
	}
	if len(store.ended) != 1 || store.ended[0] != store.begun[0].sessionID {
		t.Fatalf("ended = %+v, begun = %+v", store.ended, store.begun)
	}
}

type blockingStreamingExtension struct{}

func (blockingStreamingExtension) Translate(ctx context.Context, _ <-chan []byte) (<-chan asr.Result, error) {
	ch := make(chan asr.Result)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func (blockingStreamingExtension) Close() error { return nil }

func TestDisplayQuestionFallback(t *testing.T) {
	if got := displayQuestion("source", "译文"); got != "译文" {
		t.Fatalf("displayQuestion with dst = %q", got)
	}
	if got := displayQuestion("source", " "); got != "source" {
		t.Fatalf("displayQuestion fallback = %q", got)
	}
}

// TestProcessLoop_NonFinalSubtitle 验证 processLoop 对 IsFinal=false 的结果立即推送
// EventSubtitle 到 emitFn，不触发意图分类或 RAG（nil classifier/retriever/generator 不会 panic）。
func TestProcessLoop_NonFinalSubtitle(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan asr.Result, 1)
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
	submitCh := make(chan hearingSubmitRequest)
	go c.processLoop(ctx, resultCh, submitCh, trans.NoopExtension{}, nil, nil, nil, "test-session", "", "zh", emitFn, nil, nil, false)

	resultCh <- asr.Result{DstText: "面试官的问题", IsFinal: false}

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

	resultCh := make(chan asr.Result, 1)
	received := make(chan SubtitleEvent, 1)
	emitFn := llm.EventEmitter(func(name string, data ...any) {
		if name == EventSubtitle && len(data) > 0 {
			if ev, ok := data[0].(SubtitleEvent); ok {
				received <- ev
			}
		}
	})

	submitCh := make(chan hearingSubmitRequest)
	go c.processLoop(ctx, resultCh, submitCh, trans.NoopExtension{}, nil, nil, nil, "test-session", "", "zh", emitFn, nil, nil, false)

	resultCh <- asr.Result{DstText: "", IsFinal: false}

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

	resultCh := make(chan asr.Result) // 无缓冲，不发送任何数据
	done := make(chan struct{})

	go func() {
		submitCh := make(chan hearingSubmitRequest)
		c.processLoop(ctx, resultCh, submitCh, trans.NoopExtension{}, nil, nil, nil, "test-session", "", "zh", nil, nil, nil, false)
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

	resultCh := make(chan asr.Result, 2)
	monitor := &fakeMonitorSink{}
	emitFn := llm.EventEmitter(func(string, ...any) {})

	submitCh := make(chan hearingSubmitRequest)
	go c.processLoop(ctx, resultCh, submitCh, trans.NoopExtension{}, nil, nil, nil, "test-session", "", "zh", emitFn, monitor, nil, false)

	resultCh <- asr.Result{DstText: "处理中", IsFinal: false}
	resultCh <- asr.Result{DstText: "请介绍一下项目经验", IsFinal: true}

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

func TestProcessLoop_InterimDraftRequiresManualSubmit(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan asr.Result, 3)
	received := make(chan SubtitleEvent, 2)
	emitFn := llm.EventEmitter(func(name string, data ...any) {
		if name == EventSubtitle && len(data) > 0 {
			if ev, ok := data[0].(SubtitleEvent); ok {
				received <- ev
			}
		}
	})

	submitCh := make(chan hearingSubmitRequest)
	go c.processLoop(ctx, resultCh, submitCh, trans.SourceOnlyExtension{}, nil, nil, nil, "test-session", "", "zh", emitFn, nil, nil, false)

	resultCh <- asr.Result{SrcText: "Tell", IsFinal: false}
	resultCh <- asr.Result{SrcText: "Tell me about", IsFinal: false}
	resultCh <- asr.Result{SrcText: "? Tell me about yourself", IsFinal: false}

	select {
	case ev := <-received:
		t.Fatalf("unexpected subtitle before submit: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}

	reply := make(chan string, 1)
	submitCh <- hearingSubmitRequest{reply: reply}
	if submitted := <-reply; submitted != "Tell me about yourself" {
		t.Fatalf("submitted = %q, want complete draft", submitted)
	}

	select {
	case ev := <-received:
		if ev.Text != "Tell me about yourself" || !ev.IsFinal {
			t.Fatalf("subtitle = %+v, want final complete draft", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: expected subtitle after submit")
	}

	select {
	case ev := <-received:
		t.Fatalf("unexpected extra subtitle: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestProcessLoop_FinalASRStillRequiresManualSubmit(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan asr.Result, 1)
	received := make(chan SubtitleEvent, 2)
	emitFn := llm.EventEmitter(func(name string, data ...any) {
		if name == EventSubtitle && len(data) > 0 {
			if ev, ok := data[0].(SubtitleEvent); ok {
				received <- ev
			}
		}
	})

	submitCh := make(chan hearingSubmitRequest)
	go c.processLoop(ctx, resultCh, submitCh, trans.SourceOnlyExtension{}, nil, nil, nil, "test-session", "", "zh", emitFn, nil, nil, false)

	resultCh <- asr.Result{SrcText: "Tell me about yourself", IsFinal: true}

	select {
	case ev := <-received:
		t.Fatalf("unexpected subtitle before submit: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}

	reply := make(chan string, 1)
	submitCh <- hearingSubmitRequest{reply: reply}
	if submitted := <-reply; submitted != "Tell me about yourself" {
		t.Fatalf("submitted = %q", submitted)
	}

	select {
	case ev := <-received:
		if ev.Text != "Tell me about yourself" {
			t.Fatalf("subtitle = %q", ev.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: expected subtitle after submit")
	}
}

func TestProcessLoop_ProviderAudioPlaysMonitorPCM(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan asr.Result, 1)
	monitor := &fakeMonitorSink{}
	emitFn := llm.EventEmitter(func(string, ...any) {})

	submitCh := make(chan hearingSubmitRequest)
	go c.processLoop(ctx, resultCh, submitCh, trans.NoopExtension{}, nil, nil, nil, "test-session", "", "zh", emitFn, monitor, nil, true)

	resultCh <- asr.Result{AudioPCM: []byte{1, 2, 3, 4}}

	deadline := time.After(2 * time.Second)
	for {
		if monitor.pcmCount() == 1 && monitor.count() == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("monitor pcm count = %d, speak count = %d; want pcm=1 speak=0", monitor.pcmCount(), monitor.count())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestMonitorAudioWorkerPlaysInOrder(t *testing.T) {
	var c Chain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := make(chan []byte, 2)
	monitor := &fakeMonitorSink{}
	done := make(chan struct{})
	go func() {
		c.monitorAudioWorker(ctx, queue, monitor)
		close(done)
	}()

	queue <- []byte{1, 2}
	queue <- []byte{3, 4}

	deadline := time.After(2 * time.Second)
	for {
		if monitor.pcmCount() == 2 {
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("worker did not stop")
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("monitor pcm count = %d, want 2", monitor.pcmCount())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

type fakeSessionStore struct {
	session.Store
	mu    sync.Mutex
	begun []struct{ sessionID, resumeID string }
	ended []string
}

func (f *fakeSessionStore) Begin(sessionID, resumeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.begun = append(f.begun, struct{ sessionID, resumeID string }{sessionID: sessionID, resumeID: resumeID})
	return nil
}

func (f *fakeSessionStore) End(sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ended = append(f.ended, sessionID)
	return nil
}
