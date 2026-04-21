package server

import (
	"testing"

	"github.com/lemonade-command/lemonade/lemon"
)

func TestCopyWSL(t *testing.T) {
	if !lemon.IsWSL() {
		t.Skip("skipping WSL clipboard test: not running in WSL")
	}

	cb := &Clipboard{}
	err := cb.Copy("hello lemonade", nil)
	if err != nil {
		t.Fatalf("Copy via clip.exe failed: %v", err)
	}

	var result string
	err = cb.Paste(struct{}{}, &result)
	if err != nil {
		t.Fatalf("Paste via powershell failed: %v", err)
	}

	if result != "hello lemonade" {
		t.Errorf("Paste returned %q, want %q", result, "hello lemonade")
	}
}
