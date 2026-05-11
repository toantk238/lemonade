package server

import (
	"time"

	"github.com/atotto/clipboard"
	"github.com/lemonade-command/lemonade/lemon"
)

type Clipboard struct{}

const clipboardTimeout = 200 * time.Millisecond

func (_ *Clipboard) Copy(text string, _ *struct{}) error {
	<-connCh
	serverLogger.Debug("Copy called", "len", len(text), "preview", truncate(text, 120))
	text = lemon.ConvertLineEnding(text, LineEndingOpt)

	done := make(chan error, 1)
	go func() {
		done <- clipboard.WriteAll(text)
	}()

	select {
	case err := <-done:
		if err != nil {
			serverLogger.Error("clipboard write error", "err", err)
		} else {
			serverLogger.Debug("Copy written to clipboard", "len", len(text))
		}
		return err
	case <-time.After(clipboardTimeout):
		go func() {
			if err := <-done; err != nil {
				serverLogger.Error("clipboard write error", "err", err)
			}
		}()
		serverLogger.Warn("Copy timed out waiting for clipboard write")
		return nil
	}
}

func (_ *Clipboard) Paste(_ struct{}, resp *string) error {
	<-connCh
	serverLogger.Debug("Paste called, reading clipboard...")
	t, err := clipboard.ReadAll()
	if err != nil {
		serverLogger.Error("clipboard read error", "err", err)
	} else {
		serverLogger.Debug("Paste read from clipboard", "len", len(t), "preview", truncate(t, 120))
	}
	*resp = t
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
