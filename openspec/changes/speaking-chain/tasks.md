## 1. 物理麦克风捕获

- [ ] 1.1 在 `internal/audio/mic.go` 实现物理麦克风 PCM 捕获（portaudio，16kHz 16bit）
- [ ] 1.2 实现 PCM 缓冲队列，供 VAD 检测使用

## 2. VAD

- [ ] 2.1 引入 `github.com/gillesdemey/go-webrtcvad`，在 `internal/vad/detector.go` 封装 VAD 逻辑
- [ ] 2.2 实现静音检测：连续静音帧超过阈值时触发回调，将缓存音频段传出
- [ ] 2.3 静音阈值从配置读取（默认 800ms），支持运行时更新
- [ ] 2.4 Wails 暴露 `StartSpeakingChain` / `StopSpeakingChain` binding

## 3. 讯飞语音翻译（说话链）

- [ ] 3.1 在 `internal/translation/xunfei_speak.go` 实现说话链的讯飞语音翻译调用（REST API，非 WebSocket）
- [ ] 3.2 设置 2s 超时，超时时触发降级（停止 Zero-PCM，真实麦克风直通）

## 4. ElevenLabs TTS

- [ ] 4.1 在 `internal/tts/elevenlabs.go` 实现 `ElevenLabsProvider`（实现 TTSProvider 接口）
- [ ] 4.2 调用 `/v1/text-to-speech/{voice_id}/stream`，流式接收音频 chunk
- [ ] 4.3 实现双缓冲队列：Zero-PCM 写入队列和 TTS 音频写入队列，按状态切换

## 5. 虚拟麦克风写入

- [ ] 5.1 在 `internal/audio/virtual_mic.go` 实现虚拟麦克风 portaudio 输出流，按设备名绑定 BlackHole/VB-Cable
- [ ] 5.2 实现 Zero-PCM 写入（全零 buffer，采样率 44100Hz）
- [ ] 5.3 实现 TTS 音频写入，首帧到达时原子切换从 Zero-PCM 到 TTS 音频
- [ ] 5.4 TTS 播完后回到 Zero-PCM 待机状态
