package system

import (
	"os/exec"
	"runtime"
	"strings"
)

// Device 表示一个音视频设备。
type Device struct {
	ID   string `json:"id"`   // 设备唯一标识符
	Name string `json:"name"` // 设备友好名称
}

// DeviceList 包含系统中枚举到的各类音视频设备。
type DeviceList struct {
	AudioInputs  []Device `json:"audio_inputs"`  // 物理麦克风 + 虚拟声卡
	AudioOutputs []Device `json:"audio_outputs"` // 音频输出设备
	VideoInputs  []Device `json:"video_inputs"`  // 摄像头 + 虚拟摄像头
}

// EnumerateDevices 扫描系统音视频设备列表（每次调用实时枚举）。
func EnumerateDevices() DeviceList {
	switch runtime.GOOS {
	case "darwin":
		return enumerateMacDevices()
	case "windows":
		return enumerateWinDevices()
	default:
		return DeviceList{}
	}
}

func enumerateMacDevices() DeviceList {
	// 使用 system_profiler 枚举 macOS 音频设备
	audioOut, _ := exec.Command(
		"system_profiler", "SPAudioDataType", "-json",
	).Output()

	// 使用 ffmpeg 枚举视频设备（ffmpeg 不一定存在，作为可选）
	videoOut, _ := exec.Command(
		"ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "",
	).CombinedOutput()

	return parseAppleDevices(audioOut, videoOut)
}

// parseAppleDevices 将 macOS 输出解析为 DeviceList。
// system_profiler JSON 结构复杂，此处采用简化字符串扫描。
func parseAppleDevices(audioRaw, videoRaw []byte) DeviceList {
	// 音频：从 ffmpeg avfoundation 列表解析（比 system_profiler 更直接）
	ffmpegAll, _ := exec.Command(
		"ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "",
	).CombinedOutput()

	dl := DeviceList{
		AudioInputs:  []Device{},
		AudioOutputs: []Device{},
		VideoInputs:  []Device{},
	}

	_ = audioRaw // 备用：若 ffmpeg 不可用

	lines := strings.Split(string(ffmpegAll), "\n")
	var section string
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "AVFoundation audio devices") {
			section = "audio"
			continue
		}
		if strings.Contains(l, "AVFoundation video devices") {
			section = "video"
			continue
		}
		// 行格式：[AVFoundation input device @ ...] [N] Device Name
		if !strings.Contains(l, "] [") {
			continue
		}
		parts := strings.SplitN(l, "] [", 2)
		if len(parts) < 2 {
			continue
		}
		right := parts[1]
		idEnd := strings.Index(right, "]")
		if idEnd < 0 {
			continue
		}
		id := right[:idEnd]
		name := strings.TrimSpace(right[idEnd+1:])
		if name == "" {
			continue
		}
		switch section {
		case "audio":
			dl.AudioInputs = append(dl.AudioInputs, Device{ID: id, Name: name})
		case "video":
			dl.VideoInputs = append(dl.VideoInputs, Device{ID: id, Name: name})
		}
	}

	// 若 ffmpeg 不可用，videoRaw 可为 system_profiler 输出备用
	_ = videoRaw

	return dl
}

func enumerateWinDevices() DeviceList {
	// Windows：使用 ffmpeg dshow 列举设备
	out, _ := exec.Command(
		"ffmpeg", "-f", "dshow", "-list_devices", "true", "-i", "dummy",
	).CombinedOutput()

	dl := DeviceList{
		AudioInputs:  []Device{},
		AudioOutputs: []Device{},
		VideoInputs:  []Device{},
	}

	lines := strings.Split(string(out), "\n")
	var section string
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "DirectShow video devices") {
			section = "video"
			continue
		}
		if strings.Contains(l, "DirectShow audio devices") {
			section = "audio"
			continue
		}
		// dshow 行格式：  "Device Name" (video/audio)
		if !strings.HasPrefix(l, "\"") {
			continue
		}
		end := strings.LastIndex(l, "\"")
		if end <= 0 {
			continue
		}
		name := l[1:end]
		switch section {
		case "audio":
			dl.AudioInputs = append(dl.AudioInputs, Device{ID: name, Name: name})
		case "video":
			dl.VideoInputs = append(dl.VideoInputs, Device{ID: name, Name: name})
		}
	}
	return dl
}

// Service 是暴露给 Wails 前端的系统服务。
type Service struct{}

// NewSystemService 创建 SystemService 实例。
func NewSystemService() *Service { return &Service{} }

// CheckDeps 检测系统依赖（BlackHole 虚拟声卡、虚拟摄像头）。
func (s *Service) CheckDeps() DepsReport {
	return CheckDeps()
}

// EnumerateDevices 实时枚举系统音视频设备。
func (s *Service) EnumerateDevices() DeviceList {
	return EnumerateDevices()
}
