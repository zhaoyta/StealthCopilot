# StealthCopilot

面向跨境求职者的桌面面试辅助工具。用户在后台用母语作答，面试官听到的是流利的克隆英文语音、看到的是口型对齐的真人画面，同时有只有用户可见的幽灵提词窗实时显示翻译字幕与基于简历的回答建议。

> 客户端完全开源（AGPL v3）· 自带 API Key 零成本运行 · macOS + Windows 双平台

---

## 核心功能

| 功能 | 说明 |
|---|---|
| 幽灵提词窗 | 对 Zoom/Teams 屏幕共享完全不可见，鼠标点击穿透 |
| 实时听译字幕 | 面试官语音 → 中文字幕，延迟 ≤500ms |
| 回答建议 | RAG 检索本地简历 + DeepSeek 生成架构级回答 |
| 语音克隆 | 用户说中文，面试官听到克隆英文语音，延迟 ≤1.2s |
| 口型同步 | Simli AI 驱动口型与英文音频 100% 对齐 |
| 数据隐私 | 简历 embedding 本地处理，从不上传云端 |

---

## 技术架构

```
听力链  面试官音频 → 讯飞实时语音翻译 → 字幕 + RAG + DeepSeek → 提词窗
说话链  用户中文语音 → 讯飞 ASR → DeepSeek → ElevenLabs TTS → 虚拟麦克风
视频链  摄像头 → Simli AI 口型同步 → 虚拟摄像头
```

**技术栈：** Wails (Go + Vue 3) · 讯飞实时语音翻译 · DeepSeek-V3 · ElevenLabs · Simli AI · multilingual-e5-large

---

## 快速开始

### 环境依赖

- Go 1.21+
- Node.js 18+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- macOS: [BlackHole](https://github.com/ExistentialAudio/BlackHole) 虚拟声卡
- Windows: [VB-Cable](https://vb-audio.com/Cable/) 虚拟声卡

### 开发模式

```bash
make dev
```

### 生产构建

```bash
make build        # 当前平台
make build-mac    # macOS arm64
make build-win    # Windows amd64
```

### 代码检查 & 测试

```bash
make lint
make test
```

### 提交代码

```bash
git add <文件>
make commit   # 交互式 Conventional Commits
make push     # 提交 + 推送一步完成
```

---

## API Key 配置

首次启动会弹出设置向导，需要准备以下 API Key：

| 服务 | 用途 | 获取地址 |
|---|---|---|
| 讯飞开放平台 | 实时语音翻译 / ASR | [xfyun.cn](https://www.xfyun.cn) |
| DeepSeek | 意图识别 / 回答生成 | [platform.deepseek.com](https://platform.deepseek.com) |
| ElevenLabs | 声音克隆 TTS | [elevenlabs.io](https://elevenlabs.io) |
| Simli AI | 实时口型同步 | [simli.com](https://simli.com) |

所有 Key 通过系统 Keychain（macOS）/ Credential Manager（Windows）加密存储，不写入任何文件。

---

## 商业授权

本项目以 **AGPL-3.0** 协议开源。  
企业商业使用（闭源二次分发、白标集成）请联系：**zhaoyta@gmail.com**

---

## 贡献

欢迎提交 Issue 和 PR，请先阅读 [CONTRIBUTING.md](stealthcopilot/CONTRIBUTING.md)。
