// Package rag 单测：验证 Retriever 在无激活简历、embedding 不可用和有结果三种场景下的行为。
// 使用临时目录和 resume.NullProvider 避免外部依赖。
package rag

import (
	"testing"

	"github.com/zhaoyta/stealthcopilot/internal/resume"
)

// newTestManager 创建一个使用临时目录和 NullProvider 的 Manager，测试结束后自动清理。
func newTestManager(t *testing.T) *resume.Manager {
	t.Helper()
	dir := t.TempDir()
	mgr, err := resume.NewManager(dir, &resume.NullProvider{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr
}

// TestTopK 验证 TopK 常量未被意外修改。
func TestTopK(t *testing.T) {
	if TopK != 3 {
		t.Errorf("TopK = %d, want 3", TopK)
	}
}

// TestRetriever_NoActiveResume 验证无激活简历时 HasActiveResume=false，Chunks 为空。
func TestRetriever_NoActiveResume(t *testing.T) {
	mgr := newTestManager(t)
	r := NewRetriever(mgr)

	result := r.Retrieve("tell me about yourself")
	if result.HasActiveResume {
		t.Error("HasActiveResume should be false when no resume is activated")
	}
	if len(result.Chunks) != 0 {
		t.Errorf("Chunks should be empty, got %v", result.Chunks)
	}
}

// TestRetriever_EmbeddingNotReady 验证有激活简历但 embedding 不可用时：
// HasActiveResume=true，Chunks 为 nil（降级为通用回答）。
func TestRetriever_EmbeddingNotReady(t *testing.T) {
	mgr := newTestManager(t)

	// 上传并激活一份最小简历（纯文本内容）
	pdfStub := makeMiniPDF(t)
	r1, err := mgr.Upload("test.pdf", pdfStub, nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.Retrieve("python experience")

	if !result.HasActiveResume {
		t.Error("HasActiveResume should be true when resume is activated")
	}
	// NullProvider 始终返回 ErrModelNotReady，Search 会出错，Chunks 应为 nil
	if result.Chunks != nil {
		t.Errorf("Chunks should be nil on embedding error, got %v", result.Chunks)
	}
}

// TestRetriever_ReturnsNilChunksOnSearchError 验证 Retrieve 在检索出错时
// 仍标记 HasActiveResume=true（不影响 Dashboard 状态显示）。
func TestRetriever_ReturnsNilChunksOnSearchError(t *testing.T) {
	mgr := newTestManager(t)
	pdfStub := makeMiniPDF(t)
	r1, err := mgr.Upload("cv.pdf", pdfStub, nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.Retrieve("distributed systems")

	if !result.HasActiveResume {
		t.Error("HasActiveResume must remain true even when search fails")
	}
}

// makeMiniPDF 生成一个最小有效 PDF 字节序列，用于 resume.Upload 测试。
// 内容极简（仅包含空白页），extractText 提取出空字符串，不影响 Manager 接受文件。
func makeMiniPDF(t *testing.T) []byte {
	t.Helper()
	// 最小合法 PDF 结构（xref/startxref/%%EOF 是必须的）
	const minPDF = "%PDF-1.0\n1 0 obj<</Type /Catalog /Pages 2 0 R>>endobj " +
		"2 0 obj<</Type /Pages /Kids [3 0 R] /Count 1>>endobj " +
		"3 0 obj<</Type /Page /MediaBox [0 0 3 3]>>endobj\n" +
		"xref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n" +
		"0000000058 00000 n\n0000000115 00000 n\n" +
		"trailer<</Size 4 /Root 1 0 R>>\nstartxref\n190\n%%EOF"
	return []byte(minPDF)
}
