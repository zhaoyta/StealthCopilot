// Package audio — voice_training_recorder.go 实现声音复刻训练录音器。
// macOS 使用 CGO + CoreAudio AudioQueue 在进程内录音（权限归属于 Wails 进程，TCC 正确触发）。
// Windows/其他平台使用 ffmpeg 子进程录音。
package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// voiceRecorderImpl 录音后端接口，由平台专用实现提供。
type voiceRecorderImpl interface {
	// start 开始录音，出错时返回错误描述。
	start() error
	// stop 停止录音并返回已录制的原始 PCM（s16le 16kHz mono）字节。
	stop() []byte
}

// VoiceTrainingRecorder 从物理麦克风录制音频供声音复刻训练使用。
// Stop 时将 PCM 封装为 WAV 返回。
type VoiceTrainingRecorder struct {
	mu      sync.Mutex
	impl    voiceRecorderImpl
	running bool
}

// Start 开始录音，deviceName 仅在 ffmpeg 实现（非 macOS CGO）时生效。
// 已在录音时会先停止当前录音再重新开始。
func (r *VoiceTrainingRecorder) Start(deviceName string) error {
	r.mu.Lock()
	if r.running && r.impl != nil {
		old := r.impl
		r.impl = nil
		r.running = false
		r.mu.Unlock()
		old.stop()
	} else {
		r.mu.Unlock()
	}

	impl := newSystemVoiceRecorder()
	if ff, ok := impl.(*ffmpegVoiceRecorder); ok {
		ff.deviceName = deviceName
	}

	if err := impl.start(); err != nil {
		return err
	}

	r.mu.Lock()
	r.impl = impl
	r.running = true
	r.mu.Unlock()
	return nil
}

// Stop 停止录音，返回 WAV 字节数据和错误信息。
func (r *VoiceTrainingRecorder) Stop() ([]byte, error) {
	r.mu.Lock()
	impl := r.impl
	r.impl = nil
	r.running = false
	r.mu.Unlock()

	if impl == nil {
		return nil, fmt.Errorf("当前未在录音")
	}

	pcm := impl.stop()
	if len(pcm) == 0 {
		return nil, fmt.Errorf("未采集到音频数据，请确认已在「系统设置 → 隐私与安全性 → 麦克风」授权本应用")
	}
	return encodePCMToWAV(pcm, SampleRate, 1, 16), nil
}

// ===== ffmpeg 实现（Windows 或 CGO 不可用时） =====

type ffmpegVoiceRecorder struct {
	deviceName string
	mu         sync.Mutex
	cancel     context.CancelFunc
	cmd        *exec.Cmd
	stderrBuf  bytes.Buffer
	pcmBuf     bytes.Buffer
}

func (r *ffmpegVoiceRecorder) start() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg 未安装，无法录音：%w", err)
	}
	args, err := voiceTrainingRecordArgs(r.deviceName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("ffmpeg 管道创建失败：%w", err)
	}
	r.stderrBuf.Reset()
	r.pcmBuf.Reset()
	cmd.Stderr = &r.stderrBuf
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("ffmpeg 启动失败：%w", err)
	}

	r.mu.Lock()
	r.cancel = cancel
	r.cmd = cmd
	r.mu.Unlock()

	go func() {
		buf := make([]byte, FrameBytes*4)
		for {
			n, rdErr := stdout.Read(buf)
			if n > 0 {
				r.mu.Lock()
				r.pcmBuf.Write(buf[:n])
				r.mu.Unlock()
			}
			if rdErr != nil {
				return
			}
		}
	}()
	return nil
}

func (r *ffmpegVoiceRecorder) stop() []byte {
	r.mu.Lock()
	cancel := r.cancel
	cmd := r.cmd
	r.cancel = nil
	r.cmd = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if cmd != nil {
		_ = cmd.Wait()
	}

	r.mu.Lock()
	pcm := make([]byte, r.pcmBuf.Len())
	copy(pcm, r.pcmBuf.Bytes())
	stderr := strings.TrimSpace(r.stderrBuf.String())
	r.mu.Unlock()

	if len(pcm) == 0 && stderr != "" {
		// ffmpeg 有错误输出时记录；调用方通过 Stop 返回的 error 得到提示
		_ = stderr
	}
	return pcm
}

// voiceTrainingRecordArgs 返回 ffmpeg 录音参数（16kHz 16bit mono PCM → stdout）。
func voiceTrainingRecordArgs(deviceName string) ([]string, error) {
	base := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	output := []string{"-ac", "1", "-ar", fmt.Sprintf("%d", SampleRate), "-f", "s16le", "pipe:1"}

	switch runtime.GOOS {
	case "darwin":
		input := ":0"
		if deviceName != "" {
			input = ":" + deviceName
		}
		base = append(base, "-f", "avfoundation", "-i", input, "-vn")
		return append(base, output...), nil
	case "windows":
		input := "audio=default"
		if deviceName != "" {
			input = "audio=" + deviceName
		}
		base = append(base, "-f", "dshow", "-i", input)
		return append(base, output...), nil
	default:
		return nil, fmt.Errorf("当前系统不支持录音：%s", runtime.GOOS)
	}
}

// encodePCMToWAV 将 PCM s16le 字节流封装为标准 WAV 文件格式。
func encodePCMToWAV(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	dataSize := uint32(len(pcm))
	byteRate := uint32(sampleRate * channels * bitsPerSample / 8)
	blockAlign := uint16(channels * bitsPerSample / 8)

	var buf bytes.Buffer
	buf.Grow(44 + len(pcm))

	writeStr := func(s string) { buf.WriteString(s) }
	writeU32 := func(v uint32) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	writeU16 := func(v uint16) { _ = binary.Write(&buf, binary.LittleEndian, v) }

	writeStr("RIFF")
	writeU32(36 + dataSize)
	writeStr("WAVE")
	writeStr("fmt ")
	writeU32(16)
	writeU16(1) // PCM
	writeU16(uint16(channels))
	writeU32(uint32(sampleRate))
	writeU32(byteRate)
	writeU16(blockAlign)
	writeU16(uint16(bitsPerSample))
	writeStr("data")
	writeU32(dataSize)
	buf.Write(pcm)

	return buf.Bytes()
}
