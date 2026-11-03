package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	townID    = frametest.ID(87)
	harbourID = frametest.ID(84)
	loftID    = frametest.ID(89)
	clerkNPC  = frametest.ID(86)
	fisherNPC = frametest.ID(90)
)

// living puts NPCs that run the workflow as their behaviour in zones, and
// tells the window so when it next asks, which a change to the project
// makes it do.
func (f *fixture) living(homes map[ids.ULID]ids.ULID) {
	f.t.Helper()
	names := map[ids.ULID]string{clerkNPC: "Clerk", fisherNPC: "Fisher"}
	var uses []protocol.Use
	for _, person := range []ids.ULID{clerkNPC, fisherNPC} {
		home, lives := homes[person]
		if !lives {
			continue
		}
		made := npcplugin.NewDocument()
		made.NPC.Zone = home
		f.docs.Put(person, names[person], made)
		uses = append(uses, protocol.Use{Document: person, Name: names[person], Format: npc.FormatName,
			Label: protocol.RunsAsBehaviour, Item: ""})
	}
	f.docs.Touch(project.IndexID())
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.answerRunners(uses)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// reachTown is Town with the office and an empty Loft above it, which no
// stairs join, and Harbour with its Quay; the workflow has a Go to to the
// office desk, and Clerk is placed on the Loft.
func reachTown(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.withTown()
	f.withZone(harbourID, "Harbour", frametest.ID(85), "Quay",
		floor.Anchor{ID: frametest.ID(83), Name: "Dock", Col: 1, Row: 1})
	loft, err := tilefloor.NewDocument(8, 8, "grass")
	if err == nil {
		err = loft.Floor.InsertPlacement(0, floor.Placement{ID: frametest.ID(91), NPC: clerkNPC, Col: 2, Row: 2})
	}
	if err != nil {
		t.Fatal(err)
	}
	f.docs.Put(loftID, "Loft", loft)
	f.docs.View(townID, func(document client.Document) {
		if err := document.Model.(*zoneplugin.Document).Zone.Insert(1, loftID); err != nil {
			t.Fatal(err)
		}
	})
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(append(entries,
		project.Entry{ID: loftID, Name: "Loft", Format: floor.FormatName})))
	f.docs.Touch(townID)
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.click("ui:place")
	f.click("place:Office desk 3")

	return f
}

// runIn makes the workflow run in a zone, or in any zone, as if it had been
// set so.
func (f *fixture) runIn(zoneID ids.ULID) {
	f.t.Helper()
	f.workflow().SetRunsIn(zoneID)
	f.docs.Touch(workflowID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestPlacesOutOfReach flags a Go to whose place is in another zone than
// the workflow runs in, or is a fixed place while it runs in any zone, and
// one on a floor no stairs lead to from where an NPC following it is
// placed, naming that NPC; each mends.
func TestPlacesOutOfReach(t *testing.T) {
	t.Parallel()
	f := reachTown(t)
	if !f.has("runs-in=Town") || f.has("problem:go-to:1") {
		t.Fatal("choosing the desk ties the workflow to Town, where it fits")
	}
	f.runIn(harbourID)
	if !f.has("problem:go-to:1=Office desk 3 is in Town, and Daily routine runs in Harbour. Choose a place in Harbour.") {
		t.Fatal("a place in another zone than the workflow's is flagged")
	}
	f.runIn(ids.ULID{})
	if !f.has("problem:go-to:1=Office desk 3 is a fixed place, and Daily routine runs in any zone. Give it a place " +
		"input or another value, or run it in Town.") {
		t.Fatal("a fixed place in a workflow running in any zone is flagged")
	}
	f.runIn(townID)
	f.living(map[ids.ULID]ids.ULID{clerkNPC: townID, fisherNPC: townID})
	if !f.has("problem:go-to:1=No stairs lead from Loft to Office, so Clerk can't reach Office desk 3.") {
		t.Fatal("a floor no stairs lead to from where Clerk is placed is flagged, naming Clerk of the two")
	}
	f.docs.View(loftID, func(document client.Document) {
		if _, err := document.Model.(*tilefloor.Document).Floor.SetTop(4, 4, "stairs.down"); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(loftID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.has("problem:go-to:1") {
		t.Fatal("stairs down from the Loft mend it")
	}
}

// TestCallsAndLocalsOutOfReach flags a Call that gives its module a place
// in another zone than the workflow runs in, and a place local that starts
// in one.
func TestCallsAndLocalsOutOfReach(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCall()
	f.withTown()
	f.withZone(harbourID, "Harbour", frametest.ID(85), "Quay",
		floor.Anchor{ID: frametest.ID(83), Name: "Dock", Col: 1, Row: 1})
	f.click("ui:input-place-place")
	f.click("place:Shelf")
	f.runIn(harbourID)
	if !f.has("problem:call-workflow:1=Its place: Shelf is in Town, and Daily routine runs in Harbour. Choose a " +
		"place in Harbour.") {
		t.Fatal("a Call giving a place in another zone is flagged")
	}
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-type-place")
	f.click("ui:value-place")
	f.click("place:Shelf")
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("out-of-reach:" + f.local().Name) {
		t.Fatal("a place local starting in another zone is flagged")
	}
	f.click("ui:value-place")
	f.click("place:Dock")
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.has("out-of-reach:" + f.local().Name) {
		t.Fatal("a start in the workflow's zone mends it")
	}
}
