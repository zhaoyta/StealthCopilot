## Why

听力链是用户感知最直接的功能——面试官说话后，用户在提词窗里看到中文字幕和回答建议。这条链需要将音频实时转写、翻译、意图分类、RAG 检索和 LLM 生成串联起来，且总延迟控制在 500ms 以内。

## What Changes

- 讯飞实时语音翻译 WebSocket 接入（src_text + dst_text 双路输出）
- 意图识别工作流（DeepSeek 分类：question / followup / statement）
- RAG 管道（multilingual-e5 本地 embedding + 向量检索）
- DeepSeek 流式回答生成（带多轮对话历史）
- 字幕和回答通过 Wails EventEmit 推送到提词窗前端

## Capabilities

### New Capabilities

- `xunfei-translation`: 讯飞实时语音翻译 WebSocket 客户端，接收音频流，输出 src_text + dst_text
- `intent-recognition`: DeepSeek 意图分类，question / followup / statement 三分类
- `rag-pipeline`: 本地简历 embedding 检索，multilingual-e5-large，返回相关简历片段
- `answer-generation`: DeepSeek 流式回答生成，带多轮对话历史上下文

### Modified Capabilities

## Impact

- 新增 `internal/translation/xunfei.go`（讯飞 WebSocket 客户端）
- 新增 `internal/intent/classifier.go`（意图分类）
- 新增 `internal/rag/retriever.go`（向量检索）
- 新增 `internal/llm/answer.go`（回答生成）
- 依赖：讯飞实时语音翻译 API、DeepSeek API、multilingual-e5-large 模型、portaudio（音频捕获）
