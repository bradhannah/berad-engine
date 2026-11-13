package orchestrator_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// A floor a zone lists, and a zone that lists floors, are not deleted out of
// step (increment 58). Taking each floor out first, then deleting it and the
// zone, in one gesture, deletes them all, and one undo brings all back.
func TestAZoneAndItsFloorsGoTogether(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	town := createDocument(t, browser, "zone", "Town", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	ground := createDocument(t, browser, floor.FormatName, "Ground", town, newFloorData(t), "")
	cellar := createDocument(t, browser, floor.FormatName, "Cellar", town, newFloorData(t), "")
	mustCommand(t, browser, town, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: ground, Index: 0})
	mustCommand(t, browser, town, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: cellar, Index: 1})

	_, err := command(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: cellar})
	if err == nil || !strings.Contains(err.Error(), "Cellar is part of Town; take it out of Town first") {
		t.Fatalf("a floor its zone lists is refused: %v", err)
	}
	_, err = command(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: town})
	if err == nil || !strings.Contains(err.Error(), "Town still holds Ground and Cellar") {
		t.Fatalf("a zone that lists floors is refused: %v", err)
	}

	for _, floorID := range []ids.ULID{ground, cellar} {
		mustCommand(t, browser, town, zone.RemoveFloorEdit, "delete", zone.FloorArgs{Floor: floorID, Index: 0})
		mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "delete", project.DocumentArgs{ID: floorID})
	}
	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "delete", project.DocumentArgs{ID: town})
	for _, gone := range []ids.ULID{town, ground, cellar} {
		if _, found := indexEntry(browser, gone); found {
			t.Fatal("each goes")
		}
	}
	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "all to come back", func() bool {
		_, hasTown := indexEntry(browser, town)
		_, hasCellar := indexEntry(browser, cellar)
		return hasTown && hasCellar
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := browser.Subscribe(ctx, town); err != nil {
		t.Fatal(err)
	}
	if floors := zoneFloors(browser, town); len(floors) != 2 || floors[0] != ground || floors[1] != cellar {
		t.Fatalf("the zone holds its floors again, in order: %v", floors)
	}
}

// Asked which other windows hold unsaved changes to a document, the
// orchestrator names each; a tab closed because its document was deleted is
// told in a notice, with what of its changes went (increment 58).
func TestOtherTabsUnsavedChangesAreNamed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	shown := h.dialView(t, "map", floorID, "", floorID)
	if err := shown.Unsaved(protocol.Unsaved{Count: 2, Name: "Floor 1"}); err != nil {
		t.Fatal(err)
	}
	var open []protocol.Use
	waitFor(t, "the tab's unsaved changes to be named", func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		uses, err := browser.OpenUnsaved(ctx, floorID)
		open = uses
		return err == nil && len(uses) == 1
	})
	if open[0].Name != "Floor 1" || open[0].Format != "map" || open[0].Label != "2 changes" {
		t.Fatalf("the tab is named with its changes: %+v", open[0])
	}
	if mine, err := shown.OpenUnsaved(context.Background(), floorID); err != nil || len(mine) != 0 {
		t.Fatalf("a window is not told of its own changes: %v %v", mine, err)
	}
	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: floorID})
	notice := nextNotice(t, browser)
	if notice.Kind != protocol.NoticeTabClosed || !strings.Contains(notice.Text, "without the 2 changes") {
		t.Fatalf("the closed tab is told, with what went: %+v", notice)
	}
}
