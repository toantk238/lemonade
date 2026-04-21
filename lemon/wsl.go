package lemon

import (
	"io"
	"os"
	"strings"
	"sync"
)

var (
	wslOnce   sync.Once
	wslResult bool
)

// IsWSL reports whether the process is running inside Windows Subsystem for Linux.
func IsWSL() bool {
	wslOnce.Do(func() {
		f, err := os.Open("/proc/version")
		if err != nil {
			return
		}
		defer f.Close()
		wslResult = isWSLFromReader(f)
	})
	return wslResult
}

func isWSLFromReader(r io.Reader) bool {
	b, err := io.ReadAll(r)
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(b))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}
