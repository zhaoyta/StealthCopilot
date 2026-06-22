## ADDED Requirements

### Requirement: DeepSeek 流式回答生成
系统 SHALL 调用 DeepSeek SSE 流式接口生成面试回答，每收到 token chunk 立即通过 Wails EventEmit 推送到提词窗，前端逐字渲染。

#### Scenario: 流式逐字输出
- **WHEN** DeepSeek 开始返回回答
- **THEN** 提词窗回答区逐字追加显示，有打字光标，不等待完整回答

#### Scenario: 同一会话内所有轮次携带历史
- **WHEN** 同一 session 内已有至少 1 轮问答历史，且当前意图为 question 或 followup
- **THEN** DeepSeek 请求携带当前会话最近 N 轮（N 由 llm.Config.HistoryMaxTurns 配置，默认 5）Q&A 对话历史作为上下文；历史从 sessions.db 加载，保证重启后可续接

#### Scenario: 不同会话历史严格隔离
- **WHEN** 当前 session 切换为新 UUID（新面试开始）
- **THEN** GetRecentTurns 仅返回当前 sessionID 下的记录，前一场面试的所有 turns 对新会话不可见

#### Scenario: 回答生成完成
- **WHEN** DeepSeek 流式输出结束
- **THEN** 提词窗光标消失，回答内容保持显示直到下一个问题到来；本轮 Q&A 异步写入 sessions.db turns 表

### Requirement: 历史轮数上限可配置
系统 SHALL 通过 llm.Config.HistoryMaxTurns 字段控制注入 DeepSeek 的最大历史轮数，前端可在设置面板「高级」区域调整；旧配置中该字段为 0 时使用默认值 5。

#### Scenario: 使用自定义历史轮数
- **WHEN** 用户在高级设置中将历史轮数设为 8
- **THEN** followup 场景下 DeepSeek 最多携带 8 轮历史问答

#### Scenario: 历史轮数为 0 或未配置
- **WHEN** llm.Config.HistoryMaxTurns 为 0
- **THEN** 系统使用默认值 5，最多携带最近 5 轮历史问答
