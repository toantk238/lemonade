# Design: Server-side clipboard watcher

**Date:** 2026-05-11
**Branch:** feature/file_copy

## Problem

`lemonade server` passively waits for RPC calls from clients. There is no way for the server to automatically detect when the user copies files in Nautilus (or any app) — the user must explicitly run `lemonade copy` on the client side. Users expect Ctrl+C in Nautilus to "just work" without a separate command.

## Goals

1. When `lemonade server` starts, it also starts a clipboard watcher goroutine.
2. The watcher detects clipboard changes using the available system tool (Wayland, X11, macOS).
3. On file URI change → files are stored in `globalFileCache` directly (no RPC).
4. On text change → `globalFileCache` is cleared (text copy invalidates file cache).
5. If no clipboard tool is available (headless server), the watcher skips silently.

## Non-goals

- No new `lemonade watch` subcommand — watching is always attempted alongside the server.
- No new CLI flag — detection is automatic.
- No changes to the client side or RPC protocol.

## Design

### 1. New file: `server/watch.go`

Contains all watcher logic. Public entry point:

```go
func StartClipboardWatcher(logger log.Logger)
```

Called once as a goroutine from `server.Serve()` before the accept loop. It probes for available backends in order: Wayland → X11 → macOS. Runs the first one that succeeds. If none available, logs `"server: no clipboard watcher available"` at debug level and returns.

### 2. Backend: Wayland (`wl-paste --watch cat`)

Starts a long-lived subprocess: `wl-paste --type text/uri-list --watch cat`. Each time the clipboard changes, `wl-paste` invokes `cat` which prints the new content to stdout. The goroutine reads stdout line-by-line.

- Non-empty output → parse as URI list → read file bytes → `globalFileCache.store(entries, "")`
- Empty output (text copied) → `globalFileCache.clear()`

Probe: attempt `wl-paste --version`; if error → not available.

### 3. Backend: X11 (`xclip -l 1`)

Runs `xclip -l 1` in a loop. This command blocks until exactly one clipboard change occurs, then exits. After it exits, read `xclip -t text/uri-list -o` to get the new content.

- Non-empty URI list → read file bytes → `globalFileCache.store(entries, "")`
- Empty URI list → `globalFileCache.clear()`
- Loop forever until process exits.

Probe: attempt `xclip -version`; if error → not available.

### 4. Backend: macOS (`pbpaste` polling)

Ticks every 500ms. Runs `pbpaste` and compares SHA-256 hash of output to the last seen hash. On change → `globalFileCache.clear()` (macOS CLI tools do not expose file URIs from the clipboard, so file caching is not supported on this backend — text change detection only).

Probe: attempt `pbpaste`; if error → not available.

### 5. Integration in `server/server.go`

Add one line to `Serve()` before the accept loop:

```go
go StartClipboardWatcher(logger)
```

No other changes to `server.go`.

### 6. `globalFileCache.store()` for watcher

The watcher stores files with `clientID = ""` (empty string). This means `PasteFile` will never match as `sameClient` for any real client — files will always be transferred. This is the correct behaviour: the server copied locally, the paste goes to a remote client.

## Affected files

| File | Change |
|------|--------|
| `server/watch.go` | New file — `StartClipboardWatcher` + 3 backends |
| `server/server.go` | Add `go StartClipboardWatcher(logger)` in `Serve()` |

## Testing

- Unit test `parseURIList` (already tested in `lemon/fileurls_test.go` — reuse logic).
- Integration test for `watchX11` / `watchWayland` would require mocking subprocess output; skip for now. Verify manually by running server with `--log-level 0` and copying files in Nautilus.
- Confirm headless path: if `xclip`, `wl-paste`, `pbpaste` all absent, server starts cleanly.
