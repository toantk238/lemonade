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
