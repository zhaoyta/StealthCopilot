## MODIFIED Requirements

### Requirement: Provider 接口定义
系统 SHALL 定义 `TTSProvider` Go interface，抽象流式语音合成能力，默认实现为默认音色 TTS，接口支持流式音频输出（chunk by chunk）。

#### Scenario: 默认 TTS Provider
- **WHEN** 应用加载默认 Provider 配置
- **THEN** TTS Provider 为默认音色
