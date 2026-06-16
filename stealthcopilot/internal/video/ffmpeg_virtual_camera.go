// Package video — ffmpeg_virtual_camera.go 实现平台相关的虚拟摄像头帧写入。
//
// 路由策略：
//   - macOS：通过 UNIX domain socket 将 BGRA 帧推送给 StealthVirtualCam DAL 插件。
//     插件监听 darwinSocketPath；未运行时降级为 NullVirtualCameraWriter。
//   - Windows：通过 FFmpeg + DirectShow 将帧写入已注册的虚拟摄像头驱动。
//   - 其他平台：始终返回 NullVirtualCameraWriter。
package video

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os/exec"
	"runtime"
	"sync"

	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
)

// darwinSocketPath 是 StealthVirtualCam DAL 插件监听的 UNIX domain socket 路径。
// 插件与此 Go 客户端共同约定该路径；协议：[4字节大端帧长][BGRA 原始数据]。
const darwinSocketPath = "/tmp/stealthvcam.sock"

// NewSystemVirtualCameraWriter 根据当前平台返回最合适的虚拟摄像头写入器。
// deviceName 为空时直接返回 NullVirtualCameraWriter（未选择设备时不尝试连接）。
func NewSystemVirtualCameraWriter(deviceName string) VirtualCameraWriter {
	writer, _ := NewSystemVirtualCameraWriterChecked(deviceName)
	return writer
}

// NewSystemVirtualCameraWriterChecked returns a writer plus a readiness message.
// When deviceName is set, ready=false means the video chain would otherwise
// discard frames into a Null writer, so callers should surface the error.
func NewSystemVirtualCameraWriterChecked(deviceName string) (VirtualCameraWriter, string) {
	if deviceName == "" {
		return &NullVirtualCameraWriter{}, ""
	}
	switch runtime.GOOS {
	case "darwin":
		writer := newDarwinSocketWriter()
		if _, ok := writer.(*NullVirtualCameraWriter); ok {
			return writer, "StealthVirtualCam 驱动未运行：请先安装/重启虚拟摄像头驱动"
		}
		return writer, ""
	case "windows":
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			return &NullVirtualCameraWriter{}, "ffmpeg 未安装，无法写入虚拟摄像头"
		}
		w, err := NewFFmpegVirtualCameraWriter(deviceName)
		if err != nil {
			return &NullVirtualCameraWriter{}, "虚拟摄像头写入器启动失败：" + err.Error()
		}
		return w, ""
	default:
		return &NullVirtualCameraWriter{}, "当前系统不支持虚拟摄像头写入"
	}
}

// ===== macOS：UNIX Socket 写入器 =====

// DarwinSocketWriter 通过 UNIX domain socket 向 StealthVirtualCam DAL 插件发送帧。
// 帧协议：每帧先发 4 字节大端序长度，再发 BGRA 原始像素数据。
// 插件不在线时 newDarwinSocketWriter 降级返回 NullVirtualCameraWriter。
type DarwinSocketWriter struct {
	mu     sync.Mutex
	conn   net.Conn
	closed bool
}

// newDarwinSocketWriter 尝试连接 DAL 插件 socket；失败时返回 NullVirtualCameraWriter。
func newDarwinSocketWriter() VirtualCameraWriter {
	conn, err := net.Dial("unix", darwinSocketPath)
	if err != nil {
		// 插件未安装或未运行，降级为丢帧模式
		return &NullVirtualCameraWriter{}
	}
	return &DarwinSocketWriter{conn: conn}
}

// WriteFrame 向 DAL 插件发送一帧 BGRA 数据。
// 协议：[4字节大端帧长 uint32][BGRA 原始字节]。
func (w *DarwinSocketWriter) WriteFrame(frame lipsync.VideoFrame) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || len(frame.Data) == 0 {
		return nil
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(frame.Data)))
	if _, err := w.conn.Write(header[:]); err != nil {
		return fmt.Errorf("虚拟摄像头写入帧头失败: %w", err)
	}
	if _, err := w.conn.Write(frame.Data); err != nil {
		return fmt.Errorf("虚拟摄像头写入帧数据失败: %w", err)
	}
	return nil
}

// Close 关闭到 DAL 插件的 socket 连接。
func (w *DarwinSocketWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.conn != nil {
		return w.conn.Close()
	}
	return nil
}

// ===== Windows：FFmpeg DirectShow 写入器 =====

// FFmpegVirtualCameraWriter 通过 FFmpeg stdin 管道将 BGRA 帧写入 Windows DirectShow 虚拟摄像头。
type FFmpegVirtualCameraWriter struct {
	mu     sync.Mutex
	stdin  io.WriteCloser
	cmd    *exec.Cmd
	closed bool
}

// NewFFmpegVirtualCameraWriter 启动 FFmpeg 进程，以 DirectShow 格式输出到 deviceName。
// 仅限 Windows；其他平台直接返回错误。
func NewFFmpegVirtualCameraWriter(deviceName string) (*FFmpegVirtualCameraWriter, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("FFmpegVirtualCameraWriter 仅支持 Windows，当前平台: %s", runtime.GOOS)
	}
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-s", fmt.Sprintf("%dx%d", DefaultWidth, DefaultHeight),
		"-r", fmt.Sprintf("%d", TargetFPS),
		"-i", "pipe:0",
		"-f", "dshow",
		"video=" + deviceName,
	}
	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &FFmpegVirtualCameraWriter{stdin: stdin, cmd: cmd}, nil
}

// WriteFrame 将 BGRA 帧写入 FFmpeg stdin 管道。
func (w *FFmpegVirtualCameraWriter) WriteFrame(frame lipsync.VideoFrame) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("虚拟摄像头写入器已关闭")
	}
	if len(frame.Data) == 0 {
		return nil
	}
	_, err := w.stdin.Write(frame.Data)
	return err
}

// Close 关闭 stdin 管道并终止 FFmpeg 进程。
func (w *FFmpegVirtualCameraWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	stdin := w.stdin
	cmd := w.cmd
	w.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}
	return nil
}
