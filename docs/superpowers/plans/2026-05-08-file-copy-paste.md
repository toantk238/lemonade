# File Copy/Paste Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend lemonade `copy`/`paste` to transfer files (images and any file type) over RPC, with server-side in-memory cache, persistent client ID for same-machine detection, and Nautilus file URI support.

**Architecture:** Two new RPC methods (`Clipboard.CopyFile`, `Clipboard.PasteFile`) run parallel to the existing text path. The client detects image magic bytes in stdin or reads file URIs from the local clipboard (Nautilus). The server caches file bytes in memory with a TTL; same-machine detection uses a persistent UUID stored in `~/.config/lemonade.toml`.

**Tech Stack:** Go, `net/rpc` (existing), `crypto/rand` (stdlib UUID), `xclip`/`wl-paste` subprocess for Nautilus clipboard reading, `github.com/inconshreveable/log15` (existing logger).

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `param/param.go` | Add `FileEntry`, `CopyFileParam`, `PasteFileParam`, `PasteFileResult` |
| Create | `lemon/detect.go` | Magic byte detection for image formats |
| Create | `lemon/detect_test.go` | Tests for magic byte detection |
| Create | `lemon/clientid.go` | Generate + persist UUID in `~/.config/lemonade.toml` |
| Create | `lemon/clientid_test.go` | Tests for UUID persistence |
| Create | `lemon/fileurls.go` | Read file URIs from clipboard via xclip/wl-paste |
| Create | `lemon/fileurls_test.go` | Tests for URI parsing and file reading |
| Modify | `lemon/cli.go` | Add `RawData`, `IsFileData`, `StdinIsTTY`, `ImageCacheTTL`, `ClientID` |
| Modify | `lemon/flag.go` | Add `--image-cache-ttl` flag, handle TTY + raw byte detection |
| Modify | `lemon/flag_test.go` | Update assert helper for `ImageCacheTTL` default |
| Modify | `server/clipboard.go` | Add `fileCache` struct, `CopyFile` and `PasteFile` RPC handlers |
| Create | `server/clipboard_test.go` | Tests for file cache |
| Modify | `server/server.go` | Wire `ImageCacheTTL` into `globalFileCache` |
| Modify | `client/client.go` | Add `CopyFile` and `PasteFile` client methods |
| Modify | `main.go` | Dispatch file vs text copy/paste, `saveToTemp`, `StdinIsTTY` init, `ClientID` loading |

---

## Task 1: Add param types

**Files:**
- Modify: `param/param.go`

- [ ] **Step 1: Write the failing test**

Create `param/param_test.go`:

```go
package param

import "testing"

func TestFileEntryRoundTrip(t *testing.T) {
	e := FileEntry{Name: "img.png", Bytes: []byte{0x89, 'P', 'N', 'G'}}
	if e.Name != "img.png" || len(e.Bytes) != 4 {
		t.Errorf("unexpected FileEntry: %+v", e)
	}

	cp := CopyFileParam{
		Files:    []FileEntry{e},
		ClientID: "abc-123",
	}
	if len(cp.Files) != 1 || cp.ClientID != "abc-123" {
		t.Errorf("unexpected CopyFileParam: %+v", cp)
	}

	pp := PasteFileParam{ClientID: "abc-123"}
	res := PasteFileResult{SameClient: true, Files: nil}
	if !res.SameClient || pp.ClientID != "abc-123" {
		t.Errorf("unexpected paste types")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /mnt/Data/Workspace/1.CloneAndRun/lemonade && go test ./param/...
```

Expected: compile error — `FileEntry`, `CopyFileParam`, etc. undefined.

- [ ] **Step 3: Add types to `param/param.go`**

```go
package param

type OpenParam struct {
	URI           string
	TransLoopback bool
}

type FileEntry struct {
	Name  string
	Bytes []byte
}

type CopyFileParam struct {
	Files    []FileEntry
	ClientID string
}

type PasteFileParam struct {
	ClientID string
}

type PasteFileResult struct {
	SameClient bool
	Files      []FileEntry
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./param/...
```

