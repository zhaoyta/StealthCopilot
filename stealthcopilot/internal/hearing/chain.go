// Package hearing 实现听力链的完整管道协调：
// 音频捕获 → ASR/Trans/TTS 扩展步骤 → 字幕推送 → 意图识别 → RAG 检索 → DeepSeek 流式回答生成。
// Chain 由 app_bindings.go 通过 Wails binding 启动和停止。
package hearing

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/asr"
	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/intent"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/pipeline"
	"github.com/zhaoyta/stealthcopilot/internal/rag"
	"github.com/zhaoyta/stealthcopilot/internal/session"
	"github.com/zhaoyta/stealthcopilot/internal/trans"
)

// 听力链向前端推送的 Wails 事件名常量
const (
	// EventSubtitle 携带 DstText（目标语言字幕）字段，前端字幕区监听此事件。
	EventSubtitle = "hearing:subtitle"
	// EventStep 携带听力链 ASR/Trans/TTS 扩展步骤的实时产出。
	EventStep = "hearing:step"
	// EventError 在讯飞重连失败或关键错误时触发，前端显示"连接中断"提示。
	EventError = "hearing:error"
)

// SubtitleEvent 是 EventSubtitle 携带的数据结构（JSON 序列化后发送给前端）。
type SubtitleEvent struct {
	Text    string `json:"text"`    // 目标语言字幕文本
	IsFinal bool   `json:"isFinal"` // true=当前句子已完整
}

const (
	hearingTransQueueSize = 64
	hearingTTSQueueSize   = 64
	hearingSubmitTimeout  = 2 * time.Second
	hearingRAGTimeout     = 3 * time.Second
)

type hearingTransItem struct {
	Result asr.Result
}

type hearingSubmitRequest struct {
	reply chan string
}

type hearingTTSItem struct {
	Text    string
	IsFinal bool
}

// ChainConfig 听力链运行时所需的 API 配置和服务依赖。
type ChainConfig struct {
	// ASRConfig configures the hearing ASR extension.
	ASRConfig asr.XunfeiRTASRLLMConfig
	// ASRExtension overrides the default hearing ASR extension.
	ASRExtension asr.StreamingExtension
	// TransExtension allows replacing the text translation/post-processing extension.
	TransExtension trans.Extension
	// LLMConfig configures OpenAI-compatible intent classification and answer generation.
	LLMConfig llm.Config
	// DeepSeekKey DeepSeek API Key
	DeepSeekKey string
	// DeepSeekModel DeepSeek 模型名称
	DeepSeekModel string
	// RAGPrompt 用户自定义 RAG 回答 Prompt 模板
	RAGPrompt string
	// TargetLang is the language used for user-visible answer suggestions.
	TargetLang string
	// VirtualMicDevice 虚拟声卡设备名称（BlackHole/VB-Cable）
	VirtualMicDevice string
	// MonitorConfig controls private translated-audio playback for the interviewee.
	MonitorConfig audio.MonitorConfig
	// MonitorSink allows tests or alternate runtimes to override system speech.
	MonitorSink audio.MonitorSink
	// MonitorPrefersExtensionAudio uses audio returned by the speech extension.
	MonitorPrefersExtensionAudio bool
	// Retriever RAG 检索器（依赖 resume.Manager）
	Retriever *rag.Retriever
	// SessionStore persists interview session history.
	SessionStore session.Store
	// ResumeSessionID resumes an existing session when provided. The default frontend path leaves it empty.
	ResumeSessionID string
	// ResumeID records the active resume associated with a session.
	ResumeID string
	// EventSink mirrors hearing/answer events to non-Wails consumers such as the native teleprompter.
	EventSink llm.EventEmitter
}

// Chain 是听力链的主协调器，持有各组件实例和运行状态。
type Chain struct {
	mu           sync.Mutex
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	sessionStore session.Store
	sessionID    string
	submitCh     chan hearingSubmitRequest
}

