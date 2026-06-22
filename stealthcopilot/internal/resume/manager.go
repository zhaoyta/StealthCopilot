package resume

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// chunkMaxChars 是文本分块的最大字符数（约 300 中文字或 500 英文词）。
const chunkMaxChars = 500

// Manager 协调简历文件存储、embedding 生成和向量检索。
type Manager struct {
	store              *fileStore
	vectors            *vectorStore
	embedder           EmbeddingProvider
	mu                 sync.RWMutex                                   // 保护 store 和 vectors 的并发访问
	wg                 sync.WaitGroup                                 // 追踪后台 embedding goroutine，Close() 时等待所有完成
	onDownloadProgress func(resumeID string, downloaded, total int64) // 可选，由 Service 注册
	onEmbedProgress    func(resumeID string, current, total int)      // 可选，每完成一个 chunk 触发
	// embedProgressMap 记录正在处理的简历的 chunk 进度（纯内存，不持久化）。
	// 前端重新挂载时可通过 EmbedProgress 接口查询当前进度，避免等待下一个事件。
	embedProgressMap map[string][2]int // resumeID → [current, total]
}

// SetDownloadProgressHandler 注册模型下载进度回调（在 Service.Startup 时调用一次）。
func (m *Manager) SetDownloadProgressHandler(fn func(resumeID string, downloaded, total int64)) {
	m.onDownloadProgress = fn
}

// SetEmbedProgressHandler 注册 embedding chunk 进度回调（在 Service.Startup 时调用一次）。
func (m *Manager) SetEmbedProgressHandler(fn func(resumeID string, current, total int)) {
	m.onEmbedProgress = fn
}

// EmbedProgress 返回指定简历当前的 chunk 进度（current, total）。
// 若该简历未处于处理中状态，返回 (0, 0)。
func (m *Manager) EmbedProgress(resumeID string) (current, total int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.embedProgressMap[resumeID]; ok {
		return p[0], p[1]
	}
	return 0, 0
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
	return m.UploadWithLanguage(name, data, ResumeLanguageMixed, onStatusChange)
}

func (m *Manager) UploadWithLanguage(name string, data []byte, language ResumeLanguage, onStatusChange func(r *Resume)) (*Resume, error) {
	m.mu.Lock()
	r, err := m.store.SaveWithLanguage(name, data, language)
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
	return m.UploadFromPathWithLanguage(path, ResumeLanguageMixed, onStatusChange)
}

func (m *Manager) UploadFromPathWithLanguage(path string, language ResumeLanguage, onStatusChange func(r *Resume)) (*Resume, error) {
	m.mu.Lock()
	r, err := m.store.SaveFromPathWithLanguage(path, language)
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
// 若模型尚未缓存，会先下载模型并通过 onDownloadProgress 推送进度，再进行 embedding。
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

	// 若 embedder 支持缓存检测且模型未缓存，先执行带进度的下载
	if checker, ok := m.embedder.(ModelCacheChecker); ok && !checker.IsModelCached() {
		if downloader, ok := m.embedder.(ModelDownloader); ok {
			setStatus(EmbeddingStatusDownloading, "")
			err := downloader.DownloadModel(context.Background(), func(downloaded, total int64) {
				if m.onDownloadProgress != nil {
					m.onDownloadProgress(resumeID, downloaded, total)
				}
			})
			if err != nil {
				setStatus(EmbeddingStatusError, "模型下载失败: "+err.Error())
				return
			}
		}
	}

	setStatus(EmbeddingStatusProcessing, "")

	var fileName string
	m.mu.RLock()
	r, err := m.store.Get(resumeID)
	if err == nil {
		fileName = r.Name
	}
	raw, readErr := m.store.ReadText(resumeID)
	m.mu.RUnlock()
	if err != nil {
		setStatus(EmbeddingStatusError, err.Error())
		return
	}
	if readErr != nil {
		setStatus(EmbeddingStatusError, readErr.Error())
		return
	}

	text := extractText(fileName, raw)
	if strings.TrimSpace(text) == "" {
		setStatus(EmbeddingStatusError, "无法提取文本内容，请确认文件格式正确")
		return
	}

	chunks := splitChunks(text, chunkMaxChars)
	embeddings := make([][]float32, 0, len(chunks))

	for i, chunk := range chunks {
		vec, err := embedPassage(m.embedder, chunk)
		if err != nil {
			m.mu.Lock()
			delete(m.embedProgressMap, resumeID)
			m.mu.Unlock()
			setStatus(EmbeddingStatusError, err.Error())
			return
		}
		embeddings = append(embeddings, vec)
		current, total := i+1, len(chunks)
		m.mu.Lock()
		if m.embedProgressMap == nil {
			m.embedProgressMap = make(map[string][2]int)
		}
		m.embedProgressMap[resumeID] = [2]int{current, total}
		m.mu.Unlock()
		if m.onEmbedProgress != nil {
			m.onEmbedProgress(resumeID, current, total)
		}
	}
	m.mu.Lock()
	delete(m.embedProgressMap, resumeID)
	m.mu.Unlock()

	m.mu.Lock()
	err = m.vectors.UpsertChunks(resumeID, chunks, embeddings)
	m.mu.Unlock()
	if err != nil {
		setStatus(EmbeddingStatusError, err.Error())
		return
	}

	setStatus(EmbeddingStatusReady, "")
}

