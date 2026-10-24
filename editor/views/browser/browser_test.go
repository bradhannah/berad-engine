package browser_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	textID  = frametest.ID(1)
	zoneID  = frametest.ID(2)
	floorID = frametest.ID(3)
	clerkID = frametest.ID(4)
	dayID   = frametest.ID(5)
)

type fixture struct {
	t        *testing.T
	state    *browser.State
	requests *frametest.Requests
	ctx      *frame.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	docs := frametest.NewDocs()
	zone := zoneplugin.NewDocument()
	if err := zone.Zone.Insert(0, floorID); err != nil {
		t.Fatal(err)
	}
	ground, err := tilefloor.NewDocument(10, 8, "grass")
	if err != nil {
		t.Fatal(err)
	}
	docs.Put(zoneID, "Town", zone)
	docs.Put(floorID, "Ground", ground)
	docs.Put(textID, "Notes", textdoc.NewDocument())
	docs.Put(clerkID, "Ada Clerk", npcplugin.NewDocument())
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: textID, Name: "Notes", Format: textdoc.FormatName}, {ID: zoneID, Name: "Town", Format: "zone"},
		{ID: floorID, Name: "Ground", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName},
	}))
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := browser.New(frametest.ID(99), theme)
	state.Start(ctx)
	first := ui.NewInput(image.Pt(0, 0))
	state.Update(&first, ctx)
	return &fixture{t: t, state: state, requests: requests, ctx: ctx}
}

func (f *fixture) click(name string, double bool) {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.ctx.Docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}
	press := ui.NewInput(point)
	press.Pressed, press.Down, press.DoubleClick = true, true, double
	f.state.Update(&press, f.ctx)
	release := ui.NewInput(point)
	release.Released = true
	f.state.Update(&release, f.ctx)
}

// TestNewZoneComesWithAFloor creates a zone, then its first floor, then puts
// the floor in the zone: three commands sharing one gesture, so one undo
// step takes them all back.
func TestNewZoneComesWithAFloor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:new-zone", false)
	creates := f.requests.Of(project.CreateEdit)
	if len(creates) != 1 {
		t.Fatalf("creates %d", len(creates))
	}
	var zoneArgs project.DocumentArgs
	if err := creates[0].Decode(&zoneArgs); err != nil || zoneArgs.Format != "zone" || zoneArgs.Name != "Zone 1" {
		t.Fatalf("zone %+v %v", zoneArgs, err)
	}
	newZone := frametest.ID(40)
	creates[0].Succeed(newZone)
	creates = f.requests.Of(project.CreateEdit)
	var floorArgs project.DocumentArgs
	if err := creates[1].Decode(&floorArgs); err != nil || floorArgs.Format != floor.FormatName || floorArgs.Scope != newZone {
		t.Fatalf("floor %+v %v", floorArgs, err)
	}
	firstFloor, err := floor.Decode(floorArgs.Data)
	if err != nil || firstFloor.Bounds() != floor.Centered(32, 24) {
		t.Fatalf("the first floor should be centred on the zone origin: %v", err)
	}
	creates[1].Succeed(frametest.ID(41))
	inserts := f.requests.Of(zoneplugin.InsertFloorEdit)
	if len(inserts) != 1 || inserts[0].Command.Document != newZone {
		t.Fatalf("inserts %+v", inserts)
	}
	gesture := creates[0].Command.Gesture
	if gesture == "" || creates[1].Command.Gesture != gesture || inserts[0].Command.Gesture != gesture {
		t.Fatal("the three commands must share one gesture")
	}
}

// TestDeleteFloorAsksThenRemovesItFromItsZone checks where the floor is used,
// confirms, then takes it out of its zone and deletes it as one step.
func TestDeleteFloorAsksThenRemovesItFromItsZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Ground", false)
	f.click("ui:delete", false)
	if len(f.requests.Uses) != 1 {
		t.Fatalf("where-used questions %d", len(f.requests.Uses))
	}
	f.requests.Uses[0](nil, nil)
	f.click("ui:delete", false)
	removes := f.requests.Of(zoneplugin.RemoveFloorEdit)
	deletes := f.requests.Of(project.DeleteEdit)
	if len(removes) != 1 || len(deletes) != 1 || removes[0].Command.Gesture != deletes[0].Command.Gesture {
		t.Fatalf("removes %d deletes %d", len(removes), len(deletes))
	}
}

