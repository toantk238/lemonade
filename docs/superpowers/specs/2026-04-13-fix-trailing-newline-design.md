# Design: Strip Trailing Newline from Stdin Input

**Date:** 2026-04-13
**Branch:** feature/fix-breakline

## Problem

When piping shell commands into `lemonade copy`, the output includes a trailing `\n` added by Unix convention (e.g. `pwd`, `echo`). This newline ends up in the clipboard, causing an unwanted extra line when pasting.

```bash
pwd | lemonade copy   # clipboard gets "/home/user/project\n" instead of "/home/user/project"
```

## Goal

Strip exactly one trailing `\n` from stdin input before storing it as `DataSource`. If no trailing `\n` is present, do nothing.

## Change

**File:** `lemon/flag.go`, line 124

```go
// Before
c.DataSource = string(b)

// After
c.DataSource = strings.TrimSuffix(string(b), "\n")
```

Add `"strings"` to the import block if not already present.

## Design Decisions

- **`TrimSuffix` not `TrimRight`**: Removes exactly one trailing `\n`. Preserves intentional multiple trailing newlines (e.g. `printf "a\n\n"` becomes `a\n`, not `a`).
- **Only `\n`, not `\r\n`**: Windows CRLF handling is already covered by the existing `--line-ending` flag. Stripping `\r\n` as a unit would require more complex logic and is out of scope.
- **Stdin only**: The fix is applied at the `ioutil.ReadAll` path. Direct CLI arguments (`lemonade copy "text"`) take a separate code path and are not affected.
- **Trim at read time, not write time**: Trimming in `flag.go` (client side) rather than `server/clipboard.go` (server side) keeps the fix local to the input path and avoids side effects on remote RPC callers.

## Behavior Matrix

| Input | Before | After |
|---|---|---|
| `pwd \| lemonade copy` | `path\n` | `path` |
| `echo "hello" \| lemonade copy` | `hello\n` | `hello` |
| `printf "a\nb\n" \| lemonade copy` | `a\nb\n` | `a\nb` |
| `printf "a\nb\n\n" \| lemonade copy` | `a\nb\n\n` | `a\nb\n` |
| `printf "text" \| lemonade copy` | `text` | `text` (no-op) |
| `lemonade copy "text"` (direct arg) | `text` | `text` (unaffected) |

## Core Features Verified Unaffected

- **Direct CLI argument path** — uses `c.DataSource = arg` at `flag.go:119`, not touched
- **`--line-ending` conversion** — runs server-side after this point, unaffected
- **Remote RPC** — trimming happens on the sender before any network call
- **`lemonade paste` / `lemonade open`** — completely separate code paths

## Testing

Add a test in `lemon/flag_test.go` covering:
- Stdin with trailing `\n` → trimmed
- Stdin without trailing `\n` → unchanged
- Stdin with multiple trailing `\n` → exactly one stripped
