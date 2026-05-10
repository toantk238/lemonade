# Design: Server mode does not require client-id

**Date:** 2026-05-11
**Branch:** feature/file_copy

## Problem

`main.go` calls `client.New(c, logger)` unconditionally before the command switch, even when the subcommand is `server`. `client.New()` always runs `lemon.LoadOrCreateClientID()`, which reads and potentially writes `~/.config/lemonade.toml`. The `SERVER` case never uses the resulting client struct. This causes unnecessary file I/O and side effects when starting the server.

Additionally, there is no `--client-id` CLI flag; the only overrides are the `LEMONADE_CLIENT_ID` env var and the config file.

## Goals

1. Starting `lemonade server` does not trigger client-id file I/O.
2. Clients can override their client-id via a `--client-id` CLI flag.
3. Existing env var and config-file resolution is preserved, unchanged.

## Non-goals

- Changing server-side client-id handling (the server already receives client-id from RPC params).
- Adding validation or enforcement of client-id format.

## Design

### 1. `CLI` struct (`lemon/cli.go`)

Add one field:

```go
ClientID string
```

### 2. Flag registration (`lemon/flag.go`)

Register in `flags()`:

```go
flags.StringVar(&c.ClientID, "client-id", "", "Override client ID (client commands only)")
```

Default is empty string — always optional, no effect on server.

### 3. `client.New()` priority chain (`client/client.go`)

Resolve `clientID` in order:

1. `c.ClientID` — the new `--client-id` flag (if non-empty)
2. `LEMONADE_CLIENT_ID` env var (existing behaviour)
3. `lemon.LoadOrCreateClientID()` (existing behaviour)

```go
func New(c *lemon.CLI, logger log.Logger) *client {
    clientID := c.ClientID
    if clientID == "" {
        clientID = os.Getenv("LEMONADE_CLIENT_ID")
    }
    if clientID == "" {
        clientID, _ = lemon.LoadOrCreateClientID()
    }
    return &client{ ..., clientID: clientID }
}
```

### 4. `main.go` restructure

Remove the pre-switch `lc := client.New(c, logger)` call. Move it inside each of the `OPEN`, `COPY`, and `PASTE` cases. The `SERVER` case calls `server.Serve(c, logger)` directly with no client construction.

```go
switch c.Type {
case lemon.OPEN:
    lc := client.New(c, logger)
    err = lc.Open(...)

case lemon.COPY:
    lc := client.New(c, logger)
    // ... existing copy logic ...

case lemon.PASTE:
    lc := client.New(c, logger)
    // ... existing paste logic ...

case lemon.SERVER:
    err = server.Serve(c, logger)   // no client, no client-id I/O
}
```

## Affected files

| File | Change |
|------|--------|
| `lemon/cli.go` | Add `ClientID string` field |
| `lemon/flag.go` | Register `--client-id` flag (empty default) |
| `client/client.go` | Check `c.ClientID` first in resolution chain |
| `main.go` | Move `client.New()` into OPEN/COPY/PASTE cases |

## Testing

- Existing tests for `FlagParse` and `param` remain valid.
- Add a flag-parse test asserting `--client-id` is captured in `CLI.ClientID`.
- Add a `client.New()` unit test asserting the priority chain (flag > env > file).
- Verify `lemonade server` start no longer creates/touches `~/.config/lemonade.toml` when the file is absent.
