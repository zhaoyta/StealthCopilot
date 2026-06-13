## ADDED Requirements

### Requirement: 熔断器与硬件直通降级
系统 SHALL 实现熔断器，在云端管道（Simli AI / ElevenLabs）不可用时，在 ≤10ms 内切换到真实摄像头 + 真实麦克风直通，面试过程无中断。

#### Scenario: UDP 心跳检测
- **WHEN** 视频管道激活
- **THEN** 后端每 50ms 向 Simli API 发送 UDP 心跳包，维护连续丢包计数器

#### Scenario: 心跳丢失触发熔断
- **WHEN** 连续 3 次心跳（150ms）无响应
- **THEN** 立即触发熔断：断开 Simli WebSocket，停止 ElevenLabs 输出，清空环形缓冲区，在 ≤10ms 内将真实摄像头帧直通虚拟摄像头，真实麦克风直通虚拟麦克风

#### Scenario: 视频延迟触发熔断
- **WHEN** 视频帧 PTS 落后音频 PTS 超过 300ms
- **THEN** 同上，立即触发熔断器

#### Scenario: 熔断期间用户体验
- **WHEN** 熔断器已激活
- **THEN** 幽灵提词窗顶部出现橙色警告条"云端管道已断开，当前为本地直通模式"，听力链字幕和 RAG 建议保持正常工作

#### Scenario: 熔断器自动恢复
- **WHEN** 熔断后连续 3 次心跳恢复正常（150ms）
- **THEN** 自动重建 Simli WebSocket 连接，恢复云端管道，警告条消失，恢复前提示用户"云端管道已恢复"

#### Scenario: 手动触发熔断
- **WHEN** 用户在幽灵提词窗点击"紧急降级"按钮
- **THEN** 立即触发熔断，无论心跳状态如何