// Start 以给定配置启动听力链。已在运行时幂等（先 Stop 再 Start）。
// wailsCtx 是 Wails 应用 context，用于 EventsEmit 推送事件。
func (c *Chain) Start(wailsCtx context.Context, cfg ChainConfig) string {
	started := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		oldCancel := c.cancel
		oldStore := c.sessionStore
		oldSessionID := c.sessionID
		oldCancel()
		c.wg.Wait()
		if oldStore != nil && oldSessionID != "" {
			_ = oldStore.End(oldSessionID)
		}
	}

	ctx, cancel := context.WithCancel(wailsCtx)
	c.cancel = cancel
	c.sessionStore = nil
	c.sessionID = ""
	c.submitCh = make(chan hearingSubmitRequest)

	// 音频捕获：用户选择设备时使用系统采集；未配置设备时保持静音降级。
	var captureProvider audio.CaptureProvider = &audio.NullCaptureProvider{}
	if cfg.VirtualMicDevice != "" {
		var captureErr string
		captureProvider, captureErr = audio.NewSystemCaptureProviderChecked()
		if captureErr != "" {
			cancel()
			c.cancel = nil
			c.submitCh = nil
			diag.Errorf("hearing capture provider failed device=%q err=%q", cfg.VirtualMicDevice, captureErr)
			return "音频捕获启动失败：" + captureErr
		}
	}
	audioStream, err := captureProvider.Start(ctx, cfg.VirtualMicDevice)
	if err != nil {
		cancel()
		c.cancel = nil
		c.submitCh = nil
		diag.Errorf("hearing capture start failed device=%q err=%v", cfg.VirtualMicDevice, err)
		return "音频捕获启动失败：" + err.Error()
	}
	diag.Infof("hearing capture started device=%q", cfg.VirtualMicDevice)

	asrExtension := cfg.ASRExtension
	if asrExtension == nil {
		if !asr.XunfeiRTASRLLMConfigReady(cfg.ASRConfig) {
			cancel()
			c.cancel = nil
			c.submitCh = nil
			diag.Errorf("hearing asr config incomplete source_lang=%s", cfg.ASRConfig.SourceLang)
			return "讯飞实时转写配置不完整：请配置 AppID、API Key、API Secret 和听力链语言"
		}
		asrExtension = asr.NewXunfeiRTASRLLMExtension(cfg.ASRConfig)
	}
	resultCh, err := asrExtension.Translate(ctx, audioStream)
	if err != nil {
		cancel()
		c.cancel = nil
		c.submitCh = nil
		diag.Errorf("hearing asr extension start failed err=%v", err)
		return "讯飞同声传译启动失败：" + err.Error()
	}
	diag.Infof("hearing asr extension started elapsed=%s", diag.Since(started))

	// 意图分类器
	llmCfg := cfg.LLMConfig
	if llmCfg.APIKey == "" {
		llmCfg.APIKey = cfg.DeepSeekKey
	}
	if llmCfg.Model == "" {
		llmCfg.Model = cfg.DeepSeekModel
	}
	classifier := intent.NewClassifierWithConfig(llmCfg)
	monitor := cfg.MonitorSink
	if monitor == nil {
		monitor = audio.NewSystemMonitorSink(cfg.MonitorConfig)
	}
	monitorAudioQueue := make(chan []byte, 64)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer monitor.Close()
		c.monitorAudioWorker(ctx, monitorAudioQueue, monitor)
	}()

	// combinedEmit 统一负责 Wails 事件推送和 EventSink 转发，processLoop 只调用此函数。
	// 两路合一避免 processLoop 内部重复"emit + if sink" 模式，也使 processLoop 可测试。
	combinedEmit := llm.EventEmitter(func(eventName string, data ...any) {
		runtime.EventsEmit(wailsCtx, eventName, data...)
		if cfg.EventSink != nil {
			cfg.EventSink(eventName, data...)
		}
	})
	generator := llm.NewAnswerGeneratorWithSessionStore(llmCfg, combinedEmit, cfg.SessionStore)

	// session ID：默认每次 StartHearingChain 创建新 session，避免跨会话混用历史
	sessionID := strings.TrimSpace(cfg.ResumeSessionID)
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	if cfg.SessionStore != nil {
		if err := cfg.SessionStore.Begin(sessionID, cfg.ResumeID); err != nil {
			cancel()
			c.cancel = nil
			c.submitCh = nil
			diag.Errorf("hearing session begin failed session=%s err=%v", sessionID, err)
			return "历史会话启动失败：" + err.Error()
		}
		c.sessionStore = cfg.SessionStore
		c.sessionID = sessionID
	}
	diag.Infof("hearing session started session=%s monitor_enabled=%t", sessionID, cfg.MonitorConfig.Enabled)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer asrExtension.Close()
		transExtension := cfg.TransExtension
		if transExtension == nil {
			transExtension = trans.NoopExtension{}
		}
		c.processLoop(ctx, resultCh, c.submitCh, transExtension, classifier, cfg.Retriever, generator, sessionID, cfg.RAGPrompt, cfg.TargetLang, combinedEmit, monitor, monitorAudioQueue, cfg.MonitorPrefersExtensionAudio)
	}()

	return ""
}

