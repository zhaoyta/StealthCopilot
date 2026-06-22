## Why

面试是一个有连续性的对话过程：面试官会追问、引申，面试者的回答也应该前后一致。当前每轮 RAG 回答建议完全无状态，LLM 不知道刚才说了什么，容易产生自相矛盾或重复的内容。通过引入面试会话记忆，系统可以将本次面试的问答历史纳入上下文，让回答建议更连贯、更有针对性。

## What Changes

- **新增**：面试会话（Session）的生命周期管理：开始、结束
- **新增**：会话内问答历史记录（Question + Answer 轮次）持久化存储到本地 SQLite
- **新增**：RAG 回答生成时将近期对话历史注入 LLM Prompt（滚动窗口，控制 token 用量）
- **新增**：设置面板「历史」Tab，查看和删除历史会话
- **新增**：历史轮数上限配置，控制注入 LLM 的最近问答轮数
- **修改**：hearing chain 在 RAG 触发时传入当前 Session ID
- **修改**：`AnswerGenerator` 从本地会话库读取近期历史，并在回答完成后写入本轮 Q&A

## Capabilities

### New Capabilities

- `interview-session`: 面试会话生命周期管理及问答历史的本地持久化存储

### Modified Capabilities

- `rag-pipeline`: RAG 回答生成时注入对话历史上下文，Prompt 模板新增 `{history}` 占位符
- `answer-generation`: `AnswerGenerator` 从 sessions.db 加载历史，同一 session 所有轮次均注入历史上下文
- `settings-panel`: 设置面板新增历史会话 Tab，并在高级配置中暴露历史轮数上限

## Impact

- `internal/session/`：新包，实现 Session 模型、SQLite 存储、Service
- `internal/llm/`：`AnswerGenerator` 接受 session store 依赖，按 session ID 读取/写入历史
- `internal/hearing/chain.go`：RAG 调用点传入 session 引用
- `app.go` / `app_bindings.go`：新增 Session 相关 Wails bindings
- 前端：新增「历史」Tab 及会话列表 UI
- 依赖：已有 `modernc.org/sqlite`（vectors.db 同款），无新外部依赖
