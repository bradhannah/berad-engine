package workflowview_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

var shelfID = frametest.ID(88)

// withTown puts the office in a zone, Town, with a shelf beside its desk,
// so the list of places has both.
func (f *fixture) withTown() {
	f.t.Helper()
	town := zoneplugin.NewDocument()
	if err := town.Zone.Insert(0, floorID); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(frametest.ID(87), "Town", town)
	f.docs.View(floorID, func(document client.Document) {
		office := document.Model.(*tilefloor.Document)
		if err := office.Floor.InsertAnchor(1, floor.Anchor{ID: shelfID, Name: "Shelf", Col: 1, Row: 1}); err != nil {
			f.t.Fatal(err)
		}
	})
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	entries = append(entries, project.Entry{ID: frametest.ID(87), Name: "Town", Format: zone.FormatName})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(entries))
}

// TestChoosingAPlaceByName chooses a Call's place input, a place local's
// start and a Go to's place from the list each box opens, the status naming
// the place and what Undo puts back, the first tying the workflow to Town;
// the list's last row picks on a map.
func TestChoosingAPlaceByName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCall()
	f.withTown()
	f.click("ui:input-place-place")
	f.click("place:Shelf")
	var given worldstate.Place
	if err := json.Unmarshal(f.nth(flow.KindCall, 1).Inputs()[placeIn], &given); err != nil ||
		given != (worldstate.Place{Floor: floorID, Anchor: shelfID}) {
		t.Fatalf("the input is given the shelf: %s", f.nth(flow.KindCall, 1).Inputs()[placeIn])
	}
	if got := f.status(); got != "Daily routine runs in Town now, the zone of the place chosen. Undo takes both back." {
		t.Fatalf("the first fixed place ties the workflow, which ran in any zone, to Town: %q", got)
	}
	f.click("ui:input-place-place")
	f.click("place:Office desk 3")
	if got := f.status(); got != "Chose “Office desk 3” for place. Undo puts “Shelf” back." {
		t.Fatalf("the status says what Undo puts back: %q", got)
	}
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-type-place")
	f.click("ui:value-place")
	f.click("place:Shelf")
	if start := f.local().Zero().Place; start != (worldstate.Place{Floor: floorID, Anchor: shelfID}) {
		t.Fatalf("the local starts at the shelf: %+v", start)
	}
	asks := len(f.requests.PickAsks)
	f.click("ui:value-place")
	f.click("ui:" + "pick-on-map")
	if len(f.requests.PickAsks) != asks+1 {
		t.Fatal("the list's last row picks on a map")
	}
	f.click("ui:value-clear-place")
	if start := f.local().Zero().Place; !start.IsZero() {
		t.Fatalf("Clear leaves no place: %+v, %q", start, f.status())
	}
}

// TestGoToChoosesAPlace chooses a Go to's place from its box's list.
func TestGoToChoosesAPlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withTown()
	f.click("node:start:1")
	f.add("" + flow.KindGoTo)
	f.click("ui:place")
	f.click("place:Shelf")
	destination, err := f.nth(flow.KindGoTo, 1).Anchor("destination")
	if err != nil || destination != (floor.AnchorRef{Floor: floorID, Anchor: shelfID}) {
		t.Fatalf("Go to goes to the shelf: %+v %v", destination, err)
	}
}

// withZone adds a zone with one floor holding one place.
func (f *fixture) withZone(zoneID ids.ULID, zoneName string, floorOf ids.ULID, floorName string, place floor.Anchor) {
	f.t.Helper()
	made := zoneplugin.NewDocument()
	if err := made.Zone.Insert(0, floorOf); err != nil {
		f.t.Fatal(err)
	}
	ground, err := tilefloor.NewDocument(8, 8, "grass")
	if err == nil {
		err = ground.Floor.InsertAnchor(0, place)
	}
	if err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(zoneID, zoneName, made)
	f.docs.Put(floorOf, floorName, ground)
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	entries = append(entries, project.Entry{ID: zoneID, Name: zoneName, Format: zone.FormatName},
		project.Entry{ID: floorOf, Name: floorName, Format: floor.FormatName})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(entries))
}

