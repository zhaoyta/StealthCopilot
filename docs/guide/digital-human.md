# 数字人与 OBS 配置

数字人视频输出是说话链的可选输出模式。开启后，StealthCopilot 将说话链的 TTS 音频发送给 Simli AI 生成口型同步视频，同时把音频写入虚拟麦克风；视频通过本机 OBS 浏览器源进入 OBS，再由 OBS Virtual Camera 输出给飞书、Zoom 或 Teams。

当前推荐链路：

```text
说话链 TTS
  -> Simli AI WebRTC 视频
  -> http://127.0.0.1:18765/
  -> OBS Browser Source
  -> OBS Virtual Camera
  -> 会议软件摄像头
```

## 前置条件

- 已安装 OBS Studio。
- OBS 的虚拟摄像头扩展可用。
- 已在应用中配置 Simli API Key 和 Face ID。
- 已在会议软件中选择虚拟麦克风，例如 `BlackHole 2ch`。

StealthCopilot 不注册自研虚拟摄像头驱动；会议软件里的摄像头应选择 `OBS Virtual Camera`。

## OBS 设置步骤

1. 打开 OBS App。
2. 在「源」区域删除或隐藏屏幕采集源。
3. 点击 `+`，选择「浏览器」。
4. URL 填写：

```text
http://127.0.0.1:18765/
```

5. 宽度和高度建议先填 `512 x 512`。
6. 确定后右键画布中的浏览器源，选择「变换」->「适配屏幕」。
7. 点击 OBS 右下角「启动虚拟摄像机」。
8. 在飞书、Zoom 或 Teams 中选择 `OBS Virtual Camera`。

注意不要把浏览器源 URL 填成 `/stream.mjpg`。OBS 中应使用根路径 `/`，由页面内部加载实时视频流。

## 使用顺序

1. 启动 StealthCopilot。
2. 启动说话链。
3. 打开 OBS，确认浏览器源出现数字人画面。
4. 启动 OBS Virtual Camera。
5. 打开会议软件并选择 `OBS Virtual Camera`。

如果说话链未启动，本地浏览器源可能没有视频帧，OBS 会显示黑屏。

## 常见问题

### 飞书选不到 OBS Virtual Camera

先确认 OBS 右下角已经点击「启动虚拟摄像机」。如果 OBS 弹出“找不到虚拟摄像头系统进程”，通常是 macOS 系统扩展未完成注册或旧扩展等待卸载：

1. 退出 OBS 和会议软件。
2. 重启 Mac。
3. 打开「系统设置」->「隐私与安全性」，允许 OBS / OBS Project 的系统扩展。
4. 再次打开 OBS 并启动虚拟摄像机。

### OBS 黑屏

先确认浏览器源 URL 是：

```text
http://127.0.0.1:18765/
```

然后确认说话链已经启动。可以在诊断日志中查找：

```text
obs browser source ready url=http://127.0.0.1:18765/
simli video: ffmpeg decoder started
simli video: frame written
```

如果没有 `frame written`，说明 Simli 视频帧还没有成功解码输出。

### 画面闪烁或块状花屏

这通常发生在 Simli VP8 视频解码或 OBS 浏览器源负载较高时。当前实现会使用 ffmpeg 解码 Simli WebRTC 视频，并异步编码为 OBS 可读的 MJPEG 流。排查顺序：

1. 重启说话链，让 Simli 会话重新建立。
2. 在 OBS 的浏览器源设置中点击「刷新当前页面」或「刷新浏览器缓存」。
3. 降低 OBS 画布分辨率，先使用 `512 x 512` 验证稳定性。
4. 查看诊断日志中的 `simli video: ffmpeg stderr=` 或 `simli video: write vp8 rtp udp err=`。

### 声音和口型不同步

Simli 只生成视频，会议听到的声音仍来自本地 TTS 写入虚拟麦克风。为补偿 Simli、ffmpeg、OBS 和会议软件带来的视频延迟，应用会在 Simli 模式下默认延迟约 `700ms` 写入本地虚拟麦克风。如果现场仍不同步，可根据日志里的 TTS 首帧时间和 `simli event=SPEAK` 时间继续调整该延迟。

### 说话链提示“同传已识别到语音，但没有返回目标语言译文”

这表示讯飞同传返回了源语言识别文本，但没有返回目标语言译文。应用会尝试走讯飞机器翻译文本接口兜底；如果兜底也失败，需要检查说话链源语言/目标语言设置，以及当前讯飞应用是否开通了机器翻译权限。

## 运行时边界

- Simli 模式下，音频和视频不是同一个云端回传流；视频来自 Simli，音频来自本地 TTS。
- OBS App 必须运行，OBS Virtual Camera 必须手动启动。
- App 只提供本地浏览器源，不直接向系统注册摄像头设备。
- 数字人不可用时，可关闭数字人输出，只保留说话链虚拟麦克风音频。
