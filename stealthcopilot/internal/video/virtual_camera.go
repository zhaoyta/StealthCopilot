// Package video — virtual_camera.go defines the frame writer contract used by
// digital-human output sinks.
package video

import (
	"sync"
	"sync/atomic"
)

// VirtualCameraWriter receives BGRA video frames from a digital-human provider.
type VirtualCameraWriter interface {
	WriteFrame(frame Frame) error
	Close() error
}

// vcState 虚拟摄像头写入器内部状态
type vcState int32

const (
	vcStateIdle    vcState = iota
	vcStateRunning         // 正在写帧
)

// NullVirtualCameraWriter discards video frames.
type NullVirtualCameraWriter struct {
	state atomic.Int32
	once  sync.Once
}

// WriteFrame 丢弃视频帧（NullWriter 无实际输出）。
func (w *NullVirtualCameraWriter) WriteFrame(_ Frame) error {
	w.state.Store(int32(vcStateRunning))
	return nil
}

// Close 标记状态为 Idle，释放资源。
func (w *NullVirtualCameraWriter) Close() error {
	w.once.Do(func() {
		w.state.Store(int32(vcStateIdle))
	})
	return nil
}
