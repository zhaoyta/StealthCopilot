## Context

说话链延迟目标 ≤1.2s。讯飞语音翻译 API 约 300-500ms，ElevenLabs 流式 TTS 首帧约 400-500ms，两者串行刚好在边界。VAD 必须准确，误触发会导致翻译不完整的句子；漏触发会让用户等待过长。

## Goals / Non-Goals

**Goals:**
- 中文说话结束到英文音频首帧输出 ≤1.2s
- 期间虚拟麦克风无母语泄漏
- VAD 灵敏度用户可在设置中调节

**Non-Goals:**
- 不实现实时流式 STT（整句 VAD 触发后批量处理）
- DeepSeek 润色默认关闭，不在此 change 实现启用逻辑

## Decisions

### D1：VAD 使用 WebRTC VAD Go binding
WebRTC VAD 是业界标准，Go 有 `github.com/gillesdemey/go-webrtcvad` binding，检测精度高，CPU 开销极低。灵敏度（0-3 级）作为设置项暴露给用户，默认 2 级。

### D2：VAD 触发后批量发讯飞翻译
用户说话期间缓存 PCM 数据，VAD 检测到静音超过阈值（默认 800ms）时，将整段音频发给讯飞语音翻译 API。不做流式 STT（避免讯飞 API 按时间计费且句子不完整时翻译质量差）。

### D3：ElevenLabs 流式 TTS，首帧即播
调用 ElevenLabs `/v1/text-to-speech/{voice_id}/stream` 接口，收到第一个音频 chunk 即开始写入虚拟麦克风，无需等待完整音频。

### D4：等待期写 Zero-PCM
从 VAD 触发到 ElevenLabs 首帧之间（约 800ms），Go 后端持续向虚拟麦克风写入全零 PCM 数据（静音），防止用户说中文的声音泄漏给面试官。

### D5：虚拟麦克风写入用 portaudio 输出流
通过设备名称绑定 BlackHole / VB-Cable 输出设备，用 portaudio 的 output stream 写入音频数据。采样率与 ElevenLabs 输出一致（44100Hz）。

## Risks / Trade-offs

- [VAD 误触发] 说话中间自然停顿可能触发 VAD → 静音阈值默认 800ms，用户可在设置中调节至 1200ms
- [ElevenLabs 配额] 按字符计费，面试中大量翻译费用较高 → 后续 Phase 3 提供自营 TTS 作为更低成本选项
- [Zero-PCM 写入时序] Zero-PCM 和 TTS 音频切换必须无缝 → 使用双缓冲队列确保切换无爆音
