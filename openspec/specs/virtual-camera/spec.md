## ADDED Requirements

### Requirement: OBS 浏览器源虚拟摄像头输出
系统 SHALL 不再注册自研虚拟摄像头驱动；数字人视频输出 SHALL 通过本机 OBS 浏览器源交给 OBS Studio，再由 OBS Virtual Camera 暴露给会议软件。

#### Scenario: 提供 OBS 浏览器源
- **WHEN** 说话链启动且数字人视频输出启用
- **THEN** 后端启动本地 HTTP 服务，提供 `http://127.0.0.1:18765/` 作为 OBS Browser Source 页面

#### Scenario: 输出实时视频流
- **WHEN** Simli WebRTC 视频帧被成功解码
- **THEN** 后端将最新视频帧异步编码为 OBS 页面可播放的 MJPEG 流，不阻塞 Simli RTP/ffmpeg 解码链路

#### Scenario: OBS 负责系统摄像头
- **WHEN** 用户需要在飞书、Zoom 或 Teams 中输出数字人画面
- **THEN** 用户 MUST 在 OBS 中添加浏览器源 `http://127.0.0.1:18765/`，启动 OBS Virtual Camera，并在会议软件中选择 `OBS Virtual Camera`

#### Scenario: OBS 不可用
- **WHEN** OBS App 未运行、OBS Virtual Camera 未启动或 macOS OBS Camera Extension 不可用
- **THEN** 会议软件可能无法选择 `OBS Virtual Camera` 或只能看到 OBS 占位/黑屏；应用 SHALL 提示用户按 OBS 配置指南处理，而不是尝试注册自研摄像头驱动

#### Scenario: 数字人关闭
- **WHEN** 数字人视频输出关闭
- **THEN** 说话链 SHALL 只输出虚拟麦克风音频，不要求 OBS 浏览器源或 OBS Virtual Camera 可用
