package resume

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	indexFileName = "resumes.json"
	filesDir      = "files"
)

// ErrUnsupportedFormat 表示文件格式不在支持列表中（仅支持 PDF/DOCX）。
var ErrUnsupportedFormat = errors.New("unsupported format: only PDF and DOCX are accepted")

// ErrNotFound 表示请求的简历 ID 不存在。
var ErrNotFound = errors.New("resume not found")

// fileStore 管理简历文件的本地存储和索引。
type fileStore struct {
	dataDir string
	resumes map[string]*Resume // keyed by ID
}

// newFileStore 初始化文件存储，dataDir 下会自动创建 files/ 子目录。
func newFileStore(dataDir string) (*fileStore, error) {
	filesPath := filepath.Join(dataDir, filesDir)
	if err := os.MkdirAll(filesPath, 0o700); err != nil {
		return nil, fmt.Errorf("fileStore: mkdir: %w", err)
	}
	fs := &fileStore{
		dataDir: dataDir,
		resumes: make(map[string]*Resume),
	}
	if err := fs.loadIndex(); err != nil {
		return nil, err
	}
	return fs, nil
}

// Save 接收源文件字节流和原始文件名，存储到 files/ 目录并添加索引条目。
// 仅支持 .pdf 和 .docx 格式，否则返回 ErrUnsupportedFormat。
func (fs *fileStore) Save(name string, data []byte) (*Resume, error) {
	return fs.SaveWithLanguage(name, data, ResumeLanguageMixed)
}

func (fs *fileStore) SaveWithLanguage(name string, data []byte, language ResumeLanguage) (*Resume, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".pdf" && ext != ".docx" {
		return nil, ErrUnsupportedFormat
	}

	id := uuid.New().String()
	dst := filepath.Join(fs.dataDir, filesDir, id+ext)
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return nil, fmt.Errorf("fileStore.Save: write: %w", err)
	}

	now := time.Now()
	r := &Resume{
		ID:              id,
		Name:            name,
		FilePath:        dst,
		ResumeLanguage:  NormalizeResumeLanguage(string(language)),
		EmbeddingStatus: EmbeddingStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	fs.resumes[id] = r
	return r, fs.saveIndex()
}

// SaveFromPath 从已存在的临时文件路径导入简历（Wails 文件对话框返回路径）。
func (fs *fileStore) SaveFromPath(srcPath string) (*Resume, error) {
	return fs.SaveFromPathWithLanguage(srcPath, ResumeLanguageMixed)
}

func (fs *fileStore) SaveFromPathWithLanguage(srcPath string, language ResumeLanguage) (*Resume, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("fileStore.SaveFromPath: read: %w", err)
	}
	return fs.SaveWithLanguage(filepath.Base(srcPath), data, language)
}

// CopyFrom 从 io.Reader 读取简历数据并保存。
func (fs *fileStore) CopyFrom(name string, r io.Reader) (*Resume, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return fs.Save(name, data)
}

// Get 按 ID 查找简历，不存在时返回 ErrNotFound。
func (fs *fileStore) Get(id string) (*Resume, error) {
	r, ok := fs.resumes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

// List 返回所有简历列表（顺序不保证）。
func (fs *fileStore) List() []*Resume {
	list := make([]*Resume, 0, len(fs.resumes))
	for _, r := range fs.resumes {
		list = append(list, r)
	}
	return list
}

// Update 更新简历元数据并持久化。
func (fs *fileStore) Update(r *Resume) error {
	r.UpdatedAt = time.Now()
	fs.resumes[r.ID] = r
	return fs.saveIndex()
}

// Delete 删除简历文件和索引条目。
func (fs *fileStore) Delete(id string) error {
	r, ok := fs.resumes[id]
	if !ok {
		return ErrNotFound
	}
	_ = os.Remove(r.FilePath)
	delete(fs.resumes, id)
	return fs.saveIndex()
}

// ReadText 读取简历文件的原始字节，用于文本提取（由 embedding 层调用）。
func (fs *fileStore) ReadText(id string) ([]byte, error) {
	r, err := fs.Get(id)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(r.FilePath)
}

// --- 持久化 ---

func (fs *fileStore) loadIndex() error {
	path := filepath.Join(fs.dataDir, indexFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("fileStore: load index: %w", err)
	}
	var list []*Resume
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("fileStore: parse index: %w", err)
	}
	for _, r := range list {
		r.ResumeLanguage = NormalizeResumeLanguage(string(r.ResumeLanguage))
		// 应用中止时若处于中间状态，重置为待处理，避免永久卡住
		if r.EmbeddingStatus == EmbeddingStatusDownloading ||
			r.EmbeddingStatus == EmbeddingStatusProcessing {
			r.EmbeddingStatus = EmbeddingStatusPending
		}
		fs.resumes[r.ID] = r
	}
	return nil
}

func (fs *fileStore) saveIndex() error {
	list := fs.List()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(fs.dataDir, indexFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
