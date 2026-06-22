// Package speaking 实现说话链的完整管道协调：
// 物理麦克风捕获 → VAD 语音段检测 → ASR/Trans/TTS 扩展步骤 → 虚拟麦克风写入。
// Chain 由 app_bindings.go 通过 Wails binding 启动和停止。
package speaking

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/asr"
	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/diag"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/pipeline"
	"github.com/zhaoyta/stealthcopilot/internal/session"
	"github.com/zhaoyta/stealthcopilot/internal/trans"
	"github.com/zhaoyta/stealthcopilot/internal/tts"
	"github.com/zhaoyta/stealthcopilot/internal/vad"
)

// 说话链向前端推送的 Wails 事件名常量
const (
	speakingMaxSpeechMs = 2400

	// EventSpeakStart VAD 触发后通知前端"正在翻译中"
	EventSpeakStart = "speaking:start"
	// EventSpeakDone TTS 播放完毕通知前端
	EventSpeakDone = "speaking:done"
	// EventSpeakError 翻译/TTS 出错或超时降级时通知前端
	EventSpeakError = "speaking:error"
	// EventSpeakResult 携带说话链最终输出文本（TTS 前）
	EventSpeakResult = "speaking:result"
	// EventSpeakStep 携带说话链 ASR/Trans/TTS 扩展步骤的实时产出。
	EventSpeakStep = "speaking:step"
)

type ResultEvent struct {
	SrcText string `json:"srcText"`
	DstText string `json:"dstText"`
	IsFinal bool   `json:"isFinal"`
}

type ttsQueueItem struct {
	SegmentID int64
	SrcText   string
	Text      string
	Index     int
	Total     int
}

type segmentQueueItem struct {
	ID  int64
	Seg vad.SpeechSegment
}

// ChainConfig 说话链运行时所需的配置和服务依赖。
type ChainConfig struct {
	// Simult 语音同传配置，默认用于获取原文和译文。
	Simult asr.XunfeiSimultConfig
	// ASRExtension overrides Xunfei when a different segmented ASR extension is selected.
	ASRExtension asr.SegmentExtension
	// TransExtension allows replacing the text translation/post-processing extension.
	TransExtension trans.Extension
	// XunfeiVoiceClone TTS 配置
	XunfeiVoiceClone tts.XunfeiVoiceCloneConfig
	// TTSExtension overrides XunfeiVoiceClone when a different TTS extension is selected.
	TTSExtension tts.Extension
	// PhysicalMicDevice 物理麦克风设备名称
	PhysicalMicDevice string
	// VirtualMicDevice 虚拟声卡设备名称（BlackHole/VB-Cable）
	VirtualMicDevice string
	// SilenceThresholdMs VAD 静音阈值（毫秒），从用户设置读取
	SilenceThresholdMs int
	// AudioSink 接收 TTS 音频 chunk，用于驱动视频口型同步链。
	AudioSink func([]byte)
	// EventSink mirrors speaking events to non-Wails consumers such as the native teleprompter.
	EventSink llm.EventEmitter
	// SessionStore/SessionID append candidate speech to the active interview history.
	SessionStore session.Store
	SessionID    string

	// --- DeepSeek 润色配置（可选，PolishEnabled=false 时完全跳过） ---

	// DeepSeekKey DeepSeek API Key；为空时即使 PolishEnabled=true 也跳过润色。
	DeepSeekKey string
	// DeepSeekModel DeepSeek 模型名称，如 "deepseek-chat"。
	DeepSeekModel string
	// LLMConfig configures OpenAI-compatible polishing.
	LLMConfig llm.Config
	// PolishPrompt 润色 Prompt 模板，包含 {input} 占位符。
	PolishPrompt string
	// PolishEnabled 为 true 时在讯飞翻译后、TTS 前调用 DeepSeek 润色。
	PolishEnabled bool
}

// Chain 是说话链的主协调器，持有各组件实例和运行状态。
type Chain struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	vadDetect *vad.EnergyDetector
	nextID    int64
}

