package audio

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/zhaoyta/stealthcopilot/internal/diag"
)

func resolveAudioDeviceIndex(deviceName string) (int, bool) {
	if idx, ok := parseAudioDeviceIndex(deviceName); ok {
		return idx, true
	}
	idx, err := lookupAudioToolboxOutputIndex(deviceName)
	if err != nil {
		diag.Warnf("audiotoolbox device lookup failed device=%q err=%v", deviceName, err)
		return -1, false
	}
	return idx, true
}

func lookupAudioToolboxOutputIndex(deviceName string) (int, error) {
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		return -1, fmt.Errorf("empty device name")
	}
	args := []string{
		"-hide_banner",
		"-f", "lavfi",
		"-i", "anullsrc=r=24000:cl=mono",
		"-t", "0.01",
		"-f", "audiotoolbox",
		"-list_devices", "true",
		"-",
	}
	out, err := exec.Command("ffmpeg", args...).CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("ffmpeg list devices: %w: %s", err, limitLogString(string(out), 500))
	}
	idx, ok := parseAudioToolboxOutputIndex(string(out), deviceName)
	if !ok {
		return -1, fmt.Errorf("device not found in AudioToolbox outputs")
	}
	return idx, nil
}

func parseAudioToolboxOutputIndex(raw, deviceName string) (int, bool) {
	want := strings.ToLower(strings.TrimSpace(deviceName))
	if want == "" {
		return -1, false
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		open := strings.Index(line, "] [")
		if open < 0 {
			continue
		}
		right := line[open+3:]
		closeIdx := strings.Index(right, "]")
		if closeIdx < 0 {
			continue
		}
		idx, err := strconv.Atoi(right[:closeIdx])
		if err != nil {
			continue
		}
		namePart := strings.TrimSpace(right[closeIdx+1:])
		if comma := strings.Index(namePart, ","); comma >= 0 {
			namePart = strings.TrimSpace(namePart[:comma])
		}
		if strings.EqualFold(namePart, deviceName) || strings.Contains(strings.ToLower(namePart), want) {
			return idx, true
		}
	}
	return -1, false
}
