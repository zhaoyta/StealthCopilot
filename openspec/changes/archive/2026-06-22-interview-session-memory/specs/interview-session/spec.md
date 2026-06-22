## ADDED Requirements

### Requirement: 面试会话生命周期管理
系统 SHALL 将每次面试（hearing chain 的一次 Start/Stop 周期）视为一个独立会话，自动创建并持久化会话记录到本地 SQLite（sessions.db）。

#### Scenario: 开始新会话
- **WHEN** 用户启动听力链（StartHearingChain）
- **THEN** 系统在 sessions 表中创建一条新记录，记录 id、started_at、关联的 resume_id

#### Scenario: 结束会话
- **WHEN** 用户停止听力链（StopHearingChain）
- **THEN** 系统更新当前 session 的 ended_at 字段为当前时间戳

#### Scenario: 孤儿会话自动关闭
- **WHEN** 应用启动时发现存在 ended_at 为 NULL 且 started_at 超过 24 小时的会话
- **THEN** 系统将该会话的 ended_at 设为 started_at + 24h，不再显示为"进行中"

### Requirement: 问答历史持久化
系统 SHALL 在每轮 RAG 回答生成完成后，将该轮问题和回答追加到 turns 表，关联当前 session_id。

#### Scenario: 保存一轮问答
- **WHEN** DeepSeek 流式回答完成（EventAnswerDone）
- **THEN** turns 表新增一条记录，字段包含 session_id、question（原始问题）、display_question（历史展示文本）、answer（英文回答建议）、created_at

#### Scenario: 回答为空时不保存
- **WHEN** DeepSeek 因网络错误或 ctx 取消未产生有效回答
- **THEN** turns 表不写入空记录

### Requirement: 历史会话查询
系统 SHALL 提供 Wails binding，供前端查询历史会话列表和单场会话的问答详情。

#### Scenario: 列出历史会话
- **WHEN** 前端调用 ListSessions(limit int)
- **THEN** 返回按 started_at 倒序排列的会话列表，每条包含 id、started_at、ended_at、关联简历名称、turns 数量

#### Scenario: 查询会话详情
- **WHEN** 前端调用 GetSessionTurns(sessionID string)
- **THEN** 返回该会话下所有问答对，按 created_at 升序排列，每条包含 display_question（展示问题）和 answer（英文回答建议）

### Requirement: 历史会话删除
系统 SHALL 支持按 session ID 删除会话及其所有关联 turns（级联删除）。

#### Scenario: 删除单场会话
- **WHEN** 用户在历史 Tab 点击删除并确认
- **THEN** sessions 表及对应 turns 表记录全部删除，前端列表实时更新

### Requirement: 续接历史会话
系统 SHALL 在后端预留传入已有 session ID 重新启动听力链的能力，使当前轮回答生成能读取该会话的历史问答；v1 前端不暴露「继续此次面试」入口。

#### Scenario: 续接上场面试
- **WHEN** 后端以已有 sessionID 启动 hearing chain
- **THEN** hearing chain 使用该 sessionID，AnswerGenerator 从 sessions.db 加载该 session 的近期 turns 作为历史上下文

#### Scenario: 默认启动新会话
- **WHEN** 用户从主界面点击启动听力链
- **THEN** 系统生成新的 sessionID，不自动续接上一场历史会话

#### Scenario: 禁止删除进行中会话
- **WHEN** 用户尝试删除 ended_at 为 NULL 的进行中会话
- **THEN** 后端拒绝删除并返回明确错误，避免后续写入 turns 失败