// Start 以给定配置启动说话链。已在运行时幂等（先停止后重启）。
// wailsCtx 是 Wails 应用 context，用于 EventsEmit 推送事件。
func (c *Chain) Start(wailsCtx context.Context, cfg ChainConfig) string {
	started := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.wg.Wait()
	}

	ctx, cancel := context.WithCancel(wailsCtx)
	c.cancel = cancel

	// 静音阈值（默认 800ms）
	threshMs := cfg.SilenceThresholdMs
	if threshMs <= 0 {
		threshMs = vad.DefaultSilenceThresholdMs
	}

	// VAD 检测器
	detector := vad.NewEnergyDetector(threshMs, 40)
	detector.SetMaxSpeechMs(speakingMaxSpeechMs)
	c.vadDetect = detector
	diag.Infof("speaking vad initialized silence_threshold_ms=%d max_speech_ms=%d", threshMs, speakingMaxSpeechMs)

	// 物理麦克风捕获：用户选择设备时使用系统采集；未配置设备时保持静音降级。
	var mic audio.MicProvider = &audio.NullMicProvider{}
	if cfg.PhysicalMicDevice != "" {
		var micErr string
		mic, micErr = audio.NewSystemMicProviderChecked()
		if micErr != "" {
			cancel()
			c.cancel = nil
			diag.Errorf("speaking mic provider failed device=%q err=%q", cfg.PhysicalMicDevice, micErr)
			return "物理麦克风启动失败：" + micErr
		}
	}
	audioStream, err := mic.Start(ctx, cfg.PhysicalMicDevice)
	if err != nil {
		cancel()
		c.cancel = nil
		diag.Errorf("speaking mic start failed device=%q err=%v", cfg.PhysicalMicDevice, err)
		return "物理麦克风启动失败：" + err.Error()
	}
	diag.Infof("speaking mic started device=%q", cfg.PhysicalMicDevice)

	asrExtension := cfg.ASRExtension
	if asrExtension == nil {
		if cfg.PhysicalMicDevice != "" && !asr.XunfeiSimultConfigReady(cfg.Simult) {
			cancel()
			_ = mic.Close()
			c.cancel = nil
			diag.Errorf("speaking asr extension config incomplete source_lang=%s target_lang=%s", cfg.Simult.SourceLang, cfg.Simult.TargetLang)
			return "讯飞同声传译配置不完整：请配置 AppID、API Key、API Secret 和说话链语言"
		}
		asrExtension = asr.NewXunfeiSimultSegmentExtension(cfg.Simult)
	}

	var ttsExtension tts.Extension = cfg.TTSExtension
	switch {
	case ttsExtension != nil:
		// injected extension
	case tts.XunfeiVoiceCloneConfigReady(cfg.XunfeiVoiceClone):
		ttsExtension = tts.NewXunfeiVoiceCloneExtension(cfg.XunfeiVoiceClone)
	default:
		if cfg.VirtualMicDevice != "" {
			cancel()
			_ = mic.Close()
			c.cancel = nil
			diag.Errorf("speaking tts config incomplete virtual_mic=%q", cfg.VirtualMicDevice)
			return "讯飞声音复刻 TTS 配置不完整：请配置 AppID、API Key、API Secret，并完成音色训练获得 Asset ID"
		}
		ttsExtension = &tts.NullExtension{}
	}

	// 虚拟麦克风写入：未配置时允许 Null，用户选择设备时必须是真实 writer。
	var virtualMic audio.VirtualMicWriter = audio.NewNullVirtualMicWriter()
	if cfg.VirtualMicDevice != "" {
		var virtualMicErr string
		virtualMic, virtualMicErr = audio.NewSystemVirtualMicWriterChecked(cfg.VirtualMicDevice)
		if virtualMicErr != "" {
			cancel()
			_ = mic.Close()
			c.cancel = nil
			diag.Errorf("speaking virtual mic writer failed device=%q err=%q", cfg.VirtualMicDevice, virtualMicErr)
			return virtualMicErr
		}
	}
	diag.Infof("speaking chain started elapsed=%s virtual_mic=%q", diag.Since(started), cfg.VirtualMicDevice)

	segmentQueue := make(chan segmentQueueItem, 8)
	ttsQueue := make(chan ttsQueueItem, 16)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.ttsWorker(ctx, wailsCtx, ttsQueue, ttsExtension, virtualMic, cfg)
	}()
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.segmentWorker(ctx, wailsCtx, segmentQueue, asrExtension, ttsQueue, cfg)
	}()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer mic.Close()
		defer diag.Infof("speaking vad worker stopped")

		// VAD 回调：每当检测到完整语音段时触发说话链管道
		detector.Run(ctx, audioStream, func(seg vad.SpeechSegment) {
			parts := splitSegmentForSpeaking(seg)
			if len(parts) > 1 {
				diag.Warnf("speaking vad segment split original_bytes=%d original_pcm_ms=%d parts=%d max_ms=%d", len(seg.PCM), pcmDurationMs(seg.PCM), len(parts), speakingMaxSpeechMs)
			}
			for _, part := range parts {
				segmentID := c.nextSegmentID()
				item := segmentQueueItem{ID: segmentID, Seg: part}
				select {
				case segmentQueue <- item:
					diag.Infof("speaking vad segment queued segment=%d bytes=%d duration_ms=%d pcm_ms=%d peak=%d queue_depth=%d", segmentID, len(part.PCM), part.DurationMs, pcmDurationMs(part.PCM), audioPeak(part.PCM), len(segmentQueue))
				case <-ctx.Done():
					return
				}
			}
		})
	}()

	return ""
}

