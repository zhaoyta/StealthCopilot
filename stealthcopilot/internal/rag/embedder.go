// Package rag 实现 RAG 检索管道，为 DeepSeek 回答生成提供简历上下文。
// 底层向量检索由 internal/resume 包的 Manager.Search() 提供。
package rag

// Embedder 是 embedding 功能的抽象接口，与 resume.EmbeddingProvider 结构一致。
// rag 包通过此接口访问 embedding，不直接耦合 resume 具体实现。
type Embedder interface {
	// Embed 对查询文本生成归一化 embedding 向量（multilingual-e5-large，1024 维）。
	Embed(text string) ([]float32, error)
	// Ready 检查 embedding 服务是否就绪（Python 环境 + 模型文件已安装）。
	Ready() bool
}
