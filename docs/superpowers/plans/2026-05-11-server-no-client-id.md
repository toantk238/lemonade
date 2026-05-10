# Server No-Client-ID Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skip client-id loading when starting as server, and add an optional `--client-id` CLI flag for client commands.

**Architecture:** Add `ClientID string` to the `CLI` struct and register the `--client-id` flag. Update `client.New()` to check the flag value first, then the env var, then the config file. Move `client.New()` inside the `OPEN`/`COPY`/`PASTE` switch cases in `main.go` so the `SERVER` case never constructs a client at all.

**Tech Stack:** Go, standard `flag` package, `github.com/mitchellh/go-homedir`, `github.com/monochromegane/conflag`

---

## File Map

| File | Change |
|------|--------|
| `lemon/cli.go` | Add `ClientID string` field to `CLI` struct |
| `lemon/flag.go` | Register `--client-id` flag (empty string default) |
| `lemon/flag_test.go` | Add test case asserting `--client-id` is captured |
| `client/client.go` | Check `c.ClientID` first in resolution chain |
| `client/client_test.go` | New file — test flag > env priority |
| `main.go` | Move `client.New()` inside `OPEN`/`COPY`/`PASTE` cases |

---

### Task 1: Add `ClientID` field and `--client-id` flag

**Files:**
- Modify: `lemon/cli.go`
- Modify: `lemon/flag.go`
- Modify: `lemon/flag_test.go`

- [ ] **Step 1: Write the failing test**

In `lemon/flag_test.go`, add this case inside `TestCLIParse`, after the last existing `assert(...)` call (before the closing `}`):

```go
assert([]string{"lemonade", "copy", "--client-id", "test-uuid-1234", "hello"}, CLI{
    Type:           COPY,
    Host:           defaultHost,
    Port:           defaultPort,
    Allow:          defaultAllow,
    DataSource:     "hello",
    ClientID:       "test-uuid-1234",
    TransLoopback:  true,
    TransLocalfile: true,
    LogLevel:       defaultLogLevel,
})
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd /mnt/Data/Workspace/1.CloneAndRun/lemonade
go test ./lemon/ -run TestCLIParse -v
```

Expected: FAIL — `CLI` has no `ClientID` field, compile error.

- [ ] **Step 3: Add `ClientID` to the `CLI` struct**

In `lemon/cli.go`, add `ClientID string` to the `CLI` struct, after `TrimNewline bool`:

```go
TrimNewline        bool
StdinIsTTY         bool
ImageCacheTTL      time.Duration
ClientID           string
```

- [ ] **Step 4: Register the `--client-id` flag**

In `lemon/flag.go`, add this line inside `flags()`, after the `flags.DurationVar(&c.Timeout, ...)` line:

```go
flags.StringVar(&c.ClientID, "client-id", "", "Override client ID (client commands only)")
```

- [ ] **Step 5: Run the test to confirm it passes**

```bash
go test ./lemon/ -run TestCLIParse -v
```

Expected: PASS.

- [ ] **Step 6: Run the full lemon package tests**

```bash
go test ./lemon/ -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add lemon/cli.go lemon/flag.go lemon/flag_test.go
git commit -m "feat: add --client-id CLI flag to CLI struct"
```

---

### Task 2: Update `client.New()` priority chain

**Files:**
- Modify: `client/client.go`
- Create: `client/client_test.go`

- [ ] **Step 1: Write the failing tests**

Create `client/client_test.go` with this content:

