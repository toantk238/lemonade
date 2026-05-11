package server

import (
	"bufio"
	"crypto/sha256"
	"os"
	"os/exec"
	"time"

	log "github.com/inconshreveable/log15"

	"github.com/lemonade-command/lemonade/lemon"
)

// StartClipboardWatcher probes for an available clipboard tool and starts
// an event-driven or polling watcher that updates globalFileCache on change.
// Returns immediately if no clipboard tool is available (e.g. headless server).
func StartClipboardWatcher(logger log.Logger) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-paste"); err == nil {
			logger.Debug("server: starting Wayland clipboard watcher")
			watchWayland(logger)
			return
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		logger.Debug("server: starting X11 clipboard watcher")
		watchX11(logger)
		return
	}
	if _, err := exec.LookPath("pbpaste"); err == nil {
		logger.Debug("server: starting macOS clipboard watcher")
		watchMacOS(logger)
		return
	}
	logger.Debug("server: no clipboard watcher available")
}

// handleClipboardChange reads the current clipboard and updates globalFileCache.
// File URIs found → store; no file URIs → clear (text was copied).
func handleClipboardChange(logger log.Logger) {
	entries, err := lemon.ReadFileEntriesFromClipboard()
	if err != nil {
		logger.Error("server: watcher clipboard read error", "err", err)
		return
	}
	if len(entries) > 0 {
		logger.Info("server: watcher cached files from clipboard", "count", len(entries))
		globalFileCache.store(entries, "")
	} else {
		globalFileCache.clear()
	}
}

// watchWayland runs wl-paste --watch with a trigger command that writes one
// line to stdout per clipboard change. Each scanned line = one change event.
func watchWayland(logger log.Logger) {
	cmd := exec.Command("wl-paste", "--watch", "sh", "-c", "echo")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("server: wl-paste stdout pipe failed", "err", err)
		return
	}
	if err := cmd.Start(); err != nil {
		logger.Error("server: wl-paste --watch start failed", "err", err)
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		handleClipboardChange(logger)
	}
	cmd.Wait()
	logger.Warn("server: wl-paste --watch exited, clipboard watcher stopped")
}

// watchX11 loops xclip -l 1, which blocks until exactly one clipboard change,
// then returns. Each return = one change event.
func watchX11(logger log.Logger) {
	for {
		if err := exec.Command("xclip", "-l", "1").Run(); err != nil {
			logger.Warn("server: xclip watch stopped", "err", err)
			return
		}
		handleClipboardChange(logger)
	}
}

// watchMacOS polls pbpaste every 500ms and clears globalFileCache on any
// content change. File URI detection is not supported via macOS CLI tools.
func watchMacOS(logger log.Logger) {
	var lastHash [32]byte
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		out, err := exec.Command("pbpaste").Output()
		if err != nil {
			logger.Debug("server: pbpaste failed, stopping macOS watcher", "err", err)
			return
		}
		h := sha256.Sum256(out)
		if h != lastHash {
			lastHash = h
			globalFileCache.clear()
		}
	}
}
