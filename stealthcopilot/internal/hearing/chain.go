// Package hearing 实现听力链的完整管道协调：
// 音频捕获 → 讯飞翻译 → 字幕推送 → 意图识别 → RAG 检索 → DeepSeek 流式回答生成。
// Chain 由 app_bindings.go 通过 Wails binding 启动和停止。
package hearing

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/intent"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/rag"
	"github.com/zhaoyta/stealthcopilot/internal/translation"
)

// 听力链向前端推送的 Wails 事件名常量
const (
	// EventSubtitle 携带 DstText（目标语言字幕）字段，前端字幕区监听此事件。
	EventSubtitle = "hearing:subtitle"
	// EventError 在讯飞重连失败或关键错误时触发，前端显示"连接中断"提示。
	EventError = "hearing:error"
)

// SubtitleEvent 是 EventSubtitle 携带的数据结构（JSON 序列化后发送给前端）。
type SubtitleEvent struct {
	Text    string `json:"text"`    // 目标语言字幕文本
	IsFinal bool   `json:"isFinal"` // true=当前句子已完整
}

// ChainConfig 听力链运行时所需的 API 配置和服务依赖。
type ChainConfig struct {
	// Xunfei 讯飞翻译 API 配置
	Xunfei translation.XunfeiConfig
	// DeepSeekKey DeepSeek API Key
	DeepSeekKey string
	// DeepSeekModel DeepSeek 模型名称
	DeepSeekModel string
	// RAGPrompt 用户自定义 RAG 回答 Prompt 模板
	RAGPrompt string
	// VirtualMicDevice 虚拟声卡设备名称（BlackHole/VB-Cable）
	VirtualMicDevice string
	// Retriever RAG 检索器（依赖 resume.Manager）
	Retriever *rag.Retriever
	// EventSink mirrors hearing/answer events to non-Wails consumers such as the native teleprompter.
	EventSink llm.EventEmitter
}

// Chain 是听力链的主协调器，持有各组件实例和运行状态。
type Chain struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start 以给定配置启动听力链。已在运行时幂等（先 Stop 再 Start）。
// wailsCtx 是 Wails 应用 context，用于 EventsEmit 推送事件。
func (c *Chain) Start(wailsCtx context.Context, cfg ChainConfig) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.wg.Wait()
	}

	ctx, cancel := context.WithCancel(wailsCtx)
	c.cancel = cancel

	// 音频捕获：用户选择设备时使用系统采集；未配置设备时保持静音降级。
	var captureProvider audio.CaptureProvider = &audio.NullCaptureProvider{}
	if cfg.VirtualMicDevice != "" {
		captureProvider = audio.NewSystemCaptureProvider()
	}
	audioStream, err := captureProvider.Start(ctx, cfg.VirtualMicDevice)
	if err != nil {
		cancel()
		c.cancel = nil
		return "音频捕获启动失败：" + err.Error()
	}

	// 讯飞翻译
	xunfei := translation.NewXunfeiProvider(cfg.Xunfei)
	resultCh, err := xunfei.Translate(ctx, audioStream)
	if err != nil {
		cancel()
		c.cancel = nil
		return "讯飞翻译启动失败：" + err.Error()
	}

	// 意图分类器
	classifier := intent.NewClassifier(cfg.DeepSeekKey, cfg.DeepSeekModel)

	// combinedEmit 统一负责 Wails 事件推送和 EventSink 转发，processLoop 只调用此函数。
	// 两路合一避免 processLoop 内部重复"emit + if sink" 模式，也使 processLoop 可测试。
	combinedEmit := llm.EventEmitter(func(eventName string, data ...any) {
		runtime.EventsEmit(wailsCtx, eventName, data...)
		if cfg.EventSink != nil {
			cfg.EventSink(eventName, data...)
		}
	})
	generator := llm.NewAnswerGenerator(cfg.DeepSeekKey, cfg.DeepSeekModel, combinedEmit)

	// session ID：每次 StartHearingChain 创建新 session，避免跨会话混用历史
	sessionID := uuid.New().String()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.processLoop(ctx, resultCh, classifier, cfg.Retriever, generator, sessionID, cfg.RAGPrompt, combinedEmit)
	}()

	return ""
}

// Stop 停止听力链，等待所有 goroutine 退出后返回。
func (c *Chain) Stop() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.mu.Unlock()
	c.wg.Wait()
}

// processLoop 是听力链的核心处理循环，协调字幕推送、意图识别、RAG 检索和回答生成。
// D2 设计决策：src_text 和 dst_text 并行分发，字幕不等待意图识别结果。
// emitFn 由 Start() 创建，内部同时推送 Wails 事件和 EventSink，processLoop 本身无 Wails 依赖。
func (c *Chain) processLoop(
	ctx context.Context,
	results <-chan translation.DualResult,
	classifier *intent.Classifier,
	retriever *rag.Retriever,
	generator *llm.AnswerGenerator,
	sessionID string,
	ragPromptTpl string,
	emitFn llm.EventEmitter,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-results:
			if !ok {
				// channel 关闭 = 讯飞重连耗尽，通知前端
				emitFn(EventError, "讯飞连接中断，请检查网络或重新启动")
				return
			}
			// 1. 立即推送字幕（dst_text）到提词窗，不等待意图分类
			subtitle := SubtitleEvent{
				Text:    result.DstText,
				IsFinal: result.IsFinal,
			}
			emitFn(EventSubtitle, subtitle)

			// 2. 仅对 is_end=true 的完整句子触发意图识别 + RAG（D3）
			if !result.IsFinal || result.SrcText == "" {
				continue
			}
			srcText := result.SrcText
			go func() {
				intentType := classifier.Classify(ctx, srcText)
				if intentType == intent.IntentStatement {
					return // 陈述不触发 RAG（D3：statement → 忽略）
				}
				// RAG 检索 top-3 简历片段（D4）
				ragResult := retriever.Retrieve(srcText)
				if !ragResult.HasActiveResume {
					// 无激活简历时发送降级提示事件
					emitFn(EventSubtitle, SubtitleEvent{Text: "（未激活简历，回答仅供参考）", IsFinal: true})
				}
				// 启动流式回答生成（D5+D6）
				generator.Generate(ctx, llm.GenerateConfig{
					SessionID:    sessionID,
					Question:     srcText,
					ResumeChunks: ragResult.Chunks,
					PromptTpl:    ragPromptTpl,
					WithHistory:  intentType == intent.IntentFollowup,
				})
			}()
		}
	}
}
