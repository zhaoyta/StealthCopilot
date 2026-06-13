## Context

Setup 向导和设置面板是用户与 StealthCopilot 交互的第一个也是最频繁的界面。设计稿已由 Claude Design 完成（`~/Downloads/StealthCopilot/screen-setup.jsx` 和 `screen-settings.jsx`），实现时直接参照。

## Goals / Non-Goals

**Goals:**
- Setup 向导完成后用户即可启动所有管道
- API Key 存系统密钥链，不落明文文件
- 简历本地 embedding，不上云
- 设置面板所有改动即时生效（响应式，无需重启）

**Non-Goals:**
- 不实现管道业务逻辑（听力/说话/视频链）
- 不实现付费订阅或账号体系

## Decisions

### D1：API Key 存储用 go-keyring
统一接口跨平台：Mac → Keychain，Win → Credential Manager。调用方只需 `keyring.Set(service, key, val)` / `keyring.Get(service, key)`，无需感知平台差异。

### D2：Setup 向导只要求填讯飞 + DeepSeek（必填），其余可选
降低首次上手门槛。ElevenLabs（TTS）和 Simli AI（视频）可在设置面板后补，声音克隆步骤可跳过（跳过后说话链降级为系统 TTS）。

### D3：声音克隆在 App 内完成，不跳转浏览器
在 Setup 向导 Step 4 内录音，调用 ElevenLabs Voice Clone API（用用户自己的 API Key），获取 Voice ID 后存 Keychain。用户无需手动操作 ElevenLabs 网站。

### D4：简历 embedding 在上传时同步触发
用户上传简历后，Go 后端立即在后台线程跑 multilingual-e5-large embedding，完成后在 UI 显示"已就绪"。不做惰性 embedding（避免首次面试时触发延迟）。

### D5：设备列表动态枚举
Tab "设备绑定" 每次打开时调用 Go 后端枚举系统音视频设备，提供"重新枚举"按钮。不缓存设备列表（设备可能随时插拔）。

### D6：高级 Prompt 折叠展示，有默认值
避免普通用户误改。每个 Prompt 配"恢复默认"按钮，默认值硬编码在 Go 后端常量中。

## Risks / Trade-offs

- [multilingual-e5-large 模型大小] 模型约 500MB，首次使用需下载 → Mitigation：Setup 向导在依赖检测步骤一并下载，显示进度
- [go-keyring CGO] Windows 上 go-keyring 依赖 CGO → Mitigation：已在脚手架阶段配置好 CGO 编译环境
- [ElevenLabs Voice Clone API 延迟] 上传和训练约 10-30s → Mitigation：异步处理，UI 显示进度条，允许后台完成后再进入主界面
