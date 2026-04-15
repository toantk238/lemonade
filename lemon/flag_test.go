package lemon

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCLIParse(t *testing.T) {
	defaultTimeout := 100 * time.Millisecond
	assert := func(args []string, expected CLI) {
		expected.In = os.Stdin
		if expected.Timeout == 0 {
			expected.Timeout = defaultTimeout
		}
		c := &CLI{In: os.Stdin}
		c.FlagParse(args, true)

		if !reflect.DeepEqual(expected, *c) {
			t.Errorf("Expected:\n %+v, but got\n %+v", expected, c)
		}
	}

	defaultPort := 2489
	defaultHost := "localhost"
	defaultAllow := "0.0.0.0/0,::/0"
	defaultLogLevel := 1

	assert([]string{"xdg-open", "http://example.com"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		DataSource:     "http://example.com",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"/usr/bin/xdg-open", "http://example.com"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		DataSource:     "http://example.com",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"xdg-open"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"pbpaste", "--port", "1124"}, CLI{
		Type:           PASTE,
		Host:           defaultHost,
		Port:           1124,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"/usr/bin/pbpaste", "--port", "1124"}, CLI{
		Type:           PASTE,
		Host:           defaultHost,
		Port:           1124,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"pbcopy", "hogefuga"}, CLI{
		Type:           COPY,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		DataSource:     "hogefuga",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"/usr/bin/pbcopy", "hogefuga"}, CLI{
		Type:           COPY,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		DataSource:     "hogefuga",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "--host", "192.168.0.1", "--port", "1124", "open", "http://example.com"}, CLI{
		Type:           OPEN,
		Host:           "192.168.0.1",
		Port:           1124,
		Allow:          defaultAllow,
		DataSource:     "http://example.com",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "copy", "hogefuga"}, CLI{
		Type:           COPY,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		DataSource:     "hogefuga",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "paste"}, CLI{
		Type:           PASTE,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "--allow", "192.168.0.0/24", "server", "--port", "1124"}, CLI{
		Type:           SERVER,
		Host:           defaultHost,
		Port:           1124,
		Allow:          "192.168.0.0/24",
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "open", "--trans-loopback=false"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		TransLoopback:  false,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "open", "--trans-loopback=true"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "open", "--trans-localfile=false"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: false,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "open", "--trans-localfile=true"}, CLI{
		Type:           OPEN,
		Host:           defaultHost,
		Port:           defaultPort,
		Allow:          defaultAllow,
		TransLoopback:  true,
		TransLocalfile: true,
		LogLevel:       defaultLogLevel,
	})

	assert([]string{"lemonade", "copy", "--no-fallback-messages", "hogefuga"}, CLI{
		Type:               COPY,
		Host:               defaultHost,
		Port:               defaultPort,
		Allow:              defaultAllow,
		DataSource:         "hogefuga",
		TransLoopback:      true,
		TransLocalfile:     true,
		NoFallbackMessages: true,
		LogLevel:           defaultLogLevel,
	})

	assert([]string{"lemonade", "paste", "--no-fallback-messages"}, CLI{
		Type:               PASTE,
		Host:               defaultHost,
		Port:               defaultPort,
		Allow:              defaultAllow,
		TransLoopback:      true,
		TransLocalfile:     true,
		NoFallbackMessages: true,
		LogLevel:           defaultLogLevel,
	})
}

func TestCopyStdinTrimNewline(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"hello\n", "hello"},
		{"hello", "hello"},
		{"/home/user/project\n", "/home/user/project"},
		{"line1\nline2\n", "line1\nline2"},
		{"line1\nline2\n\n", "line1\nline2\n"},
	}

	for _, tc := range cases {
		c := &CLI{In: strings.NewReader(tc.input)}
		c.FlagParse([]string{"lemonade", "copy", "--trim-newline"}, true)
		if c.DataSource != tc.expected {
			t.Errorf("input %q: expected DataSource %q, got %q", tc.input, tc.expected, c.DataSource)
		}
	}
}

func TestCopyStdinNoTrimByDefault(t *testing.T) {
	cases := []struct {
		input string
	}{
		{"hello\n"},
		{"hello"},
		{"/home/user/project\n"},
		{"line1\nline2\n"},
	}

	for _, tc := range cases {
		c := &CLI{In: strings.NewReader(tc.input)}
		c.FlagParse([]string{"lemonade", "copy"}, true)
		if c.DataSource != tc.input {
			t.Errorf("input %q: expected DataSource to be unchanged, got %q", tc.input, c.DataSource)
		}
	}
}
