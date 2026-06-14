package ui

import "errors"

// ErrTeleprompterUnavailable means the current platform cannot create the
// native floating teleprompter and callers should use the Wails view fallback.
var ErrTeleprompterUnavailable = errors.New("native teleprompter window unavailable")
