## ADDED Requirements

### Requirement: 本地 VAD 检测说话结束
系统 SHALL 使用 WebRTC VAD 对物理麦克风输入实时检测语音活动，在用户停止说话超过阈值时间后触发翻译流程。

#### Scenario: 检测说话结束
- **WHEN** 用户停止说话且静音持续超过阈值（默认 800ms）
- **THEN** VAD 触发，将缓存的完整音频段发给讯飞语音翻译 API

#### Scenario: 静音阈值可配置
- **WHEN** 用户在设置中调整 VAD 灵敏度
- **THEN** 静音阈值在 400ms-2000ms 范围内调整，即时生效
