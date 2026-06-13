## 1. 音频捕获

- [ ] 1.1 在 `internal/audio/capture.go` 实现虚拟声卡音频捕获，使用 portaudio 按设备名绑定 BlackHole/VB-Cable
- [ ] 1.2 Wails 暴露 `StartHearingChain` / `StopHearingChain` binding

## 2. 讯飞实时语音翻译

- [ ] 2.1 在 `internal/translation/xunfei.go` 实现 `XunfeiTranslationProvider`（实现 TranslationProvider 接口）
- [ ] 2.2 实现讯飞 WebSocket 鉴权（HMAC-SHA256 签名 URL）
- [ ] 2.3 实现音频帧分片发送（40ms/帧，PCM 16kHz 16bit）
- [ ] 2.4 解析 WebSocket 响应，提取 src_text 和 dst_text，通过 channel 分发
- [ ] 2.5 实现断连指数退避重连（最多 3 次），失败后 EventEmit 通知前端

## 3. 意图识别

- [ ] 3.1 在 `internal/intent/classifier.go` 实现 `Classify(srcText string) IntentType`
- [ ] 3.2 使用 DeepSeek API，System Prompt 固定为分类指令，解析 JSON 响应
- [ ] 3.3 仅对 is_end=true 的完整句子触发分类，中间结果忽略
- [ ] 3.4 分类结果路由：question/followup → RAG，statement → 忽略

## 4. RAG 管道

- [ ] 4.1 在 `internal/rag/embedder.go` 封装 multilingual-e5-large embedding 调用（本地推理）
- [ ] 4.2 在 `internal/rag/retriever.go` 实现向量相似度检索（top-3，余弦相似度）
- [ ] 4.3 实现查询 embedding 生成 + 检索，返回相关简历片段列表
- [ ] 4.4 无激活简历时返回空列表，触发降级提示

## 5. DeepSeek 回答生成

- [ ] 5.1 在 `internal/llm/answer.go` 实现流式回答生成，调用 DeepSeek SSE 接口
- [ ] 5.2 构建 System Prompt（RAG 上下文 + 简历片段 + 对话历史）
- [ ] 5.3 维护对话历史（内存，最近 3 轮 Q&A，key 为 session ID）
- [ ] 5.4 每收到 token chunk 通过 Wails EventEmit 推送到前端，事件名 `answer:token`
- [ ] 5.5 流结束时发送 `answer:done` 事件
