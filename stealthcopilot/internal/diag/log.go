package diag

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	logger  *log.Logger
	logFile *os.File
	logPath string
)

const maxLogBytes int64 = 2 * 1024 * 1024

// Init prepares the local diagnostic log file. It is safe to call more than once.
func Init(dataDir string) string {
	mu.Lock()
	defer mu.Unlock()

	path := filepath.Join(dataDir, "diagnostics.log")
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogBytes {
		_ = os.Rename(path, filepath.Join(dataDir, "diagnostics.old.log"))
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		logger = log.New(os.Stderr, "[diag] ", log.LstdFlags|log.Lmicroseconds)
		logPath = path
		logger.Printf("diagnostic log file unavailable: %v", err)
		return path
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = f
	logPath = path
	logger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("app diagnostics started path=%s", path)
	return path
}

func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

func Infof(format string, args ...any) {
	write("INFO", format, args...)
}

func Warnf(format string, args ...any) {
	write("WARN", format, args...)
}

func Errorf(format string, args ...any) {
	write("ERROR", format, args...)
}

func write(level, format string, args ...any) {
	mu.Lock()
	l := logger
	mu.Unlock()
	if l == nil {
		l = log.New(os.Stderr, "[diag] ", log.LstdFlags|log.Lmicroseconds)
	}
	l.Printf("%s %s", level, fmt.Sprintf(format, args...))
}

func Since(start time.Time) string {
	return time.Since(start).Truncate(time.Millisecond).String()
}
