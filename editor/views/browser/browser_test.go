package browser_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

var (
	textID  = frametest.ID(1)
	zoneID  = frametest.ID(2)
	floorID = frametest.ID(3)
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
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: textID, Name: "Notes", Format: textdoc.FormatName}, {ID: zoneID, Name: "Town", Format: "zone"},
		{ID: floorID, Name: "Ground", Format: floor.FormatName},
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
	f.click("ui:rename", false)
	typed := ui.NewInput(image.Pt(0, 0))
	typed.Text = "Ideas"
	f.state.Update(&typed, f.ctx)
	enter := ui.NewInput(image.Pt(0, 0))
	enter.Presses = []ui.Press{ui.ParsePress("Enter")}
	f.state.Update(&enter, f.ctx)
	renames := f.requests.Of(project.RenameEdit)
	if len(renames) != 1 {
		t.Fatalf("renames %d", len(renames))
	}
	var args project.RenameArgs
	if err := renames[0].Decode(&args); err != nil || args.ID != textID || args.Name != "Ideas" {
		t.Fatalf("rename %+v %v", args, err)
	}
}

// TestRenameButtonSavesTheTypedName types a new name into the field, then
// clicks Rename instead of pressing Enter: the button saves the name.
func TestRenameButtonSavesTheTypedName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Notes", false)
	f.click("ui:name", false)
	selectAll := ui.NewInput(image.Pt(0, 0))
	selectAll.Presses = []ui.Press{ui.ParsePress("Cmd+A")}
	f.state.Update(&selectAll, f.ctx)
	typed := ui.NewInput(image.Pt(0, 0))
	typed.Text = "Ideas"
	f.state.Update(&typed, f.ctx)
	f.click("ui:rename", false)
	renames := f.requests.Of(project.RenameEdit)
	if len(renames) != 1 {
		t.Fatalf("renames %d", len(renames))
	}
	var args project.RenameArgs
	if err := renames[0].Decode(&args); err != nil || args.Name != "Ideas" {
		t.Fatalf("rename %+v %v", args, err)
	}
	renames[0].Succeed(ids.ULID{})
	if message, _ := f.ctx.Frame.Message(); message != "Renamed to “Ideas”" {
		t.Fatalf("status %q", message)
	}
}
