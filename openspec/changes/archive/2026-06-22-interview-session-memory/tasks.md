## 1. 数据层：internal/session 包

- [x] 1.1 新建 `internal/session/model.go`：定义 `Session`（含预留 `Label string`）、`Turn`（含 `DisplayQuestion string`）结构体
- [x] 1.2 新建 `internal/session/store.go`：实现 `Store` 接口（Begin/End/AppendTurn/GetRecentTurns/ListSessions/GetTurns/Delete/CloseOrphanSessions/Close）及 SQLite 实现（sessions.db，WAL 模式）
- [x] 1.3 建表包含 `sessions.label` 和 `turns.display_question` 字段；`display_question` 优先写入 dst_text，缺失时回退 src_text
- [x] 1.4 新建 `internal/session/store_test.go`：覆盖 Begin/End、AppendTurn、GetRecentTurns、ListSessions、GetTurns、Delete、禁止删除进行中会话、孤儿会话自动关闭
- [x] 1.5 新建 `internal/session/service.go`：实现 `SessionService`（Wails binding 层：ListSessions/GetSessionTurns/DeleteSession）
- [x] 1.6 新建 `internal/session/service_test.go`：覆盖 service 层各 binding 的正常和错误路径

## 2. LLM 层：历史读取与写入

- [x] 2.1 `internal/llm/config.go`：`Config` 新增 `HistoryMaxTurns int` 字段，添加 `EffectiveHistoryMaxTurns()` 方法（0 时返回默认值 5）
- [x] 2.2 `internal/llm/answer.go`：`AnswerGenerator` 接受可选 `session.Store` 依赖；`Generate` 按 session ID 从 Store 加载最近 N 轮历史
- [x] 2.3 `internal/llm/answer.go`：`GenerateConfig` 新增 `DisplayQuestion string`；回答非空时异步调用 `store.AppendTurn(sessionID, question, displayQuestion, answer)`
- [x] 2.4 `internal/llm/answer.go`：历史注入不再依赖 `WithHistory`；只要同一 session 有历史，question/followup 均注入
- [x] 2.5 `internal/llm/answer.go`：`buildSystemPrompt` 支持 `{history}` 占位符；模板不含占位符且有历史时追加历史块
- [x] 2.6 `internal/llm/answer_test.go`：新增 store 历史注入、HistoryMaxTurns 默认值/自定义值、`{history}` 占位符、回答为空不写入 turns 的单元测试
- [x] 2.7 集成 Store 验证通过后删除 `AnswerGenerator.history map[string][]QAPair` 内存历史死代码

## 3. Hearing 层：会话生命周期接入

- [x] 3.1 `internal/hearing/chain.go`：`ChainConfig` 新增 `SessionStore session.Store`、`ResumeSessionID string` 字段
- [x] 3.2 `Start()` 默认生成新 UUID；若 `ResumeSessionID` 非空则复用该 ID（v1 前端不传）；调用 `store.Begin(sessionID, resumeID)` 创建/打开会话
- [x] 3.3 `Stop()` 调用 `store.End(sessionID)` 标记当前会话结束；Stop 不额外触发新的 RAG
- [x] 3.4 `startRAG` 调用 `AnswerGenerator.Generate` 时传入当前 sessionID 和 `DisplayQuestion`（优先 dstText，缺失时 srcText）
- [x] 3.5 保持现有意图识别和句子触发策略不变：仍由当前 `Classifier` 返回 question/followup/statement 决定是否触发 RAG
- [x] 3.6 hearing chain 单元/集成测试：验证 Start 创建 session、Stop 结束 session、RAG 完成后写入 turns、不同 session 历史隔离

## 4. App 层接入

- [x] 4.1 `app.go`：初始化 `session.Store`（sessions.db 与 vectors.db 同目录）；`Startup` 时调用 `store.CloseOrphanSessions(24h)`
- [x] 4.2 `app.go`：将 `session.Store` 注入 `hearing.ChainConfig` 和 `llm.AnswerGenerator`
- [x] 4.3 `app_bindings.go`：注册 `SessionService` 的 Wails bindings（ListSessions/GetSessionTurns/DeleteSession）
- [x] 4.4 `app_bindings.go`：保留 `StartHearingChain()` 前端签名不变；如需后端续接测试，新增内部 helper 或可选方法，前端 v1 不暴露继续入口

## 5. 设置面板：历史 Tab 与高级配置

- [x] 5.1 App config 新增 `HistoryMaxTurns int`（默认 5）；`LoadSettings` 兼容旧配置，字段缺失或 0 时使用默认值
- [x] 5.2 前端设置面板「高级」区域新增「历史轮数」数字输入（1-20），绑定 `HistoryMaxTurns`
- [x] 5.3 新建 `frontend/src/views/settings/TabHistory.vue`：会话列表（时间、关联简历名、问答轮数、状态）+ 删除按钮
- [x] 5.4 `TabHistory.vue`：点击会话展开问答详情，显示 `display_question` 和 answer；进行中会话禁用删除或展示后端错误
- [x] 5.5 `frontend/src/views/Settings.vue`：新增「历史会话」Tab，使用 `v-show` 挂载，`:is-active` prop 控制数据加载
- [x] 5.6 更新 `zh-CN.json` / `en-US.json`：新增历史 Tab、历史轮数相关 i18n key
- [x] 5.7 `TabHistory.vue` 单元测试（.spec.ts）：覆盖列表渲染、展开详情、删除确认、进行中会话禁删场景

## 6. 文档与验证

- [x] 6.1 更新 `README.md` 或 `docs/` 用户帮助文档：描述「历史会话」功能，标注历史只保存在本地
- [x] 6.2 更新 `openspec/specs/answer-generation/spec.md`：归档历史注入改为 session 级别、历史轮数可配置、sessions.db 来源
- [x] 6.3 更新 `openspec/specs/rag-pipeline/spec.md`：归档 session ID 传递和 `{history}` 占位符
- [x] 6.4 新建 `openspec/specs/interview-session/spec.md`：归档新 capability 完整 spec
- [x] 6.5 运行 Go 单元测试：`go test ./...`
- [x] 6.6 运行前端测试/类型检查（按项目现有脚本）
- [x] 6.7 运行 `openspec validate interview-session-memory --strict`
