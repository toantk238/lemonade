# WSL clip.exe Support Design

**Date:** 2026-04-21  
**Branch:** feature/fix-breakline  
**Status:** Approved

---

## Problem

On WSL (Windows Subsystem for Linux), `github.com/atotto/clipboard` relies on `xclip` or `xsel`, which require an X11 display server. Standard WSL environments do not have X11, so clipboard copy and paste operations fail silently or with an error.

## Goal

Make lemonade's clipboard operations work natively on WSL by using `clip.exe` (write) and `powershell.exe Get-Clipboard` (read) when running inside WSL, with no user configuration required.

---

## Design

### 1. WSL Detection — `lemon/wsl.go`

A new file exposes a single exported function:

```go
func IsWSL() bool
```

- Reads `/proc/version` once at program startup.
- Returns `true` if the content contains `"microsoft"` or `"WSL"` (case-insensitive).
- Result is cached via `sync.Once` to avoid repeated file reads.

This is the standard detection heuristic used by the broader Go/Linux ecosystem.

### 2. Clipboard Backend — `server/clipboard.go`

Both RPC methods (`Copy`, `Paste`) branch on `lemon.IsWSL()`:

**Copy (WSL path):**
- Create `exec.Command("clip.exe")`.
- Open stdin pipe, write text to it, close pipe.
- Wait for the command to exit.
- On error, return the error.

**Paste (WSL path):**
- Run `exec.Command("powershell.exe", "-command", "Get-Clipboard")`.
- Capture stdout, trim trailing newline.
- Return the result string.

**Non-WSL path:** unchanged — delegates to `github.com/atotto/clipboard` as before.

### 3. No changes required to

- `lemon/cli.go` — no new flags
- `client/client.go` — client is unaffected
- `lemon/flag.go` — no new options
- `go.mod` — no new dependencies

---

## Error Handling

- If `clip.exe` is not found (unlikely in WSL but possible), the error is returned to the RPC caller and logged by the server.
- If `powershell.exe` exits non-zero, the error is returned to the caller.
- No silent fallback — failure surfaces explicitly.

---

## Testing

- Unit test `IsWSL()` by mocking the `/proc/version` content (table-driven: WSL1, WSL2, plain Linux, macOS).
- Integration test for WSL clipboard path requires a real WSL environment; mark with a build tag or skip guard (`t.Skip` if not WSL).

---

## Scope

This change is intentionally narrow: WSL detection + two method branches in `server/clipboard.go` + one new file `lemon/wsl.go`. No refactoring of the clipboard abstraction, no new dependencies, no config changes.