Expected: `ok github.com/lemonade-command/lemonade/param`

- [ ] **Step 5: Commit**

```bash
git add param/param.go param/param_test.go
git commit -m "feat: add file copy/paste param types"
```

---

## Task 2: Magic byte detection

**Files:**
- Create: `lemon/detect.go`
- Create: `lemon/detect_test.go`

- [ ] **Step 1: Write the failing test**

Create `lemon/detect_test.go`:

```go
package lemon

import "testing"

func TestDetectFileExt(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantExt string
		wantOK  bool
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, ".png", true},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, ".jpg", true},
		{"gif87", []byte{'G', 'I', 'F', '8', '7', 'a'}, ".gif", true},
		{"gif89", []byte{'G', 'I', 'F', '8', '9', 'a'}, ".gif", true},
		{"bmp", []byte{'B', 'M', 0x00, 0x00, 0x00, 0x00}, ".bmp", true},
		{"webp", []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 0x00}, ".webp", true},
		{"text", []byte("hello world"), "", false},
		{"empty", []byte{}, "", false},
		{"short", []byte{0x89}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExt, gotOK := DetectFileExt(tt.input)
			if gotExt != tt.wantExt || gotOK != tt.wantOK {
				t.Errorf("DetectFileExt() = (%q, %v), want (%q, %v)", gotExt, gotOK, tt.wantExt, tt.wantOK)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./lemon/... -run TestDetectFileExt
```

Expected: compile error — `DetectFileExt` undefined.

- [ ] **Step 3: Implement `lemon/detect.go`**

```go
package lemon

// DetectFileExt returns the file extension if b begins with known image magic
// bytes. Returns ("", false) for non-image data or insufficient bytes.
func DetectFileExt(b []byte) (ext string, ok bool) {
	switch {
	case len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return ".png", true
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return ".jpg", true
	case len(b) >= 4 && b[0] == 'G' && b[1] == 'I' && b[2] == 'F' && b[3] == '8':
		return ".gif", true
	case len(b) >= 2 && b[0] == 'B' && b[1] == 'M':
		return ".bmp", true
	case len(b) >= 12 && b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P':
		return ".webp", true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./lemon/... -run TestDetectFileExt -v
```

Expected: all 9 subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add lemon/detect.go lemon/detect_test.go
git commit -m "feat: add image magic byte detection"
```

---

## Task 3: Persistent client ID

**Files:**
- Create: `lemon/clientid.go`
- Create: `lemon/clientid_test.go`

- [ ] **Step 1: Write the failing tests**

Create `lemon/clientid_test.go`:

```go
package lemon

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateClientID_Unique(t *testing.T) {
	id1, err := generateClientID()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := generateClientID()
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Error("generateClientID should produce unique IDs")
	}
	parts := strings.Split(id1, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 UUID parts, got %d in %q", len(parts), id1)
	}
}

func TestLoadOrCreateClientIDFromPath_CreatesAndPersists(t *testing.T) {
	f, err := os.CreateTemp("", "lemonade-test-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	id1, err := loadOrCreateClientIDFromPath(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if id1 == "" {
		t.Fatal("expected non-empty client ID")
	}

	id2, err := loadOrCreateClientIDFromPath(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("expected same ID on reload, got %q and %q", id1, id2)
	}
}

