package lemon

import (
	"strings"
	"testing"
)

func TestIsWSLFromReader(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		expected bool
	}{
		{"WSL2 kernel", "Linux version 5.15.90.1-microsoft-standard-WSL2", true},
		{"WSL1 kernel", "Linux version 4.4.0-19041-Microsoft", true},
		{"plain Linux", "Linux version 5.15.0-91-generic (buildd@lcy02-amd64-059)", false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isWSLFromReader(strings.NewReader(tc.content))
			if got != tc.expected {
				t.Errorf("isWSLFromReader(%q) = %v, want %v", tc.content, got, tc.expected)
			}
		})
	}
}
