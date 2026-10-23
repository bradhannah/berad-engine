package mapview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

var (
	zoneID   = frametest.ID(1)
	groundID = frametest.ID(2)
	upperID  = frametest.ID(3)
	deskID   = frametest.ID(4)
)

// groundFloor is a 12x10 grass floor with a room of family "wall.a" from
// (2, 2) to (7, 6), a door at (4, 6) and an anchor at (4, 4).
func groundFloor(t *testing.T) *tilefloor.Document {
	t.Helper()
	document, err := tilefloor.NewDocument(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	for col := 2; col <= 7; col++ {
		for row := 2; row <= 6; row++ {
			if col != 2 && col != 7 && row != 2 && row != 6 {
				continue
			}
			tile := "wall.a"
			if col == 4 && row == 6 {
				tile = "door"
			}
			if _, err := document.Floor.SetTop(col, row, tile); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := document.Floor.InsertAnchor(0, floor.Anchor{ID: deskID, Name: "Desk", Col: 4, Row: 4}); err != nil {
		t.Fatal(err)
	}
	return document
}

type fixture struct {
	t        *testing.T
	state    *mapview.State
	docs     *frametest.Docs
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
	upper, err := tilefloor.NewDocument(12, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	zone := zoneplugin.NewDocument()
	for index, id := range []ids.ULID{groundID, upperID} {
		if err := zone.Zone.Insert(index, id); err != nil {
			t.Fatal(err)
		}
	}
	docs.Put(zoneID, "Town", zone)
	docs.Put(groundID, "Ground", groundFloor(t))
	docs.Put(upperID, "Upper", upper)
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: upperID, Name: "Upper", Format: floor.FormatName},
	}))
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := mapview.New(frametest.ID(99), zoneID, theme)
	state.Start(ctx)
	snapshot := zoneID
	state.Changed(client.Update{Snapshot: &snapshot}, ctx)
	first := ui.NewInput(image.Pt(0, 0))
	state.Update(&first, ctx)
	return &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx}
}

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.ctx.Docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}
	return point
}

func (f *fixture) frame(input ui.Input) {
	f.state.Update(&input, f.ctx)
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) press(name string, double bool) {
	input := ui.NewInput(f.at(name))
	input.Pressed, input.Down, input.DoubleClick = true, true, double
	f.frame(input)
}

func (f *fixture) hold(name string) {
	input := ui.NewInput(f.at(name))
	input.Down = true
	f.frame(input)
}

func (f *fixture) release(name string) {
	input := ui.NewInput(f.at(name))
	input.Released = true
	f.frame(input)
}

// click presses and releases at one place, found once: a dialog closes on the
// press, so its button is gone by the release.
func (f *fixture) click(name string) {
	at := f.at(name)
	f.frame(pointer(at, true, true, false))
	f.frame(pointer(at, false, false, true))
}

func paints(t *testing.T, requests *frametest.Requests) []tilefloor.Cell {
	t.Helper()
	var cells []tilefloor.Cell
	for _, command := range requests.Of(tilefloor.PaintEdit) {
		var args tilefloor.PaintArgs
		if err := command.Decode(&args); err != nil {
			t.Fatal(err)
		}
		cells = append(cells, args.Cells...)
	}
	return cells
}

// TestDragPaintsEveryCellAsOneStroke drags quickly past cells: the skipped
// cells are filled in, and every command of the stroke shares a gesture so it
// undoes as one step.
func TestDragPaintsEveryCellAsOneStroke(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("3")
	f.press("cell:8,8", false)
	f.hold("cell:11,8")
	f.release("cell:11,8")
	cells := paints(t, f.requests)
	if len(cells) != 4 {
		t.Fatalf("painted %d cells, want 4: %+v", len(cells), cells)
	}
	for _, cell := range cells {
		if cell.Layer != tilefloor.LayerGround || cell.Tile != "road" {
			t.Fatalf("cell %+v", cell)
		}
	}
	commands := f.requests.Of(tilefloor.PaintEdit)
	if len(commands) != 2 || commands[0].Command.Gesture == "" || commands[0].Command.Gesture != commands[1].Command.Gesture {
		t.Fatalf("stroke commands %+v", commands)
	}
}

