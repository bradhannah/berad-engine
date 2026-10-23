package textview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/textview"
)

var (
	textID  = frametest.ID(1)
	zoneID  = frametest.ID(2)
	floorID = frametest.ID(3)
	mapID   = frametest.ID(4)
)

type fixture struct {
	t        *testing.T
	state    *textview.State
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
	document := textdoc.NewDocument()
	document.Body = "First line"
	docs.Put(textID, "Notes", document)
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: textID, Name: "Notes", Format: textdoc.FormatName}, {ID: zoneID, Name: "Town", Format: "zone"},
	}))
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := textview.New(frametest.ID(99), textID, theme)
	state.Start(ctx)
	fixture := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx}
	fixture.frame(ui.NewInput(image.Pt(0, 0)))
	return fixture
}

func (f *fixture) frame(input ui.Input) {
	f.state.Update(&input, f.ctx)
}

func (f *fixture) click(name string) {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.ctx.Docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}
	press := ui.NewInput(point)
	press.Pressed, press.Down = true, true
	f.frame(press)
	release := ui.NewInput(point)
	release.Released = true
	f.frame(release)
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) typed(text string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

// TestBodySavesOnEnter types a new line and text, presses Enter, and checks
// one command saves the whole body.
func TestBodySavesOnEnter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:body")
	f.keys("Cmd+Down")
	f.keys("Ctrl+J")
	f.typed("Second line")
	if len(f.requests.Of(textdoc.SetBodyEdit)) != 0 {
		t.Fatal("saved before Enter")
	}
	f.keys("Enter")
	saves := f.requests.Of(textdoc.SetBodyEdit)
	if len(saves) != 1 {
		t.Fatalf("saves %d", len(saves))
	}
	var args textdoc.SetBodyArgs
	if err := saves[0].Decode(&args); err != nil || args.Body != "First line\nSecond line" {
		t.Fatalf("body %q %v", args.Body, err)
	}
}

// TestPickWithOneMapOpen asks which maps are open; with one, the pick starts
// there, and the answer sets the location with an ordinary edit.
func TestPickWithOneMapOpen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:pick")
	if len(f.requests.PickAsks) != 1 {
		t.Fatal("did not ask which maps are open")
	}
	f.requests.PickAsks[0]([]protocol.PickMap{{Window: mapID, Zone: zoneID, Shows: true}}, nil)
	if len(f.requests.PickStarts) != 1 || f.requests.PickStarts[0].Window != mapID || !f.state.Picking() {
		t.Fatalf("starts %+v", f.requests.PickStarts)
	}
	answer := protocol.PickAnswer{Pick: f.requests.PickStarts[0].Pick, Floor: floorID, Item: frametest.ID(5).String()}
	f.state.Changed(client.Update{PickAnswer: &answer}, f.ctx)
	sets := f.requests.Of(textdoc.SetLocationEdit)
	if len(sets) != 1 || f.state.Picking() {
		t.Fatalf("location commands %d", len(sets))
	}
	var args textdoc.SetLocationArgs
	if err := sets[0].Decode(&args); err != nil || args.Location.Floor != floorID || args.Location.Anchor != frametest.ID(5) {
		t.Fatalf("location %+v %v", args.Location, err)
	}
}

// TestPickWithNoMapOpenAsksForAZone opens a map on the chosen zone.
func TestPickWithNoMapOpenAsksForAZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:pick")
	f.requests.PickAsks[0](nil, nil)
	f.click("ui:open")
	if len(f.requests.PickStarts) != 1 || f.requests.PickStarts[0].Zone != zoneID || !f.requests.PickStarts[0].Window.IsZero() {
		t.Fatalf("starts %+v", f.requests.PickStarts)
	}
	f.keys("Escape")
	if len(f.requests.PickCancels) != 1 || f.state.Picking() {
		t.Fatal("Escape did not cancel the pick")
	}
}

// TestPickWithSeveralMapsAsksWhich chooses between open maps.
func TestPickWithSeveralMapsAsksWhich(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:pick")
	other := frametest.ID(8)
	f.requests.PickAsks[0]([]protocol.PickMap{
		{Window: other, Zone: zoneID, Shows: false}, {Window: mapID, Zone: zoneID, Shows: true},
	}, nil)
	f.click("ui:use")
	if len(f.requests.PickStarts) != 1 || f.requests.PickStarts[0].Window != mapID {
		t.Fatalf("the map with anchors should be chosen first: %+v", f.requests.PickStarts)
	}
}
