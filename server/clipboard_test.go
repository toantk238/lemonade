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
