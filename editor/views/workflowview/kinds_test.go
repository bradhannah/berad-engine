package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
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
	f.add("" + flow.KindGoTo)
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
	f.add("" + flow.KindGoTo)
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
	f.add("" + flow.KindGoTo)
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

// withOthers adds Town, Harbour and other workflows: Errand running in
// Town, Roam in any zone, Harbour errand in Harbour, and Market day, a zone
// loop running in Town, by name.
func (f *fixture) withOthers() map[string]ids.ULID {
	f.t.Helper()
	f.withTown()
	town, harbour := frametest.ID(87), frametest.ID(84)
	f.withZone(harbour, "Harbour", frametest.ID(85), "Quay",
		floor.Anchor{ID: frametest.ID(83), Name: "Dock", Col: 1, Row: 1})
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	made := map[string]ids.ULID{}
	for index, each := range []struct {
		name    string
		purpose flow.Purpose
		runs    ids.ULID
	}{
		{"Errand", flow.PurposeBehaviour, town}, {"Roam", flow.PurposeBehaviour, ids.ULID{}},
		{"Harbour errand", flow.PurposeBehaviour, harbour}, {"Market day", flow.PurposeZoneLoop, town},
	} {
		workflow, err := flow.NewFor(frametest.ID(60), each.purpose)
		if err != nil {
			f.t.Fatal(err)
		}
		workflow.SetRunsIn(each.runs)
		made[each.name] = frametest.ID(byte(70 + index))
		f.docs.Put(made[each.name], each.name, &workflowplugin.Document{Workflow: workflow})
		entries = append(entries, project.Entry{
			ID: made[each.name], Name: each.name, Format: flow.FormatName, Folder: "", Tags: nil,
			Kind: string(each.purpose),
		})
	}
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(entries))

	return made
}

// TestCallsThatDoNotFit flags a Call in a behaviour running in Town to one
// running in Harbour, and to a zone loop.
func TestCallsThatDoNotFit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	made := f.withOthers()
	f.workflow().SetRunsIn(frametest.ID(87))
	f.docs.Touch(workflowID)
	f.click("node:start:1")
	f.add("" + flow.KindCall)
	f.click("ui:call-workflow")
	f.click("menu:workflow:" + made["Roam"].String())
	if f.has("problem:call-workflow:1") {
		t.Fatal("a behaviour running in any zone fits")
	}
	for name, want := range map[string]string{
		"Harbour errand": "Harbour errand runs in Harbour, and Daily routine runs in Town. Call one that runs there " +
			"or in any zone.",
		"Market day": "Market day is a zone loop, and Daily routine an NPC behaviour. Call one of its own kind.",
	} {
		call := f.nth(flow.KindCall, 1)
		call, err := call.WithParam(flow.ParamWorkflow, made[name].String())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.workflow().Replace(call); err != nil {
			t.Fatal(err)
		}
		f.docs.Touch(workflowID)
		f.frame(ui.NewInput(image.Pt(0, 0)))
		f.frame(ui.NewInput(image.Pt(0, 0)))
		if !f.has("problem:call-workflow:1=" + want) {
			t.Fatalf("calling %s is flagged", name)
		}
	}
}

// TestACallOffersWhatFits lists, for a Call in a behaviour running in Town,
// the behaviours running in Town or any zone, then, disabled, one running in
// Harbour, and no zone loop; running in any zone, it offers only those
// running in any zone.
func TestACallOffersWhatFits(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withOthers()
	town := frametest.ID(87)
	f.workflow().SetRunsIn(town)
	f.docs.Touch(workflowID)
	f.click("node:start:1")
	f.add("" + flow.KindCall)
	f.click("ui:call-workflow")
	if !f.has("call-menu=Errand / Roam / (Harbour errand · runs in Harbour)") {
		t.Fatal("a Call offers the behaviours that fit, and greys the one running elsewhere")
	}
	f.keys("Escape")
	f.workflow().SetRunsIn(ids.ULID{})
	f.docs.Touch(workflowID)
	f.click("node:call-workflow:1")
	f.click("ui:call-workflow")
	if !f.has("call-menu=Roam / (Errand · runs in Town) / (Harbour errand · runs in Harbour)") {
		t.Fatal("running in any zone, it calls only behaviours running in any zone")
	}
}