func TestWallsToolDrawsTheChosenFamily(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("W")
	f.keys("3")
	f.press("cell:9,1", false)
	f.hold("cell:9,3")
	f.release("cell:9,3")
	for _, cell := range paints(t, f.requests) {
		if cell.Layer != tilefloor.LayerTop || cell.Tile != "wall.b" {
			t.Fatalf("cell %+v", cell)
		}
	}
	if got := len(paints(t, f.requests)); got != 3 {
		t.Fatalf("painted %d walls", got)
	}
}

// TestRoomFamilyChange double-clicks inside the room to select its walls and
// changes their family in one command; the door is not a wall and stays.
func TestRoomFamilyChange(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("S")
	f.click("cell:5,4")
	f.press("cell:5,4", true)
	f.release("cell:5,4")
	if got := len(f.state.Selected()); got != 17 {
		t.Fatalf("selected %d walls, want the 17 around the room", got)
	}
	f.keys("4")
	commands := f.requests.Of(tilefloor.PaintEdit)
	if len(commands) != 1 {
		t.Fatalf("family change sent %d commands", len(commands))
	}
	for _, cell := range paints(t, f.requests) {
		if cell.Tile != "wall.c" || (cell.Col == 4 && cell.Row == 6) {
			t.Fatalf("cell %+v", cell)
		}
	}

	f.click("cell:10,8")
	f.press("cell:10,8", true)
	f.release("cell:10,8")
	if len(f.state.Selected()) != 0 {
		t.Fatal("open ground is not a room")
	}
	f.click("cell:2,3")
	if got := len(f.state.Selected()); got != 17 {
		t.Fatalf("clicking a wall selected a run of %d", got)
	}
}

// TestAnsweringAPick shows the window answering a pick by clicking an anchor,
// and cancelling one with Escape.
func TestAnsweringAPick(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	start := protocol.PickStart{Pick: "p1", What: protocol.PickAnchor, Label: "x", Window: ids.ULID{}, Zone: ids.ULID{}, From: frametest.ID(7)}
	f.state.Changed(client.Update{PickStart: &start}, f.ctx)
	if !f.state.Picking() {
		t.Fatal("not picking")
	}
	f.click("cell:5,5")
	if len(f.requests.PickAnswers) != 0 || !f.state.Picking() {
		t.Fatal("a cell without an anchor answered the pick")
	}
	f.click("cell:4,4")
	if len(f.requests.PickAnswers) != 1 || f.requests.PickAnswers[0].Item != deskID.String() ||
		f.requests.PickAnswers[0].Floor != groundID || f.state.Picking() {
		t.Fatalf("answers %+v", f.requests.PickAnswers)
	}
	if len(f.requests.Of(tilefloor.PaintEdit)) != 0 {
		t.Fatal("picking painted the floor")
	}
	start.Pick = "p2"
	f.state.Changed(client.Update{PickStart: &start}, f.ctx)
	f.keys("Escape")
	if len(f.requests.PickCancels) != 1 || f.requests.PickCancels[0].Pick != "p2" || f.state.Picking() {
		t.Fatalf("cancels %+v", f.requests.PickCancels)
	}
}

// TestAddingAnAnchorFocusesItsName clicks with the Anchors tool: the window
// asks for a new anchor, then focuses its name so typing renames it.
func TestAddingAnAnchorFocusesItsName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("A")
	f.click("cell:9,8")
	added := f.requests.Of(tilefloor.AddAnchorEdit)
	if len(added) != 1 {
		t.Fatalf("add anchor commands %d", len(added))
	}
	var args tilefloor.AddAnchorArgs
	if err := added[0].Decode(&args); err != nil || args.Anchor.Name != "Anchor 1" || !args.Anchor.ID.IsZero() {
		t.Fatalf("args %+v %v", args, err)
	}
	added[0].Succeed(frametest.ID(50))
	if f.state.SelectedAnchor() != frametest.ID(50) || f.state.Focused() == nil {
		t.Fatal("the new anchor is not selected with its name focused")
	}
}

