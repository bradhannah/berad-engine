package orchestrator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestTheArrangementIsKeptPerPerson saves the window's arrangement in the
// project's state folder as the window publishes it, and gives it back to a
// window joining after the project is opened again (increment 53).
func TestTheArrangementIsKeptPerPerson(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", ids.ULID{}, "")
	value, err := json.Marshal(protocol.Layout{Screens: map[string]protocol.Arrangement{
		"1512x982": {Kinds: map[string]protocol.KindLayout{"map": {Inspector: 340, LeftFolded: true}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := browser.Publish(protocol.TopicLayout, value); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(dir, ".berad", "layout.json")
	waitFor(t, "the arrangement saved", func() bool {
		data, err := os.ReadFile(saved)

		return err == nil && strings.Contains(string(data), "1512x982")
	})
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	again := start(t, dir, nil)
	window := again.dialView(t, "browser", ids.ULID{}, "")
	nextUpdate(t, window, "the arrangement on joining", func(update client.Update) bool {
		if update.Session == nil || update.Session.Topic != protocol.TopicLayout {
			return false
		}
		var back protocol.Layout

		return json.Unmarshal(update.Session.Value, &back) == nil &&
			back.Screens["1512x982"].Kinds["map"].Inspector == 340
	})
}
