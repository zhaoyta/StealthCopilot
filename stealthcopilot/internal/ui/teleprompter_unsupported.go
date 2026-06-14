//go:build !darwin || !cgo

package ui

type noopTeleprompterWindow struct{}

func newPlatformTeleprompterWindow() TeleprompterWindow {
	return noopTeleprompterWindow{}
}

func (noopTeleprompterWindow) Show() error                { return ErrTeleprompterUnavailable }
func (noopTeleprompterWindow) Hide() error                { return nil }
func (noopTeleprompterWindow) AppendSubtitle(_ string)    {}
func (noopTeleprompterWindow) AppendAnswerToken(_ string) {}
func (noopTeleprompterWindow) FinishAnswer()              {}
func (noopTeleprompterWindow) Close() error               { return nil }
func (noopTeleprompterWindow) Available() bool            { return false }
