## Context

听力链是延迟最敏感的管道，目标 ≤500ms。讯飞 WebSocket 已包含 ASR + 翻译，src_text 和 dst_text 并行分发到两条处理路径。意图识别是轻量分类，不应成为瓶颈。RAG + LLM 回答生成在意图确认为 question/followup 后异步触发，不阻塞字幕显示。

## Goals / Non-Goals

**Goals:**
- dst_text 字幕延迟 ≤500ms
- RAG + 回答生成不阻塞字幕路径
- followup 类型问题携带最近 3 轮对话历史

**Non-Goals:**
- 不实现多说话人区分
- 不实现本地 embedding 模型训练（使用预训练 multilingual-e5-large）

## Decisions

### D1：讯飞 WebSocket 长连接，断连自动重连
维护一个 Go goroutine 保持 WebSocket 连接，指数退避重连，最大 3 次后通知前端"连接中断"。

### D2：src_text 和 dst_text 并行处理
讯飞返回消息后，Go 后端同时：
1. 通过 Wails EventEmit 推送 dst_text 到提词窗字幕区
2. 将 src_text 异步发给意图识别 goroutine（不阻塞）

### D3：意图识别用 DeepSeek 极简 Prompt
单次 API 调用，System Prompt 固定为分类指令，User 内容为 src_text，响应期望为 JSON `{"intent": "question"|"followup"|"statement"}`。使用讯飞 `is_end=true` 标志触发（句子完整才分类，不对片段分类）。

### D4：RAG 检索 top-3 片段，拼接为 Context
从向量库检索余弦相似度最高的 3 个简历片段，拼接为 DeepSeek 的 Context。片段长度控制在 200 token 以内，避免超出 context window。

### D5：对话历史存内存，保留最近 3 轮
`followup` 类型时，将最近 3 轮的 Q&A 对追加到 DeepSeek System Prompt 前。历史只在内存中，重启清空。

### D6：DeepSeek 流式输出 token 推送前端
使用 DeepSeek SSE 流式接口，每收到 token chunk 即通过 Wails EventEmit 推送到提词窗回答区，前端逐字追加渲染。

## Risks / Trade-offs

- [讯飞 is_end 延迟] 讯飞 is_end=true 可能比实际句子结束晚 200-300ms → 意图识别触发稍晚，但字幕已先行显示，用户体验可接受
- [DeepSeek 意图分类延迟] 约 300-500ms，与字幕并行不影响字幕 → 回答建议比字幕晚 0.5-1s 出现，合理
- [向量库冷启动] 首次检索时模型加载约 2-3s → 在 Setup 向导中预热模型，面试开始前已加载完毕
