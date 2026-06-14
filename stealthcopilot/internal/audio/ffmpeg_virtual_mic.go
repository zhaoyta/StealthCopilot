package audio

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// NewSystemVirtualMicWriter returns a real virtual mic writer when the platform
// exposes a writable ffmpeg sink. macOS BlackHole is usually selected as an
// output device by the OS/app rather than written via ffmpeg, so this falls back
// to Null there until a CoreAudio writer is introduced.
func NewSystemVirtualMicWriter(deviceName string) VirtualMicWriter {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return NewNullVirtualMicWriter()
	}
	if runtime.GOOS != "windows" || deviceName == "" {
		return NewNullVirtualMicWriter()
	}
	w, err := NewFFmpegVirtualMicWriter(deviceName)
	if err != nil {
		return NewNullVirtualMicWriter()
	}
	return w
}

// FFmpegVirtualMicWriter streams PCM into a writable Windows DirectShow audio
// sink and preserves the Zero-PCM/TTS state machine used by the speaking chain.
type FFmpegVirtualMicWriter struct {
	state  atomic.Int32
	mu     sync.Mutex
	stdin  io.WriteCloser
	cmd    *exec.Cmd
	done   chan struct{}
	once   sync.Once
	closed bool
}

func NewFFmpegVirtualMicWriter(deviceName string) (*FFmpegVirtualMicWriter, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("当前系统不支持直接写入 ffmpeg 虚拟麦克风: %s", runtime.GOOS)
	}
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-nostdin",
		"-f", "s16le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", SampleRate),
		"-i", "pipe:0",
		"-f", "dshow",
		"audio=" + deviceName,
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

	w := &FFmpegVirtualMicWriter{
		stdin: stdin,
		cmd:   cmd,
		done:  make(chan struct{}),
	}
	go w.zeroPCMLoop()
	return w, nil
}

func (w *FFmpegVirtualMicWriter) BeginZeroPCM() {
	w.state.Store(int32(micStateZeroPCM))
}

func (w *FFmpegVirtualMicWriter) WriteChunk(chunk []byte) {
	w.state.CompareAndSwap(int32(micStateZeroPCM), int32(micStateTTS))
	w.write(chunk)
}

func (w *FFmpegVirtualMicWriter) EndTTS() {
	w.state.Store(int32(micStateIdle))
}

func (w *FFmpegVirtualMicWriter) Close() {
	w.once.Do(func() {
		close(w.done)
		w.mu.Lock()
		w.closed = true
		stdin := w.stdin
		cmd := w.cmd
		w.stdin = nil
		w.cmd = nil
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
	})
}

func (w *FFmpegVirtualMicWriter) zeroPCMLoop() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	silence := make([]byte, SampleRate/100*BytesPerSample)
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			if virtualMicState(w.state.Load()) == micStateZeroPCM {
				w.write(silence)
			}
		}
	}
}

func (w *FFmpegVirtualMicWriter) write(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.stdin == nil {
		return
	}
	_, _ = w.stdin.Write(chunk)
}
