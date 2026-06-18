## ADDED Requirements

### Requirement: Simli AI 实时口型同步
系统 SHALL 通过 Simli AI 官方 SaaS API（WebSocket 流式）实现用户真实人脸的口型同步，音频帧与视频帧同步输入，接收口型同步后的视频帧输出。

#### Scenario: 建立 Simli 会话
- **WHEN** 说话链开始（VAD 触发后讯飞声音复刻首帧到达）
- **THEN** 后端使用 Simli API Key 建立 WebSocket 连接，发送 session 初始化参数（face_id = 用户配置的 Face ID）

#### Scenario: 音频+视频帧输入
- **WHEN** 讯飞声音复刻 TTS 产出 PCM 音频 chunk
- **THEN** 同时将该 PCM chunk 和对应时间戳发送给 Simli WebSocket 输入流

#### Scenario: 口型同步帧输出
- **WHEN** Simli API 返回口型同步视频帧
- **THEN** 后端将视频帧写入 A/V 环形缓冲区，等待时间戳对齐后输出到虚拟摄像头

#### Scenario: API Key 未配置
- **WHEN** Simli API Key 或 Face ID 为空
- **THEN** 跳过 Simli 管道，虚拟摄像头直接输出摄像头原始画面（无口型同步）

#### Scenario: Simli 连接断开
- **WHEN** Simli WebSocket 连接意外断开
- **THEN** 触发熔断器，在 ≤10ms 内切换到真实摄像头直通
