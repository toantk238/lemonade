# File Copy/Paste Support

**Date:** 2026-05-08
**Branch:** feature/img_copy

## Overview

Extend lemonade's `copy`/`paste` commands to support files (images and any file type) in addition to text. When clients are on different machines, file bytes are transferred eagerly over the existing RPC connection. When the paste requester is the same machine that issued the copy, no transfer occurs.

## Goals

- `cat image.png | lemonade copy` copies a file via stdin
- Copying files in Nautilus (or any file manager), then running `lemonade copy` transfers those files
- `lemonade paste` on a different machine outputs temp file paths (one per line)
- `lemonade paste` on the same machine does nothing (files already local)
- Server caches the last copied files in memory with a configurable TTL
- Works through SSH port forwarding (no HTTP server required)

## Non-Goals

- Lazy/on-demand transfer (dropped — incompatible with SSH port forwarding)
- Writing files to disk on the server side
- Clipboard history (more than one copy entry)

## Architecture

The feature adds a parallel RPC path for files alongside the existing text path. No existing text behavior is modified.

```
Client (copy)          Server              Client (paste)
     │                    │                     │
     │─ CopyFile([]File) ─▶│                     │
     │                    │ store in memory      │
     │                    │ start TTL timer      │
     │                    │                     │
     │                    │◀─ PasteFile() ───────│
     │                    │ compare IPs          │
     │                    │─ {SameClient:true} ──▶│ (no-op)
     │                    │   OR                 │
     │                    │─ {Files:[...]} ───────▶│ save to /tmp, print paths
```

## New Types (`param` package)

```go
type FileEntry struct {
    Name  string
    Bytes []byte
}

type CopyFileParam struct {
    Files []FileEntry
}

type PasteFileResult struct {
    SameClient bool
    Files      []FileEntry
}
```

## Server Cache (`server/clipboard.go`)

```go
type imageCache struct {
    mu        sync.Mutex
    files     []FileEntry
    sourceIP  string
    expiresAt time.Time
    timer     *time.Timer
}
```

- `CopyFile`: acquires lock, cancels existing timer, stores files + sourceIP, starts TTL timer.
- TTL fires: clears files and sourceIP from memory.
- `PasteFile`: checks expiry, compares `conn.RemoteAddr()` IP to `sourceIP`.
  - Same IP → returns `{SameClient: true}` with no bytes.
  - Different IP → returns full `PasteFileResult` with file bytes.
  - Cache empty/expired → returns empty result (client falls through to text paste).
- Default TTL: 30 minutes. Configurable via `--image-cache-ttl` flag (Go duration string).
- Multiple clients can paste from the same cache entry (cache is not cleared on paste).

## Copy Flow (client)

### With stdin

```
read all bytes from stdin
  → check magic bytes:
      PNG:  \x89PNG
      JPEG: \xFF\xD8\xFF
      GIF:  GIF8
      BMP:  BM
      WebP: RIFF....WEBP
  → match → CopyFile RPC with FileEntry{Name: "clipboard.<ext>", Bytes: all bytes}
  → no match → existing Copy(string) RPC (text, unchanged)
```

### Without stdin (Nautilus / file manager copy)

```
read local clipboard for file URIs:
  X11:    xclip -t text/uri-list -o
  Wayland: wl-paste --type text/uri-list

  → URIs found → parse file:// paths → read each file bytes
                → CopyFile RPC with all FileEntry items (one per file)
  → no URIs   → existing Copy(string) RPC (text, unchanged)
```

## Paste Flow (client)

```
call PasteFile RPC
  → SameClient: true      → do nothing (files already on this machine)
  → files present         → for each file:
                              save to /tmp/<original-name>
                              (use /tmp/lemonade_<n>_<name> on collision)
                              print path to stdout (one per line)
  → empty/expired cache   → fall through to existing Clipboard.Paste (text)
```

## CLI Changes

### `lemon/cli.go`
Add field: `ImageCacheTTL time.Duration`

### `lemon/flag.go`
Add flag: `--image-cache-ttl=30m`

### `lemon/main.go`
- `copy`: detect stdin vs no-stdin, route to file or text path
- `paste`: try PasteFile first, fall through to text paste

## Same-Machine Detection

The server compares `conn.RemoteAddr().(*net.TCPAddr).IP.String()` at paste time against the IP recorded at copy time.

**Known limitation:** Through SSH port forwarding (`ssh -R`), all clients appear as `127.0.0.1`. Two different physical machines tunneling through the same SSH session cannot be distinguished by IP. In this scenario, the second client's paste will incorrectly return `SameClient: true`. This is an accepted limitation; users with multiple clients over SSH should use direct TCP connections.

## Dependencies

No new Go module dependencies required.

External tools used as subprocesses for reading file URIs from local clipboard:
- X11: `xclip` (typically pre-installed on Linux desktops)
- Wayland: `wl-clipboard` (`wl-paste`)

If neither is available, the Nautilus use case degrades gracefully: lemonade falls back to reading the clipboard as text.

## Debug Logging

Use the existing `log15` logger (already in use throughout the codebase) at appropriate levels:

| Event | Level | Message |
|---|---|---|
| Copy: detected image in stdin | Debug | `"copy: detected image from stdin" "ext" ".png" "size" 204800` |
| Copy: detected file URIs from clipboard | Debug | `"copy: detected file URIs from clipboard" "count" 3` |
| Copy: reading file from URI | Debug | `"copy: reading file" "path" "/home/user/photo.png" "size" 204800` |
| Copy: sending files over RPC | Info | `"copy: sending files" "count" 3 "total_bytes" 512000` |
| Server: received CopyFile | Info | `"server: cached files" "count" 3 "source_ip" "192.168.1.5" "ttl" "30m0s"` |
| Server: cache expired | Debug | `"server: image cache expired"` |
| Paste: SameClient detected | Debug | `"paste: same client, skipping transfer" "ip" "192.168.1.5"` |
| Paste: transferring files | Info | `"paste: receiving files" "count" 3` |
| Paste: file saved | Debug | `"paste: saved file" "path" "/tmp/photo.png"` |
| Paste: cache empty/expired | Debug | `"paste: no file cache, falling back to text"` |

Log level follows the existing `--log-level` flag (0=Debug, 4=Critical).

## Testing

- Unit test: magic byte detection covers all 5 formats + non-image fallback
- Unit test: `imageCache` TTL expiry clears files
- Unit test: `PasteFile` returns `SameClient: true` when IPs match
- Unit test: `PasteFile` returns bytes when IPs differ
- Unit test: multiple paste calls on same cache entry all succeed
- Integration test: copy via stdin → paste on same host (SameClient path)
- Integration test: copy via stdin → paste from different IP (transfer path)
- Integration test: multiple files → paste outputs one path per line
