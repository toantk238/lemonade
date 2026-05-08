package lemon

import "testing"

func TestDetectFileExt(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantExt string
		wantOK  bool
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, ".png", true},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, ".jpg", true},
		{"gif87", []byte{'G', 'I', 'F', '8', '7', 'a'}, ".gif", true},
		{"gif89", []byte{'G', 'I', 'F', '8', '9', 'a'}, ".gif", true},
		{"bmp", []byte{'B', 'M', 0x00, 0x00, 0x00, 0x00}, ".bmp", true},
		{"webp", []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 0x00}, ".webp", true},
		{"text", []byte("hello world"), "", false},
		{"empty", []byte{}, "", false},
		{"short", []byte{0x89}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExt, gotOK := DetectFileExt(tt.input)
			if gotExt != tt.wantExt || gotOK != tt.wantOK {
				t.Errorf("DetectFileExt() = (%q, %v), want (%q, %v)", gotExt, gotOK, tt.wantExt, tt.wantOK)
			}
		})
	}
}
