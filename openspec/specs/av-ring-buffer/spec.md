## ADDED Requirements

### Requirement: A/V 环形缓冲区时间戳对齐
系统 SHALL 使用 Go 环形缓冲区对齐音频帧与 Simli 返回的视频帧时间戳，补偿 Simli API 约 200-400ms 的云端处理延迟，保证输出到虚拟摄像头的音视频帧 delta ≤ 40ms。

#### Scenario: 音频帧入队
- **WHEN** ElevenLabs TTS 产出 PCM 音频 chunk
- **THEN** 音频帧携带 PTS（presentation timestamp，单位 ms）写入音频环形缓冲区

#### Scenario: 视频帧入队
- **WHEN** Simli AI 返回口型同步视频帧
- **THEN** 视频帧携带对应的 PTS 写入视频环形缓冲区

#### Scenario: 帧对齐输出
- **WHEN** 音频缓冲区和视频缓冲区均有 PTS 差值 ≤ 40ms 的帧对
- **THEN** 同时弹出并输出：音频帧写入虚拟麦克风，视频帧写入虚拟摄像头

#### Scenario: 视频帧延迟超出容忍范围
- **WHEN** 视频帧 PTS 落后音频帧超过 300ms（Simli 严重延迟）
- **THEN** 触发熔断器逻辑，停止等待，切换到摄像头原始直通

#### Scenario: 缓冲区溢出保护
- **WHEN** 任一缓冲区积压超过 2s 的帧数据（约 60fps × 2s = 120 帧）
- **THEN** 丢弃最旧帧，保持缓冲区不超过上限，防止内存增长
