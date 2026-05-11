package server

import (
	"os"
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
)

func TestStartClipboardWatcherNoTools(t *testing.T) {
	// With PATH cleared, no clipboard tools are found — watcher must return without panic.
	orig := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", orig)

	done := make(chan struct{})
	go func() {
		StartClipboardWatcher(log.New())
		close(done)
	}()

	select {
	case <-done:
		// passed — returned cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("StartClipboardWatcher did not return within 2s when no tools available")
	}
}