// TestOpening opens a zone in a Map window, a floor in its zone's Map window
// and a text document in a Text window.
func TestOpening(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Town", true)
	f.click("row:Ground", true)
	f.click("row:Notes", true)
	if len(f.requests.Opened) != 3 {
		t.Fatalf("opened %+v", f.requests.Opened)
	}
	want := []struct {
		view   string
		target any
	}{{"map", zoneID}, {"map", zoneID}, {"text", textID}}
	for index, opened := range f.requests.Opened {
		if opened.View != want[index].view || opened.Target != want[index].target || opened.NewView {
			t.Fatalf("open %d: %+v", index, opened)
		}
	}
}

func TestRenameInTheInspector(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Notes", false)
	f.click("ui:name", false)
	f.keys("Cmd+A")
	f.typeText("Ideas")
	f.keys("Enter")
	renames := f.requests.Of(project.RenameEdit)
	if len(renames) != 1 {
		t.Fatalf("renames %d", len(renames))
	}
	var args project.RenameArgs
	if err := renames[0].Decode(&args); err != nil || args.ID != textID || args.Name != "Ideas" {
		t.Fatalf("rename %+v %v", args, err)
	}
}

// TestSaveBarSavesAndReverts types a new name, then saves it with the save
// bar's Save, and reverts a second change with Revert.
func TestSaveBarSavesAndReverts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Notes", false)
	if f.state.Unsaved(f.ctx.Docs) != 0 {
		t.Fatal("nothing typed yet")
	}
	f.click("ui:name", false)
	f.keys("Cmd+A")
	f.typeText("Ideas")
	if f.state.Unsaved(f.ctx.Docs) != 1 {
		t.Fatalf("unsaved %d", f.state.Unsaved(f.ctx.Docs))
	}
	f.click("ui:save", false)
	renames := f.requests.Of(project.RenameEdit)
	var args project.RenameArgs
	if len(renames) != 1 || renames[0].Decode(&args) != nil || args.Name != "Ideas" {
		t.Fatalf("rename %d %+v", len(renames), args)
	}
	if f.state.Unsaved(f.ctx.Docs) != 0 {
		t.Fatal("a name on its way is no longer unsaved")
	}
	renames[0].Succeed(ids.ULID{})
	if message, _ := f.ctx.Frame.Message(); message != "Renamed to “Ideas”" {
		t.Fatalf("status %q", message)
	}
	f.click("ui:name", false)
	f.typeText(" more")
	f.click("ui:revert", false)
	if f.state.Unsaved(f.ctx.Docs) != 0 || len(f.requests.Of(project.RenameEdit)) != 1 {
		t.Fatal("Revert drops the change and saves nothing")
	}
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.state.Update(&input, f.ctx)
}

func (f *fixture) typeText(text string) {
	typed := ui.NewInput(image.Pt(0, 0))
	typed.Text = text
	f.state.Update(&typed, f.ctx)
}

