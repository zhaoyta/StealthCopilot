## ADDED Requirements

### Requirement: 讯飞语音翻译（说话链）
系统 SHALL 将 VAD 捕获的完整音频段发给讯飞语音翻译 API，获取目标语言文本，发给 TTS 模块。

#### Scenario: 语音翻译成功
- **WHEN** VAD 触发并发送音频到讯飞
- **THEN** 在 500ms 内返回目标语言文本，立即传给 ElevenLabs TTS

#### Scenario: 翻译 API 超时
- **WHEN** 讯飞 API 超过 2s 未响应
- **THEN** 取消本次翻译请求，虚拟麦克风停止 Zero-PCM，恢复真实麦克风直通
