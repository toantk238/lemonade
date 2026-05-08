package lemon

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lemonade-command/lemonade/param"
)

// ReadFileEntriesFromClipboard reads file URIs from the system clipboard using
// xclip (X11) or wl-paste (Wayland), then reads each file's bytes.
// Returns nil, nil when no file URIs are found or no clipboard tool is available.
func ReadFileEntriesFromClipboard() ([]param.FileEntry, error) {
	out, err := exec.Command("xclip", "-t", "text/uri-list", "-o").Output()
	if err != nil {
		out, err = exec.Command("wl-paste", "--type", "text/uri-list").Output()
		if err != nil {
			return nil, nil
		}
	}
	uris := parseURIList(string(out))
	if len(uris) == 0 {
		return nil, nil
	}
	return readFileEntriesFromURIs(uris)
}

func readFileEntriesFromURIs(uris []string) ([]param.FileEntry, error) {
	var entries []param.FileEntry
	for _, uri := range uris {
		u, err := url.Parse(uri)
		if err != nil || u.Scheme != "file" {
			continue
		}
		b, err := os.ReadFile(u.Path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", u.Path, err)
		}
		entries = append(entries, param.FileEntry{
			Name:  filepath.Base(u.Path),
			Bytes: b,
		})
	}
	return entries, nil
}

func parseURIList(output string) []string {
	var uris []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		uris = append(uris, line)
	}
	return uris
}
