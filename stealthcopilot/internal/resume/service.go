package resume

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// eventResumeStatusChanged 是向前端发送简历状态变更的 Wails 事件名。
const eventResumeStatusChanged = "resume:status_changed"

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

// Startup 在 Wails OnStartup 时调用，保存 context 用于后续事件推送。
func (s *Service) Startup(ctx context.Context) {
	s.ctx = ctx
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
	onStatusChange := s.makeStatusChangeCallback()
	_, err := s.manager.UploadFromPath(path, onStatusChange)
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

// InternalManager 供其他 Go 包访问底层 Manager（如 RAG 检索）。
func (s *Service) InternalManager() *Manager {
	return s.manager
}
