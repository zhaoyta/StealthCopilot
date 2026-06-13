## Why

用户需要在使用任何管道功能之前完成一次性初始化配置：安装系统驱动依赖、录入 API Key、克隆声音。同时需要一个完整的设置面板供后续调整所有参数。这两个模块是整个应用的入口，必须在业务管道之前就绪。

## What Changes

- 首次启动 Setup 向导（5步）：欢迎 → 依赖检测与安装 → 核心 API Key 录入 → 声音克隆录制 → 完成
- 设置面板（6 Tab）：API 凭证 / 语言配置 / 设备绑定 / 简历管理 / 提词窗外观 / 高级 Prompt
- API Key 安全存储：go-keyring 统一接口，macOS Keychain / Windows Credential Manager
- 简历本地管理：上传 PDF/DOCX，本地 embedding，多份可切换激活

## Capabilities

### New Capabilities

- `setup-wizard`: 5 步首次启动向导，含依赖检测、一键安装、API Key 录入、ElevenLabs 声音克隆录制上传
- `settings-panel`: 6 Tab 设置面板 Vue 组件，覆盖所有可配置项
- `keyring-storage`: go-keyring 封装，统一 API Key 读写接口，密钥存系统密钥链
- `resume-manager`: 简历上传、本地 embedding、多份管理、激活切换

### Modified Capabilities

## Impact

- 新增 Go 后端：`internal/config/keyring.go`（go-keyring 封装）、`internal/resume/`（embedding pipeline）
- 新增 Vue 组件：`src/views/SetupWizard.vue`、`src/views/Settings.vue` 及各 Tab 子组件
- 依赖：`github.com/zalando/go-keyring`、ElevenLabs Voice Clone API、multilingual-e5-large 模型
- 首次安装需 admin 权限（安装系统驱动）
