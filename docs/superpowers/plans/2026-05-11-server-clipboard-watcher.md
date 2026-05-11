# Server Clipboard Watcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start a clipboard watcher goroutine alongside `lemonade server` so that copying files (or text) in Nautilus or any app automatically updates the server's file cache without requiring a manual `lemonade copy` command.

**Architecture:** A new `server/watch.go` file contains `StartClipboardWatcher(logger)` which probes for available clipboard tools (Wayland → X11 → macOS) and runs the matching event-driven or polling backend. Each backend calls `handleClipboardChange()` on every clipboard event, which reads file URIs via `lemon.ReadFileEntriesFromClipboard()` and directly updates `globalFileCache` — no RPC involved. `server.Serve()` launches the watcher as a goroutine before the accept loop.

**Tech Stack:** Go, `os/exec`, `bufio`, `crypto/sha256`, `github.com/lemonade-command/lemonade/lemon`, existing `globalFileCache` in `server/clipboard.go`

---

## File Map

| File | Change |
|------|--------|
| `server/watch.go` | New — `StartClipboardWatcher`, `handleClipboardChange`, `watchWayland`, `watchX11`, `watchMacOS` |
| `server/server.go` | Modify — add `go StartClipboardWatcher(logger)` before the accept loop in `Serve()` |

---

### Task 1: Create `server/watch.go`

**Files:**
- Create: `server/watch.go`

Context you need:
- `globalFileCache` is `*fileCache` defined in `server/clipboard.go` — it has `store(files []param.FileEntry, clientID string)` and `clear()` methods.
- `lemon.ReadFileEntriesFromClipboard()` is in `lemon/fileurls.go` — it runs `xclip -t text/uri-list -o` (X11) or `wl-paste --type text/uri-list` (Wayland) and returns `([]param.FileEntry, error)`. Returns `nil, nil` when no file URIs are found or no tool is available.
- `serverLogger` is the package-level `log.Logger` in `server/server.go` — set by `Serve()`. `watch.go` is in the same `server` package so it can use `serverLogger` directly.
- Module path: `github.com/lemonade-command/lemonade`

- [ ] **Step 1: Write a smoke test for `StartClipboardWatcher` — no tools available path**

Create `server/watch_test.go`:

```go
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
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd /mnt/Data/Workspace/1.CloneAndRun/lemonade
GOPATH=/tmp/gopath go test ./server/ -run TestStartClipboardWatcherNoTools -v
```

Expected: compile error — `StartClipboardWatcher` not defined.

- [ ] **Step 3: Write `server/watch.go`**

Create `/mnt/Data/Workspace/1.CloneAndRun/lemonade/server/watch.go` with this exact content:

```go
package server

import (
	"bufio"
	"crypto/sha256"
	"os/exec"
	"time"

	log "github.com/inconshreveable/log15"

	"github.com/lemonade-command/lemonade/lemon"
)

// StartClipboardWatcher probes for an available clipboard tool and starts
// an event-driven or polling watcher that updates globalFileCache on change.
// Returns immediately if no clipboard tool is available (e.g. headless server).
func StartClipboardWatcher(logger log.Logger) {
	if _, err := exec.LookPath("wl-paste"); err == nil {
		logger.Debug("server: starting Wayland clipboard watcher")
		watchWayland(logger)
		return
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
}

// watchX11 loops xclip -l 1, which blocks until exactly one clipboard change,
// then returns. Each return = one change event.
func watchX11(logger log.Logger) {
	for {
		if err := exec.Command("xclip", "-l", "1").Run(); err != nil {
			logger.Debug("server: xclip watch stopped", "err", err)
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
```

- [ ] **Step 4: Run tests**

```bash
GOPATH=/tmp/gopath go test ./server/ -run TestStartClipboardWatcherNoTools -v
```

Expected: PASS — `StartClipboardWatcher` returns immediately when PATH is empty.

- [ ] **Step 5: Run full server package tests**

```bash
GOPATH=/tmp/gopath go test ./server/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git -C /mnt/Data/Workspace/1.CloneAndRun/lemonade add server/watch.go server/watch_test.go
git -C /mnt/Data/Workspace/1.CloneAndRun/lemonade commit -m "feat: add server-side clipboard watcher (Wayland/X11/macOS)"
```

---

### Task 2: Wire `StartClipboardWatcher` into `server.Serve()`

**Files:**
- Modify: `server/server.go`

Context: `Serve()` currently accepts a `logger log.Logger` parameter and runs a `for` loop starting at line 42. The watcher goroutine must be started before the loop so it starts watching as soon as the server is up.

- [ ] **Step 1: Add the goroutine call in `server/server.go`**

In `server/server.go`, find the line that reads:

```go
	for {
		conn, err := l.Accept()
```

Add `go StartClipboardWatcher(logger)` immediately before it:

```go
	go StartClipboardWatcher(logger)

	for {
		conn, err := l.Accept()
```

- [ ] **Step 2: Build**

```bash
GOPATH=/tmp/gopath go build . 2>&1 && echo "BUILD OK"
```

Expected: `BUILD OK` with no other output.

- [ ] **Step 3: Run all tests**

```bash
GOPATH=/tmp/gopath go test ./lemon/ ./client/ ./param/ ./server/ -v 2>&1 | tail -20
```

Expected: all packages PASS.

- [ ] **Step 4: Commit**

```bash
git -C /mnt/Data/Workspace/1.CloneAndRun/lemonade add server/server.go
git -C /mnt/Data/Workspace/1.CloneAndRun/lemonade commit -m "feat: start clipboard watcher goroutine alongside server"
```
