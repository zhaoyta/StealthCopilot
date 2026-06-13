## Why

视频链让面试官看到用户口型与英文音频完全对齐的真人画面。Simli AI API 驱动真实人脸做口型同步，Ring Buffer 补偿云端延迟保证 A/V 对齐，熔断机制在网络异常时无感切回真实摄像头，不让面试官看到任何异常。

## What Changes

- Simli AI 实时口型同步 API 接入（LipSyncProvider 实现）
- 自研虚拟摄像头驱动捆绑（macOS CoreMediaIO DAL 插件 + Windows DirectShow Filter）
- Ring Buffer 音视频时间戳对齐
- 50ms UDP 心跳 + 熔断切换机制

## Capabilities

### New Capabilities

- `simli-provider`: Simli AI LipSyncProvider 实现，WebSocket 流式传输视频帧 + 音频，接收处理后视频帧
- `virtual-camera`: 自研虚拟摄像头驱动，macOS CoreMediaIO DAL 插件 + Windows DirectShow Filter（基于 AkVirtualCamera）
- `av-ring-buffer`: 音视频 Ring Buffer，时间戳对齐，补偿 Simli 云端处理延迟
- `circuit-breaker`: 50ms UDP 心跳监测，触发条件下 ≤10ms 切回真实硬件

### Modified Capabilities

## Impact

- 新增 `internal/lipsync/simli.go`（Simli AI Provider 实现）
- 新增 `internal/video/ring_buffer.go`（A/V 同步缓冲）
- 新增 `internal/video/virtual_cam.go`（虚拟摄像头写入接口）
- 新增 `internal/circuit/breaker.go`（熔断逻辑）
- 新增平台驱动：`drivers/mac/StealthVirtualCam.plugin`、`drivers/win/StealthVirtualCam.dll`
- 依赖：Simli AI API、OpenCV（摄像头帧捕获）、portaudio（音频帧）