// SubmitDraft commits the latest hearing ASR draft to translation/RAG.
func (c *Chain) SubmitDraft(ctx context.Context) string {
	c.mu.Lock()
	submitCh := c.submitCh
	c.mu.Unlock()
	if submitCh == nil {
		return ""
	}
	req := hearingSubmitRequest{reply: make(chan string, 1)}
	timeout := time.After(hearingSubmitTimeout)
	select {
	case submitCh <- req:
	case <-ctx.Done():
		return ""
	case <-timeout:
		return ""
	}
	select {
	case text := <-req.reply:
		return text
	case <-ctx.Done():
		return ""
	case <-time.After(hearingSubmitTimeout):
		return ""
	}
}

func (c *Chain) monitorAudioWorker(ctx context.Context, queue <-chan []byte, monitor audio.MonitorSink) {
	for {
		select {
		case <-ctx.Done():
			return
		case pcm := <-queue:
			if len(pcm) == 0 || monitor == nil {
				continue
			}
			if err := monitor.PlayPCM(ctx, pcm); err != nil {
				diag.Warnf("hearing monitor pcm failed err=%v", err)
			} else {
				diag.Infof("hearing monitor pcm played bytes=%d queue_depth=%d", len(pcm), len(queue))
			}
		}
	}
}

// Stop 停止听力链，等待所有 goroutine 退出后返回。
func (c *Chain) Stop() {
	c.mu.Lock()
	var store session.Store
	var sessionID string
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
		store = c.sessionStore
		sessionID = c.sessionID
		c.sessionStore = nil
		c.sessionID = ""
		c.submitCh = nil
	}
	c.mu.Unlock()
	c.wg.Wait()
	if store != nil && sessionID != "" {
		if err := store.End(sessionID); err != nil {
			diag.Warnf("hearing session end failed session=%s err=%v", sessionID, err)
		}
	}
	diag.Infof("hearing chain stopped")
}

func (c *Chain) CurrentSession() (session.Store, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionStore, c.sessionID
}

