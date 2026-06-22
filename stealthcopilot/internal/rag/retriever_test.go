// Package rag 单测：验证 Retriever 在无激活简历、embedding 不可用和有结果三种场景下的行为。
// 使用临时目录和 resume.NullProvider 避免外部依赖。
package rag

import (
	"strings"
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

func TestRetriever_BroadQuestionUsesFullResumeEvenWhenEmbeddingUnavailable(t *testing.T) {
	mgr := newTestManager(t)
	r1, err := mgr.Upload("profile.pdf", []byte(`%PDF-1.4
BT
(Senior backend engineer) Tj
(High performance payment systems and architecture) Tj
(Led Kafka and Go migration projects) Tj
ET
%%EOF`), nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.RetrieveWithContext("Tell me about yourself", "介绍一下你自己", nil)

	if !result.HasActiveResume {
		t.Fatal("HasActiveResume should be true")
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want full resume context chunk: %v", len(result.Chunks), result.Chunks)
	}
	if !strings.Contains(result.Chunks[0], "High performance payment systems") {
		t.Fatalf("full resume context missing expected content: %q", result.Chunks[0])
	}
}

func TestRetriever_ProjectSpecificQuestionUsesMatchingProjectSection(t *testing.T) {
	mgr := newTestManager(t)
	r1, err := mgr.Upload("projects.pdf", []byte(`%PDF-1.4
BT
(Project Experience) Tj
(Payment Platform) Tj
(Built high performance payment APIs and transaction architecture) Tj
(Recommendation System) Tj
(Designed ranking models and feed personalization) Tj
ET
%%EOF`), nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.RetrieveWithContext("How did you design the payment project?", "支付项目怎么设计", nil)

	if !result.HasActiveResume {
		t.Fatal("HasActiveResume should be true")
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want one matching project chunk: %v", len(result.Chunks), result.Chunks)
	}
	if !strings.Contains(result.Chunks[0], "Payment Platform") {
		t.Fatalf("project context missing payment section: %q", result.Chunks[0])
	}
	if strings.Contains(result.Chunks[0], "Recommendation System") {
		t.Fatalf("project context should not include unrelated project section: %q", result.Chunks[0])
	}
}

func TestRetriever_ProjectFollowupUsesHistoryToResolveProjectSection(t *testing.T) {
	mgr := newTestManager(t)
	r1, err := mgr.Upload("projects.pdf", []byte(`%PDF-1.4
BT
(Project Experience) Tj
(Payment Platform) Tj
(Built high performance payment APIs and transaction architecture) Tj
(Recommendation System) Tj
(Designed ranking models and feed personalization) Tj
ET
%%EOF`), nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.RetrieveWithContext(
		"What did you do in this project?",
		"你在这个项目里做了什么",
		[]string{"Can you talk about the Payment Platform project?"},
	)

	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want one matching project chunk: %v", len(result.Chunks), result.Chunks)
	}
	if !strings.Contains(result.Chunks[0], "Payment Platform") {
		t.Fatalf("project follow-up should resolve to payment section: %q", result.Chunks[0])
	}
	if strings.Contains(result.Chunks[0], "Recommendation System") {
		t.Fatalf("project follow-up should not include unrelated project section: %q", result.Chunks[0])
	}
}

func TestRetriever_ProjectFollowupUsesPreviousAnswerToResolveProjectSection(t *testing.T) {
	mgr := newTestManager(t)
	r1, err := mgr.Upload("projects.pdf", []byte(`%PDF-1.4
BT
(Project Experience) Tj
(Payment Platform) Tj
(Built high performance payment APIs and transaction architecture) Tj
(Recommendation System) Tj
(Designed ranking models and feed personalization) Tj
ET
%%EOF`), nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.RetrieveWithContext(
		"What did you do in this project?",
		"你在这个项目里做了什么",
		[]string{"I would talk about the Payment Platform because it best demonstrates architecture depth."},
	)

	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want one matching project chunk: %v", len(result.Chunks), result.Chunks)
	}
	if !strings.Contains(result.Chunks[0], "Payment Platform") {
		t.Fatalf("project follow-up should resolve from previous answer: %q", result.Chunks[0])
	}
}

func TestRetriever_GlobalQuestionIgnoresProjectHistoryForScope(t *testing.T) {
	mgr := newTestManager(t)
	r1, err := mgr.Upload("projects.pdf", []byte(`%PDF-1.4
BT
(Summary) Tj
(Senior backend engineer) Tj
(Project Experience) Tj
(Payment Platform) Tj
(Built payment APIs) Tj
(Recommendation System) Tj
(Designed ranking models) Tj
ET
%%EOF`), nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.RetrieveWithContext(
		"Tell me about yourself",
		"介绍一下你自己",
		[]string{"Can you talk about the Payment Platform project?"},
	)

	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want full resume context: %v", len(result.Chunks), result.Chunks)
	}
	if !strings.Contains(result.Chunks[0], "Senior backend engineer") || !strings.Contains(result.Chunks[0], "Recommendation System") {
		t.Fatalf("global question should use full resume despite project history: %q", result.Chunks[0])
	}
}

func TestRetriever_ProjectOverviewQuestionUsesFullResume(t *testing.T) {
	mgr := newTestManager(t)
	r1, err := mgr.Upload("projects.pdf", []byte(`%PDF-1.4
BT
(Project Experience) Tj
(Payment Platform) Tj
(Built payment APIs) Tj
(Recommendation System) Tj
(Designed ranking models) Tj
ET
%%EOF`), nil)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if err := mgr.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive error: %v", err)
	}

	ret := NewRetriever(mgr)
	result := ret.RetrieveWithContext("Tell me about your project experience", "介绍一下你的项目经验", nil)

	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want full resume context: %v", len(result.Chunks), result.Chunks)
	}
	if !strings.Contains(result.Chunks[0], "Payment Platform") || !strings.Contains(result.Chunks[0], "Recommendation System") {
		t.Fatalf("full resume context should include multiple projects: %q", result.Chunks[0])
	}
}

func TestNeedsFullResumeContext(t *testing.T) {
	tests := []string{
		"Tell me about yourself",
		"Walk me through your resume",
		"您是否拥有高性能系统设计和体系结构",
		"介绍一下你的项目经验",
	}
	for _, text := range tests {
		if !needsFullResumeContext(text) {
			t.Fatalf("needsFullResumeContext(%q) = false, want true", text)
		}
	}
}

func TestNeedsProjectContext(t *testing.T) {
	if !needsProjectContext("What was your role in the payment project?") {
		t.Fatal("payment project should need project context")
	}
	if needsProjectContext("介绍一下你的项目经验") {
		t.Fatal("project overview should use full resume context, not one project section")
	}
}

func TestRetrievalQueries_ByResumeLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language resume.ResumeLanguage
		src      string
		dst      string
		want     []string
	}{
		{
			name:     "english resume prefers source text",
			language: resume.ResumeLanguageEN,
			src:      "Tell me about your backend experience",
			dst:      "介绍一下你的后端经验",
			want:     []string{"Tell me about your backend experience", "介绍一下你的后端经验"},
		},
		{
			name:     "chinese resume prefers translated text",
			language: resume.ResumeLanguageZH,
			src:      "Tell me about your backend experience",
			dst:      "介绍一下你的后端经验",
			want:     []string{"介绍一下你的后端经验", "Tell me about your backend experience"},
		},
		{
			name:     "other language uses both available texts",
			language: resume.ResumeLanguageJA,
			src:      "Tell me about your backend experience",
			dst:      "介绍一下你的后端经验",
			want:     []string{"Tell me about your backend experience", "介绍一下你的后端经验"},
		},
		{
			name:     "deduplicates same source and translation",
			language: resume.ResumeLanguageMixed,
			src:      "项目经验",
			dst:      "项目经验",
			want:     []string{"项目经验"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retrievalQueries(tt.language, tt.src, tt.dst)
			if len(got) != len(tt.want) {
				t.Fatalf("len(retrievalQueries) = %d, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("retrievalQueries[%d] = %q, want %q; all=%v", i, got[i], tt.want[i], got)
				}
			}
		})
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