// TestNPCInspector creates an NPC, then gives the selected NPC a behaviour
// from the workflows in a dialog and saves notes typed into the inspector;
// Enter starts a new line in them.
func TestNPCInspector(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:new-npc", false)
	created := f.requests.Of(project.CreateEdit)
	var args project.DocumentArgs
	if len(created) != 1 || created[0].Decode(&args) != nil || args.Format != npc.FormatName || args.Name != "NPC 1" {
		t.Fatalf("create %d %+v", len(created), args)
	}
	f.click("row:Ada Clerk", false)
	f.click("ui:choose-behaviour", false)
	f.click("row:Clerk day", false)
	f.click("ui:use", false)
	set := f.requests.Of(npcplugin.SetBehaviourEdit)
	var behaviour npcplugin.SetBehaviourArgs
	if len(set) != 1 || set[0].Command.Document != clerkID || set[0].Decode(&behaviour) != nil ||
		behaviour.Behaviour != dayID {
		t.Fatalf("set behaviour %d %+v", len(set), behaviour)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:notes", false)
	f.typeText("Opens the shop")
	f.keys("Enter")
	f.typeText("at eight.")
	if len(f.requests.Of(npcplugin.SetNotesEdit)) != 0 {
		t.Fatal("Enter in the notes starts a new line rather than saving")
	}
	f.state.Save(f.ctx)
	notes := f.requests.Of(npcplugin.SetNotesEdit)
	var saved npcplugin.SetNotesArgs
	if len(notes) != 1 || notes[0].Decode(&saved) != nil || saved.Notes != "Opens the shop\nat eight." {
		t.Fatalf("notes %d %+v", len(notes), saved)
	}
}

// TestTreeShowsTypesAndHidesRows lists sections with counts, icons and
// details, and hides a zone's floors or a whole section when toggled.
func TestTreeShowsTypesAndHidesRows(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	rows := map[string]ui.Row{}
	for _, row := range f.state.Rows(f.ctx.Docs) {
		rows[row.Label] = row
	}
	if zones := rows["Zones"]; !zones.Section || zones.Detail != "1" || !zones.Add {
		t.Fatalf("zones section %+v", zones)
	}
	if town := rows["Town"]; town.Icon != ui.IconZone || !town.Strong || town.Toggle != ui.DisclosureOpen ||
		town.Detail != "1 floor" {
		t.Fatalf("zone row %+v", town)
	}
	if ground := rows["Ground"]; ground.Icon != ui.IconFloor || ground.Guide != ui.GuideLast || ground.Indent != 1 {
		t.Fatalf("floor row %+v", ground)
	}
	if clerk := rows["Ada Clerk"]; clerk.Icon != ui.IconNPC || clerk.Detail != "no behaviour" {
		t.Fatalf("NPC row %+v", clerk)
	}
	f.click("toggle:Town", false)
	if _, shown := f.state.Locate("row:Ground", f.ctx.Docs); shown {
		t.Fatal("a closed zone hides its floors")
	}
	f.keys("Right")
	f.click("row:Town", false)
	f.keys("Right")
	if _, shown := f.state.Locate("row:Ground", f.ctx.Docs); !shown {
		t.Fatal("Right opens the selected zone")
	}
	f.click("section:text", false)
	if _, shown := f.state.Locate("row:Notes", f.ctx.Docs); shown {
		t.Fatal("a closed section hides its rows")
	}
	f.click("row:Ground", false)
	f.keys("Left")
	if row, _ := f.state.SelectedRow(); row.Label != "Town" {
		t.Fatalf("Left on a floor selects its zone, not %q", row.Label)
	}
}

// TestNewMenuAndSectionAdd create a workflow from New ▾ and a text document
// from its section's [+]; a floor needs a zone selected.
func TestNewMenuAndSectionAdd(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:new", false)
	f.click("menu:new-floor", false)
	if len(f.requests.Of(project.CreateEdit)) != 0 {
		t.Fatal("no zone is selected, so New floor is greyed out")
	}
	f.click("ui:new", false)
	f.click("menu:new-workflow", false)
	f.click("ui:new-text", false)
	created := f.requests.Of(project.CreateEdit)
	formats := []string{}
	for _, create := range created {
		var args project.DocumentArgs
		if err := create.Decode(&args); err != nil {
			t.Fatal(err)
		}
		formats = append(formats, args.Format)
	}
	if len(formats) != 2 || formats[0] != flow.FormatName || formats[1] != textdoc.FormatName {
		t.Fatalf("created %v", formats)
	}
	f.click("row:Ground", false)
	f.click("ui:move-up", false)
	if len(f.requests.Of(zoneplugin.MoveFloorEdit)) != 0 {
		t.Fatal("the only floor cannot move up")
	}
}

// TestSelectingAwayAsksAboutUnsavedChanges types a name, then selects
// another document: Cancel stays, Save saves then moves, and Don't save
// moves without saving.
func TestSelectingAwayAsksAboutUnsavedChanges(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Notes", false)
	f.click("ui:name", false)
	f.typeText(" two")
	f.click("row:Town", false)
	f.click("ui:cancel", false)
	if row, _ := f.state.SelectedRow(); row.Label != "Notes" || f.state.Unsaved(f.ctx.Docs) != 1 {
		t.Fatalf("Cancel stays on Notes with the change: %q", row.Label)
	}
	f.click("row:Town", false)
	f.click("ui:save-changes", false)
	renames := f.requests.Of(project.RenameEdit)
	if row, _ := f.state.SelectedRow(); row.Label != "Town" || len(renames) != 1 {
		t.Fatalf("Save saves, then selects Town: %q, %d renames", row.Label, len(renames))
	}
	f.click("ui:name", false)
	f.typeText(" centre")
	f.click("row:Notes", false)
	f.click("ui:discard-changes", false)
	if row, _ := f.state.SelectedRow(); row.Label != "Notes" || len(f.requests.Of(project.RenameEdit)) != 1 {
		t.Fatalf("Don't save selects Notes and saves nothing: %q", row.Label)
	}
	f.click("ui:name", false)
	f.typeText("!")
	closed := false
	f.state.Leave(f.ctx, func() { closed = true })
	f.click("ui:cancel", false)
	if closed {
		t.Fatal("Cancel keeps the window open")
	}
}
