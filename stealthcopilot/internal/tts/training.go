package tts

import (
	"encoding/binary"
	"fmt"
)

const (
	// TrainStateSubmitted 表示训练已提交但尚未完成。
	TrainStateSubmitted = "submitted"
	// TrainStateDone 表示训练成功且 Asset ID 可用。
	TrainStateDone = "done"
	// TrainStateFailed 表示训练失败，可重新提交。
	TrainStateFailed = "failed"
)

const minTrainingAudioDurationMs = 3000

// XunfeiVoiceTrainState 将讯飞训练状态码归一成前端稳定状态。
func XunfeiVoiceTrainState(result XunfeiVoiceTrainResult) (state string, canRetry bool) {
	switch result.TrainStatus {
	case 1:
		return TrainStateDone, false
	case -1, 2:
		return TrainStateSubmitted, false
	default:
		return TrainStateFailed, true
	}
}

// ValidateTrainingWAV 检查前端提交的训练音频是否像有效 WAV，且时长足够。
func ValidateTrainingWAV(wavAudio []byte) error {
	if len(wavAudio) == 0 {
		return fmt.Errorf("录音为空，请重新录制")
	}
	if len(wavAudio) < 44 {
		return fmt.Errorf("录音格式不正确：WAV 数据过短")
	}
	if string(wavAudio[0:4]) != "RIFF" || string(wavAudio[8:12]) != "WAVE" {
		return fmt.Errorf("录音格式不正确：请提交 WAV 音频")
	}
	sampleRate := int(binary.LittleEndian.Uint32(wavAudio[24:28]))
	bitsPerSample := int(binary.LittleEndian.Uint16(wavAudio[34:36]))
	channels := int(binary.LittleEndian.Uint16(wavAudio[22:24]))
	if sampleRate <= 0 || bitsPerSample <= 0 || channels <= 0 {
		return fmt.Errorf("录音格式不正确：缺少采样率或声道信息")
	}
	bytesPerSecond := sampleRate * channels * bitsPerSample / 8
	if bytesPerSecond <= 0 {
		return fmt.Errorf("录音格式不正确：无法计算音频时长")
	}
	dataSize := wavDataSize(wavAudio)
	if dataSize <= 0 {
		return fmt.Errorf("录音格式不正确：缺少音频数据")
	}
	durationMs := dataSize * 1000 / bytesPerSecond
	if durationMs < minTrainingAudioDurationMs {
		return fmt.Errorf("录音太短，请至少录制 3 秒")
	}
	return nil
}

func wavDataSize(wavAudio []byte) int {
	for offset := 12; offset+8 <= len(wavAudio); {
		chunkID := string(wavAudio[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(wavAudio[offset+4 : offset+8]))
		if chunkSize < 0 || offset+8+chunkSize > len(wavAudio) {
			return 0
		}
		if chunkID == "data" {
			return chunkSize
		}
		offset += 8 + chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	return 0
}
