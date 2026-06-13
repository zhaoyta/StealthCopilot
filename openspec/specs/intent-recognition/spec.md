## ADDED Requirements

### Requirement: 三分类意图识别
系统 SHALL 对讯飞返回 `is_end=true` 的 src_text 调用 DeepSeek 进行意图分类，返回 question（新问题）、followup（追问）、statement（陈述/闲聊）三种类型之一。

#### Scenario: 识别为新问题
- **WHEN** 面试官提出一个独立的新问题
- **THEN** 意图分类返回 question，触发 RAG 检索 + 回答生成

#### Scenario: 识别为追问
- **WHEN** 面试官基于上一个回答提出追问
- **THEN** 意图分类返回 followup，触发带对话历史的 RAG + 回答生成

#### Scenario: 识别为陈述不触发 RAG
- **WHEN** 面试官在解释背景信息或进行闲聊
- **THEN** 意图分类返回 statement，不触发 RAG，只显示字幕

#### Scenario: 仅对完整句子分类
- **WHEN** 讯飞返回 is_end=false 的中间结果
- **THEN** 不触发意图识别，等待 is_end=true 的完整句子
