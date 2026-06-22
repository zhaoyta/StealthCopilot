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

## ADR-003 RAG 根据简历语言选择检索查询

**决策：** 上传简历时由用户标记简历主要语言。英文简历优先使用 `src_text`（面试官原始语言）检索，中文简历优先使用 `dst_text`（译文）检索，其他语言或多语言简历同时使用 `src_text` 与 `dst_text` 检索并合并 topK。

**取舍：** 个人简历可能是中文、英文或其他语言，固定使用原文会让中文简历命中率下降；固定使用译文又会损失英文简历的英英检索优势。用户标记 + 双查询兜底能覆盖更多真实简历形态，代价是多语言简历会多做一次 query embedding。

---

## ADR-004 说话链：去掉 DeepSeek 润色，默认关闭

**决策：** 说话链核心路径为：VAD → 讯飞语音翻译（中文语音 → 英文文本）→ 讯飞声音复刻 TTS。DeepSeek 润色步骤作为可选开关，默认 OFF。

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

## ADR-007 声音复刻：使用讯飞声音复刻

**决策：** 声音复刻是可选增强。用户可以跳过复刻并使用默认音色完成说话链；如需个人复刻音色，用户按讯飞训练文本录音，App 使用用户配置的讯飞声音复刻 AppID、API Key、API Secret 创建训练任务；训练完成后保存 Asset ID，并在说话链中使用该 Asset ID 做流式 TTS。

**取舍：** 与听力链和机器翻译共用讯飞生态，减少用户需要注册和维护的供应商数量；声纹录音仍只提交给用户自己配置的第三方服务，App 不托管训练账户或音频资产。

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