// TestNegativeCells paints left of and above the zone origin on a floor
// centred on it: those cells have negative numbers.
func TestNegativeCells(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	centred, err := tilefloor.NewDocumentAt(floor.Centered(8, 6), "grass")
	if err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", centred)
	f.keys("Cmd+0")
	f.keys("3")
	f.click("cell:-4,-3")
	f.click("cell:0,0")
	cells := paints(t, f.requests)
	if len(cells) != 2 || cells[0].Col != -4 || cells[0].Row != -3 || cells[1].Col != 0 || cells[1].Row != 0 {
		t.Fatalf("painted %+v", cells)
	}
	f.click("cell:4,0")
	if got := len(paints(t, f.requests)); got != 2 {
		t.Fatal("a click past the right edge painted a cell")
	}
}

func resizes(t *testing.T, requests *frametest.Requests) []tilefloor.ResizeArgs {
	t.Helper()
	var found []tilefloor.ResizeArgs
	for _, command := range requests.Of(tilefloor.ResizeEdit) {
		var args tilefloor.ResizeArgs
		if err := command.Decode(&args); err != nil {
			t.Fatal(err)
		}
		found = append(found, args)
	}
	return found
}

// TestResizeByDraggingAnEdge drags the left edge two cells outwards: one
// command adds two columns on the left, filled with the floor's most common
// ground.
func TestResizeByDraggingAnEdge(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("R")
	from := f.at("edge:left")
	size := f.at("cell:1,0").X - f.at("cell:0,0").X
	f.frame(pointer(from, true, true, false))
	f.frame(pointer(from.Add(image.Pt(-size, 0)), false, true, false))
	f.frame(pointer(from.Add(image.Pt(-2*size, 0)), false, true, false))
	f.frame(pointer(from.Add(image.Pt(-2*size, 0)), false, false, true))
	got := resizes(t, f.requests)
	want := tilefloor.Bounds{Col: -2, Row: 0, Width: 14, Height: 10}
	if len(got) != 1 || got[0].Bounds != want || got[0].Ground != "grass" {
		t.Fatalf("resizes %+v, want bounds %+v filled with grass", got, want)
	}
}

// TestResizeButtonsAskBeforeRemovingAnAnchor removes rows from the bottom
// until the next would take the Desk anchor: then the window asks where the
// anchor is used and confirms first.
func TestResizeButtonsAskBeforeRemovingAnAnchor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("R")
	f.click("ui:grow-top")
	if got := resizes(t, f.requests); len(got) != 1 || got[0].Bounds != (tilefloor.Bounds{Col: 0, Row: -1, Width: 12, Height: 11}) {
		t.Fatalf("grow top %+v", got)
	}
	shrunk, err := tilefloor.NewDocument(12, 5, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := shrunk.Floor.InsertAnchor(0, floor.Anchor{ID: deskID, Name: "Desk", Col: 4, Row: 4}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", shrunk)
	f.click("ui:shrink-bottom")
	if len(f.requests.Uses) != 1 || len(resizes(t, f.requests)) != 1 {
		t.Fatal("removing the row with the anchor should ask where it is used first")
	}
	f.requests.Uses[0](nil, nil)
	f.click("ui:resize")
	got := resizes(t, f.requests)
	if len(got) != 2 || got[1].Bounds != (tilefloor.Bounds{Col: 0, Row: 0, Width: 12, Height: 4}) {
		t.Fatalf("shrink bottom %+v", got)
	}
}

func pointer(at image.Point, pressed, down, released bool) ui.Input {
	input := ui.NewInput(at)
	input.Pressed, input.Down, input.Released = pressed, down, released
	return input
}

func TestSwitchingFloors(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if f.state.Floor() != groundID {
		t.Fatalf("starts on %v", f.state.Floor())
	}
	f.keys("PageUp")
	if f.state.Floor() != upperID {
		t.Fatal("PageUp did not go to the upper floor")
	}
	f.keys("PageUp")
	if f.state.Floor() != upperID {
		t.Fatal("there is no floor above the top")
	}
	f.click("floor:1")
	if f.state.Floor() != groundID {
		t.Fatal("clicking the bottom floor in the list did not switch to it")
	}
}
