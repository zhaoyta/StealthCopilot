package session

import "errors"

// ResumeNameResolver optionally enriches session rows with resume names.
type ResumeNameResolver func(resumeID string) string

// Service exposes session history operations to Wails bindings.
type Service struct {
	store      Store
	resumeName ResumeNameResolver
}

func NewService(store Store, resumeName ResumeNameResolver) *Service {
	return &Service{store: store, resumeName: resumeName}
}

func (s *Service) ListSessions(limit int) []SessionSummary {
	if s == nil || s.store == nil {
		return nil
	}
	items, err := s.store.ListSessions(limit)
	if err != nil {
		return nil
	}
	result := make([]SessionSummary, 0, len(items))
	for _, item := range items {
		summary := SessionSummary{Session: item}
		if s.resumeName != nil && item.ResumeID != "" {
			summary.ResumeName = s.resumeName(item.ResumeID)
		}
		result = append(result, summary)
	}
	return result
}

func (s *Service) GetSessionTurns(sessionID string) []Turn {
	if s == nil || s.store == nil {
		return nil
	}
	turns, err := s.store.GetTurns(sessionID)
	if err != nil {
		return nil
	}
	return turns
}

func (s *Service) DeleteSession(sessionID string) string {
	if s == nil || s.store == nil {
		return "历史会话服务未初始化"
	}
	if err := s.store.Delete(sessionID); err != nil {
		switch {
		case errors.Is(err, ErrSessionActive):
			return "进行中的会话不能删除，请先停止听力链"
		case errors.Is(err, ErrSessionNotFound):
			return "历史会话不存在"
		default:
			return err.Error()
		}
	}
	return ""
}
