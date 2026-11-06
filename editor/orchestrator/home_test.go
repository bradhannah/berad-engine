package orchestrator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// homeOpened reports the documents an update says were opened lately, and
// whether it is that topic at all.
func homeOpened(update client.Update) ([]ids.ULID, bool) {
	if update.Session == nil || update.Session.Topic != protocol.TopicHomeOpened {
		return nil, false
	}
	var opened protocol.HomeOpened
	if err := json.Unmarshal(update.Session.Value, &opened); err != nil {
		return nil, false
	}
	documents := make([]ids.ULID, 0, len(opened.Opened))
	for _, open := range opened.Opened {
		documents = append(documents, open.Document)
	}
	return documents, true
}

// TestHomeRemembersWhatWasOpenedAndKept opens a floor in a Map twice: Home
// notes it once, for every window, and saves it. The Project window's pins
// and folder views are saved beside it, and reopening the project brings
// both back. The state folder keeps itself out of version control.
func TestHomeRemembersWhatWasOpenedAndKept(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	h.editorWindow(t)
	browser := h.dialView(t, "browser", ids.ULID{}, "")
	for range 2 {
		if _, err := browser.Open(pickContext(t), orchestrator.MapView, h.floor, false); err != nil {
			t.Fatal(err)
		}
		nextUpdate(t, browser, "the floor noted as opened", func(update client.Update) bool {
			documents, ok := homeOpened(update)
			return ok && len(documents) == 1 && documents[0] == h.floor
		})
	}
	kept := protocol.HomeKept{
		Pins:    []ids.ULID{h.floor},
		Folders: map[string]protocol.FolderChoice{"sprite:Props": {View: "pictures", Sort: "", Descending: false}},
	}
	value, err := json.Marshal(kept)
	if err != nil {
		t.Fatal(err)
	}
	if err := browser.Publish(protocol.TopicHomeKept, value); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(dir, ".berad", "home.json")
	waitFor(t, "Home's memory saved", func() bool {
		data, err := os.ReadFile(saved)
		return err == nil && strings.Contains(string(data), "pictures")
	})
	if data, err := os.ReadFile(filepath.Join(dir, ".berad", ".gitignore")); err != nil || string(data) != "*\n" {
		t.Fatalf("the state folder ignores itself: %q %v", data, err)
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	again := start(t, dir, nil)
	window := again.dialView(t, "browser", ids.ULID{}, "")
	var gotOpened, gotKept bool
	nextUpdate(t, window, "Home's memory on joining", func(update client.Update) bool {
		if documents, ok := homeOpened(update); ok {
			gotOpened = len(documents) == 1 && documents[0] == again.floor
		}
		if update.Session != nil && update.Session.Topic == protocol.TopicHomeKept {
			var back protocol.HomeKept
			gotKept = json.Unmarshal(update.Session.Value, &back) == nil && len(back.Pins) == 1 &&
				back.Folders["sprite:Props"].View == "pictures"
		}
		return gotOpened && gotKept
	})
}

// TestOnlyTheOrchestratorNotesWhatWasOpened ignores a window that tries to
// set what was opened.
func TestOnlyTheOrchestratorNotesWhatWasOpened(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	one := h.dialView(t, "browser", ids.ULID{}, "")
	value, err := json.Marshal(protocol.HomeOpened{Opened: []protocol.HomeOpen{{Document: h.floor}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := one.Publish(protocol.TopicHomeOpened, value); err != nil {
		t.Fatal(err)
	}
	other := h.dialView(t, "map", h.floor, "")
	nextUpdate(t, other, "the empty list on joining", func(update client.Update) bool {
		documents, ok := homeOpened(update)
		if ok && len(documents) != 0 {
			t.Fatalf("a window set what was opened: %v", documents)
		}
		return ok
	})
}
