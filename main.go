package main

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

var logLevelMap = map[int]log.Lvl{
	0: log.LvlDebug,
	1: log.LvlInfo,
	2: log.LvlWarn,
	3: log.LvlError,
	4: log.LvlCrit,
}

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

func writeError(c *lemon.CLI, err error) {
	fmt.Fprintln(c.Err, err.Error())
}
