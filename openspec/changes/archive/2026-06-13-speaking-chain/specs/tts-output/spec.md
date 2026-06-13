## ADDED Requirements

### Requirement: ElevenLabs 流式 TTS 输出
系统 SHALL 调用 ElevenLabs 流式 TTS API，使用用户克隆的 Voice ID，将目标语言文本转为音频流，首帧到达即写入虚拟麦克风。

#### Scenario: 流式首帧即播
- **WHEN** ElevenLabs 返回第一个音频 chunk
- **THEN** 立即写入虚拟麦克风开始播放，不等待完整音频

#### Scenario: Zero-PCM 静音保护
- **WHEN** VAD 触发到 ElevenLabs 首帧到达之间
- **THEN** Go 后端持续向虚拟麦克风写入全零 PCM，面试官听到静音而非用户母语

#### Scenario: TTS 与静音无缝切换
- **WHEN** ElevenLabs 首帧到达，从 Zero-PCM 切换为 TTS 音频
- **THEN** 切换无爆音、无断裂，音频连续
