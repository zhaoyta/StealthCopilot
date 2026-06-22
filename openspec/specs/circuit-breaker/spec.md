## ADDED Requirements

### Requirement: 数字人输出失败降级
系统 SHALL 在数字人 Provider、视频解码或 OBS 输出链路不可用时停止当前数字人输出并提示用户。系统 SHALL 保留听力链和不启用数字人的说话链能力，避免数字人故障阻断会议音频。

#### Scenario: Simli 启动失败
- **WHEN** Simli token、WebSocket、WebRTC SDP 或视频轨道接收失败
- **THEN** 系统 SHALL 拒绝启动数字人输出并展示明确错误；用户可关闭数字人后启动说话链虚拟麦克风音频

#### Scenario: 视频解码失败
- **WHEN** ffmpeg VP8/H264 解码失败或无法持续输出帧
- **THEN** 系统 SHALL 记录 `simli video: ffmpeg stderr=` 或相关诊断日志，并提示用户检查 Simli/ffmpeg/OBS 配置

#### Scenario: OBS 不可用
- **WHEN** OBS App 未运行、OBS Browser Source 未添加或 OBS Virtual Camera 未启动
- **THEN** 系统 SHALL 提示用户按 OBS 配置指南操作；系统 SHALL NOT 尝试将真实摄像头直通到自研虚拟摄像头

#### Scenario: 保留音频路径
- **WHEN** 用户关闭数字人输出
- **THEN** 说话链 SHALL 继续使用虚拟麦克风输出目标语言 TTS，不要求 Simli 或 OBS 可用
