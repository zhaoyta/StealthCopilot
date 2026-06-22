# StealthCopilot — 产品需求文档（最终版）

> 最后更新：2026-06-13
> 状态：已确认，可作为所有 OpenSpec change 的需求来源

---

## 第一部分：产品定位

**StealthCopilot** 是一款面向跨境求职者的桌面面试辅助工具。用户在后台用母语作答，面试官听到的是流利的克隆语音、看到的是口型对齐的真人画面，同时有只有用户自己可见的幽灵提词窗实时显示翻译字幕与基于简历的回答建议。

### 核心用户故事

- **低配流畅：** 普通 Windows 办公本或旧款 Intel Mac 均可一键启动，不卡顿、不发热。
- **绝对隐形：** Zoom/Teams 强制全局屏幕共享时，提词窗对面试官完全不可见，且支持鼠标点击穿透。
- **完美化身：** 用户说母语，面试官看到的是口型与流利英文 100% 对齐的真人画面，听到的是用户自己声音的克隆英文。

### 商业模式

- **开源 + 云服务（Open Core）：** 客户端完全开源（AGPL v3），用户自带 API Key 免费使用。
- **双授权：** 企业商业使用（闭源二次分发、白标）需购买商业 License。AGPL 保证 fork 方必须开源修改，形成护城河。
- **自营云服务（后续）：** 提供口型同步等服务作为 Simli AI 的平替，打包为 StealthCopilot Cloud 订阅，降低用户上手门槛。

---

## 第二部分：三条核心管道

### 管道 1 — 听力链（目标延迟 ≤500ms）

```
面试官说话
  → BlackHole / VB-Cable 虚拟声卡截取音频
  → 讯飞实时语音翻译 API（单次 WebSocket 调用）
      ├─ dst_text（目标语言译文）──► 幽灵提词窗字幕区
      └─ src_text（源语言原文）──► 意图识别
                                    ├─ question（新问题）──► RAG 检索简历 → DeepSeek 生成回答 → 幽灵提词窗回答区
                                    ├─ followup（追问）──► 带多轮对话历史 + RAG → DeepSeek → 幽灵提词窗
                                    └─ statement（陈述/闲聊）──► 忽略，不触发 RAG
```

**关键设计：**
- 讯飞一次 WebSocket 调用同时返回 src_text 和 dst_text，两路并行处理，不串行等待
- 意图识别用 DeepSeek 做轻量分类（question / followup / statement），防止面试官解释/闲聊误触发 RAG
- RAG 使用 src_text（源语言原文）检索，不用译文，精度更高，且可与翻译并行

### 管道 2 — 说话链（目标延迟 ≤1.2s）

```
用户说母语（麦克风）
  → 本地 VAD 检测说话结束
  → 讯飞语音翻译 API（母语语音 → 目标语言文本，单次调用）
  → 默认音色或讯飞声音复刻流式 TTS（目标语言文本 → 默认/个人复刻音色音频）
  → 虚拟麦克风写入（BlackHole / VB-Cable）→ 面试官听到目标语言语音
```

**关键设计：**
- 等待 TTS 生成期间，Go 后端持续向虚拟麦克风写入 Zero-PCM 静音块，防止用户母语背景音泄漏给面试官
- DeepSeek 润色作为可选开关（设置里"高质量模式"），默认关闭以保证延迟
- 讯飞语音翻译 API 同时完成 ASR + 翻译，无需两次调用

### 管道 3 — 数字人视频输出（说话链可选输出模式）

```
说话链 TTS PCM
  → Simli AI WebRTC 视频
  → 本机 OBS Browser Source: http://127.0.0.1:18765/
  → OBS Virtual Camera → 面试官看到口型同步数字人画面
```

**关键设计：**
- Simli AI 调用其官方 SaaS API，不自建 GPU 集群
- Simli 仅生成视频，会议音频仍由本地 TTS 写入虚拟麦克风
- 本地音频默认延迟约 700ms 写入虚拟麦克风，以补偿 Simli/OBS/会议软件的视频链路延迟
- OBS App 负责系统虚拟摄像头输出，StealthCopilot 不注册自研摄像头驱动

---

## 第三部分：简历 RAG 系统

- 用户本地上传简历（PDF / DOCX），在本地做 embedding，**不上云**
- Embedding 模型：默认 `multilingual-e5-small`（支持多语言简历和轻量本地索引）
- 向量库：轻量本地实现（sqlite-vss 或内存级 hnswlib）
- RAG 触发条件：讯飞返回 `is_end=true` 的 src_text 且意图识别为 question / followup
- 多份简历可管理，支持切换激活

---

## 第四部分：幽灵提词窗

提词窗是独立的浮窗，对屏幕共享和截图完全不可见，支持鼠标点击穿透。

- **macOS（CGO）：** `[window setSharingType:NSWindowSharingNone]` + `[window setIgnoresMouseEvents:YES]`
- **Windows（Syscall）：** `SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE)` + `WS_EX_TRANSPARENT`

窗口内容分两区：
- **上区：** 面试官话语实时字幕（讯飞 dst_text，滚动显示）
- **下区：** AI 基于简历生成的回答建议（DeepSeek 流式逐字输出）

---

## 第五部分：熔断机制

远程面试中云端断流是致命的，系统必须在 10ms 内无感切回真实设备。

