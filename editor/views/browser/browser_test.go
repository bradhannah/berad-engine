package browser_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
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
	docs.Put(clerkID, "Ada Clerk", npcplugin.NewDocument())
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"},
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

// labels lists the main pane's rows by name.
func (f *fixture) labels() []string {
	var labels []string
	for _, row := range f.state.Rows(f.ctx.Docs) {
		labels = append(labels, row.Label)
	}

	return labels
}

// rowNamed returns the main pane's row of a name.
func (f *fixture) rowNamed(label string) ui.Row {
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if row.Label == label {
			return row
		}
	}

	return ui.Row{}
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
	if len(f.whereUsed()) != 1 {
		t.Fatalf("where-used questions %d", len(f.whereUsed()))
	}
	f.whereUsed()[0](nil, nil)
	f.click("ui:delete", false)
	removes := f.requests.Of(zoneplugin.RemoveFloorEdit)
	deletes := f.requests.Of(project.DeleteEdit)
	if len(removes) != 1 || len(deletes) != 1 || removes[0].Command.Gesture != deletes[0].Command.Gesture {
		t.Fatalf("removes %d deletes %d", len(removes), len(deletes))
	}
}

// TestOpening opens a zone in a Map window, a floor in its zone's Map window
// and a workflow in a Workflow window.
func TestOpening(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Town", true)
	f.click("row:Ground", true)
	f.click("row:Clerk day", true)
	if len(f.requests.Opened) != 3 {
		t.Fatalf("opened %+v", f.requests.Opened)
	}
	want := []struct {
		view   string
		target any
	}{{"map", zoneID}, {"map", zoneID}, {"workflow", dayID}}
	for index, opened := range f.requests.Opened {
		if opened.View != want[index].view || opened.Target != want[index].target || opened.NewView {
			t.Fatalf("open %d: %+v", index, opened)
		}
	}
}

// TestWorldStatePlace shows World state's Values, Signals and Constants, and
// opens each in its own window on the world-state document.
func TestWorldStatePlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("place:World state", false)
	if labels := f.labels(); !slices.Equal(labels, []string{"Values", "Signals", "Constants"}) {
		t.Fatalf("World state holds Values, Signals and Constants: %v", labels)
	}
	f.click("row:Values", true)
	f.click("row:Signals", true)
	f.click("row:Constants", true)
	want := []string{"world-state", "world-signals", "world-constants"}
	if len(f.requests.Opened) != len(want) {
		t.Fatalf("opened %+v", f.requests.Opened)
	}
	for index, opened := range f.requests.Opened {
		if opened.View != want[index] || opened.Target != worldstate.ID() {
			t.Fatalf("open %d: %+v", index, opened)
		}
	}
}

func TestRenameInTheInspector(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Ada Clerk", false)
	f.click("ui:name", false)
	f.keys("Cmd+A")
	f.typeText("Ideas")
	f.keys("Enter")
	renames := f.requests.Of(project.RenameEdit)
	if len(renames) != 1 {
		t.Fatalf("renames %d", len(renames))
	}
	var args project.RenameArgs
	if err := renames[0].Decode(&args); err != nil || args.ID != clerkID || args.Name != "Ideas" {
		t.Fatalf("rename %+v %v", args, err)
	}
}

// TestSaveBarSavesAndReverts types a new name, then saves it with the save
// bar's Save, and reverts a second change with Revert.
func TestSaveBarSavesAndReverts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Clerk day", false)
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

// TestPlacesAndWhatTheyHold opens on Home and lists the places: Home, World
// state, Settings, then each kind with how many it holds and a [+]. A kind
// shows its things: a zone with its floors, which close and open, and an
// NPC with what it follows.
func TestPlacesAndWhatTheyHold(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if !f.has("place=Home") || len(f.labels()) != 1 {
		t.Fatalf("the window opens on Home, with nothing done yet: %v", f.labels())
	}
	places := map[string]ui.Row{}
	order := []string{}
	for _, row := range f.state.Places(f.ctx.Docs) {
		places[row.Label] = row
		order = append(order, row.Label)
	}
	if len(order) < 4 || !slices.Equal(order[:3], []string{"Home", "World state", "Settings"}) {
		t.Fatalf("Home, World state and Settings come first: %v", order)
	}
	if zones := places["Zones"]; zones.Detail != "1" || !zones.Add || !zones.Strong {
		t.Fatalf("Zones says how many and has a [+]: %+v", zones)
	}
	f.click("place:Zones", false)
	f.click("ui:view-rows", false)
	rows := map[string]ui.Row{}
	for _, row := range f.state.Rows(f.ctx.Docs) {
		rows[row.Label] = row
	}
	if town := rows["Town"]; town.Icon != ui.IconZone || !town.Strong || town.Toggle != ui.DisclosureOpen ||
		town.Detail != "1 floor" {
		t.Fatalf("zone row %+v", town)
	}
	if ground := rows["Ground"]; ground.Icon != ui.IconFloor || ground.Guide != ui.GuideLast || ground.Indent != 1 {
		t.Fatalf("floor row %+v", ground)
	}
	f.click("toggle:Town", false)
	if _, shown := f.state.Locate("row:Ground", f.ctx.Docs); shown && !f.has("place=Zones") {
		t.Fatal("a closed zone hides its floors")
	}
	f.click("row:Town", false)
	f.keys("Right")
	if !slices.Contains(f.labels(), "Ground") {
		t.Fatal("Right opens the selected zone")
	}
	f.click("row:Ground", false)
	f.keys("Left")
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Town" {
		t.Fatalf("Left on a floor selects its zone, not %q", row.Label)
	}
	f.click("place:NPCs", false)
	if clerk := f.rowNamed("Ada Clerk"); clerk.Icon != ui.IconNPC || clerk.Detail != "no behaviour" {
		t.Fatalf("NPC row %+v", clerk)
	}
}

