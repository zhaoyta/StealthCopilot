package video

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"

	"github.com/zhaoyta/stealthcopilot/internal/lipsync"
)

// NewSystemVirtualCameraWriter returns a writer for a real virtual camera when
// the current platform exposes a writable ffmpeg sink. macOS OBS Virtual Camera
// is not such a sink, so it intentionally falls back to Null there.
func NewSystemVirtualCameraWriter(deviceName string) VirtualCameraWriter {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return &NullVirtualCameraWriter{}
	}
	if runtime.GOOS != "windows" || deviceName == "" {
		return &NullVirtualCameraWriter{}
	}
	w, err := NewFFmpegVirtualCameraWriter(deviceName)
	if err != nil {
		return &NullVirtualCameraWriter{}
	}
	return w
}

// FFmpegVirtualCameraWriter streams raw BGRA frames into a writable ffmpeg
// virtual camera sink. This is currently only practical on Windows DirectShow
// style sinks; macOS OBS Virtual Camera requires the OBS process/plugin path.
type FFmpegVirtualCameraWriter struct {
	mu     sync.Mutex
	stdin  io.WriteCloser
	cmd    *exec.Cmd
	closed bool
}

func NewFFmpegVirtualCameraWriter(deviceName string) (*FFmpegVirtualCameraWriter, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("当前系统不支持直接写入 ffmpeg 虚拟摄像头: %s", runtime.GOOS)
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
