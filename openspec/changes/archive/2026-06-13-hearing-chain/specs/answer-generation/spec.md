## ADDED Requirements

### Requirement: DeepSeek 流式回答生成
系统 SHALL 调用 DeepSeek SSE 流式接口生成面试回答，每收到 token chunk 立即通过 Wails EventEmit 推送到提词窗，前端逐字渲染。

#### Scenario: 流式逐字输出
- **WHEN** DeepSeek 开始返回回答
- **THEN** 提词窗回答区逐字追加显示，有打字光标，不等待完整回答

#### Scenario: followup 携带对话历史
- **WHEN** 意图识别为 followup
- **THEN** DeepSeek 请求携带最近 3 轮 Q&A 对话历史作为上下文

#### Scenario: 回答生成完成
- **WHEN** DeepSeek 流式输出结束
- **THEN** 提词窗光标消失，回答内容保持显示直到下一个问题到来
