## ADDED Requirements

### Requirement: 自绑定虚拟摄像头驱动
系统 SHALL 内置基于 AkVirtualCamera 的虚拟摄像头驱动，无需用户安装 OBS，App 首次启动时自动注册驱动，向 Zoom/Teams 等会议软件暴露为可选摄像头设备。

#### Scenario: 驱动自动注册（macOS）
- **WHEN** App 首次启动或驱动未检测到
- **THEN** 自动注册 CoreMediaIO DAL 插件，需要一次性用户授权（系统扩展或 root 权限弹窗），注册完成后无需重复

#### Scenario: 驱动自动注册（Windows）
- **WHEN** App 首次启动或驱动未检测到
- **THEN** 自动注册 DirectShow Filter（regsvr32），需要 UAC 提权弹窗，注册完成后无需重复

#### Scenario: 视频帧写入虚拟摄像头
- **WHEN** 虚拟摄像头驱动已注册且处于激活状态
- **THEN** Go 后端通过共享内存 / named pipe 将 BGRA 视频帧推送到驱动，驱动向系统暴露该帧流

#### Scenario: 驱动未注册时的降级
- **WHEN** 驱动注册失败或用户拒绝授权
- **THEN** 前端提示用户"虚拟摄像头不可用，视频管道将禁用"，说话链和听力链不受影响

#### Scenario: 设备枚举
- **WHEN** 用户在设置中打开设备绑定
- **THEN** 系统枚举所有可用摄像头设备（包含已注册的虚拟摄像头），用户可选择作为输出设备