// TestNewMenuAndSectionAdd create a workflow from New ▾ and an NPC from its
// section's [+]; a floor needs a zone selected.
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
	f.click("ui:new-npc", false)
	created := f.requests.Of(project.CreateEdit)
	formats := []string{}
	for _, create := range created {
		var args project.DocumentArgs
		if err := create.Decode(&args); err != nil {
			t.Fatal(err)
		}
		formats = append(formats, args.Format)
	}
	if len(formats) != 2 || formats[0] != flow.FormatName || formats[1] != npc.FormatName {
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
	f.click("row:Ada Clerk", false)
	f.click("ui:name", false)
	f.typeText(" two")
	f.click("row:Town", false)
	f.click("ui:cancel", false)
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Ada Clerk" || f.state.Unsaved(f.ctx.Docs) != 1 {
		t.Fatalf("Cancel stays on Ada Clerk with the change: %q", row.Label)
	}
	f.click("row:Town", false)
	f.click("ui:save-changes", false)
	renames := f.requests.Of(project.RenameEdit)
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Town" || len(renames) != 1 {
		t.Fatalf("Save saves, then selects Town: %q, %d renames", row.Label, len(renames))
	}
	f.click("ui:name", false)
	f.typeText(" centre")
	f.click("row:Ada Clerk", false)
	f.click("ui:discard-changes", false)
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Ada Clerk" || len(f.requests.Of(project.RenameEdit)) != 1 {
		t.Fatalf("Don't save selects Ada Clerk and saves nothing: %q", row.Label)
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

// TestNPCPathName gives a new NPC a path name suggested from its name, and
// saves one typed into the inspector, refusing one with punctuation.
func TestNPCPathName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:new-npc", false)
	var args project.DocumentArgs
	if created := f.requests.Of(project.CreateEdit); len(created) != 1 || created[0].Decode(&args) != nil {
		t.Fatalf("create %d", len(created))
	}
	made, err := npc.Decode(args.Data)
	if err != nil || made.Path != "npc_1" {
		t.Fatalf("a new NPC's path name comes from its name: %+v %v", made, err)
	}
	f.click("row:Ada Clerk", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.typeInto("npc-path", "ada's")
	f.state.Save(f.ctx)
	if len(f.requests.Of(npcplugin.SetPathEdit)) != 0 || f.status() == "" {
		t.Fatalf("punctuation is refused, saying why: %q", f.status())
	}
	f.click("ui:npc-path", false)
	f.keys("Cmd+A", "Backspace")
	f.typeText("ada")
	f.state.Save(f.ctx)
	set := f.requests.Of(npcplugin.SetPathEdit)
	var saved npcplugin.SetPathArgs
	if len(set) != 1 || set[0].Command.Document != clerkID || set[0].Decode(&saved) != nil || saved.Path != "ada" {
		t.Fatalf("path name %d %+v", len(set), saved)
	}
}

// TestDeleteZoneNamesAllThatGoes asks before deleting a zone, naming its
// floors and the tab that holds unsaved changes to it, then takes each floor
// out of the zone before deleting it, and the zone last, as one step
// (increment 58).
func TestDeleteZoneNamesAllThatGoes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.requests.OpenUnsavedUses = map[ids.ULID][]protocol.Use{
		floorID: {{Document: floorID, Name: "Town", Format: "map", Label: "2 changes", Item: ""}},
	}
	f.click("row:Town", false)
	f.click("ui:delete", false)
	for _, answer := range f.whereUsed() {
		answer(nil, nil)
	}
	lines := strings.Join(f.state.DialogLines(), "\n")
	for _, want := range []string{"Its 1 floor go with it: Ground.", "Town tab · 2 changes"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("the question names %q:\n%s", want, lines)
		}
	}
	f.click("ui:delete", false)
	removes, deletes := f.requests.Of(zoneplugin.RemoveFloorEdit), f.requests.Of(project.DeleteEdit)
	if len(removes) != 1 || len(deletes) != 2 || removes[0].Command.Gesture != deletes[1].Command.Gesture {
		t.Fatalf("removes %d deletes %d", len(removes), len(deletes))
	}
}
