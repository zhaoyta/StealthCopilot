## 1. 物理摄像头捕获

- [x] 1.1 在 `internal/video/capture.go` 用 OpenCV (gocv) 捕获物理摄像头帧（BGRA，≥30fps）
- [x] 1.2 Wails 暴露 `StartVideoChain` / `StopVideoChain` binding
- [x] 1.3 实现摄像头设备枚举，返回可用设备列表供前端下拉

## 2. 虚拟摄像头驱动

- [x] 2.1 macOS：将 CoreMediaIO DAL 插件二进制内嵌到 App bundle，首次启动时以 launchctl / admin 权限注册
- [x] 2.2 Windows：将 DirectShow Filter DLL 内嵌到安装包，首次启动时 regsvr32 注册（触发 UAC）
- [x] 2.3 在 `internal/video/virtual_camera.go` 实现通过共享内存/named pipe 写入帧的接口
- [x] 2.4 实现驱动已注册检测逻辑，避免重复注册

## 3. Simli AI Provider

- [x] 3.1 在 `internal/lipsync/simli.go` 实现 `SimliProvider`（实现 LipSyncProvider 接口）
- [x] 3.2 实现 Simli WebSocket 连接建立（API Key 鉴权，发送 face_id）
- [x] 3.3 实现音频帧 + 时间戳的 WebSocket 输入流
- [x] 3.4 实现视频帧 + 时间戳的 WebSocket 输出流解析
- [x] 3.5 实现连接断开时的自动重连（指数退避，最多 3 次）

## 4. A/V 环形缓冲区

- [x] 4.1 在 `internal/video/ring_buffer.go` 实现音频/视频双通道环形缓冲区（容量 120 帧）
- [x] 4.2 实现 PTS 对齐逻辑：遍历两队列，找 delta ≤ 40ms 的帧对弹出
- [x] 4.3 实现溢出保护：缓冲区超限时丢弃最旧帧
- [x] 4.4 实现延迟监控：视频 PTS 落后音频 >300ms 时触发熔断回调

## 5. 熔断器

- [x] 5.1 在 `internal/circuit/breaker.go` 实现熔断器状态机（Closed / Open / Half-Open）
- [x] 5.2 实现 50ms UDP 心跳发送，维护连续丢包计数器
- [x] 5.3 触发熔断时：断开 Simli 连接，清空环形缓冲区，将摄像头帧直通虚拟摄像头
- [x] 5.4 触发熔断时：停止 ElevenLabs 输出，真实麦克风直通虚拟麦克风
- [x] 5.5 恢复检测：心跳连续 3 次正常后自动重建 Simli 连接
- [x] 5.6 Wails EventEmit 推送熔断状态变化（`circuit:open` / `circuit:closed`）到前端
- [x] 5.7 前端幽灵提词窗监听 `circuit:open`，显示橙色警告条和"紧急降级"按钮
