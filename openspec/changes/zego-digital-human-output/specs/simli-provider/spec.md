## ADDED Requirements

### Requirement: Simli AI 说话链数字人视频 Provider
系统 SHALL 将 Simli AI 作为说话链数字人视频 Provider。Simli 模式 SHALL 使用说话链 TTS PCM 驱动 Simli WebRTC 视频；音频仍由本地 TTS 写入虚拟麦克风，视频通过 OBS Browser Source 输出。

#### Scenario: 建立 Simli 会话
- **WHEN** 用户启动说话链且数字人 Provider 为 Simli
- **THEN** 后端使用 Simli API Key 和 Face ID 获取会话 token，建立 WebSocket / WebRTC 会话，并注册视频轨道接收器

#### Scenario: 发送 TTS 音频驱动口型
- **WHEN** 讯飞声音复刻或默认 TTS 产出 PCM chunk
- **THEN** 后端 SHALL 将 PCM 发送给 Simli 驱动口型同步视频
- **AND** 后端 SHALL 延迟约 700ms 将本地 TTS PCM 写入虚拟麦克风，以补偿视频链路延迟

#### Scenario: 解码视频并输出给 OBS
- **WHEN** Simli WebRTC 返回 VP8 或 H264 视频轨道
- **THEN** 后端 SHALL 使用 ffmpeg 解码视频帧，并发布到本机 OBS 浏览器源 `http://127.0.0.1:18765/`

#### Scenario: OBS 输出要求
- **WHEN** 用户希望会议软件看到数字人画面
- **THEN** 用户 MUST 打开 OBS App，添加应用提供的浏览器源，启动 OBS Virtual Camera，并在会议软件中选择 `OBS Virtual Camera`

#### Scenario: Simli 视频不可用
- **WHEN** Simli token、WebSocket、WebRTC、ffmpeg 解码或 OBS 浏览器源输出失败
- **THEN** 系统 SHALL 记录诊断日志并提示用户；系统 SHALL NOT 尝试注册自研虚拟摄像头驱动
