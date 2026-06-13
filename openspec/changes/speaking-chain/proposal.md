## Why

说话链让面试官听到用户自己声音的克隆英文，而不是机器音色。用户说母语，系统在 1.2s 内完成翻译和 TTS，期间向虚拟麦克风写静音防止母语泄漏。VAD 精确检测说话结束，避免截断或等待过长。

## What Changes

- 本地 VAD（Voice Activity Detection）检测用户说话结束
- 讯飞语音翻译 API 接入（母语语音 → 目标语言文本，单次调用）
- ElevenLabs 流式 TTS（目标语言文本 → 克隆音色音频）
- 虚拟麦克风写入（音频输出 + 等待期 Zero-PCM 静音）

## Capabilities

### New Capabilities

- `vad`: 本地语音活动检测，检测用户说话开始和结束
- `speaking-translation`: 讯飞语音翻译接入，母语语音 → 目标语言文本
- `tts-output`: ElevenLabs 流式 TTS + 虚拟麦克风写入 + Zero-PCM 静音管理

### Modified Capabilities

## Impact

- 新增 `internal/vad/detector.go`（VAD，使用 WebRTC VAD 或 Silero VAD Go binding）
- 新增 `internal/tts/elevenlabs.go`（ElevenLabs TTS Provider 实现）
- 新增 `internal/audio/virtual_mic.go`（虚拟麦克风写入，portaudio）
- 依赖：讯飞语音翻译 API、ElevenLabs Streaming TTS API、portaudio