func (c *Chain) nextSegmentID() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return c.nextID
}

// Stop 停止说话链，等待所有 goroutine 退出后返回。
func (c *Chain) Stop() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.mu.Unlock()
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		diag.Infof("speaking chain stopped")
	case <-time.After(3 * time.Second):
		diag.Warnf("speaking chain stop timed out")
	}
}

// SetSilenceThreshold 运行时更新 VAD 静音阈值（毫秒），即时生效。
func (c *Chain) SetSilenceThreshold(ms int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vadDetect != nil {
		c.vadDetect.SetSilenceThreshold(ms)
	}
}

func (c *Chain) segmentWorker(
	ctx context.Context,
	wailsCtx context.Context,
	queue <-chan segmentQueueItem,
	asrExtension asr.SegmentExtension,
	ttsQueue chan<- ttsQueueItem,
	cfg ChainConfig,
) {
	defer diag.Infof("speaking segment worker stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-queue:
			diag.Infof("speaking segment dequeued segment=%d queue_depth=%d", item.ID, len(queue))
			c.handleSegment(ctx, wailsCtx, item, asrExtension, ttsQueue, cfg)
		}
	}
}

// handleSegment 处理一段 VAD 检测到的完整语音：翻译 → [DeepSeek润色] → TTS → 虚拟麦克风写入。
// 流程（时序关键）：
//  1. 立即 BeginZeroPCM（防止母语泄漏）
//  2. 调用 ASR extension 获取源语言文本与目标语言文本
//  3. [可选] PolishEnabled=true 时调用 DeepSeek 润色（约 1-2s，超时降级使用原文）
//  4. 获取最终文本 → 调用 TTS 流式合成
//  5. 首帧到达时 WriteChunk（原子切换 Zero-PCM → TTS 音频）
//  6. 流结束 → EndTTS（恢复 Idle）
func (c *Chain) handleSegment(
	ctx context.Context,
	wailsCtx context.Context,
	item segmentQueueItem,
	asrExtension asr.SegmentExtension,
	ttsQueue chan<- ttsQueueItem,
	cfg ChainConfig,
) {
	started := time.Now()
	runtime.EventsEmit(wailsCtx, EventSpeakStart)
	pcmMs := pcmDurationMs(item.Seg.PCM)
	diag.Infof("speaking segment start segment=%d bytes=%d duration_ms=%d pcm_ms=%d", item.ID, len(item.Seg.PCM), item.Seg.DurationMs, pcmMs)

	// 2. 讯飞语音翻译（2s 超时）
	asrStarted := time.Now()
	diag.Infof("speaking asr_trans begin segment=%d pcm_ms=%d peak=%d", item.ID, pcmMs, audioPeak(item.Seg.PCM))
	speechText, err := asrExtension.Translate(ctx, item.Seg.PCM)
	if err != nil {
		if isContextCanceled(ctx, err) {
			diag.Infof("speaking translate canceled segment=%d asr_elapsed=%s elapsed=%s err=%v", item.ID, diag.Since(asrStarted), diag.Since(started), err)
			runtime.EventsEmit(wailsCtx, EventSpeakDone)
			return
		}
		if errors.Is(err, asr.ErrNoSpeechRecognized) {
			diag.Infof("speaking recognition skipped segment=%d asr_elapsed=%s elapsed=%s reason=%q", item.ID, diag.Since(asrStarted), diag.Since(started), err)
			runtime.EventsEmit(wailsCtx, EventSpeakDone)
			return
		}
		if errors.Is(err, asr.ErrNoTranslationReturned) {
			diag.Warnf("speaking translation missing segment=%d asr_elapsed=%s elapsed=%s err=%v", item.ID, diag.Since(asrStarted), diag.Since(started), err)
			runtime.EventsEmit(wailsCtx, EventSpeakError, "同传已识别到语音，但没有返回目标语言译文；请检查说话链源语言/目标语言设置和讯飞同传权限")
			return
		}
		diag.Warnf("speaking translate failed segment=%d asr_elapsed=%s elapsed=%s err=%v", item.ID, diag.Since(asrStarted), diag.Since(started), err)
		runtime.EventsEmit(wailsCtx, EventSpeakError, "语音翻译失败，请检查讯飞 API 配置、网络或输入语言")
		return
	}
	diag.Infof("speaking asr_trans done segment=%d elapsed=%s src_chars=%d dst_chars=%d final=%t", item.ID, diag.Since(asrStarted), len(speechText.SrcText), len(speechText.DstText), speechText.IsFinal)
	translatedText := speechText.DstText
	if translatedText == "" {
		translatedText = speechText.SrcText
	}
	runtime.EventsEmit(wailsCtx, EventSpeakStep, pipeline.StepEvent{
		Chain:   "speaking",
		Step:    pipeline.StepASR,
		SrcText: speechText.SrcText,
		IsFinal: true,
	})

	transExtension := cfg.TransExtension
	if transExtension == nil {
		transExtension = trans.NoopExtension{}
	}
	transStarted := time.Now()
	processedText, transErr := transExtension.Process(ctx, asr.Result{
		SrcText: speechText.SrcText,
		DstText: translatedText,
		IsFinal: true,
	})
	if transErr != nil {
		diag.Warnf("speaking trans extension skipped segment=%d elapsed=%s err=%v", item.ID, diag.Since(transStarted), transErr)
	} else {
		speechText = processedText
		translatedText = speechText.DstText
		if translatedText == "" {
			translatedText = speechText.SrcText
		}
		diag.Infof("speaking trans extension done segment=%d elapsed=%s src_chars=%d dst_chars=%d", item.ID, diag.Since(transStarted), len(speechText.SrcText), len(translatedText))
	}
	diag.Infof("speaking translate ok segment=%d elapsed=%s translated_chars=%d", item.ID, diag.Since(started), len(translatedText))
	runtime.EventsEmit(wailsCtx, EventSpeakStep, pipeline.StepEvent{
		Chain:   "speaking",
		Step:    pipeline.StepTrans,
		DstText: translatedText,
		IsFinal: true,
	})
	c.emitResult(wailsCtx, cfg, ResultEvent{
		SrcText: speechText.SrcText,
		DstText: translatedText,
		IsFinal: !cfg.PolishEnabled,
	})

	// 3. [可选] DeepSeek 润色：将目标文本润色为更流利的英文
	//    PolishEnabled=true 且 DeepSeekKey 非空时调用；超时/失败时静默降级使用原译文。
	llmCfg := cfg.LLMConfig
	if llmCfg.APIKey == "" {
		llmCfg.APIKey = cfg.DeepSeekKey
	}
	if llmCfg.Model == "" {
		llmCfg.Model = cfg.DeepSeekModel
	}
	if cfg.PolishEnabled && llmCfg.APIKey != "" {
		polishStarted := time.Now()
		diag.Infof("speaking polish begin segment=%d chars=%d", item.ID, len(translatedText))
		if polished, polishErr := llm.PolishWithConfig(ctx, llmCfg, cfg.PolishPrompt, translatedText); polishErr == nil {
			translatedText = polished
			diag.Infof("speaking polish ok segment=%d elapsed=%s chars=%d", item.ID, diag.Since(polishStarted), len(translatedText))
		} else {
			diag.Warnf("speaking polish skipped segment=%d elapsed=%s err=%v", item.ID, diag.Since(polishStarted), polishErr)
		}
		// polish 出错时 translatedText 保持文本翻译原文，不中断流程
	}
	if cfg.PolishEnabled {
		c.emitResult(wailsCtx, cfg, ResultEvent{
			SrcText: speechText.SrcText,
			DstText: translatedText,
			IsFinal: true,
		})
	}
	c.appendCandidateHistory(cfg, speechText.SrcText, translatedText)

	sentences := splitForTTS(translatedText)
	if len(sentences) == 0 {
		runtime.EventsEmit(wailsCtx, EventSpeakDone)
		return
	}
	for i, sentence := range sentences {
		item := ttsQueueItem{
			SegmentID: item.ID,
			SrcText:   speechText.SrcText,
			Text:      sentence,
			Index:     i + 1,
			Total:     len(sentences),
		}
		select {
		case ttsQueue <- item:
			diag.Infof("speaking tts queued segment=%d sentence=%d/%d chars=%d queue_depth=%d", item.SegmentID, item.Index, item.Total, len(item.Text), len(ttsQueue))
		case <-ctx.Done():
			return
		}
	}
	diag.Infof("speaking segment queued segment=%d elapsed=%s sentences=%d", item.ID, diag.Since(started), len(sentences))
}

