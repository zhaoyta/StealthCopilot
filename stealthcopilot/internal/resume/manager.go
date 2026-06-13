package resume

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// chunkMaxChars 是文本分块的最大字符数（约 300 中文字或 500 英文词）。
const chunkMaxChars = 500

// Manager 协调简历文件存储、embedding 生成和向量检索。
type Manager struct {
	store    *fileStore
	vectors  *vectorStore
	embedder EmbeddingProvider
	mu       sync.RWMutex // 保护 store 和 vectors 的并发访问
	wg       sync.WaitGroup // 追踪后台 embedding goroutine，Close() 时等待所有完成
}

// NewManager 创建简历 Manager。
// dataDir 是应用数据目录；embedder 为 embedding 提供者（可为 NullProvider）。
func NewManager(dataDir string, embedder EmbeddingProvider) (*Manager, error) {
	fs, err := newFileStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resume.NewManager: %w", err)
	}
	vs, err := newVectorStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resume.NewManager: %w", err)
	}
	return &Manager{
		store:    fs,
		vectors:  vs,
		embedder: embedder,
	}, nil
}

// Upload 保存简历文件，并在后台异步生成 embedding。
// onStatusChange 回调在状态变化时被调用（可为 nil），用于通知前端刷新。
func (m *Manager) Upload(name string, data []byte, onStatusChange func(r *Resume)) (*Resume, error) {
	m.mu.Lock()
	r, err := m.store.Save(name, data)
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}

	// 后台异步生成 embedding，不阻塞上传返回；WaitGroup 追踪以便 Close() 安全等待
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.generateEmbedding(r.ID, onStatusChange)
	}()

	return r, nil
}

// UploadFromPath 从本地路径导入简历（Wails 文件对话框返回的路径）。
func (m *Manager) UploadFromPath(path string, onStatusChange func(r *Resume)) (*Resume, error) {
	m.mu.Lock()
	r, err := m.store.SaveFromPath(path)
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.generateEmbedding(r.ID, onStatusChange)
	}()
	return r, nil
}

// generateEmbedding 在后台线程提取文本、分块、生成 embedding 并写入向量库。
func (m *Manager) generateEmbedding(resumeID string, onStatusChange func(r *Resume)) {
	setStatus := func(status EmbeddingStatus, errMsg string) {
		m.mu.Lock()
		r, err := m.store.Get(resumeID)
		if err != nil {
			m.mu.Unlock()
			return
		}
		r.EmbeddingStatus = status
		r.ErrMsg = errMsg
		_ = m.store.Update(r)
		m.mu.Unlock()
		if onStatusChange != nil {
			onStatusChange(r)
		}
	}

	setStatus(EmbeddingStatusProcessing, "")

	// 读取文件字节（此处直接使用原始字节作为文本，PDF/DOCX 的完整解析可在 Phase 2 扩展）
	m.mu.RLock()
	raw, err := m.store.ReadText(resumeID)
	m.mu.RUnlock()
	if err != nil {
		setStatus(EmbeddingStatusError, err.Error())
		return
	}

	// 将字节转为文本（若是二进制格式则仅取可打印 UTF-8 部分）
	text := extractText(raw)
	if strings.TrimSpace(text) == "" {
		setStatus(EmbeddingStatusError, "无法提取文本内容，请确认文件格式正确")
		return
	}

	chunks := splitChunks(text, chunkMaxChars)
	embeddings := make([][]float32, 0, len(chunks))

	for _, chunk := range chunks {
		vec, err := m.embedder.Embed(chunk)
		if err != nil {
			setStatus(EmbeddingStatusError, err.Error())
			return
		}
		embeddings = append(embeddings, vec)
	}

	m.mu.Lock()
	err = m.vectors.UpsertChunks(resumeID, chunks, embeddings)
	m.mu.Unlock()
	if err != nil {
		setStatus(EmbeddingStatusError, err.Error())
		return
	}

	setStatus(EmbeddingStatusReady, "")
}

// List 返回所有简历，按创建时间降序排列。
func (m *Manager) List() []*Resume {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := m.store.List()
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list
}

// SetActive 将指定简历设为激活（同时取消其他简历的激活状态）。
func (m *Manager) SetActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, err := m.store.Get(id)
	if err != nil {
		return err
	}
	// 取消所有激活标记
	for _, r := range m.store.List() {
		if r.IsActive {
			r.IsActive = false
			_ = m.store.Update(r)
		}
	}
	target.IsActive = true
	return m.store.Update(target)
}

// Delete 删除简历文件、索引和对应向量数据。
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.vectors.DeleteResume(id); err != nil {
		return err
	}
	return m.store.Delete(id)
}

// Search 在当前激活简历中搜索最相关的段落（用于 RAG）。
// 若 embedding 不可用，返回 ErrModelNotReady（调用方应降级处理）。
func (m *Manager) Search(query string, topK int) ([]SearchResult, error) {
	if !m.embedder.Ready() {
		return nil, ErrModelNotReady
	}
	vec, err := m.embedder.Embed(query)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	activeID := m.activeResumeID()
	m.mu.RUnlock()

	return m.vectors.Search(vec, activeID, topK)
}

// HasActiveResume 检查当前是否有激活的简历（供 RAG 检索降级判断使用）。
func (m *Manager) HasActiveResume() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeResumeID() != ""
}

// activeResumeID 返回当前激活简历的 ID；若无激活简历返回空字符串。
// 调用方需持有读锁。
func (m *Manager) activeResumeID() string {
	for _, r := range m.store.List() {
		if r.IsActive {
			return r.ID
		}
	}
	return ""
}

// Close 等待所有后台 embedding goroutine 完成，然后释放向量库数据库连接。
func (m *Manager) Close() error {
	m.wg.Wait()
	return m.vectors.Close()
}

// --- 文本处理工具 ---

// extractText 尝试将字节转为 UTF-8 文本（PDF/DOCX 的深度解析留给 Phase 2）。
func extractText(data []byte) string {
	s := string(data)
	if utf8.ValidString(s) {
		return s
	}
	// 过滤非 UTF-8 字节，保留可读内容
	var b strings.Builder
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// splitChunks 将长文本按最大字符数分割为重叠块（重叠 50 字符以减少边界截断影响）。
func splitChunks(text string, maxChars int) []string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return []string{text}
	}
	const overlap = 50
	var chunks []string
	for start := 0; start < len(runes); start += maxChars - overlap {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == len(runes) {
			break
		}
	}
	return chunks
}
