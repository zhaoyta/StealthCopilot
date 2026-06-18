//go:build !darwin || !cgo

// Package audio — cgo_recorder_other.go 在非 macOS 或无 CGO 环境下提供 ffmpeg 实现。
package audio

// newSystemVoiceRecorder 返回 ffmpeg 录音实现（非 darwin）。
func newSystemVoiceRecorder() voiceRecorderImpl {
	return &ffmpegVoiceRecorder{}
}
