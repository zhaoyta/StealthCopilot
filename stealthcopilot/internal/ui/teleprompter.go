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
	AppendSubtitle(text string)
	AppendAnswerToken(token string)
	FinishAnswer()
	Close() error
	Available() bool
}

// NewTeleprompterWindow returns the best native implementation for the current
// platform, or a no-op fallback when unsupported.
func NewTeleprompterWindow() TeleprompterWindow {
	return newPlatformTeleprompterWindow()
}