// processLoop 是听力链的核心处理循环，协调字幕推送、意图识别、RAG 检索和回答生成。
// D2 设计决策：src_text 和 dst_text 并行分发，字幕不等待意图识别结果。
// emitFn 由 Start() 创建，内部同时推送 Wails 事件和 EventSink，processLoop 本身无 Wails 依赖。
func (c *Chain) processLoop(
	ctx context.Context,
	results <-chan asr.Result,
	submitCh <-chan hearingSubmitRequest,
	transExtension trans.Extension,
	classifier *intent.Classifier,
	retriever *rag.Retriever,
	generator *llm.AnswerGenerator,
	sessionID string,
	ragPromptTpl string,
	targetLang string,
	emitFn llm.EventEmitter,
	monitor audio.MonitorSink,
	monitorAudioQueue chan<- []byte,
	monitorPrefersExtensionAudio bool,
) {
	transQueue := make(chan hearingTransItem, hearingTransQueueSize)
	ttsQueue := make(chan hearingTTSItem, hearingTTSQueueSize)

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		c.transWorker(ctx, transQueue, ttsQueue, transExtension, classifier, retriever, generator, sessionID, ragPromptTpl, targetLang, emitFn, monitor, monitorAudioQueue, monitorPrefersExtensionAudio)
	}()
	go func() {
		defer c.wg.Done()
		c.ttsWorker(ctx, ttsQueue, monitor, emitFn)
	}()
	defer close(transQueue)

	var draftText string
	var lastSubmitted string
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-submitCh:
			submitted := compactHearingDraftText(draftText)
			if submitted != "" {
				draftText = ""
				emitFn(EventStep, pipeline.StepEvent{
					Chain:   "hearing",
					Step:    pipeline.StepASR,
					SrcText: "",
					IsFinal: false,
				})
				if hearingDraftDuplicate(submitted, lastSubmitted) {
					diag.Infof("hearing draft submit ignored duplicate chars=%d text=%q", len(submitted), trimHearingLog(submitted, 120))
				} else {
					lastSubmitted = submitted
					diag.Infof("hearing draft submitted chars=%d text=%q", len(submitted), trimHearingLog(submitted, 120))
					c.queueHearingSentence(ctx, transQueue, submitted)
				}
			} else {
				diag.Infof("hearing draft submit ignored empty")
			}
			req.reply <- submitted
		case result, ok := <-results:
			if !ok {
				if ctx.Err() != nil {
					return
				}
				// channel 关闭 = 讯飞重连耗尽，通知前端
				diag.Warnf("hearing result channel closed")
				emitFn(EventError, "讯飞连接中断，请检查网络或重新启动")
				return
			}
			diag.Infof("hearing result final=%t src_len=%d dst_len=%d audio_bytes=%d", result.IsFinal, len(result.SrcText), len(result.DstText), len(result.AudioPCM))
			switch {
			case result.SrcText != "":
				updateText := trimSubmittedHearingPrefix(result.SrcText, lastSubmitted)
				draftText = compactHearingDraftText(mergeHearingInterim(draftText, updateText))
				diag.Infof("hearing asr draft updated final=%t stable=%t chars=%d text=%q", result.IsFinal, result.Stable, len(draftText), trimHearingLog(draftText, 120))
				emitFn(EventStep, pipeline.StepEvent{
					Chain:   "hearing",
					Step:    pipeline.StepASR,
					SrcText: draftText,
					IsFinal: false,
				})
			case result.DstText == "" && len(result.AudioPCM) == 0:
				select {
				case transQueue <- hearingTransItem{Result: result}:
					diag.Infof("hearing trans queued empty final=%t queue_depth=%d", result.IsFinal, len(transQueue))
				case <-ctx.Done():
					return
				}
			case result.DstText != "" || len(result.AudioPCM) > 0:
				select {
				case transQueue <- hearingTransItem{Result: result}:
					diag.Infof("hearing trans queued final=%t src_chars=%d dst_chars=%d audio_bytes=%d queue_depth=%d", result.IsFinal, len(result.SrcText), len(result.DstText), len(result.AudioPCM), len(transQueue))
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (c *Chain) queueHearingSentence(ctx context.Context, transQueue chan<- hearingTransItem, sentence string) {
	sentence = strings.TrimSpace(sentence)
	if sentence == "" {
		return
	}
	result := asr.Result{SrcText: sentence, IsFinal: true}
	select {
	case transQueue <- hearingTransItem{Result: result}:
		diag.Infof("hearing sentence queued chars=%d queue_depth=%d text=%q", len(sentence), len(transQueue), trimHearingLog(sentence, 120))
	case <-ctx.Done():
		return
	}
}

func trimHearingLog(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "...(truncated)"
}

func (c *Chain) transWorker(
	ctx context.Context,
	queue <-chan hearingTransItem,
	ttsQueue chan<- hearingTTSItem,
	transExtension trans.Extension,
	classifier *intent.Classifier,
	retriever *rag.Retriever,
	generator *llm.AnswerGenerator,
	sessionID string,
	ragPromptTpl string,
	targetLang string,
	emitFn llm.EventEmitter,
	monitor audio.MonitorSink,
	monitorAudioQueue chan<- []byte,
	monitorPrefersExtensionAudio bool,
) {
	defer close(ttsQueue)
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-queue:
			if !ok {
				return
			}
			result := item.Result
			if result.IsFinal && transExtension != nil {
				started := time.Now()
				diag.Infof("hearing trans begin src_chars=%d queue_depth=%d", len(result.SrcText), len(queue))
				processed, err := transExtension.Process(ctx, result)
				if err != nil {
					diag.Warnf("hearing trans extension skipped err=%v", err)
				} else {
					result = processed
					diag.Infof("hearing trans done elapsed=%s src_chars=%d dst_chars=%d", diag.Since(started), len(result.SrcText), len(result.DstText))
				}
			}
			if result.DstText != "" {
				emitFn(EventStep, pipeline.StepEvent{
					Chain:   "hearing",
					Step:    pipeline.StepTrans,
					DstText: result.DstText,
					IsFinal: result.IsFinal,
				})
			}
			if len(result.AudioPCM) > 0 && monitor != nil && (monitorPrefersExtensionAudio || result.DstText == "") {
				c.queueExtensionAudio(ctx, result, emitFn, monitor, monitorAudioQueue)
			}
			if result.SrcText == "" && result.DstText == "" && len(result.AudioPCM) > 0 {
				continue
			}
			if result.DstText != "" {
				emitFn(EventSubtitle, SubtitleEvent{
					Text:    result.DstText,
					IsFinal: result.IsFinal,
				})
			} else if result.SrcText == "" && len(result.AudioPCM) == 0 {
				emitFn(EventSubtitle, SubtitleEvent{
					Text:    "",
					IsFinal: result.IsFinal,
				})
			}
			if result.IsFinal && result.DstText != "" && !monitorPrefersExtensionAudio && monitor != nil {
				select {
				case ttsQueue <- hearingTTSItem{Text: result.DstText, IsFinal: result.IsFinal}:
					diag.Infof("hearing tts queued chars=%d queue_depth=%d", len(result.DstText), len(ttsQueue))
				case <-ctx.Done():
					return
				}
			}
			if result.IsFinal && result.SrcText != "" && classifier != nil && retriever != nil && generator != nil {
				c.startRAG(ctx, result.SrcText, result.DstText, classifier, retriever, generator, sessionID, ragPromptTpl, targetLang, emitFn)
			}
		}
	}
}

