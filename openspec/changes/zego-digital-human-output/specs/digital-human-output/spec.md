## ADDED Requirements

### Requirement: 说话链数字人输出模式
系统 SHALL 在说话链中提供数字人输出模式开关。关闭时说话链 SHALL 使用当前虚拟音频输出路径；开启时说话链 SHALL 根据所选 Provider 驱动数字人视频输出。默认 Provider 为 Simli AI，企业级 Provider 可选 ZEGO。

#### Scenario: 数字人关闭时使用虚拟音频
- **WHEN** 用户启动说话链且数字人输出模式关闭
- **THEN** 系统将 TTS 音频直接写入本机虚拟麦克风，不启动数字人 Provider，不要求 OBS Virtual Camera 可用

#### Scenario: 数字人开启时使用 Simli 视频输出
- **WHEN** 用户启动说话链且数字人输出模式开启
- **THEN** 系统默认启动 Simli Provider，发送 TTS PCM 获取口型同步视频，并将视频发布到 OBS 浏览器源

#### Scenario: Simli 模式输出视频、本地输出音频
- **WHEN** 数字人 Provider 为 Simli
- **THEN** 系统 SHALL 将 TTS PCM 立即发送给 Simli 生成 WebRTC 视频
- **AND** 系统 SHALL 将 Simli 视频解码后发布到 OBS 浏览器源
- **AND** 系统 SHALL 延迟约 700ms 将本地 TTS PCM 写入虚拟麦克风以补偿视频延迟

#### Scenario: OBS 作为会议摄像头出口
- **WHEN** 数字人 Provider 已输出视频帧
- **THEN** 系统 SHALL 在本机提供 `http://127.0.0.1:18765/` OBS Browser Source
- **AND** 用户 SHALL 在 OBS 中启动 OBS Virtual Camera，并在会议软件选择 `OBS Virtual Camera`

### Requirement: Simli 会话生命周期
系统 SHALL 通过 Simli API 管理数字人会话生命周期，包括获取 token、建立 WebSocket / WebRTC、发送 PCM 音频、接收视频轨道，以及停止清理会话资源。

#### Scenario: 创建 Simli 会话
- **WHEN** 数字人输出模式启动且 Provider 为 Simli
- **THEN** 系统使用 Simli API Key 和 Face ID 获取会话 token，并建立实时会话

#### Scenario: 停止数字人输出
- **WHEN** 用户停止说话链或数字人启动过程中失败
- **THEN** 系统关闭 Simli WebSocket / WebRTC、停止 ffmpeg 解码，并停止向 OBS Browser Source 发布新帧

### Requirement: 数字人配置校验
系统 SHALL 在数字人输出模式启动前校验所有必需配置和设备依赖，缺失时返回明确错误并保持说话链未启动。

#### Scenario: Simli 凭证缺失
- **WHEN** 用户开启数字人输出模式但 Simli API Key 未配置
- **THEN** 系统拒绝启动说话链，并提示用户补全 Simli API Key

#### Scenario: Face ID 缺失
- **WHEN** 用户开启数字人输出模式但 Simli Face ID 未配置
- **THEN** 系统拒绝启动说话链，并提示用户补全 Simli Face ID

#### Scenario: 虚拟设备缺失
- **WHEN** 用户开启数字人输出模式但虚拟麦克风不可用
- **THEN** 系统拒绝启动说话链，并提示缺失的本机虚拟麦克风；Simli 模式下 OBS Virtual Camera 由 OBS App 提供，App 不要求自身枚举到 OBS 摄像头设备

### Requirement: 数字人运行时诊断
系统 SHALL 记录数字人输出模式的关键阶段诊断，且不得记录明文 API Key 或其他敏感凭证。

#### Scenario: 记录阶段流转
- **WHEN** 数字人输出模式运行
- **THEN** 诊断日志记录 Provider 启动、WebSocket/WebRTC 连接、PCM 发送、视频解码、OBS 浏览器源输出、虚拟麦写入和停止清理阶段

#### Scenario: 隐藏敏感信息
- **WHEN** Simli 鉴权或请求失败
- **THEN** 错误信息和诊断日志 MUST NOT 输出明文 API Key，只能输出是否配置、长度、请求 ID、错误码和非敏感摘要

### Requirement: 首页只呈现两条业务链
首页 SHALL 只将听力链和说话链作为顶层业务链展示。数字人输出 SHALL 作为说话链内部开关、状态和配置提示展示。

#### Scenario: 数字人不是第三条链
- **WHEN** 用户打开首页
- **THEN** 页面显示听力链和说话链两个主卡片或主区域，不显示独立的数字人链或视频链主开关

#### Scenario: 说话链展示输出模式
- **WHEN** 用户查看说话链状态
- **THEN** 页面显示当前输出模式为虚拟音频或数字人音视频，并显示对应目标设备或配置状态
