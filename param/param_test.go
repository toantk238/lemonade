package param

import "testing"

func TestFileEntryRoundTrip(t *testing.T) {
	e := FileEntry{Name: "img.png", Bytes: []byte{0x89, 'P', 'N', 'G'}}
	if e.Name != "img.png" || len(e.Bytes) != 4 {
		t.Errorf("unexpected FileEntry: %+v", e)
	}

	cp := CopyFileParam{
		Files:    []FileEntry{e},
		ClientID: "abc-123",
	}
	if len(cp.Files) != 1 || cp.ClientID != "abc-123" {
		t.Errorf("unexpected CopyFileParam: %+v", cp)
	}

	pp := PasteFileParam{ClientID: "abc-123"}
	res := PasteFileResult{SameClient: true, Files: nil}
	if !res.SameClient || pp.ClientID != "abc-123" {
		t.Errorf("unexpected paste types")
	}
}
