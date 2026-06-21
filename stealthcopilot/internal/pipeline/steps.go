// Package pipeline defines shared chain step events emitted to the UI.
package pipeline

type StepKind string

const (
	StepASR   StepKind = "asr"
	StepTrans StepKind = "trans"
	StepTTS   StepKind = "tts"
)

type StepEvent struct {
	Chain      string   `json:"chain"`
	Step       StepKind `json:"step"`
	SrcText    string   `json:"srcText,omitempty"`
	DstText    string   `json:"dstText,omitempty"`
	IsFinal    bool     `json:"isFinal"`
	AudioBytes int      `json:"audioBytes,omitempty"`
	Provider   string   `json:"provider,omitempty"`
}
