package lemon

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// LoadOrCreateClientID returns the persistent machine UUID from
// ~/.config/lemonade.toml, generating and saving one if absent.
func LoadOrCreateClientID() (string, error) {
	path, err := homedir.Expand("~/.config/lemonade.toml")
	if err != nil {
		return generateClientID()
	}
	return loadOrCreateClientIDFromPath(path)
}

func loadOrCreateClientIDFromPath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "client-id") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					id := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if id != "" {
						return id, nil
					}
				}
			}
		}
	}

	id, err := generateClientID()
	if err != nil {
		return "", err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return id, nil // generated but not persisted — degraded gracefully
	}
	defer f.Close()
	fmt.Fprintf(f, "\nclient-id = %q\n", id)
	return id, nil
}

func generateClientID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // UUID v4 version bits
	b[8] = (b[8] & 0x3f) | 0x80 // UUID variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
