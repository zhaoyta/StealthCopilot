package translation

import "context"

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

// ResultStage is the extension point between ASR and TTS. Implementations may
// translate, rewrite, normalize, or simply pass through a DualResult.
type ResultStage interface {
	Process(ctx context.Context, result DualResult) (DualResult, error)
}

type NoopResultStage struct{}

func (NoopResultStage) Process(_ context.Context, result DualResult) (DualResult, error) {
	return result, nil
}

type SourceOnlyResultStage struct{}

func (SourceOnlyResultStage) Process(_ context.Context, result DualResult) (DualResult, error) {
	result.DstText = result.SrcText
	return result, nil
}