```go
package client

import (
	"os"
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/lemonade-command/lemonade/lemon"
)

func testLogger() log.Logger {
	l := log.New()
	l.SetHandler(log.DiscardHandler())
	return l
}

func TestClientNewFlagTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv("LEMONADE_CLIENT_ID", "env-id")
	c := &lemon.CLI{
		Host:          "localhost",
		Port:          2489,
		Timeout:       100 * time.Millisecond,
		ImageCacheTTL: 30 * time.Minute,
		ClientID:      "flag-id",
	}
	cl := New(c, testLogger())
	if cl.clientID != "flag-id" {
		t.Errorf("expected clientID %q, got %q", "flag-id", cl.clientID)
	}
}

func TestClientNewEnvUsedWhenFlagEmpty(t *testing.T) {
	t.Setenv("LEMONADE_CLIENT_ID", "env-id")
	c := &lemon.CLI{
		Host:          "localhost",
		Port:          2489,
		Timeout:       100 * time.Millisecond,
		ImageCacheTTL: 30 * time.Minute,
		ClientID:      "",
	}
	cl := New(c, testLogger())
	if cl.clientID != "env-id" {
		t.Errorf("expected clientID %q, got %q", "env-id", cl.clientID)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./client/ -run "TestClientNew" -v
```

Expected: FAIL — `cl.clientID` is `"env-id"` in first test (env wins instead of flag).

- [ ] **Step 3: Update `client.New()` to check flag first**

Replace the `clientID` resolution block in `client/client.go` (lines 30–33):

```go
// Before:
clientID := os.Getenv("LEMONADE_CLIENT_ID")
if clientID == "" {
    clientID, _ = lemon.LoadOrCreateClientID()
}
```

```go
// After:
clientID := c.ClientID
if clientID == "" {
    clientID = os.Getenv("LEMONADE_CLIENT_ID")
}
if clientID == "" {
    clientID, _ = lemon.LoadOrCreateClientID()
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

```bash
go test ./client/ -run "TestClientNew" -v
```

Expected: both PASS.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add client/client.go client/client_test.go
git commit -m "feat: check --client-id flag before env var in client.New()"
```

---

### Task 3: Move `client.New()` inside OPEN/COPY/PASTE cases in `main.go`

**Files:**
- Modify: `main.go`

The SERVER case must call `server.Serve(c, logger)` directly with no `client.New()` call. The `lc` variable is local to each case so declare it with `:=` inside each case.

- [ ] **Step 1: Replace the pre-switch `lc` declaration and restructure `Do()`**

In `main.go`, remove line 69 (`lc := client.New(c, logger)`) and add `lc := client.New(c, logger)` as the first line inside each of the `OPEN`, `COPY`, and `PASTE` cases. The `SERVER` case is left unchanged. The full updated switch block should look like this:

```go
var err error

switch c.Type {
case lemon.OPEN:
    lc := client.New(c, logger)
    logger.Debug("Opening URL")
    err = lc.Open(c.DataSource, c.TransLocalfile, c.TransLoopback)

case lemon.COPY:
    lc := client.New(c, logger)
    if c.IsFileData {
        ext, _ := lemon.DetectFileExt(c.RawData)
        logger.Debug("copy: detected image from stdin", "ext", ext, "size", len(c.RawData))
        entry := param.FileEntry{Name: "clipboard" + ext, Bytes: c.RawData}
        err = lc.CopyFile([]param.FileEntry{entry})
    } else if c.StdinIsTTY {
        logger.Debug("copy: no stdin, reading file URIs from clipboard")
        var entries []param.FileEntry
        entries, err = lemon.ReadFileEntriesFromClipboard()
        if err == nil && len(entries) > 0 {
            logger.Debug("copy: detected file URIs from clipboard", "count", len(entries))
            err = lc.CopyFile(entries)
        } else if err == nil {
            logger.Debug("copy: no file URIs, falling back to text")
            err = lc.Copy(c.DataSource)
        }
    } else {
        logger.Debug("Copying text")
        err = lc.Copy(c.DataSource)
    }

case lemon.PASTE:
    lc := client.New(c, logger)
    logger.Debug("Pasting")
    var handled bool
    var files []param.FileEntry
    var sameClient bool
    files, sameClient, err = lc.PasteFile()
    if err == nil && sameClient {
        logger.Debug("paste: same client, skipping transfer")
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
```

- [ ] **Step 2: Verify the build is clean**

```bash
go build ./...
```

Expected: exits 0 with no output.

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "refactor: move client.New() into OPEN/COPY/PASTE cases, server needs no client"
```
