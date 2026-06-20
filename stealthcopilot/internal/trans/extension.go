// Package trans defines text transformation extensions between ASR and TTS.
package trans

import (
	"context"

	"github.com/zhaoyta/stealthcopilot/internal/asr"
)

// Extension is the text transform extension point between ASR and TTS.
// Implementations may translate, rewrite, normalize, or pass through a result.
type Extension interface {
	Process(ctx context.Context, result asr.Result) (asr.Result, error)
}

type NoopExtension struct{}

func (NoopExtension) Process(_ context.Context, result asr.Result) (asr.Result, error) {
	return result, nil
}

type SourceOnlyExtension struct{}

func (SourceOnlyExtension) Process(_ context.Context, result asr.Result) (asr.Result, error) {
	result.DstText = result.SrcText
	return result, nil
}