func (c *Chain) ttsWorker(ctx context.Context, queue <-chan hearingTTSItem, monitor audio.MonitorSink, emitFn llm.EventEmitter) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-queue:
			if !ok {
				return
			}
			if item.Text == "" || monitor == nil {
				continue
			}
			emitFn(EventStep, pipeline.StepEvent{
				Chain:   "hearing",
				Step:    pipeline.StepTTS,
				DstText: item.Text,
				IsFinal: item.IsFinal,
			})
			started := time.Now()
			diag.Infof("hearing tts begin chars=%d queue_depth=%d", len(item.Text), len(queue))
			if err := monitor.Speak(ctx, item.Text); err != nil {
				diag.Warnf("hearing monitor speak failed err=%v", err)
			} else {
				diag.Infof("hearing monitor spoke elapsed=%s chars=%d", diag.Since(started), len(item.Text))
			}
		}
	}
}

func (c *Chain) queueExtensionAudio(
	ctx context.Context,
	result asr.Result,
	emitFn llm.EventEmitter,
	monitor audio.MonitorSink,
	monitorAudioQueue chan<- []byte,
) {
	emitFn(EventStep, pipeline.StepEvent{
		Chain:      "hearing",
		Step:       pipeline.StepTTS,
		DstText:    result.DstText,
		IsFinal:    result.IsFinal,
		AudioBytes: len(result.AudioPCM),
	})
	pcm := append([]byte(nil), result.AudioPCM...)
	if monitorAudioQueue != nil {
		select {
		case monitorAudioQueue <- pcm:
			diag.Infof("hearing monitor pcm queued bytes=%d queue_depth=%d", len(pcm), len(monitorAudioQueue))
		case <-ctx.Done():
		}
	} else if err := monitor.PlayPCM(ctx, pcm); err != nil {
		diag.Warnf("hearing monitor pcm failed err=%v", err)
	}
}

