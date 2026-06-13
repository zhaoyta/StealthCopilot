## ADDED Requirements

### Requirement: 本地向量检索
系统 SHALL 使用 multilingual-e5-large 对 src_text 生成 query embedding，在本地向量库中检索余弦相似度最高的 3 个简历片段，作为 DeepSeek 回答生成的 Context。

#### Scenario: 检索相关简历片段
- **WHEN** 意图识别为 question 或 followup
- **THEN** 在 500ms 内返回 top-3 相关简历片段

#### Scenario: 无激活简历时跳过 RAG
- **WHEN** 用户未激活任何简历
- **THEN** RAG 步骤跳过，DeepSeek 不携带简历 Context 直接生成通用回答，提词窗显示提示"未激活简历，回答仅供参考"