- 客户端与 Simli AI 之间维持 50ms 周期的 UDP 心跳
- 触发条件：连续 3 个心跳丢失，或视频流延迟超过 300ms
- 熔断行为：立即断开虚拟麦克风和虚拟摄像头，将真实麦克风和真实摄像头重新直连 Zoom/Teams
- 切换过程：无黑屏、无断音、无静止

---

## 第六部分：技术栈

| 层 | 技术 |
|---|---|
| 桌面框架 | Wails v2（Go + WebView2/WebKit），macOS + Windows 双平台 |
| 前端 | Vue 3 + TypeScript + Tailwind CSS |
| UI 国际化 | vue-i18n v9，locale JSON 按模块分层，禁止硬编码字符串 |
| 音频路由 | BlackHole（macOS）/ VB-Cable（Windows），引导用户安装 |
| 虚拟摄像头 | OBS Virtual Camera；App 提供本机 OBS Browser Source，不注册自研摄像头驱动 |
| 听力链 | 讯飞实时语音翻译 API（WebSocket，src_text + dst_text 双输出） |
| 说话链 STT | 讯飞语音翻译 API（VAD 触发，母语语音 → 目标语言文本） |
| 说话链 TTS | 默认音色 TTS + 讯飞声音复刻流式 TTS（个人复刻音色为可选增强） |
| 意图识别 | DeepSeek（轻量分类：question / followup / statement） |
| 回答生成 | DeepSeek-V3（流式输出，带多轮对话历史） |
| 简历 Embedding | multilingual-e5-small + 本地向量库 |
| 数字人视频 | Simli AI WebRTC 视频 + OBS Browser Source + OBS Virtual Camera |
| API Key 存储 | go-keyring（macOS Keychain / Windows Credential Manager） |
| CI/CD | GitHub Actions（macOS runner + Windows runner 分别原生编译，CGO 不交叉编译） |

---

## 第七部分：设置面板模块

| Tab | 内容 |
|---|---|
| API 凭证 | 讯飞 RTASR、讯飞机器翻译、讯飞声音复刻（AppID/APIKey/APISecret）、DeepSeek（Key/模型）、Simli AI（Key），各有连通性测试按钮；Task ID / Asset ID 由声音复刻流程自动保存，不手填 |
| 语言配置 | 听力链「源语言→目标语言」、说话链「输入语言→输出语言」，独立下拉，讯飞支持的语言对 |
| 设备绑定 | 虚拟声卡、物理麦克风、监听输出、OBS Virtual Camera、数字人 Provider 和浏览器源地址 |
| 简历管理 | 上传 PDF/DOCX，本地 embedding，多份可切换，当前激活标记 |
| 提词窗外观 | 字号、透明度、位置预设（左上/右上/自定义） |
| 高级（折叠） | RAG 回答生成 Prompt、说话链润色 Prompt，各有默认值 + 一键重置 |

---

## 第八部分：首次启动 Setup 向导（5步）

1. **欢迎**：产品介绍，约需 3 分钟完成配置
2. **依赖检测**：检测 BlackHole / FFmpeg / OBS Virtual Camera 支持，缺失时提供安装或官方页面指引
3. **核心 API Key**：填写讯飞 RTASR、讯飞机器翻译、讯飞声音复刻和 DeepSeek（必填），Simli 可稍后补充
4. **声音复刻录制**：可跳过并使用默认音色；如需个人复刻音色，则在 app 内按讯飞训练文本录音并提交训练，训练完成后自动保存 Asset ID
5. **完成**：进入主界面

---

## 第九部分：工程规范

### Provider 接口原则
所有外部服务（STT、TTS、Translation、LipSync）抽象为 Go interface，通过依赖注入在启动时实例化，禁止在业务逻辑中直接 new 具体实现，为供应商切换和自营云服务扩展留口。

### 文档同步（强制）
- 框架：VitePress
- 自动生成：godoc（Go）、TypeDoc（Vue）、git-cliff（CHANGELOG）
- GitHub Actions 硬卡关：PR 含 `feat:` 或 `fix:` 提交但未修改 `docs/` → CI fail，阻断合并
- 发版前 CHANGELOG 未更新 → release workflow fail

### 代码规范
- Conventional Commits（feat / fix / docs / chore）
- golangci-lint + ESLint（含 eslint-plugin-vue-i18n 检测硬编码字符串）
- husky pre-commit hooks + commitlint

### Makefile 目标
`dev` / `build` / `build-mac` / `build-win` / `commit` / `tag-patch` / `tag-minor` / `tag-major` / `release` / `docs` / `docs-build`

### 开源合规
- License：AGPL v3 + 商业双授权，源文件头部 SPDX 声明
- CLA Assistant：外部贡献者必须签署（双授权模型必须）
- 合规文件：CONTRIBUTING.md、CODE_OF_CONDUCT.md、SECURITY.md、THIRD_PARTY_LICENSES、Issue/PR 模板
- 代码签名：macOS（Apple Developer + Notarize）、Windows（代码签名证书）

---

## 第十部分：实施阶段

| 阶段 | 目标 |
|---|---|
| Phase 1 | Wails 骨架 + 幽灵 UI + 设置面板 + Setup 向导 + 听力链（字幕 + RAG 回答） |
| Phase 2 | 说话链（VAD + 讯飞翻译 + 讯飞声音复刻 TTS + 虚拟麦克风）+ 数字人视频输出（Simli AI + OBS Virtual Camera） |
| Phase 3 | SaaS 计费、自营云服务（StealthCloudProvider）、Homebrew/Scoop 分发、开启内测 |