func TestLoadOrCreateClientIDFromPath_ReadsExisting(t *testing.T) {
	f, err := os.CreateTemp("", "lemonade-test-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("port = 1234\nclient-id = \"existing-uuid-abc\"\n")
	f.Close()
	defer os.Remove(f.Name())

	id, err := loadOrCreateClientIDFromPath(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if id != "existing-uuid-abc" {
		t.Errorf("expected existing-uuid-abc, got %q", id)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./lemon/... -run TestGenerateClientID -run TestLoadOrCreateClientID
```

Expected: compile error — `generateClientID`, `loadOrCreateClientIDFromPath` undefined.

- [ ] **Step 3: Implement `lemon/clientid.go`**

```go
package lemon

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// LoadOrCreateClientID returns the persistent machine UUID from
// ~/.config/lemonade.toml, generating and saving one if absent.
func LoadOrCreateClientID() (string, error) {
	path, err := homedir.Expand("~/.config/lemonade.toml")
	if err != nil {
		return generateClientID()
	}
	return loadOrCreateClientIDFromPath(path)
}

func loadOrCreateClientIDFromPath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "client-id") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					id := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if id != "" {
						return id, nil
					}
				}
			}
		}
	}

	id, err := generateClientID()
	if err != nil {
		return "", err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return id, nil // generated but not persisted — degraded gracefully
	}
	defer f.Close()
	fmt.Fprintf(f, "\nclient-id = %q\n", id)
	return id, nil
}

func generateClientID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // UUID v4 version bits
	b[8] = (b[8] & 0x3f) | 0x80 // UUID variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./lemon/... -run TestGenerateClientID -run TestLoadOrCreateClientID -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add lemon/clientid.go lemon/clientid_test.go
git commit -m "feat: add persistent client ID for same-machine detection"
```

---

## Task 4: Server file cache

**Files:**
- Modify: `server/clipboard.go`
- Create: `server/clipboard_test.go`

- [ ] **Step 1: Write the failing tests**

Create `server/clipboard_test.go`:

```go
package server

import (
	"testing"
	"time"

	"github.com/lemonade-command/lemonade/param"
)

func newTestCache(ttl time.Duration) *fileCache {
	return &fileCache{ttl: ttl}
}

func TestFileCache_DifferentClient_ReturnsFiles(t *testing.T) {
	fc := newTestCache(time.Minute)
	files := []param.FileEntry{{Name: "test.png", Bytes: []byte{0x89, 'P', 'N', 'G'}}}
	fc.store(files, "client-abc")

	got, sameClient, hasData := fc.get("client-def")
	if !hasData {
		t.Fatal("expected hasData=true")
	}
	if sameClient {
		t.Error("expected sameClient=false for different client ID")
	}
	if len(got) != 1 || got[0].Name != "test.png" {
		t.Errorf("unexpected files: %v", got)
	}
}

func TestFileCache_SameClient_ReturnsNoBytes(t *testing.T) {
	fc := newTestCache(time.Minute)
	fc.store([]param.FileEntry{{Name: "img.png", Bytes: []byte{1}}}, "same-id")

	got, sameClient, hasData := fc.get("same-id")
	if !hasData {
		t.Fatal("expected hasData=true")
	}
	if !sameClient {
		t.Error("expected sameClient=true")
	}
	if len(got) != 0 {
		t.Errorf("expected no bytes for same client, got %d files", len(got))
	}
}

func TestFileCache_TTLExpiry_ClearsCache(t *testing.T) {
	fc := newTestCache(20 * time.Millisecond)
	fc.store([]param.FileEntry{{Name: "img.png", Bytes: []byte{1}}}, "client-x")

	time.Sleep(60 * time.Millisecond)

	_, _, hasData := fc.get("other-client")
	if hasData {
		t.Error("expected cache to be expired")
	}
}

func TestFileCache_MultiplePastes_AllSucceed(t *testing.T) {
	fc := newTestCache(time.Minute)
	fc.store([]param.FileEntry{{Name: "img.png", Bytes: []byte{2}}}, "copier")

	for i := 0; i < 3; i++ {
		files, sameClient, hasData := fc.get("paster")
		if !hasData || sameClient || len(files) == 0 {
			t.Errorf("paste %d: hasData=%v sameClient=%v files=%d", i, hasData, sameClient, len(files))
		}
	}
}