func (c *Chain) startRAG(
	ctx context.Context,
	srcText string,
	dstText string,
	classifier *intent.Classifier,
	retriever *rag.Retriever,
	generator *llm.AnswerGenerator,
	sessionID string,
	ragPromptTpl string,
	targetLang string,
	emitFn llm.EventEmitter,
) {
	go func() {
		intentType := classifier.Classify(ctx, srcText)
		diag.Infof("hearing intent classified intent=%s question_chars=%d", intentType, len(srcText))
		if intentType == intent.IntentStatement {
			diag.Infof("hearing rag skipped intent=%s", intentType)
			return
		}
		history := generator.RecentHistory(sessionID)
		historyTexts := historyTextsForRetrieval(history)
		diag.Infof("hearing rag begin intent=%s question_chars=%d history_turns=%d", intentType, len(srcText), len(history))
		ragResult := retrieveHearingRAG(ctx, retriever, srcText, dstText, historyTexts)
		diag.Infof("hearing rag retrieved active_resume=%t chunks=%d intent=%s history_turns=%d", ragResult.HasActiveResume, len(ragResult.Chunks), intentType, len(history))
		if !ragResult.HasActiveResume {
			emitFn(EventSubtitle, SubtitleEvent{Text: "（未激活简历，回答仅供参考）", IsFinal: true})
		}
		diag.Infof("hearing answer generation queued session=%s chunks=%d with_history=%t", sessionID, len(ragResult.Chunks), intentType == intent.IntentFollowup)
		generator.Generate(ctx, llm.GenerateConfig{
			SessionID:       sessionID,
			Question:        srcText,
			DisplayQuestion: displayQuestion(srcText, dstText),
			TargetLanguage:  targetLang,
			ResumeChunks:    ragResult.Chunks,
			PromptTpl:       ragPromptTpl,
			WithHistory:     intentType == intent.IntentFollowup,
		})
	}()
}

func historyTextsForRetrieval(history []llm.QAPair) []string {
	texts := make([]string, 0, len(history)*2)
	for _, turn := range history {
		if strings.TrimSpace(turn.Question) != "" {
			texts = append(texts, turn.Question)
		}
		if strings.TrimSpace(turn.Answer) != "" {
			texts = append(texts, turn.Answer)
		}
	}
	return texts
}

func retrieveHearingRAG(ctx context.Context, retriever *rag.Retriever, srcText, dstText string, historyTexts []string) rag.RetrieveResult {
	type result struct {
		value rag.RetrieveResult
	}
	done := make(chan result, 1)
	go func() {
		done <- result{value: retriever.RetrieveWithContext(srcText, dstText, historyTexts)}
	}()
	timer := time.NewTimer(hearingRAGTimeout)
	defer timer.Stop()
	select {
	case got := <-done:
		return got.value
	case <-timer.C:
		diag.Warnf("hearing rag timeout elapsed=%s; continuing without resume chunks", hearingRAGTimeout)
		return rag.RetrieveResult{HasActiveResume: true, Chunks: nil}
	case <-ctx.Done():
		return rag.RetrieveResult{HasActiveResume: true, Chunks: nil}
	}
}

func displayQuestion(srcText, dstText string) string {
	if strings.TrimSpace(dstText) != "" {
		return dstText
	}
	return srcText
}