// answerRunners answers every question of who runs the workflow still
// waiting.
func (f *fixture) answerRunners(uses []protocol.Use) {
	for index, done := range f.requests.RunnerAsks {
		if done != nil {
			done(uses, nil)
			f.requests.RunnerAsks[index] = nil
		}
	}
}

// placesFor opens a new Go to's list of places in a workflow Clerk, living
// in Town, runs as its behaviour or by a call.
func placesFor(t *testing.T, label string) *fixture {
	t.Helper()
	f := newFixture(t)
	f.withTown()
	clerk := frametest.ID(86)
	f.withZone(frametest.ID(84), "Harbour", frametest.ID(85), "Quay",
		floor.Anchor{ID: frametest.ID(83), Name: "Dock", Col: 1, Row: 1})
	clerkDoc := npcplugin.NewDocument()
	clerkDoc.NPC.Zone = frametest.ID(87)
	f.docs.Put(clerk, "Clerk", clerkDoc)
	f.click("node:start:1")
	f.add("" + flow.KindGoTo)
	f.answerRunners([]protocol.Use{{Document: clerk, Name: "Clerk", Format: npc.FormatName, Label: label, Item: ""}})
	f.click("ui:place")

	return f
}

// TestPlacesWhereItRuns lists a workflow running in any zone with a new
// place input and another value first, then every place; once it runs in
// Town, Town's places come first and the others' grey after them, and a
// grey place can still be chosen.
func TestPlacesWhereItRuns(t *testing.T) {
	t.Parallel()
	anywhere := placesFor(t, protocol.RunsAsBehaviour)
	if !anywhere.has("places-list=[Daily routine runs in any zone] / New place input… / Another value… / " +
		"[Or a fixed place: Daily routine then runs in its zone] / [Harbour · Quay] / Dock / [Town · Office] / " +
		"Office desk 3 / Shelf") {
		t.Fatal("a workflow running in any zone offers a new input and another value first, then every place")
	}
	inTown := placesFor(t, protocol.RunsAsBehaviour)
	inTown.keys("Escape")
	inTown.workflow().SetRunsIn(frametest.ID(87))
	inTown.docs.Touch(workflowID)
	inTown.frame(ui.NewInput(image.Pt(0, 0)))
	inTown.click("ui:place")
	if !inTown.has("places-list=[Daily routine runs in Town] / [Town · Office] / Office desk 3 / Shelf / " +
		"[Other zones: flagged when chosen here] / [Harbour · Quay] / (Dock)") {
		t.Fatal("a workflow running in Town lists Town's places first, the others grey")
	}
	inTown.click("place:Dock")
	if destination, err := inTown.nth(flow.KindGoTo, 1).Anchor("destination"); err != nil ||
		destination.Anchor != frametest.ID(83) {
		t.Fatalf("a grey place can still be chosen: %+v %v", destination, err)
	}
}

// TestANewPlaceInput chooses New place input… for a Go to in a workflow
// running in any zone: a place input is declared and the Go to goes to it,
// in one gesture.
func TestANewPlaceInput(t *testing.T) {
	t.Parallel()
	f := placesFor(t, protocol.RunsAsBehaviour)
	f.click("place:New place input…")
	changes, replaces := f.requests.Of(workflowplugin.ChangeLocalsEdit), f.requests.Of(workflowplugin.ReplaceEdit)
	var change stateplugin.ChangeArgs
	if len(changes) != 1 || changes[0].Decode(&change) != nil || len(change.Add) != 1 ||
		change.Add[0].Role != worldstate.RoleInput || change.Add[0].Type != worldstate.TypePlace ||
		change.Add[0].Name != "place" || len(replaces) == 0 ||
		replaces[len(replaces)-1].Command.Gesture != changes[0].Command.Gesture {
		t.Fatalf("a place input is declared and the Go to changed in one gesture: %+v %+v", changes, replaces)
	}
	goTo := f.nth(flow.KindGoTo, 1)
	ref, err := goTo.Ref(flow.ParamPlace)
	declared, declaredOK := f.workflow().Locals().Value(ref.Value)
	if goes, _ := goTo.Choice(flow.ParamGoes); goes != flow.GoesToValue || err != nil || !declaredOK ||
		declared.Name != "place" {
		t.Fatalf("the Go to goes to the new input: %v %v", goes, err)
	}
}
