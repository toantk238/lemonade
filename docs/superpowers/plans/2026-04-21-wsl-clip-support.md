# WSL clip.exe Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make lemonade clipboard operations work on WSL by auto-detecting WSL at runtime and routing copy/paste through `clip.exe` and `powershell.exe` instead of `atotto/clipboard`.

**Architecture:** A new `lemon/wsl.go` file exposes `IsWSL() bool`, which reads `/proc/version` once via `sync.Once` and checks for "microsoft"/"WSL". `server/clipboard.go` branches on `IsWSL()` in both `Copy` and `Paste` — WSL path shells out to `clip.exe`/`powershell.exe`, non-WSL path is unchanged.

**Tech Stack:** Go standard library only — `os`, `strings`, `sync`, `os/exec`. No new dependencies.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `lemon/wsl.go` | `IsWSL()` — WSL detection via `/proc/version` |
| Create | `lemon/wsl_test.go` | Unit tests for `IsWSL()` with mocked reader |
| Modify | `server/clipboard.go` | Branch `Copy`/`Paste` on `IsWSL()` |
| Create | `server/clipboard_test.go` | Unit tests for clipboard WSL branch (skipped on non-WSL) |

---

## Task 1: WSL detection — `lemon/wsl.go`

**Files:**
- Create: `lemon/wsl.go`
- Create: `lemon/wsl_test.go`

- [ ] **Step 1: Write the failing test**

Create `lemon/wsl_test.go`:

```go
package lemon

import (
	"strings"
	"testing"
)

func TestIsWSLFromReader(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		expected bool
	}{
		{"WSL2 kernel", "Linux version 5.15.90.1-microsoft-standard-WSL2", true},
		{"WSL1 kernel", "Linux version 4.4.0-19041-Microsoft", true},
		{"plain Linux", "Linux version 5.15.0-91-generic (buildd@lcy02-amd64-059)", false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isWSLFromReader(strings.NewReader(tc.content))
			if got != tc.expected {
				t.Errorf("isWSLFromReader(%q) = %v, want %v", tc.content, got, tc.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/rock/Workspace/lemonade && go test ./lemon/ -run TestIsWSLFromReader -v
```

Expected: compile error — `isWSLFromReader` undefined.

- [ ] **Step 3: Implement `lemon/wsl.go`**

Create `lemon/wsl.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/rock/Workspace/lemonade && go test ./lemon/ -run TestIsWSLFromReader -v
```

Expected:
```
=== RUN   TestIsWSLFromReader
=== RUN   TestIsWSLFromReader/WSL2_kernel
=== RUN   TestIsWSLFromReader/WSL1_kernel
=== RUN   TestIsWSLFromReader/plain_Linux
=== RUN   TestIsWSLFromReader/empty
--- PASS: TestIsWSLFromReader (0.00s)
PASS
```

- [ ] **Step 5: Verify the full lemon package still builds and passes**

```bash
cd /home/rock/Workspace/lemonade && go test ./lemon/ -v
```

Expected: all existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add lemon/wsl.go lemon/wsl_test.go
git commit -m "feat: add IsWSL() detection via /proc/version"
```

---

## Task 2: WSL clipboard branch — `server/clipboard.go`

**Files:**
- Modify: `server/clipboard.go`
- Create: `server/clipboard_test.go`

- [ ] **Step 1: Write the failing test**

Create `server/clipboard_test.go`:

```go
package server

import (
	"testing"

	"github.com/lemonade-command/lemonade/lemon"
)

func TestCopyWSL(t *testing.T) {
	if !lemon.IsWSL() {
		t.Skip("skipping WSL clipboard test: not running in WSL")
	}

	cb := &Clipboard{}
	err := cb.Copy("hello lemonade", nil)
	if err != nil {
		t.Fatalf("Copy via clip.exe failed: %v", err)
	}

	var result string
	err = cb.Paste(struct{}{}, &result)
	if err != nil {
		t.Fatalf("Paste via powershell failed: %v", err)
	}

	if result != "hello lemonade" {
		t.Errorf("Paste returned %q, want %q", result, "hello lemonade")
	}
}
```

- [ ] **Step 2: Run test to verify it skips on non-WSL or fails on WSL**

```bash
cd /home/rock/Workspace/lemonade && go test ./server/ -run TestCopyWSL -v
```

Expected on non-WSL: `--- SKIP: TestCopyWSL (skipping WSL clipboard test: not running in WSL)`  
Expected on WSL before implementation: FAIL (clip.exe not called yet).

- [ ] **Step 3: Add WSL copy/paste helpers to `server/clipboard.go`**

Replace the full contents of `server/clipboard.go` with:

```go
package server

import (
	"bytes"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/lemonade-command/lemonade/lemon"
)

type Clipboard struct{}

const clipboardTimeout = 200 * time.Millisecond

func (_ *Clipboard) Copy(text string, _ *struct{}) error {
	<-connCh
	text = lemon.ConvertLineEnding(text, LineEndingOpt)

	if lemon.IsWSL() {
		return wslCopy(text)
	}

	done := make(chan error, 1)
	go func() {
		done <- clipboard.WriteAll(text)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(clipboardTimeout):
		go func() {
			if err := <-done; err != nil {
				serverLogger.Error("clipboard write error", "err", err)
			}
		}()
		return nil
	}
}

func (_ *Clipboard) Paste(_ struct{}, resp *string) error {
	<-connCh
	if lemon.IsWSL() {
		out, err := wslPaste()
		if err != nil {
			return err
		}
		*resp = out
		return nil
	}
	t, err := clipboard.ReadAll()
	*resp = t
	return err
}

func wslCopy(text string) error {
	cmd := exec.Command("clip.exe")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

func wslPaste() (string, error) {
	out, err := exec.Command("powershell.exe", "-command", "Get-Clipboard").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
cd /home/rock/Workspace/lemonade && go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 5: Run all tests**

```bash
cd /home/rock/Workspace/lemonade && go test ./... -v
```

Expected: all existing tests pass; `TestCopyWSL` skips on non-WSL, passes on WSL.

- [ ] **Step 6: Commit**

```bash
git add server/clipboard.go server/clipboard_test.go
git commit -m "feat: use clip.exe and powershell for clipboard on WSL"
```

---

## Self-Review

**Spec coverage:**
- [x] `IsWSL()` via `/proc/version` with `sync.Once` cache — Task 1
- [x] `Copy` WSL path via `clip.exe` — Task 2 Step 3
- [x] `Paste` WSL path via `powershell.exe Get-Clipboard` — Task 2 Step 3
- [x] Non-WSL path unchanged — Task 2 Step 3 (atotto/clipboard still used)
- [x] Error surfaces explicitly, no silent fallback — Task 2 Step 3
- [x] No new dependencies — only `os/exec`, `bytes`, `strings` (all stdlib)

**Placeholder scan:** None found.

**Type consistency:** `IsWSL()` defined in Task 1, used in Task 2. `wslCopy`/`wslPaste` defined and called in same task. `Clipboard.Copy` signature `(text string, _ *struct{}) error` matches original. `Clipboard.Paste` signature `(_ struct{}, resp *string) error` matches original.
