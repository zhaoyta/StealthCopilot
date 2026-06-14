// Package speaking 实现说话链的完整管道协调：
// 物理麦克风捕获 → VAD 语音段检测 → 讯飞语音翻译 → ElevenLabs 流式 TTS → 虚拟麦克风写入。
// Chain 由 app_bindings.go 通过 Wails binding 启动和停止。
package speaking

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhaoyta/stealthcopilot/internal/audio"
	"github.com/zhaoyta/stealthcopilot/internal/llm"
	"github.com/zhaoyta/stealthcopilot/internal/translation"
	"github.com/zhaoyta/stealthcopilot/internal/tts"
	"github.com/zhaoyta/stealthcopilot/internal/vad"
)

// 说话链向前端推送的 Wails 事件名常量
const (
	// EventSpeakStart VAD 触发后通知前端"正在翻译中"
	EventSpeakStart = "speaking:start"
	// EventSpeakDone TTS 播放完毕通知前端
	EventSpeakDone = "speaking:done"
	// EventSpeakError 翻译/TTS 出错或超时降级时通知前端
	EventSpeakError = "speaking:error"
)

// ChainConfig 说话链运行时所需的配置和服务依赖。
type ChainConfig struct {
	// Xunfei 讯飞翻译 API 配置（复用听力链凭据）
	Xunfei translation.XunfeiSpeakConfig
	// Translator overrides Xunfei when a different speech translation provider is selected.
	Translator translation.SpeakProvider
	// ElevenLabs TTS 配置
	ElevenLabs tts.ElevenLabsConfig
	// TTSProvider overrides ElevenLabs when a different TTS provider is selected.
	TTSProvider tts.Provider
	// PhysicalMicDevice 物理麦克风设备名称
	PhysicalMicDevice string
	// VirtualMicDevice 虚拟声卡设备名称（BlackHole/VB-Cable）
	VirtualMicDevice string
	// SilenceThresholdMs VAD 静音阈值（毫秒），从用户设置读取
	SilenceThresholdMs int
	// AudioSink 接收 TTS 音频 chunk，用于驱动视频口型同步链。
	AudioSink func([]byte)

	// --- DeepSeek 润色配置（可选，PolishEnabled=false 时完全跳过） ---

	// DeepSeekKey DeepSeek API Key；为空时即使 PolishEnabled=true 也跳过润色。
	DeepSeekKey string
	// DeepSeekModel DeepSeek 模型名称，如 "deepseek-chat"。
	DeepSeekModel string
	// LLMConfig configures OpenAI-compatible polishing.
	LLMConfig llm.Config
	// PolishPrompt 润色 Prompt 模板，包含 {input} 占位符。
	PolishPrompt string
	// PolishEnabled 为 true 时在讯飞翻译后、ElevenLabs TTS 前调用 DeepSeek 润色。
	PolishEnabled bool
}

// Chain 是说话链的主协调器，持有各组件实例和运行状态。
type Chain struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	vadDetect *vad.EnergyDetector
}

// Start 以给定配置启动说话链。已在运行时幂等（先停止后重启）。
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

	// 静音阈值（默认 800ms）
	threshMs := cfg.SilenceThresholdMs
	if threshMs <= 0 {
		threshMs = vad.DefaultSilenceThresholdMs
	}

	// VAD 检测器
	detector := vad.NewEnergyDetector(threshMs, 40)
	c.vadDetect = detector

	// 物理麦克风捕获：用户选择设备时使用系统采集；未配置设备时保持静音降级。
	var mic audio.MicProvider = &audio.NullMicProvider{}
	if cfg.PhysicalMicDevice != "" {
		mic = audio.NewSystemMicProvider()
	}
	audioStream, err := mic.Start(ctx, cfg.PhysicalMicDevice)
	if err != nil {
		cancel()
		c.cancel = nil
		return "物理麦克风启动失败：" + err.Error()
	}

	translator := cfg.Translator
	if translator == nil {
		translator = translation.NewXunfeiSpeakProvider(cfg.Xunfei)
	}

	var ttsProvider tts.Provider = cfg.TTSProvider
	switch {
	case ttsProvider != nil:
		// injected provider
	case cfg.ElevenLabs.APIKey != "" && cfg.ElevenLabs.VoiceID != "":
		ttsProvider = tts.NewElevenLabsProvider(cfg.ElevenLabs)
	default:
		ttsProvider = &tts.NullTTSProvider{}
	}

	// 虚拟麦克风写入：支持可写系统 sink 时使用真实 writer，否则降级为 Null writer。
	virtualMic := audio.NewSystemVirtualMicWriter(cfg.VirtualMicDevice)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer mic.Close()
		defer ttsProvider.Close()
		defer virtualMic.Close()

		// VAD 回调：每当检测到完整语音段时触发说话链管道
		detector.Run(ctx, audioStream, func(seg vad.SpeechSegment) {
			c.handleSegment(ctx, wailsCtx, seg, translator, ttsProvider, virtualMic, cfg)
		})
	}()

	return ""
}

