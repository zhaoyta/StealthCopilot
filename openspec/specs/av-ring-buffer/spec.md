## ADDED Requirements

### Requirement: Simli 数字人音视频延迟补偿
系统 SHALL 在 Simli 数字人模式下补偿视频链路延迟。Simli 仅生成视频，会议音频仍由本地 TTS 写入虚拟麦克风；系统 SHALL 对本地音频写入施加可控延迟，使会议侧感知到的声音尽量贴近 OBS 输出的视频口型。

#### Scenario: TTS 同时驱动 Simli 和本地音频
- **WHEN** 说话链 TTS 产出 PCM chunk 且数字人 Provider 为 Simli
- **THEN** 系统 SHALL 立即将 PCM chunk 发送给 Simli 驱动生成口型同步视频
- **AND** 系统 SHALL 延迟约 700ms 后再将同一 chunk 写入本机虚拟麦克风

#### Scenario: 视频帧异步输出
- **WHEN** ffmpeg 解码出 Simli VP8 视频帧
- **THEN** 系统 SHALL 立即读取解码帧以避免 RTP/ffmpeg 背压
- **AND** 系统 SHALL 通过异步编码循环以稳定帧率向 OBS 浏览器源发布最新帧

#### Scenario: 延迟需要调参
- **WHEN** 用户反馈会议侧声音领先或落后口型
- **THEN** 开发者 SHOULD 对比诊断日志中的首个 `speaking tts chunk` 时间和 `simli event=SPEAK` 时间，调整 Simli 本地音频延迟

#### Scenario: ZEGO 不使用本地音频延迟
- **WHEN** 数字人 Provider 抑制本地直出音频
- **THEN** 系统 SHALL 以 Provider 返回的音频为准，不额外写入本地 TTS PCM，也不应用 Simli 本地音频延迟
