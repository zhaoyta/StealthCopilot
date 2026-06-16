package ui

// TeleprompterContent is the native teleprompter state mirrored from the main
// Wails event stream.
type TeleprompterContent struct {
	Subtitle string
	Answer   string
}

// TeleprompterWindow is a small native, capture-protected floating window.
type TeleprompterWindow interface {
	Show() error
	Hide() error
	SetAppearance(fontSize int, opacity float64)
	AppendSubtitle(text string)
	AppendAnswerToken(token string)
	FinishAnswer()
	SetError(message string)
	SetCircuitOpen(open bool)
	Reset()
	Close() error
	Available() bool
}

// NewTeleprompterWindow returns the best native implementation for the current
// platform, or a no-op fallback when unsupported.
func NewTeleprompterWindow() TeleprompterWindow {
	return newPlatformTeleprompterWindow()
}
