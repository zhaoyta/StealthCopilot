package tts

import (
	"encoding/binary"
	"testing"
)

func TestXunfeiVoiceTrainState(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		want     string
		canRetry bool
	}{
		{name: "done", status: 1, want: TrainStateDone},
		{name: "pending negative", status: -1, want: TrainStateSubmitted},
		{name: "pending two", status: 2, want: TrainStateSubmitted},
		{name: "failed", status: 9, want: TrainStateFailed, canRetry: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, canRetry := XunfeiVoiceTrainState(XunfeiVoiceTrainResult{TrainStatus: tt.status})
			if got != tt.want || canRetry != tt.canRetry {
				t.Fatalf("state = %q/%v, want %q/%v", got, canRetry, tt.want, tt.canRetry)
			}
		})
	}
}

func TestValidateTrainingWAV(t *testing.T) {
	if err := ValidateTrainingWAV(nil); err == nil {
		t.Fatal("expected empty error")
	}
	if err := ValidateTrainingWAV([]byte("nope")); err == nil {
		t.Fatal("expected invalid wav error")
	}
	if err := ValidateTrainingWAV(testWAV(44100, 1, 16, 2)); err == nil {
		t.Fatal("expected short wav error")
	}
	if err := ValidateTrainingWAV(testWAV(44100, 1, 16, 3)); err != nil {
		t.Fatalf("valid wav: %v", err)
	}
}

func testWAV(sampleRate, channels, bitsPerSample, seconds int) []byte {
	dataSize := sampleRate * channels * bitsPerSample / 8 * seconds
	wav := make([]byte, 44+dataSize)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(36+dataSize))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(wav[24:28], uint32(sampleRate))
	byteRate := sampleRate * channels * bitsPerSample / 8
	binary.LittleEndian.PutUint32(wav[28:32], uint32(byteRate))
	blockAlign := channels * bitsPerSample / 8
	binary.LittleEndian.PutUint16(wav[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(wav[34:36], uint16(bitsPerSample))
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataSize))
	return wav
}
