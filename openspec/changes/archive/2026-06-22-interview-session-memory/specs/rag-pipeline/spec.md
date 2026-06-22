## MODIFIED Requirements

### Requirement: 本地向量检索
系统 SHALL 默认使用 multilingual-e5-small 对查询文本生成 embedding，在本地向量库中检索余弦相似度最高的 3 个简历片段，作为 DeepSeek 回答生成的 Context。

#### Scenario: 检索相关简历片段
- **WHEN** 意图识别为 question 或 followup
- **THEN** 在 500ms 内返回 top-3 相关简历片段

#### Scenario: 无激活简历时跳过 RAG
- **WHEN** 用户未激活任何简历
- **THEN** RAG 步骤跳过，DeepSeek 不携带简历 Context 直接生成通用回答，提词窗显示提示"未激活简历，回答仅供参考"

#### Scenario: RAG 触发时携带 session ID
- **WHEN** RAG 检索触发并调用 AnswerGenerator.Generate
- **THEN** Generate 调用时传入当前 hearing session ID，供 AnswerGenerator 从 sessions.db 加载该 session 历史

## ADDED Requirements

### Requirement: Prompt 模板支持历史占位符
系统 SHALL 在 RAG Prompt 模板中支持可选的 `{history}` 占位符，若模板包含该占位符则注入格式化后的历史问答；若不包含则追加在 Prompt 末尾（保持向下兼容）。

#### Scenario: 模板含 {history} 占位符
- **WHEN** 用户自定义 Prompt 模板中包含 `{history}`
- **THEN** 系统将近期对话历史格式化后替换到对应位置

#### Scenario: 模板不含 {history} 占位符
- **WHEN** 用户未在模板中添加 `{history}` 占位符
- **THEN** 对话历史以固定格式（「对话历史（最近若干轮）：Q:…A:…」）追加在 Prompt 末尾，已有模板行为不变
