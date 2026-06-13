# 架构决策记录（ADR）

记录 StealthCopilot 各模块的关键设计取舍，补充 prd.md 中未展开的"为什么"。

---

## ADR-001 听力链：讯飞单次调用返回双路文本

**决策：** 使用讯飞实时语音翻译 API 一次 WebSocket 调用，同时获取 `src_text`（原文）和 `dst_text`（译文），两路并行分发。

**取舍：** 不拆成"ASR → 独立翻译"两步，避免增加一跳延迟和一组 API Key。讯飞翻译 API 本身已包含 ASR，单次调用即可。

**影响：** `dst_text` 直接推送字幕窗；`src_text` 送意图识别 → RAG，不用译文，保留原语言语义以提升检索准确率。

---

## ADR-002 听力链：意图识别防止 RAG 误触发

**决策：** `src_text` 在进入 RAG 前先过 DeepSeek 轻量分类（`question` / `followup` / `statement`），`statement` 直接丢弃，不触发检索和回答生成。

**取舍：** 增加一次 DeepSeek 调用延迟（约 100-200ms），但避免面试官解释/闲聊时持续触发 RAG 干扰提词窗。

**影响：** 只有 `is_end=true` 的完整句子才触发分类，中间结果忽略。

---

## ADR-003 RAG 检索用原文（src_text），不用译文

**决策：** 向量检索时用 `src_text`（面试官原始语言）而非 `dst_text`（中文译文）作为查询向量。

**取舍：** 简历通常为英文，技术术语在跨语言 embedding 中可能失准；multilingual-e5-large 对英英检索效果优于英译中再检索。

---

## ADR-004 说话链：去掉 DeepSeek 润色，默认关闭

**决策：** 说话链核心路径为：VAD → 讯飞语音翻译（中文语音 → 英文文本）→ ElevenLabs TTS。DeepSeek 润色步骤作为可选开关，默认 OFF。

**取舍：** 去掉润色可将说话链延迟控制在 ≤1.2s 预算内；润色带来的质量提升不值得牺牲实时性，面试官会察觉明显延迟。

---

## ADR-005 口型同步：Simli AI SaaS，不自建 GPU 集群

**决策：** 使用 Simli AI 官方 SaaS API（WebSocket 流式），驱动用户真实人脸，不自建 MuseTalk GPU 集群。

**取舍：** SaaS 有云端延迟（~200-400ms），通过 A/V 环形缓冲区补偿；自建集群成本和运维复杂度远超当前阶段。

**扩展性：** `LipSyncProvider` 接口预留 `StealthCloudProvider` 实现，Phase 3 可切换到自营云服务。

---

## ADR-006 虚拟摄像头：自捆绑驱动，不依赖 OBS

**决策：** 基于 AkVirtualCamera 自研捆绑虚拟摄像头驱动：macOS 用 CoreMediaIO DAL 插件，Windows 用 DirectShow Filter。App 首次启动时 Setup 向导一键安装，需一次 admin/UAC 授权。

**取舍：** 要求用户安装 OBS 体验重、依赖外部软件版本；自捆绑驱动体积增加约 5-10MB，但用户体验更完整。

---

## ADR-007 声音克隆：用用户自己的 ElevenLabs 账户

**决策：** 声音克隆在 Setup 向导中完成，用户录制约 15 秒语音，上传到用户自己的 ElevenLabs 账户，App 使用用户的 API Key + Voice ID。

**取舍：** 不托管用户账户，避免声纹数据隐私问题和账户管理成本；用户需自行注册 ElevenLabs（免费计划即可）。

---

## ADR-008 API Key 存储：go-keyring 系统级安全存储

**决策：** 所有第三方 API Key 通过 `go-keyring` 存储：macOS → Keychain，Windows → Credential Manager，提供统一接口。

**取舍：** 不存 `.env` 文件或本地明文配置，防止密钥随截图/同步软件泄露。

---

## ADR-009 商业模式：AGPL v3 + 双授权（Open Core）

**决策：** 客户端代码采用 AGPL v3 开源；商业用途（转售、SaaS 集成）需购买商业授权。所有外部贡献者必须签署 CLA（通过 CLA Assistant）。

**取舍：** AGPL 传染性阻止竞争对手直接 fork 商业化；CLA 允许后续切换许可证或双授权不受贡献者限制。

---

## ADR-010 前端：Vue 3，不用 React

**决策：** 前端选 Vue 3 + TypeScript + Tailwind CSS，不用 React。

**取舍：** Vue 3 DevTools 和单文件组件调试体验更直观；Wails 对 Vue 的 HMR 支持更成熟。

---

## ADR-011 前后端职责：枚举/Prompt/转换逻辑放后端

**决策：** 业务枚举映射、Prompt 模板、数据格式转换一律放 Go 后端，通过 Wails binding 暴露给前端。前端只做展示和交互。

**取舍：** 保持前端逻辑简单，便于后续多端复用；后端逻辑可独立测试，不依赖浏览器环境。
