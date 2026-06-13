package rag

import (
	"github.com/zhaoyta/stealthcopilot/internal/resume"
)

// TopK 是 RAG 检索返回的最大简历片段数。
const TopK = 3

// Retriever 在当前激活简历的向量库中检索与查询最相关的段落。
// 底层调用 resume.Manager 的 Search() 方法，复用已有向量检索能力。
type Retriever struct {
	manager *resume.Manager
}

// NewRetriever 创建 Retriever。manager 通过 resume.Service.InternalManager() 获取。
func NewRetriever(manager *resume.Manager) *Retriever {
	return &Retriever{manager: manager}
}

// RetrieveResult 是 RAG 检索的返回结构。
type RetrieveResult struct {
	// Chunks 是相关简历片段列表（按相似度降序，最多 TopK 条）。
	Chunks []string
	// HasActiveResume 为 false 表示无激活简历，回答生成应降级为通用回答。
	HasActiveResume bool
}

// Retrieve 对 queryText 生成 embedding，在激活简历向量库中检索 TopK 相关段落。
//   - 无激活简历：返回 HasActiveResume=false，Chunks 为空
//   - embedding 不可用或检索出错：返回空 Chunks，调用方降级处理
func (r *Retriever) Retrieve(queryText string) RetrieveResult {
	if !r.manager.HasActiveResume() {
		return RetrieveResult{HasActiveResume: false}
	}

	results, err := r.manager.Search(queryText, TopK)
	if err != nil {
		// embedding 未就绪（ErrModelNotReady）或其他错误：降级为无 context 回答
		return RetrieveResult{HasActiveResume: true, Chunks: nil}
	}

	chunks := make([]string, 0, len(results))
	for _, res := range results {
		chunks = append(chunks, res.ChunkText)
	}
	return RetrieveResult{HasActiveResume: true, Chunks: chunks}
}
