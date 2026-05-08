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