func (c *Chain) ttsWorker(
	ctx context.Context,
	wailsCtx context.Context,
	queue <-chan ttsQueueItem,
	ttsExtension tts.Extension,
	virtualMic audio.VirtualMicWriter,
	cfg ChainConfig,
) {
	defer ttsExtension.Close()
	defer virtualMic.Close()
	defer diag.Infof("speaking tts worker stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-queue:
			diag.Infof("speaking tts dequeued segment=%d sentence=%d/%d queue_depth=%d", item.SegmentID, item.Index, item.Total, len(queue))
			c.playTTSItem(ctx, wailsCtx, item, ttsExtension, virtualMic, cfg)
			if item.Index == item.Total && len(queue) == 0 {
				runtime.EventsEmit(wailsCtx, EventSpeakDone)
			}
		}
	}
}

func (c *Chain) playTTSItem(
	ctx context.Context,
	wailsCtx context.Context,
	item ttsQueueItem,
	ttsExtension tts.Extension,
	virtualMic audio.VirtualMicWriter,
	cfg ChainConfig,
) {
	started := time.Now()
	diag.Infof("speaking tts sentence begin segment=%d sentence=%d/%d virtual_mic=%q chars=%d", item.SegmentID, item.Index, item.Total, cfg.VirtualMicDevice, len(item.Text))
	runtime.EventsEmit(wailsCtx, EventSpeakStep, pipeline.StepEvent{
		Chain:   "speaking",
		Step:    pipeline.StepTTS,
		DstText: item.Text,
		IsFinal: item.Index == item.Total,
	})
	virtualMic.BeginZeroPCM()
	stopLoopbackCheck := startVirtualMicLoopbackCheck(ctx, cfg.VirtualMicDevice, item.SegmentID)
	defer stopLoopbackCheck()
	audioCh, err := ttsExtension.Synthesize(ctx, item.Text)
	if err != nil {
		virtualMic.EndTTS()
		if isContextCanceled(ctx, err) {
			diag.Infof("speaking tts synth canceled segment=%d sentence=%d/%d err=%v", item.SegmentID, item.Index, item.Total, err)
			return
		}
		diag.Warnf("speaking tts failed segment=%d sentence=%d/%d err=%v", item.SegmentID, item.Index, item.Total, err)
		runtime.EventsEmit(wailsCtx, EventSpeakError, "TTS 合成失败："+err.Error())
		return
	}
	diag.Infof("speaking tts stream started segment=%d sentence=%d/%d chars=%d", item.SegmentID, item.Index, item.Total, len(item.Text))

	chunkCount := 0
	byteCount := 0
	var playbackStarted time.Time
	for chunk := range audioCh {
		select {
		case <-ctx.Done():
			virtualMic.EndTTS()
			diag.Infof("speaking tts stream canceled segment=%d chunks=%d bytes=%d", item.SegmentID, chunkCount, byteCount)
			return
		default:
		}
		chunkCount++
		byteCount += len(chunk)
		if playbackStarted.IsZero() {
			playbackStarted = time.Now()
		}
		if chunkCount == 1 || chunkCount%20 == 0 {
			diag.Infof("speaking tts chunk segment=%d sentence=%d/%d chunks=%d bytes=%d last_chunk=%d peak=%d", item.SegmentID, item.Index, item.Total, chunkCount, byteCount, len(chunk), audioPeak(chunk))
		}
		virtualMic.WriteChunk(chunk)
		if cfg.AudioSink != nil {
			cfg.AudioSink(chunk)
		}
		if !sleepUntilAudioClock(ctx, playbackStarted, byteCount) {
			virtualMic.EndTTS()
			diag.Warnf("speaking tts pacing canceled segment=%d chunks=%d bytes=%d", item.SegmentID, chunkCount, byteCount)
			return
		}
	}
	virtualMic.EndTTS()
	diag.Infof("speaking tts sentence done segment=%d elapsed=%s sentence=%d/%d chunks=%d bytes=%d", item.SegmentID, diag.Since(started), item.Index, item.Total, chunkCount, byteCount)
}

