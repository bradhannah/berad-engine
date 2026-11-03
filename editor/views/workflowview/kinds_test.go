package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// asZoneLoop makes the fixture's workflow a zone loop, as if it had been
// made as one.
func (f *fixture) asZoneLoop() {
	f.t.Helper()
	loop, err := flow.NewFor(startID, flow.PurposeZoneLoop)
	if err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(workflowID, "Market day", &workflowplugin.Document{Workflow: loop})
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestEachKindsPalette offers an NPC behaviour every node, and a zone loop
// none that moves a body: no Go to, Move or Find NPC nearby.
func TestEachKindsPalette(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, kind := range []string{flow.KindGoTo, flow.KindMove, flow.KindFindNPC, flow.KindWait} {
		if _, found := f.state.Locate("palette:"+kind, f.ctx.Docs); !found {
			t.Fatalf("an NPC behaviour offers %s", kind)
		}
	}
	f.asZoneLoop()
	var purpose flow.Purpose
	f.docs.View(workflowID, func(document client.Document) {
		purpose = document.Model.(*workflowplugin.Document).Workflow.Purpose()
	})
	if purpose != flow.PurposeZoneLoop {
		t.Fatal("the fixture's workflow is a zone loop now")
	}
	for _, kind := range []string{flow.KindGoTo, flow.KindMove, flow.KindFindNPC} {
		if _, found := f.state.Locate("palette:"+kind, f.ctx.Docs); found {
			t.Fatalf("a zone loop offers no %s", kind)
		}
	}
	if _, found := f.state.Locate("palette:"+flow.KindWait, f.ctx.Docs); !found {
		t.Fatal("a zone loop waits")
	}
}

// TestRunsIn reads the strip of a behaviour Ada Clerk follows, then sets it
// to run in Town from the Runs in box, the picker counting its place there,
// and back to any zone.
func TestRunsIn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withTown()
	town := frametest.ID(87)
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.answerRunners([]protocol.Use{{
		Document: adaID, Name: "Ada Clerk", Format: npc.FormatName, Label: protocol.RunsAsBehaviour, Item: "",
	}})
	goTo := f.workflow().Nodes()[1]
	goTo, err := goTo.WithParam("destination", floor.AnchorRef{Floor: floorID, Anchor: deskID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.workflow().Replace(goTo); err != nil {
		t.Fatal(err)
	}
	f.docs.Touch(workflowID)
	f.keys("Escape")
	if !f.has("strip=NPC behaviour · Runs in any zone · Followed by Ada Clerk") || !f.has("runs-in=Any zone") {
		t.Fatal("the strip names the kind, where it runs and who follows it")
	}
	f.click("ui:runs-in")
	if !f.has("zone-picker") || !f.has("zone-detail:Town=1 of its places") {
		t.Fatal("the Runs in box opens the zone picker, counting the workflow's places in each zone")
	}
	f.click("zone:Town")
	f.keys("Enter")
	set := f.requests.Of(workflowplugin.SetRunsInEdit)
	var args workflowplugin.SetRunsInArgs
	if len(set) != 1 || set[0].Decode(&args) != nil || args.Zone != town || f.has("zone-picker") {
		t.Fatalf("choosing Town sets it: %+v", set)
	}
	if !f.has("runs-in=Town") || !f.has("strip=NPC behaviour · Runs in Town · Followed by Ada Clerk") {
		t.Fatal("the workflow runs in Town")
	}
	f.click("ui:runs-in")
	f.click("ui:" + zonepicker.NoneID)
	f.click("ui:runs-in-change")
	if set := f.requests.Of(workflowplugin.SetRunsInEdit); len(set) != 2 || set[1].Decode(&args) != nil ||
		!args.Zone.IsZero() || !f.has("runs-in=Any zone") {
		t.Fatalf("Any zone sets it back, once confirmed, as it leaves its place in Town: %+v", set)
	}
}

// withDeskGoTo gives the workflow a Go to the desk in Town, running in
// Town, that Ada Clerk, living in Town, follows; Harbour is another zone.
func (f *fixture) withDeskGoTo() {
	f.t.Helper()
	f.withTown()
	town := frametest.ID(87)
	f.withZone(frametest.ID(84), "Harbour", frametest.ID(85), "Quay",
		floor.Anchor{ID: frametest.ID(83), Name: "Dock", Col: 1, Row: 1})
	ada := npcplugin.NewDocument()
	ada.NPC.Path, ada.NPC.Behaviour, ada.NPC.Zone = "ada", workflowID, town
	f.docs.Put(adaID, "Ada Clerk", ada)
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.answerRunners([]protocol.Use{{
		Document: adaID, Name: "Ada Clerk", Format: npc.FormatName, Label: protocol.RunsAsBehaviour, Item: "",
	}})
	goTo, err := f.workflow().Nodes()[1].WithParam("destination", floor.AnchorRef{Floor: floorID, Anchor: deskID})
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.workflow().Replace(goTo); err != nil {
		f.t.Fatal(err)
	}
	f.workflow().SetRunsIn(town)
	f.docs.Touch(workflowID)
	f.keys("Escape")
}

// TestChangingWhereItRunsAsks asks before running a behaviour in Harbour,
// naming Ada Clerk, who lives in Town, and the desk left behind, and
// changes it only once confirmed; Any zone asks about its places, and
// Cancel leaves it.
func TestChangingWhereItRunsAsks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withDeskGoTo()
	f.click("ui:runs-in")
	f.click("zone:Harbour")
	f.keys("Enter")
	if !f.has("dialog") || !f.has("dialog-line=Town → Harbour") || !f.has("dialog-line=Ada Clerk lives elsewhere") ||
		!f.has("dialog-row=Office desk 3") || len(f.requests.Of(workflowplugin.SetRunsInEdit)) != 0 {
		t.Fatal("running it in Harbour asks first, naming who and what it breaks")
	}
	f.click("ui:" + "runs-in-change")
	set := f.requests.Of(workflowplugin.SetRunsInEdit)
	var args workflowplugin.SetRunsInArgs
	if len(set) != 1 || set[0].Decode(&args) != nil || args.Zone != frametest.ID(84) || f.has("dialog") {
		t.Fatalf("confirming runs it in Harbour: %+v", set)
	}
	f.click("ui:runs-in")
	f.click("ui:" + zonepicker.NoneID)
	if !f.has("dialog-line=until it comes from an input or another value") || f.has("dialog-line=lives elsewhere") {
		t.Fatal("running it in any zone asks about its fixed places only")
	}
	f.keys("Escape")
	if len(f.requests.Of(workflowplugin.SetRunsInEdit)) != 1 || f.has("dialog") || !f.has("runs-in=Harbour") {
		t.Fatal("Cancel leaves it running in Harbour")
	}
}

// TestAFixedPlaceTiesItsZone chooses a place in Town for a Go to in a
// workflow running in any zone, which then runs in Town in the same undo
// step; nobody follows it, so nothing asks.
func TestAFixedPlaceTiesItsZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withTown()
	f.docs.Put(adaID, "Ada Clerk", npcplugin.NewDocument())
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.answerRunners(nil)
	f.click("ui:place")
	f.click("place:Shelf")
	replaces, set := f.requests.Of(workflowplugin.ReplaceEdit), f.requests.Of(workflowplugin.SetRunsInEdit)
	var args workflowplugin.SetRunsInArgs
	if len(set) != 1 || set[0].Decode(&args) != nil || args.Zone != frametest.ID(87) || len(replaces) == 0 ||
		set[0].Command.Gesture == "" || replaces[len(replaces)-1].Command.Gesture != set[0].Command.Gesture {
		t.Fatalf("the place and Town are set in one gesture: %+v %+v", replaces, set)
	}
	if !f.has("runs-in=Town") || f.has("dialog") {
		t.Fatal("it runs in Town, without asking")
	}
}
