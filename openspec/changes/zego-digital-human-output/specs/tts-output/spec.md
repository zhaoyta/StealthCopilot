## MODIFIED Requirements

### Requirement: 默认音色与讯飞声音复刻流式 TTS 输出
系统 SHALL 支持默认音色和个人复刻音色两种 TTS 合成来源。默认音色无需声音复刻训练；个人复刻音色 SHALL 调用讯飞声音复刻流式 TTS API，使用用户训练完成后获得的 Asset ID，将目标语言文本转为音频流。TTS 音频 SHALL 再进入说话链输出模式：数字人关闭时写入虚拟麦克风；数字人开启时发送到数字人输出链路，并由数字人链路的拉流音频写入虚拟麦克风。

#### Scenario: 首帧即播
- **WHEN** TTS provider 返回第一个音频 chunk 且数字人输出模式关闭
- **THEN** 系统立即写入虚拟麦克风，不等待完整音频

#### Scenario: Zero-PCM 静音保护
- **WHEN** VAD 触发到可输出音频首帧到达之间
- **THEN** 系统持续向虚拟麦克风写入全零 PCM，避免用户母语声音泄漏

#### Scenario: TTS 音频覆盖静音
- **WHEN** TTS provider 返回首个音频 chunk 且数字人输出模式关闭
- **THEN** 虚拟麦克风从 Zero-PCM 切换为 TTS 音频流

#### Scenario: 默认音色输出
- **WHEN** 用户未完成声音复刻训练或主动选择默认音色
- **THEN** 说话链使用默认音色合成目标语言音频，并明确显示当前不是个人复刻音色

#### Scenario: 个人复刻音色缺少 Asset ID
- **WHEN** 用户未完成声音复刻训练或未保存 Asset ID
- **THEN** 个人复刻音色不可用，应用提示用户先完成声音复刻训练或切换为默认音色

#### Scenario: 数字人模式接管 TTS 音频输出
- **WHEN** TTS provider 返回音频 chunk 且数字人输出模式开启
- **THEN** 系统将 TTS 音频 chunk 发送给数字人输出链路，不直接写入虚拟麦克风