func TestFileCache_NewCopyOverwritesPrevious(t *testing.T) {
	fc := newTestCache(time.Minute)
	fc.store([]param.FileEntry{{Name: "old.png", Bytes: []byte{1}}}, "client-a")
	fc.store([]param.FileEntry{{Name: "new.png", Bytes: []byte{2}}}, "client-b")

	files, _, hasData := fc.get("client-c")
	if !hasData || files[0].Name != "new.png" {
		t.Errorf("expected new.png, got %+v hasData=%v", files, hasData)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./server/... -run TestFileCache
```

Expected: compile error — `fileCache` undefined.

- [ ] **Step 3: Add `fileCache` and RPC handlers to `server/clipboard.go`**

Add after the existing imports and `Clipboard` struct. The full updated file:

```go
package server

import (
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/lemonade-command/lemonade/lemon"
	"github.com/lemonade-command/lemonade/param"
)

type Clipboard struct{}

const clipboardTimeout = 200 * time.Millisecond

func (_ *Clipboard) Copy(text string, _ *struct{}) error {
	<-connCh
	text = lemon.ConvertLineEnding(text, LineEndingOpt)

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
	t, err := clipboard.ReadAll()
	*resp = t
	return err
}

// fileCache holds the last file(s) sent via CopyFile in memory with a TTL.
type fileCache struct {
	mu             sync.Mutex
	files          []param.FileEntry
	sourceClientID string
	expiresAt      time.Time
	timer          *time.Timer
	ttl            time.Duration
}

var globalFileCache = &fileCache{ttl: 30 * time.Minute}

func (fc *fileCache) store(files []param.FileEntry, clientID string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.timer != nil {
		fc.timer.Stop()
	}
	fc.files = files
	fc.sourceClientID = clientID
	ttl := fc.ttl
	if ttl == 0 {
		ttl = 30 * time.Minute
	}
	fc.expiresAt = time.Now().Add(ttl)
	fc.timer = time.AfterFunc(ttl, func() {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		fc.files = nil
		fc.sourceClientID = ""
		serverLogger.Debug("server: image cache expired")
	})
}

// get returns (files, sameClient, hasData).
// sameClient=true means the requester is the source — no bytes are returned.
func (fc *fileCache) get(requesterClientID string) ([]param.FileEntry, bool, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.files) == 0 || time.Now().After(fc.expiresAt) {
		return nil, false, false
	}
	if fc.sourceClientID == requesterClientID {
		return nil, true, true
	}
	return fc.files, false, true
}

func (_ *Clipboard) CopyFile(p param.CopyFileParam, _ *struct{}) error {
	<-connCh
	serverLogger.Info("server: cached files",
		"count", len(p.Files), "client_id", p.ClientID, "ttl", globalFileCache.ttl)
	globalFileCache.store(p.Files, p.ClientID)
	return nil
}

func (_ *Clipboard) PasteFile(p param.PasteFileParam, resp *param.PasteFileResult) error {
	<-connCh
	files, sameClient, hasData := globalFileCache.get(p.ClientID)
	if !hasData {
		serverLogger.Debug("server: no file cache, falling back to text")
		return nil
	}
	if sameClient {
		serverLogger.Debug("server: same client, skipping transfer", "client_id", p.ClientID)
		resp.SameClient = true
		return nil
	}
	serverLogger.Info("server: sending files", "count", len(files))
	resp.Files = files
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./server/... -run TestFileCache -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Run full server tests to catch regressions**

```
go test ./server/...
```

Expected: `ok github.com/lemonade-command/lemonade/server`

- [ ] **Step 6: Commit**

```bash
git add server/clipboard.go server/clipboard_test.go
git commit -m "feat: add server-side file cache with TTL and CopyFile/PasteFile RPC handlers"
```

---

## Task 5: Wire `ImageCacheTTL` into server

**Files:**
- Modify: `server/server.go`

- [ ] **Step 1: Update `Serve()` in `server/server.go`**

Add TTL wiring after the existing `LineEndingOpt = c.LineEnding` line:

```go
func Serve(c *lemon.CLI, logger log.Logger) error {
	serverLogger = logger
	port := c.Port
	allowIP := c.Allow
	LineEndingOpt = c.LineEnding
	if c.ImageCacheTTL > 0 {
		globalFileCache.ttl = c.ImageCacheTTL
	}
	// ... rest unchanged
```

- [ ] **Step 2: Run all tests**

```
go test ./server/...
```

Expected: `ok github.com/lemonade-command/lemonade/server`

- [ ] **Step 3: Commit**

```bash
git add server/server.go
git commit -m "feat: wire ImageCacheTTL from CLI into file cache"
```

---

## Task 6: File URI reading (Nautilus support)

**Files:**
- Create: `lemon/fileurls.go`
- Create: `lemon/fileurls_test.go`

- [ ] **Step 1: Write the failing tests**

Create `lemon/fileurls_test.go`:

```go
package lemon

import (
	"os"
	"testing"
)

func TestParseURIList_Normal(t *testing.T) {
	input := "file:///home/user/a.png\n# comment\nfile:///home/user/b.pdf\n"
	got := parseURIList(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 URIs, got %d: %v", len(got), got)
	}
	if got[0] != "file:///home/user/a.png" {
		t.Errorf("got[0] = %q", got[0])
	}
	if got[1] != "file:///home/user/b.pdf" {
		t.Errorf("got[1] = %q", got[1])
	}
}

func TestParseURIList_Empty(t *testing.T) {
	if got := parseURIList(""); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
	if got := parseURIList("# only comments\n"); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestReadFileEntriesFromURIs(t *testing.T) {
	f, err := os.CreateTemp("", "lemonade-test-*.png")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A}
	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	entries, err := readFileEntriesFromURIs([]string{"file://" + f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if string(entries[0].Bytes) != string(content) {
		t.Error("file bytes mismatch")
	}
	if entries[0].Name == "" {
		t.Error("expected non-empty Name")
	}
}

func TestReadFileEntriesFromURIs_SkipsNonFileScheme(t *testing.T) {
	entries, err := readFileEntriesFromURIs([]string{"http://example.com/file.png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for http URI, got %d", len(entries))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./lemon/... -run TestParseURIList -run TestReadFileEntries
```

Expected: compile error — `parseURIList`, `readFileEntriesFromURIs` undefined.

- [ ] **Step 3: Implement `lemon/fileurls.go`**

```go
package lemon

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lemonade-command/lemonade/param"
)

// ReadFileEntriesFromClipboard reads file URIs from the system clipboard using
// xclip (X11) or wl-paste (Wayland), then reads each file's bytes.
// Returns nil, nil when no file URIs are found or no clipboard tool is available.
func ReadFileEntriesFromClipboard() ([]param.FileEntry, error) {
	out, err := exec.Command("xclip", "-t", "text/uri-list", "-o").Output()
	if err != nil {
		out, err = exec.Command("wl-paste", "--type", "text/uri-list").Output()
		if err != nil {
			return nil, nil
		}
	}
	uris := parseURIList(string(out))
	if len(uris) == 0 {
		return nil, nil
	}
	return readFileEntriesFromURIs(uris)
}

func readFileEntriesFromURIs(uris []string) ([]param.FileEntry, error) {
	var entries []param.FileEntry
	for _, uri := range uris {
		u, err := url.Parse(uri)
		if err != nil || u.Scheme != "file" {
			continue
		}
		b, err := os.ReadFile(u.Path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", u.Path, err)
		}
		entries = append(entries, param.FileEntry{
			Name:  filepath.Base(u.Path),
			Bytes: b,
		})
	}
	return entries, nil
}

func parseURIList(output string) []string {
	var uris []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		uris = append(uris, line)
	}
	return uris
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./lemon/... -run TestParseURIList -run TestReadFileEntries -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add lemon/fileurls.go lemon/fileurls_test.go
git commit -m "feat: add Nautilus file URI clipboard reading"
```

---

## Task 7: CLI struct and flag updates

**Files:**
- Modify: `lemon/cli.go`
- Modify: `lemon/flag.go`
- Modify: `lemon/flag_test.go`

- [ ] **Step 1: Update `lemon/cli.go`** — add new fields to `CLI` struct

```go
package lemon

import (
	"io"
	"time"
)

type CommandType int

const (
	OPEN CommandType = iota + 1
	COPY
	PASTE
	SERVER
)

const (
	Success        = 0
	FlagParseError = iota + 10
	RPCError
	Help
)

type CommandStyle int

const (
	ALIAS CommandStyle = iota + 1
	SUBCOMMAND
)

type CLI struct {
	In       io.Reader
	Out, Err io.Writer

	Type       CommandType
	DataSource string
	RawData    []byte // set when stdin contains binary file data
	IsFileData bool   // true when RawData is populated from stdin

	// options
	Port           int
	Allow          string
	Host           string
	TransLoopback  bool
	TransLocalfile bool
	LineEnding     string
	LogLevel       int
	Timeout        time.Duration

	Help bool

	NoFallbackMessages bool
	TrimNewline        bool
	StdinIsTTY         bool // set in main before FlagParse; true when stdin is a terminal
	ImageCacheTTL      time.Duration
	ClientID           string
}
```

- [ ] **Step 2: Update `lemon/flag.go`** — add `--image-cache-ttl` flag and handle binary stdin detection

Replace the `flags()` method and the `else` branch of `parse()`:

In `flags()`, add after the existing `flags.BoolVar(&c.TrimNewline, ...)` line:
```go
flags.DurationVar(&c.ImageCacheTTL, "image-cache-ttl", 30*time.Minute, "Server-side file cache TTL")
```

Replace the `else` block at the end of `parse()` (currently `b, err := ioutil.ReadAll(c.In)...`) with:

```go
} else if c.StdinIsTTY {
    // stdin is a terminal — no piped data; Nautilus path handled in main.go
} else {
    b, err := ioutil.ReadAll(c.In)
    if err != nil {
        return err
    }
    if _, ok := DetectFileExt(b); ok {
        c.RawData = b
        c.IsFileData = true
    } else {
        c.DataSource = string(b)
        if c.TrimNewline {
            c.DataSource = strings.TrimSuffix(c.DataSource, "\n")
        }
    }
}
```

- [ ] **Step 3: Update `lemon/flag_test.go`** — handle `ImageCacheTTL` default in assert helper

In `TestCLIParse`, update the assert helper to add default handling:

```go
const defaultCacheTTL = 30 * time.Minute
assert := func(args []string, expected CLI) {
    expected.In = os.Stdin
    if expected.Timeout == 0 {
        expected.Timeout = defaultTimeout
    }
    if expected.ImageCacheTTL == 0 {
        expected.ImageCacheTTL = defaultCacheTTL
    }
    c := &CLI{In: os.Stdin}
    c.FlagParse(args, true)

    if !reflect.DeepEqual(expected, *c) {
        t.Errorf("Expected:\n %+v, but got\n %+v", expected, c)
    }
}
```

- [ ] **Step 4: Run all lemon tests to verify nothing broke**

```
go test ./lemon/...
```

Expected: `ok github.com/lemonade-command/lemonade/lemon`

- [ ] **Step 5: Run full test suite**

```
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add lemon/cli.go lemon/flag.go lemon/flag_test.go
git commit -m "feat: add RawData, IsFileData, StdinIsTTY, ImageCacheTTL, ClientID to CLI"
```

---

## Task 8: Client CopyFile and PasteFile

**Files:**
- Modify: `client/client.go`

- [ ] **Step 1: Add `CopyFile` and `PasteFile` to `client/client.go`**

Add after the existing `Copy` method:

```go
func (c *client) CopyFile(files []param.FileEntry, clientID string) error {
	totalBytes := 0
	for _, f := range files {
		totalBytes += len(f.Bytes)
	}
	c.logger.Info("copy: sending files", "count", len(files), "total_bytes", totalBytes)
	return c.withRPCClient(func(rc *rpc.Client) error {
		p := param.CopyFileParam{Files: files, ClientID: clientID}
		return rc.Call("Clipboard.CopyFile", p, &struct{}{})
	})
}

func (c *client) PasteFile(clientID string) ([]param.FileEntry, bool, error) {
	var resp param.PasteFileResult
	err := c.withRPCClient(func(rc *rpc.Client) error {
		p := param.PasteFileParam{ClientID: clientID}
		return rc.Call("Clipboard.PasteFile", p, &resp)
	})
	return resp.Files, resp.SameClient, err
}
```

Also add `"github.com/lemonade-command/lemonade/param"` to the import block if not already present.

- [ ] **Step 2: Build to verify compilation**

```
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add client/client.go
git commit -m "feat: add CopyFile and PasteFile client methods"
```

---

## Task 9: Main dispatch wiring

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add `saveToTemp` helper to `main.go`**

Add before the `main()` function:

```go
import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/inconshreveable/log15"

	"github.com/lemonade-command/lemonade/client"
	"github.com/lemonade-command/lemonade/lemon"
	"github.com/lemonade-command/lemonade/param"
	"github.com/lemonade-command/lemonade/server"
)

func saveToTemp(f param.FileEntry) (string, error) {
	path := filepath.Join(os.TempDir(), f.Name)
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(f.Name)
		base := f.Name[:len(f.Name)-len(ext)]
		for i := 1; ; i++ {
			path = filepath.Join(os.TempDir(), fmt.Sprintf("%s_%d%s", base, i, ext))
			if _, err := os.Stat(path); os.IsNotExist(err) {
				break
			}
		}
	}
	return path, os.WriteFile(path, f.Bytes, 0644)
}
```

- [ ] **Step 2: Update `main()` to set `StdinIsTTY`**

```go
func main() {
	fi, _ := os.Stdin.Stat()
	stdinIsTTY := (fi.Mode() & os.ModeCharDevice) != 0

	cli := &lemon.CLI{
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		StdinIsTTY: stdinIsTTY,
	}
	os.Exit(Do(cli, os.Args))
}
```

- [ ] **Step 3: Update `Do()` to load `ClientID` and dispatch file copy/paste**

Replace the `Do` function body:

```go
func Do(c *lemon.CLI, args []string) int {
	logger := log.New()
	logger.SetHandler(log.LvlFilterHandler(log.LvlError, log.StdoutHandler))

	if err := c.FlagParse(args, false); err != nil {
		writeError(c, err)
		return lemon.FlagParseError
	}

	logLevel := logLevelMap[c.LogLevel]
	logger.SetHandler(log.LvlFilterHandler(logLevel, log.StdoutHandler))

	if c.Help {
		fmt.Fprint(c.Err, lemon.Usage)
		return lemon.Help
	}

	if clientID, err := lemon.LoadOrCreateClientID(); err == nil {
		c.ClientID = clientID
	}

	lc := client.New(c, logger)
	var err error

	switch c.Type {
	case lemon.OPEN:
		logger.Debug("Opening URL")
		err = lc.Open(c.DataSource, c.TransLocalfile, c.TransLoopback)

	case lemon.COPY:
		if c.IsFileData {
			ext, _ := lemon.DetectFileExt(c.RawData)
			logger.Debug("copy: detected image from stdin", "ext", ext, "size", len(c.RawData))
			entry := param.FileEntry{Name: "clipboard" + ext, Bytes: c.RawData}
			err = lc.CopyFile([]param.FileEntry{entry}, c.ClientID)
		} else if c.StdinIsTTY {
			logger.Debug("copy: no stdin, reading file URIs from clipboard")
			var entries []param.FileEntry
			entries, err = lemon.ReadFileEntriesFromClipboard()
			if err == nil && len(entries) > 0 {
				logger.Debug("copy: detected file URIs from clipboard", "count", len(entries))
				err = lc.CopyFile(entries, c.ClientID)
			} else if err == nil {
				logger.Debug("copy: no file URIs, falling back to text")
				err = lc.Copy(c.DataSource)
			}
		} else {
			logger.Debug("Copying text")
			err = lc.Copy(c.DataSource)
		}

	case lemon.PASTE:
		logger.Debug("Pasting")
		var handled bool
		if c.ClientID != "" {
			var files []param.FileEntry
			var sameClient bool
			files, sameClient, err = lc.PasteFile(c.ClientID)
			if err == nil && sameClient {
				logger.Debug("paste: same client, skipping transfer", "client_id", c.ClientID)
				handled = true
			} else if err == nil && len(files) > 0 {
				logger.Info("paste: receiving files", "count", len(files))
				handled = true
				for _, f := range files {
					var path string
					path, err = saveToTemp(f)
					if err != nil {
						break
					}
					logger.Debug("paste: saved file", "path", path)
					fmt.Fprintln(c.Out, path)
				}
			} else {
				err = nil // reset RPC error; fall through to text paste
			}
		}
		if !handled && err == nil {
			var text string
			text, err = lc.Paste()
			if err == nil {
				c.Out.Write([]byte(text))
			}
		}

	case lemon.SERVER:
		logger.Debug("Starting Server")
		err = server.Serve(c, logger)

	default:
		panic("Unreachable code")
	}

	if err != nil {
		writeError(c, err)
		return lemon.RPCError
	}
	return lemon.Success
}
```

- [ ] **Step 4: Build to verify compilation**

```
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Run full test suite**

```
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Smoke test — text copy/paste still works**

```bash
echo "hello lemonade" | go run . copy
go run . paste
```

Expected: `hello lemonade` printed to stdout (via local fallback).

- [ ] **Step 7: Smoke test — image copy via stdin**

```bash
# Create a minimal valid PNG (1x1 pixel)
printf '\x89PNG\r\n\x1a\n' > /tmp/test.png
cat /tmp/test.png | go run . copy
go run . paste
```

Expected: a path like `/tmp/clipboard.png` printed (via local fallback, same-machine detection returns SameClient=true... wait).

Actually with local fallback (ServeLocal), the client connects to `localhost`. Both copy and paste have the same `ClientID` → `SameClient=true` → paste does nothing (correct: same machine). Verify this is the expected behavior.

- [ ] **Step 8: Commit**

```bash
git add main.go
git commit -m "feat: wire file copy/paste dispatch in main, add saveToTemp"
```

---

## Task 10: Update Usage string

**Files:**
- Modify: `lemon/main.go`

- [ ] **Step 1: Update the `Usage` string to document `--image-cache-ttl`**

In `lemon/main.go`, update the `Usage` variable to add the new flag:

```go
var Usage = fmt.Sprintf(`Usage: lemonade [options]... SUB_COMMAND [arg]
Sub Commands:
  open [URL]                  Open URL by browser
  copy [text]                 Copy text or file (auto-detected from stdin).
  paste                       Paste text or file path.
  server                      Start lemonade server.

Options:
  --port=2489                 TCP port number
  --line-ending               Convert Line Ending (CR/CRLF)
  --allow="0.0.0.0/0,::/0"    Allow IP Range                [Server only]
  --host="localhost"          Destination hostname          [Client only]
  --no-fallback-messages      Do not show fallback messages [Client only]
  --trim-newline              Trim trailing newline from stdin input [copy only]
  --image-cache-ttl=30m       File cache TTL on server      [Server only]
  --trans-loopback=true       Translate loopback address    [open subcommand only]
  --trans-localfile=true      Translate local file path     [open subcommand only]
  --log-level=1               Log level                     [4 = Critical, 0 = Debug]
  --help                      Show this message


Version:
  %s`, Version)
```

- [ ] **Step 2: Run full test suite one final time**

```
go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Final commit**

```bash
git add lemon/main.go
git commit -m "docs: update usage string for file copy/paste and --image-cache-ttl"
```
