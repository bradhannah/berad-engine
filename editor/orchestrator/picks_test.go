package orchestrator_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// zoneWithAnchor puts the project's floor in a new zone and gives it one
// anchor, returning the zone and anchor IDs.
func zoneWithAnchor(t *testing.T, h *harness, c *client.Client) (ids.ULID, ids.ULID) {
	t.Helper()
	town := createDocument(t, c, "zone", "Town", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	mustCommand(t, c, town, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: h.floor, Index: 0})
	added := mustCommand(t, c, h.floor, tilefloor.AddAnchorEdit, "", tilefloor.AddAnchorArgs{
		Anchor: floor.Anchor{ID: ids.ULID{}, Name: "Desk", Col: 2, Row: 2}, Index: 0,
	})
	return town, added.Created
}

// nextUpdate waits for the first update that matches.
func nextUpdate(t *testing.T, c *client.Client, what string, match func(client.Update) bool) client.Update {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case update, open := <-c.Updates():
			if !open {
				t.Fatalf("connection closed waiting for %s", what)
			}
			if match(update) {
				return update
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func pickContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// D13: the four pick messages travel through the orchestrator. The asking
// window learns which maps could answer, the Map window is asked and draws
// attention, and its answer reaches the asking window.
func TestPickThroughAnOpenMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	asker := h.dialView(t, "text", ids.ULID{}, "", project.IndexID())
	town, anchor := zoneWithAnchor(t, h, asker)
	mapWindow := h.dialView(t, orchestrator.MapView, town, "map:"+town.String(), town)

	maps, err := asker.PickAsk(pickContext(t), protocol.PickAnchor)
	if err != nil || len(maps) != 1 || maps[0].Window != mapWindow.Window() || maps[0].Zone != town || !maps[0].Shows {
		t.Fatalf("maps %+v %v", maps, err)
	}
	start := protocol.PickStart{Pick: "p1", What: protocol.PickAnchor, Label: "Location", Window: mapWindow.Window()}
	if _, err := asker.PickStart(pickContext(t), start); err != nil {
		t.Fatal(err)
	}
	asked := nextUpdate(t, mapWindow, "the pick", func(u client.Update) bool { return u.PickStart != nil })
	if asked.PickStart.From != asker.Window() || asked.PickStart.Label != "Location" {
		t.Fatalf("forwarded start %+v", asked.PickStart)
	}
	nextUpdate(t, mapWindow, "attention", func(u client.Update) bool { return u.Attention })
	if err := mapWindow.PickAnswer(protocol.PickAnswer{Pick: "p1", Floor: h.floor, Item: anchor.String()}); err != nil {
		t.Fatal(err)
	}
	answer := nextUpdate(t, asker, "the answer", func(u client.Update) bool { return u.PickAnswer != nil })
	if answer.PickAnswer.Floor != h.floor || answer.PickAnswer.Item != anchor.String() {
		t.Fatalf("answer %+v", answer.PickAnswer)
	}
}

// D13: either side can cancel, a second pick replaces the first, and a Map
// window closing ends its pick.
func TestPickCancellation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	asker := h.dialView(t, "text", ids.ULID{}, "", project.IndexID())
	town, _ := zoneWithAnchor(t, h, asker)
	mapWindow := h.dialView(t, orchestrator.MapView, town, "", town)
	begin := func(token string) {
		t.Helper()
		if _, err := asker.PickStart(pickContext(t), protocol.PickStart{
			Pick: token, What: protocol.PickAnchor, Label: "", Window: mapWindow.Window(),
		}); err != nil {
			t.Fatal(err)
		}
		nextUpdate(t, mapWindow, token, func(u client.Update) bool { return u.PickStart != nil && u.PickStart.Pick == token })
	}

	begin("p1")
	if err := asker.PickCancel(protocol.PickCancel{Pick: "p1", Reason: "changed my mind"}); err != nil {
		t.Fatal(err)
	}
	nextUpdate(t, mapWindow, "the cancel", func(u client.Update) bool { return u.PickCancel != nil && u.PickCancel.Pick == "p1" })

	begin("p2")
	begin("p3")
	nextUpdate(t, asker, "p2 replaced", func(u client.Update) bool { return u.PickCancel != nil && u.PickCancel.Pick == "p2" })

	if err := mapWindow.Close(); err != nil {
		t.Fatal(err)
	}
	nextUpdate(t, asker, "p3 ended", func(u client.Update) bool { return u.PickCancel != nil && u.PickCancel.Pick == "p3" })

	if err := asker.PickAnswer(protocol.PickAnswer{Pick: "p3", Floor: h.floor, Item: "x"}); err != nil {
		t.Fatal(err)
	}
	nextUpdate(t, asker, "a stale answer refused", func(u client.Update) bool { return u.PickCancel != nil && u.PickCancel.Pick == "p3" })
}

// D13: with no Map window open, the orchestrator opens one on the chosen zone
// and starts the pick once it connects.
func TestPickOpensAMapOnAZone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	h := start(t, dir, func(cfg *orchestrator.Config) {
		cfg.Launcher = orchestrator.ExecutableLauncher{Path: os.Args[0], Env: []string{
			envRole + "=" + rolePickWindow, envPickItem + "=chosen", envPickFloor + "=" + floorID.String(),
		}}
	})
	asker := h.dialView(t, "text", ids.ULID{}, "", project.IndexID())
	town, _ := zoneWithAnchor(t, h, asker)
	if _, err := asker.PickStart(pickContext(t), protocol.PickStart{Pick: "z1", What: protocol.PickAnchor, Label: "", Zone: town}); err != nil {
		t.Fatal(err)
	}
	answer := nextUpdate(t, asker, "the answer from the new map", func(u client.Update) bool { return u.PickAnswer != nil })
	if answer.PickAnswer.Item != "chosen" || answer.PickAnswer.Floor != h.floor {
		t.Fatalf("answer %+v", answer.PickAnswer)
	}
}

// D13: opening something already open draws attention to its window rather
// than opening another; New view opens another on purpose.
func TestOpeningAgainDrawsAttention(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	markers := t.TempDir()
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Launcher = testLauncher(roleIdleWindow, markers) })
	opener := h.dialView(t, "browser", project.IndexID(), "")
	open := func(newView bool) {
		t.Helper()
		if _, err := opener.Open(pickContext(t), orchestrator.MapView, h.floor, newView); err != nil {
			t.Fatal(err)
		}
	}
	logs := func() int {
		entries, _ := os.ReadDir(filepath.Join(dir, ".berad", "logs"))
		return len(entries)
	}
	open(false)
	waitFor(t, "the first window", func() bool { return logs() == 1 })
	waitFor(t, "it to connect", func() bool {
		maps, err := opener.PickAsk(pickContext(t), protocol.PickAnchor)
		return err == nil && len(maps) == 1
	})
	open(false)
	time.Sleep(300 * time.Millisecond)
	if logs() != 1 {
		t.Fatal("opening again started another window")
	}
	open(true)
	waitFor(t, "the new view", func() bool { return logs() == 2 })
}
