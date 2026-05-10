package client

import (
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/lemonade-command/lemonade/lemon"
)

func testLogger() log.Logger {
	l := log.New()
	l.SetHandler(log.DiscardHandler())
	return l
}

func TestClientNewFlagTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv("LEMONADE_CLIENT_ID", "env-id")
	c := &lemon.CLI{
		Host:          "localhost",
		Port:          2489,
		Timeout:       100 * time.Millisecond,
		ImageCacheTTL: 30 * time.Minute,
		ClientID:      "flag-id",
	}
	cl := New(c, testLogger())
	if cl.clientID != "flag-id" {
		t.Errorf("expected clientID %q, got %q", "flag-id", cl.clientID)
	}
}

func TestClientNewEnvUsedWhenFlagEmpty(t *testing.T) {
	t.Setenv("LEMONADE_CLIENT_ID", "env-id")
	c := &lemon.CLI{
		Host:          "localhost",
		Port:          2489,
		Timeout:       100 * time.Millisecond,
		ImageCacheTTL: 30 * time.Minute,
		ClientID:      "",
	}
	cl := New(c, testLogger())
	if cl.clientID != "env-id" {
		t.Errorf("expected clientID %q, got %q", "env-id", cl.clientID)
	}
}