func audioPeak(frame []byte) int {
	return audio.PCMPeak(frame)
}

func isContextCanceled(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (c *Chain) appendCandidateHistory(cfg ChainConfig, srcText, dstText string) {
	if cfg.SessionStore == nil || cfg.SessionID == "" {
		return
	}
	srcText = strings.TrimSpace(srcText)
	dstText = strings.TrimSpace(dstText)
	if srcText == "" && dstText == "" {
		return
	}
	display := srcText
	if dstText != "" && dstText != srcText {
		display = srcText + "\n" + dstText
	}
	if err := cfg.SessionStore.AppendTurn(cfg.SessionID, "候选人发言", display, srcTextOrTarget(srcText, dstText)); err != nil {
		diag.Warnf("speaking candidate history append failed session=%s err=%v", cfg.SessionID, err)
	}
}

func (c *Chain) emitResult(wailsCtx context.Context, cfg ChainConfig, result ResultEvent) {
	runtime.EventsEmit(wailsCtx, EventSpeakResult, result)
	if cfg.EventSink != nil {
		cfg.EventSink(EventSpeakResult, result)
	}
}

func srcTextOrTarget(srcText, dstText string) string {
	if strings.TrimSpace(srcText) != "" {
		return strings.TrimSpace(srcText)
	}
	return strings.TrimSpace(dstText)
}

func splitSegmentForSpeaking(seg vad.SpeechSegment) []vad.SpeechSegment {
	if pcmDurationMs(seg.PCM) <= speakingMaxSpeechMs {
		return []vad.SpeechSegment{seg}
	}
	maxFrames := speakingMaxSpeechMs / int(audio.FrameDur.Milliseconds())
	if maxFrames <= 0 {
		maxFrames = 1
	}
	chunkBytes := maxFrames * audio.FrameBytes
	if chunkBytes <= 0 {
		return []vad.SpeechSegment{seg}
	}
	parts := make([]vad.SpeechSegment, 0, (len(seg.PCM)+chunkBytes-1)/chunkBytes)
	for offset := 0; offset < len(seg.PCM); offset += chunkBytes {
		end := offset + chunkBytes
		if end > len(seg.PCM) {
			end = len(seg.PCM)
		}
		pcm := append([]byte(nil), seg.PCM[offset:end]...)
		parts = append(parts, vad.SpeechSegment{
			PCM:        pcm,
			DurationMs: pcmDurationMs(pcm),
		})
	}
	return parts
}

func pcmDurationMs(pcm []byte) int {
	if len(pcm) == 0 {
		return 0
	}
	return len(pcm) * 1000 / (audio.SampleRate * audio.BytesPerSample)
}

func virtualMicPCMDuration(bytes int) time.Duration {
	if bytes <= 0 {
		return 0
	}
	return time.Duration(bytes) * time.Second / time.Duration(audio.VirtualMicSampleRate*audio.BytesPerSample)
}

func sleepUntilAudioClock(ctx context.Context, started time.Time, bytes int) bool {
	target := started.Add(virtualMicPCMDuration(bytes))
	wait := time.Until(target)
	if wait <= 0 {
		return true
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
