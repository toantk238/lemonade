package server

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/lemonade-command/lemonade/lemon"
)

type Clipboard struct{}

const clipboardTimeout = 200 * time.Millisecond
const wslClipboardTimeout = 5 * time.Second

func (_ *Clipboard) Copy(text string, _ *struct{}) error {
	<-connCh
	serverLogger.Debug("Copy called", "len", len(text), "preview", truncate(text, 120))
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
	if lemon.IsWSL() {
		out, err := wslPaste()
		if err != nil {
			return err
		}
		*resp = out
		return nil
	}
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

func wslCopy(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), wslClipboardTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "clip.exe")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

func wslPaste() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wslClipboardTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "powershell.exe", "-command", "Get-Clipboard").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}
