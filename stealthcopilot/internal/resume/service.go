package resume

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// eventResumeStatusChanged 是向前端发送简历状态变更的 Wails 事件名。
const eventResumeStatusChanged = "resume:status_changed"

// eventResumeDownloadProgress 是向前端推送模型下载进度的 Wails 事件名。
const eventResumeDownloadProgress = "resume:download_progress"

// eventResumeEmbedProgress 是向前端推送 embedding chunk 进度的 Wails 事件名。
const eventResumeEmbedProgress = "resume:embed_progress"

// Service 是暴露给 Wails 前端的简历管理服务。
type Service struct {
	manager *Manager
	ctx     context.Context
}

// NewService 创建 ResumeService。
// dataDir 是应用数据目录；embedder 为 embedding 提供者。
func NewService(dataDir string, embedder EmbeddingProvider) (*Service, error) {
	m, err := NewManager(dataDir, embedder)
	if err != nil {
		return nil, fmt.Errorf("resume.NewService: %w", err)
	}
	return &Service{manager: m}, nil
}

// Startup 在 Wails OnStartup 时调用，保存 context 并注册各类进度回调。
func (s *Service) Startup(ctx context.Context) {
	s.ctx = ctx
	s.manager.SetDownloadProgressHandler(func(id string, downloaded, total int64) {
		if s.ctx == nil {
			return
		}
		runtime.EventsEmit(s.ctx, eventResumeDownloadProgress, map[string]any{
			"id":         id,
			"downloaded": downloaded,
			"total":      total,
		})
	})
	s.manager.SetEmbedProgressHandler(func(id string, current, total int) {
		if s.ctx == nil {
			return
		}
		runtime.EventsEmit(s.ctx, eventResumeEmbedProgress, map[string]any{
			"id":      id,
			"current": current,
			"total":   total,
		})
	})
	// 重启上次中断的 embedding 任务（processing/downloading 已被 store 重置为 pending）
	s.manager.RestartPendingEmbeddings(s.makeStatusChangeCallback())
}

// ListResumes 返回所有简历的前端视图列表（按上传时间降序）。
func (s *Service) ListResumes() []FrontendResume {
	list := s.manager.List()
	result := make([]FrontendResume, 0, len(list))
	for _, r := range list {
		result = append(result, r.ToFrontend())
	}
	return result
}

// UploadResume 从用户选择的文件路径上传简历。
// path 由 Wails 文件对话框返回；上传后异步生成 embedding。
// 返回空字符串表示成功，否则返回错误描述。
func (s *Service) UploadResume(path string) string {
	return s.UploadResumeWithLanguage(path, string(ResumeLanguageMixed))
}

// UploadResumeWithLanguage 从用户选择的文件路径上传简历，并保存用户指定的简历语言。
func (s *Service) UploadResumeWithLanguage(path string, language string) string {
	onStatusChange := s.makeStatusChangeCallback()
	_, err := s.manager.UploadFromPathWithLanguage(path, NormalizeResumeLanguage(language), onStatusChange)
	if err != nil {
		return err.Error()
	}
	return ""
}

// DeleteResume 删除指定 ID 的简历及其 embedding 数据。
func (s *Service) DeleteResume(id string) string {
	if err := s.manager.Delete(id); err != nil {
		return err.Error()
	}
	return ""
}

// SetActiveResume 将指定简历设为激活，之前激活的自动取消。
func (s *Service) SetActiveResume(id string) string {
	if err := s.manager.SetActive(id); err != nil {
		return err.Error()
	}
	return ""
}

// IsEmbeddingReady 检查 embedding 提供者是否可用（用于 Setup Wizard 依赖检测）。
func (s *Service) IsEmbeddingReady() bool {
	return s.manager.embedder.Ready()
}

// makeStatusChangeCallback 返回在 embedding 状态变更时向前端推送事件的回调函数。
func (s *Service) makeStatusChangeCallback() func(r *Resume) {
	return func(r *Resume) {
		if s.ctx == nil {
			return
		}
		runtime.EventsEmit(s.ctx, eventResumeStatusChanged, r.ToFrontend())
	}
}

// EmbeddingModelInfo 描述本地 embedding 模型的缓存状态，用于前端状态栏展示。
type EmbeddingModelInfo struct {
	Cached    bool   `json:"cached"`
	CachePath string `json:"cache_path"`
}

// GetEmbeddingModelInfo 返回 multilingual-e5-small 模型的本地缓存状态和路径。
func (s *Service) GetEmbeddingModelInfo() EmbeddingModelInfo {
	info := EmbeddingModelInfo{}
	type cacheChecker interface{ IsModelCached() bool }
	type cachePather interface{ ModelCachePath() string }
	if c, ok := s.manager.embedder.(cacheChecker); ok {
		info.Cached = c.IsModelCached()
	}
	if p, ok := s.manager.embedder.(cachePather); ok {
		info.CachePath = p.ModelCachePath()
	}
	return info
}

// EmbedProgressInfo 描述某份简历当前的 chunk 处理进度，供前端重新挂载时还原进度条。
type EmbedProgressInfo struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// GetResumeEmbedProgress 返回指定简历的当前 chunk 进度（处理中时有意义）。
func (s *Service) GetResumeEmbedProgress(id string) EmbedProgressInfo {
	current, total := s.manager.EmbedProgress(id)
	return EmbedProgressInfo{Current: current, Total: total}
}

// InternalManager 供其他 Go 包访问底层 Manager（如 RAG 检索）。
func (s *Service) InternalManager() *Manager {
	return s.manager
}