// Stop 停止说话链，等待所有 goroutine 退出后返回。
func (c *Chain) Stop() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.mu.Unlock()
	c.wg.Wait()
}

// SetSilenceThreshold 运行时更新 VAD 静音阈值（毫秒），即时生效。
func (c *Chain) SetSilenceThreshold(ms int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vadDetect != nil {
		c.vadDetect.SetSilenceThreshold(ms)
	}
}

// handleSegment 处理一段 VAD 检测到的完整语音：翻译 → [DeepSeek润色] → TTS → 虚拟麦克风写入。
// 流程（时序关键）：
//  1. 立即 BeginZeroPCM（防止母语泄漏）
//  2. 调用讯飞翻译 API（约 500ms）
//  3. [可选] PolishEnabled=true 时调用 DeepSeek 润色（约 1-2s，超时降级使用原文）
//  4. 获取最终文本 → 调用 ElevenLabs TTS 流式合成
//  5. 首帧到达时 WriteChunk（原子切换 Zero-PCM → TTS 音频）
//  6. 流结束 → EndTTS（恢复 Idle）
func (c *Chain) handleSegment(
	ctx context.Context,
	wailsCtx context.Context,
	seg vad.SpeechSegment,
	translator translation.SpeakProvider,
	ttsProvider tts.Provider,
	virtualMic audio.VirtualMicWriter,
	cfg ChainConfig,
) {
	// 1. 立即开始写 Zero-PCM，阻断母语泄漏
	virtualMic.BeginZeroPCM()
	runtime.EventsEmit(wailsCtx, EventSpeakStart)

	// 2. 讯飞语音翻译（2s 超时）
	translatedText, err := translator.Translate(ctx, seg.PCM)
	if err != nil {
		// 超时或翻译失败：停止 Zero-PCM，降级（真实麦克风直通由用户手动切换）
		virtualMic.EndTTS()
		runtime.EventsEmit(wailsCtx, EventSpeakError, "语音翻译超时，请检查讯飞 API 配置")
		return
	}

	// 3. [可选] DeepSeek 润色：将讯飞翻译结果润色为更流利的英文
	//    PolishEnabled=true 且 DeepSeekKey 非空时调用；超时/失败时静默降级使用原译文。
	llmCfg := cfg.LLMConfig
	if llmCfg.APIKey == "" {
		llmCfg.APIKey = cfg.DeepSeekKey
	}
	if llmCfg.Model == "" {
		llmCfg.Model = cfg.DeepSeekModel
	}
	if cfg.PolishEnabled && llmCfg.APIKey != "" {
		if polished, polishErr := llm.PolishWithConfig(ctx, llmCfg, cfg.PolishPrompt, translatedText); polishErr == nil {
			translatedText = polished
		}
		// polish 出错时 translatedText 保持讯飞翻译原文，不中断流程
	}

	// 4. ElevenLabs TTS 流式合成
	audioCh, err := ttsProvider.Synthesize(ctx, translatedText)
	if err != nil {
		virtualMic.EndTTS()
		runtime.EventsEmit(wailsCtx, EventSpeakError, "TTS 合成失败："+err.Error())
		return
	}

	// 5. 流式写入虚拟麦克风（首帧自动切换 Zero-PCM → TTS 音频）
	for chunk := range audioCh {
		select {
		case <-ctx.Done():
			virtualMic.EndTTS()
			return
		default:
			virtualMic.WriteChunk(chunk)
			if cfg.AudioSink != nil {
				cfg.AudioSink(chunk)
			}
		}
	}

	// 5. TTS 结束，回到 Idle
	virtualMic.EndTTS()
	runtime.EventsEmit(wailsCtx, EventSpeakDone)
}
