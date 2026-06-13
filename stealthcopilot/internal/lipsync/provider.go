// Package lipsync 定义口型同步 Provider 接口。
// Simli AI 实现在同包的 simli.go 中提供。
// LipSyncProvider 接口预留 StealthCloudProvider 切换点（Phase 3 自营云服务）。
package lipsync

import "context"

// VideoFrame 表示一帧视频数据。
type VideoFrame struct {
	Data []byte // BGRA 格式像素数据
	PTS  int64  // Presentation Timestamp，单位毫秒
}

// AudioChunk 表示一段音频数据，附带时间戳用于 A/V 对齐。
type AudioChunk struct {
	Data []byte // PCM 音频数据
	PTS  int64  // Presentation Timestamp，单位毫秒
}

// Provider 是口型同步服务的统一抽象接口。
// 输入：视频帧 + 同步音频；输出：口型处理后的视频帧。
type Provider interface {
	// Start 建立会话，初始化连接。faceID 为用户配置的面部 ID（Simli Face ID）。
	Start(ctx context.Context, faceID string) error

	// SendAudio 向口型同步服务发送音频 chunk，附带时间戳。
	SendAudio(chunk AudioChunk) error

	// SendVideo 向口型同步服务发送原始视频帧，附带时间戳。
	SendVideo(frame VideoFrame) error

	// Output 返回口型同步后的视频帧 channel。
	Output() <-chan VideoFrame

	// Close 关闭连接，释放资源。
	Close() error
}
