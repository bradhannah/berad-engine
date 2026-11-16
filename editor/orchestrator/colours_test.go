package orchestrator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// The colour last painted with in each sprite is kept for this person and
// shared with every window that joins after the project opens again
// (increment 60).
func TestColoursLastUsedAreKept(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	kept, err := json.Marshal(protocol.PixelColours{Sprites: map[string]json.RawMessage{
		"01SPRITE": json.RawMessage(`{"value":3}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := view.Publish(protocol.TopicPixelColours, kept); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the colours to be kept", func() bool {
		_, err := os.Stat(filepath.Join(dir, ".berad", "colours.json"))
		return err == nil
	})
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	again := start(t, dir, nil)
	update := nextUpdate(t, again.dial(t, ""), "the colours kept", func(update client.Update) bool {
		return update.Session != nil && update.Session.Topic == protocol.TopicPixelColours
	})
	if !strings.Contains(string(update.Session.Value), "01SPRITE") {
		t.Fatalf("the colours are shared again: %s", update.Session.Value)
	}
}
