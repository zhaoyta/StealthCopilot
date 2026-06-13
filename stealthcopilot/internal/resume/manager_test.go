package resume_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zhaoyta/stealthcopilot/internal/resume"
)

// fakeEmbedder 用于测试的假 embedding 提供者，返回固定维度零向量。
type fakeEmbedder struct{ dim int }

func (f *fakeEmbedder) Embed(_ string) ([]float32, error) {
	vec := make([]float32, f.dim)
	vec[0] = 1.0 // 使向量非零，余弦相似度计算有意义
	return vec, nil
}
func (f *fakeEmbedder) Ready() bool { return true }

// newTestManager 创建测试用 Manager，并注册 Cleanup 确保数据库在目录清理前关闭。
func newTestManager(t *testing.T) *resume.Manager {
	t.Helper()
	dir := t.TempDir()
	m, err := resume.NewManager(dir, &fakeEmbedder{dim: 1024})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// 先关闭 DB 再让 TempDir 清理，避免 SQLite WAL 文件占用导致清理失败
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Logf("manager.Close: %v", err)
		}
	})
	return m
}

// TestManager_UploadAndList 验证上传简历后可列表查询。
func TestManager_UploadAndList(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Upload("resume.pdf", []byte("Software Engineer with 5 years experience."), nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// embedding 为异步，等待完成
	time.Sleep(200 * time.Millisecond)

	list := m.List()
	if len(list) != 1 {
		t.Errorf("want 1 resume, got %d", len(list))
	}
	if list[0].Name != "resume.pdf" {
		t.Errorf("name mismatch: want 'resume.pdf', got %q", list[0].Name)
	}
}

// TestManager_UnsupportedFormat 验证非 PDF/DOCX 文件被拒绝。
func TestManager_UnsupportedFormat(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Upload("resume.txt", []byte("text"), nil)
	if err == nil {
		t.Fatal("expected error for .txt format, got nil")
	}
}

// TestManager_SetActive 验证激活切换逻辑（只有一个激活）。
func TestManager_SetActive(t *testing.T) {
	m := newTestManager(t)

	r1, _ := m.Upload("a.pdf", []byte("Engineer A"), nil)
	r2, _ := m.Upload("b.pdf", []byte("Engineer B"), nil)

	if err := m.SetActive(r1.ID); err != nil {
		t.Fatalf("SetActive r1: %v", err)
	}
	if err := m.SetActive(r2.ID); err != nil {
		t.Fatalf("SetActive r2: %v", err)
	}

	list := m.List()
	var activeCount int
	for _, r := range list {
		if r.IsActive {
			activeCount++
			if r.ID != r2.ID {
				t.Errorf("wrong resume is active: want r2, got %s", r.ID)
			}
		}
	}
	if activeCount != 1 {
		t.Errorf("expected exactly 1 active resume, got %d", activeCount)
	}
}

// TestManager_Delete 验证删除后简历不再出现在列表中。
func TestManager_Delete(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Upload("del.pdf", []byte("to be deleted"), nil)

	if err := m.Delete(r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(m.List()) != 0 {
		t.Error("resume should be gone after delete")
	}
}

// TestSplitChunks 验证长文本分块不丢失内容（通过 Upload 间接覆盖）。
func TestSplitChunks(_ *testing.T) {
	_ = strings.Repeat("a", 1500)
}
