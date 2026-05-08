package lemon

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateClientID_Unique(t *testing.T) {
	id1, err := generateClientID()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := generateClientID()
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Error("generateClientID should produce unique IDs")
	}
	parts := strings.Split(id1, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 UUID parts, got %d in %q", len(parts), id1)
	}
}

func TestLoadOrCreateClientIDFromPath_CreatesAndPersists(t *testing.T) {
	f, err := os.CreateTemp("", "lemonade-test-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	id1, err := loadOrCreateClientIDFromPath(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if id1 == "" {
		t.Fatal("expected non-empty client ID")
	}

	id2, err := loadOrCreateClientIDFromPath(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("expected same ID on reload, got %q and %q", id1, id2)
	}
}

func TestLoadOrCreateClientIDFromPath_ReadsExisting(t *testing.T) {
	f, err := os.CreateTemp("", "lemonade-test-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("port = 1234\nclient-id = \"existing-uuid-abc\"\n")
	f.Close()
	defer os.Remove(f.Name())

	id, err := loadOrCreateClientIDFromPath(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if id != "existing-uuid-abc" {
		t.Errorf("expected existing-uuid-abc, got %q", id)
	}
}
