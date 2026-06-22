// Package resume 实现简历的本地存储、文本提取和 embedding 管理。
package resume

import "time"

// EmbeddingStatus 表示简历 embedding 的处理状态。
type EmbeddingStatus string

const (
	// EmbeddingStatusPending 表示尚未开始处理。
	EmbeddingStatusPending EmbeddingStatus = "pending"
	// EmbeddingStatusDownloading 表示正在从 HuggingFace 下载本地模型文件。
	EmbeddingStatusDownloading EmbeddingStatus = "downloading"
	// EmbeddingStatusProcessing 表示正在后台生成 embedding。
	EmbeddingStatusProcessing EmbeddingStatus = "processing"
	// EmbeddingStatusReady 表示 embedding 已生成，可用于 RAG 检索。
	EmbeddingStatusReady EmbeddingStatus = "ready"
	// EmbeddingStatusError 表示处理过程中发生错误。
	EmbeddingStatusError EmbeddingStatus = "error"
)

// ResumeLanguage 表示用户上传时指定的简历主要语言。
type ResumeLanguage string

const (
	ResumeLanguageZH    ResumeLanguage = "zh"
	ResumeLanguageEN    ResumeLanguage = "en"
	ResumeLanguageJA    ResumeLanguage = "ja"
	ResumeLanguageKO    ResumeLanguage = "ko"
	ResumeLanguageFR    ResumeLanguage = "fr"
	ResumeLanguageDE    ResumeLanguage = "de"
	ResumeLanguageES    ResumeLanguage = "es"
	ResumeLanguageOther ResumeLanguage = "other"
	ResumeLanguageMixed ResumeLanguage = "mixed"
)

// NormalizeResumeLanguage 将外部输入规范化为支持的简历语言类型。
func NormalizeResumeLanguage(v string) ResumeLanguage {
	switch ResumeLanguage(v) {
	case ResumeLanguageZH, ResumeLanguageEN, ResumeLanguageJA, ResumeLanguageKO, ResumeLanguageFR, ResumeLanguageDE, ResumeLanguageES, ResumeLanguageOther, ResumeLanguageMixed:
		return ResumeLanguage(v)
	default:
		return ResumeLanguageMixed
	}
}

// Resume 表示一份已上传的简历及其元数据。
type Resume struct {
	ID              string          `json:"id"`                // UUID
	Name            string          `json:"name"`              // 原始文件名
	FilePath        string          `json:"file_path"`         // 本地存储路径
	ResumeLanguage  ResumeLanguage  `json:"resume_language"`   // 用户指定的简历主要语言
	EmbeddingStatus EmbeddingStatus `json:"embedding_status"`  // embedding 处理状态
	ErrMsg          string          `json:"err_msg,omitempty"` // 处理失败时的错误信息
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	IsActive        bool            `json:"is_active"` // 是否为当前激活简历
}

// FrontendResume 是返回给前端的简历视图（文件路径等敏感字段替换为安全版本）。
type FrontendResume struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	ResumeLanguage  ResumeLanguage  `json:"resume_language"`
	EmbeddingStatus EmbeddingStatus `json:"embedding_status"`
	ErrMsg          string          `json:"err_msg,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	IsActive        bool            `json:"is_active"`
}

// ToFrontend 将 Resume 转换为前端视图。
func (r *Resume) ToFrontend() FrontendResume {
	return FrontendResume{
		ID:              r.ID,
		Name:            r.Name,
		ResumeLanguage:  NormalizeResumeLanguage(string(r.ResumeLanguage)),
		EmbeddingStatus: r.EmbeddingStatus,
		ErrMsg:          r.ErrMsg,
		CreatedAt:       r.CreatedAt,
		IsActive:        r.IsActive,
	}
}