// RestartPendingEmbeddings 在应用启动时调用，为所有 pending 状态的简历重新触发 embedding。
// 上次运行中途被中断（processing/downloading）的简历已由 store 重置为 pending。
func (m *Manager) RestartPendingEmbeddings(onStatusChange func(r *Resume)) {
	m.mu.RLock()
	var pendingIDs []string
	for _, r := range m.store.List() {
		if r.EmbeddingStatus == EmbeddingStatusPending {
			pendingIDs = append(pendingIDs, r.ID)
		}
	}
	m.mu.RUnlock()

	for _, id := range pendingIDs {
		id := id
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.generateEmbedding(id, onStatusChange)
		}()
	}
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

// ActiveResumeLanguage 返回当前激活简历的用户标记语言。
func (m *Manager) ActiveResumeLanguage() ResumeLanguage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id := m.activeResumeID()
	if id == "" {
		return ResumeLanguageMixed
	}
	r, err := m.store.Get(id)
	if err != nil {
		return ResumeLanguageMixed
	}
	return NormalizeResumeLanguage(string(r.ResumeLanguage))
}

// HasActiveResume 检查当前是否有激活的简历（供 RAG 检索降级判断使用）。
func (m *Manager) HasActiveResume() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeResumeID() != ""
}

// ActiveResumeID returns the currently active resume ID.
func (m *Manager) ActiveResumeID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeResumeID()
}

// ActiveResumeText extracts the full text of the currently active resume.
func (m *Manager) ActiveResumeText() (string, bool, error) {
	m.mu.RLock()
	id := m.activeResumeID()
	if id == "" {
		m.mu.RUnlock()
		return "", false, nil
	}
	r, err := m.store.Get(id)
	if err != nil {
		m.mu.RUnlock()
		return "", true, err
	}
	name := r.Name
	raw, err := m.store.ReadText(id)
	m.mu.RUnlock()
	if err != nil {
		return "", true, err
	}
	return extractText(name, raw), true, nil
}

// ResumeName returns the display name for a resume ID.
func (m *Manager) ResumeName(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, err := m.store.Get(id)
	if err != nil {
		return ""
	}
	return r.Name
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

func embedPassage(provider EmbeddingProvider, chunk string) ([]float32, error) {
	if passageProvider, ok := provider.(PassageEmbeddingProvider); ok {
		return passageProvider.EmbedPassage(chunk)
	}
	return provider.Embed(chunk)
}

func extractText(fileName string, data []byte) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".docx":
		return extractDOCXText(data)
	case ".pdf":
		return extractPDFText(data)
	default:
		return extractUTF8Text(data)
	}
}

func extractUTF8Text(data []byte) string {
	s := string(data)
	if utf8.ValidString(s) {
		return strings.TrimSpace(s)
	}
	var b strings.Builder
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError && (unicode.IsPrint(r) || unicode.IsSpace(r)) {
			b.WriteRune(r)
		}
		i += size
	}
	return strings.TrimSpace(b.String())
}

func extractDOCXText(data []byte) string {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return extractUTF8Text(data)
	}
	for _, f := range reader.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return ""
		}
		defer rc.Close()
		xmlBytes, err := io.ReadAll(rc)
		if err != nil {
			return ""
		}
		return xmlText(xmlBytes)
	}
	return ""
}

func extractPDFText(data []byte) string {
	text := extractPDFLiteralText(data)
	if strings.TrimSpace(text) != "" {
		return text
	}
	return extractUTF8Text(data)
}

var (
	xmlParagraphRE = regexp.MustCompile(`(?i)</w:p>`)
	xmlTagRE       = regexp.MustCompile(`<[^>]+>`)
	pdfLiteralRE   = regexp.MustCompile(`\((?:\\.|[^\\)])*\)\s*T[Jj]`)
)

func xmlText(data []byte) string {
	s := string(data)
	s = xmlParagraphRE.ReplaceAllString(s, "\n")
	s = xmlTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return normalizeWhitespace(s)
}

func extractPDFLiteralText(data []byte) string {
	matches := pdfLiteralRE.FindAll(data, -1)
	var parts []string
	for _, match := range matches {
		start := bytes.IndexByte(match, '(')
		end := bytes.LastIndexByte(match, ')')
		if start < 0 || end <= start {
			continue
		}
		part := unescapePDFLiteral(string(match[start+1 : end]))
		if strings.TrimSpace(part) != "" {
			parts = append(parts, part)
		}
	}
	return normalizeWhitespace(strings.Join(parts, "\n"))
}

func unescapePDFLiteral(s string) string {
	replacer := strings.NewReplacer(
		`\\`, `\`,
		`\(`, `(`,
		`\)`, `)`,
		`\n`, "\n",
		`\r`, "\n",
		`\t`, "\t",
	)
	return replacer.Replace(s)
}

func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
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
