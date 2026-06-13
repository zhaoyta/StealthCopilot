## Context

视频链是三条链中技术复杂度最高的。Simli AI API 有约 200-400ms 云端处理延迟，Ring Buffer 必须补偿这个延迟才能保证 A/V 同步。虚拟摄像头需要平台级驱动，且必须在 Setup 向导中一次性安装。熔断机制是最后的保障，必须在 10ms 内完成切换。

## Goals / Non-Goals

**Goals:**
- 视频帧率 ≥30fps
- 音视频差 ≤40ms
- 熔断切换 ≤10ms，无黑屏无断音
- 虚拟摄像头不依赖 OBS

**Non-Goals:**
- 不实现多摄像头切换
- 不实现视频录制功能
- StealthCloudProvider 预留接口但不实现

## Decisions

### D1：Simli AI WebSocket 流式传输
向 Simli API 发送：视频帧（JPEG，256×256 crop 面部区域）+ 对应时间戳的音频 chunk（PCM）。Simli 返回处理后的视频帧，Go 后端写入虚拟摄像头。

### D2：Ring Buffer 补偿 Simli 延迟
Go 后端维护一个循环缓冲区，存储过去 1s 的原始视频帧（带时间戳）。向 Simli 发送时带上时间戳，Simli 返回处理帧时携带对应时间戳，Go 后端按时间戳将处理帧与原始帧对齐后输出到虚拟摄像头。

### D3：虚拟摄像头基于 AkVirtualCamera 改造
AkVirtualCamera 是 webcamoid 的虚拟摄像头组件，支持 macOS CoreMediaIO DAL 和 Windows DirectShow。fork 后精简为仅包含必要驱动代码，去除 webcamoid UI 依赖，打包为 `StealthVirtualCam.plugin`（Mac）和 `StealthVirtualCam.dll`（Win），在 Setup 向导中安装。

### D4：熔断用双轨并行架构
真实摄像头和麦克风始终保持捕获（不释放），只是输出端切换。正常模式：输出走 Simli 处理后的数据；熔断模式：直接输出原始数据。切换只是改一个 atomic bool，≤1ms。

### D5：心跳用 UDP 而非 TCP
TCP 重传机制会掩盖网络抖动，用 UDP 可以精确检测丢包。心跳包 50ms 一个，连续 3 个丢失（150ms）即触发熔断，不等 TCP 超时重传。

## Risks / Trade-offs

- [macOS CoreMediaIO DAL 签名] DAL 插件需要 Apple Developer 签名，否则系统拒绝加载 → 需提前申请 Apple Developer 账号（$99/年），代码签名 change 单独处理
- [Simli API 网络抖动] Simli 响应时间不稳定时 Ring Buffer 可能溢出 → Buffer 容量设为 2s，超出时丢弃最旧帧并记录日志
- [面部 crop 精度] 256×256 面部裁剪需要实时人脸检测 → 使用 OpenCV 内置 Haar Cascade，CPU 轻量
