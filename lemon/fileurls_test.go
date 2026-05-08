package lemon

import (
	"os"
	"testing"
)

func TestParseURIList_Normal(t *testing.T) {
	input := "file:///home/user/a.png\n# comment\nfile:///home/user/b.pdf\n"
	got := parseURIList(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 URIs, got %d: %v", len(got), got)
	}
	if got[0] != "file:///home/user/a.png" {
		t.Errorf("got[0] = %q", got[0])
	}
	if got[1] != "file:///home/user/b.pdf" {
		t.Errorf("got[1] = %q", got[1])
	}
}

func TestParseURIList_Empty(t *testing.T) {
	if got := parseURIList(""); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
	if got := parseURIList("# only comments\n"); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestReadFileEntriesFromURIs(t *testing.T) {
	f, err := os.CreateTemp("", "lemonade-test-*.png")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A}
	f.Write(content)
	f.Close()
	defer os.Remove(f.Name())

	entries, err := readFileEntriesFromURIs([]string{"file://" + f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if string(entries[0].Bytes) != string(content) {
		t.Error("file bytes mismatch")
	}
	if entries[0].Name == "" {
		t.Error("expected non-empty Name")
	}
}

func TestReadFileEntriesFromURIs_SkipsNonFileScheme(t *testing.T) {
	entries, err := readFileEntriesFromURIs([]string{"http://example.com/file.png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for http URI, got %d", len(entries))
	}
}
