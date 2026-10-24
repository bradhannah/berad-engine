package clockview_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

var (
	zoneID = frametest.ID(1)
	runID  = frametest.ID(2)
)

type fixture struct {
	t        *testing.T
	state    *clockview.State
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
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{{ID: zoneID, Name: "Town", Format: "zone"}}))
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(clockview.Width, clockview.Height))
	state := clockview.New(theme)
	state.Start(ctx)
	f := &fixture{t: t, state: state, requests: requests, ctx: ctx}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func (f *fixture) frame(input ui.Input) { f.state.Update(&input, f.ctx) }

func (f *fixture) has(name string) bool {
	_, found := f.state.Locate(name, f.ctx.Docs)

	return found
}

func (f *fixture) click(name string) {
	f.t.Helper()
	at, found := f.state.Locate("ui:"+name, f.ctx.Docs)
	if !found {
		f.t.Fatalf("cannot locate %s", name)
	}
	press := ui.NewInput(at)
	press.Pressed, press.Down = true, true
	f.frame(press)
	release := ui.NewInput(at)
	release.Released = true
	f.frame(release)
}

func (f *fixture) typeTime(text string) {
	f.click("time")
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
	enter := ui.NewInput(image.Pt(0, 0))
	enter.Presses = []ui.Press{ui.ParsePress("Enter")}
	f.frame(enter)
}

func (f *fixture) published() float64 {
	f.t.Helper()
	sets := f.requests.Published
	if len(sets) == 0 || sets[len(sets)-1].Topic != protocol.TopicClockStart {
		f.t.Fatal("no start time published")
	}
	var start protocol.ClockStart
	if err := json.Unmarshal(sets[len(sets)-1].Value, &start); err != nil {
		f.t.Fatal(err)
	}

	return start.Seconds
}

func (f *fixture) lastControl() protocol.PreviewControl {
	f.t.Helper()
	controls := f.requests.PreviewControls
	if len(controls) == 0 {
		f.t.Fatal("no control sent")
	}

	return controls[len(controls)-1]
}

func (f *fixture) preview(clockAt float64, rate int, stopped bool, tick uint64) {
	f.state.Changed(client.Update{PreviewState: &protocol.PreviewState{
		Run: runID, Tick: tick, Clock: clockAt, Rate: rate, ClockStopped: stopped,
	}}, f.ctx)
}

// TestClockSetsTheNextStart shows 08:00 until told otherwise, and sets the
// time the next preview starts at for every window with the hour buttons
// and the time field; the rates and Stop clock wait for a preview.
func TestClockSetsTheNextStart(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if !f.has("state:time:08:00") || f.has("state:running") {
		t.Fatal("before a preview it shows 08:00")
	}
	f.click("hour-on")
	if f.published() != 9*3600 || !f.has("state:time:09:00") {
		t.Fatalf("+1h sets 09:00: %v", f.published())
	}
	f.click("hour-back")
	f.click("hour-back")
	if f.published() != 7*3600 {
		t.Fatalf("−1h twice: %v", f.published())
	}
	f.typeTime("17:30")
	if f.published() != 17*3600+30*60 {
		t.Fatalf("typed 17:30: %v", f.published())
	}
	f.typeTime("lunch")
	if f.published() != 17*3600+30*60 {
		t.Fatal("a time that is not one sets nothing")
	}
	f.click("rate-600")
	f.click("stop-clock")
	if len(f.requests.PreviewControls) != 0 {
		t.Fatal("with no preview, nothing is sent to one")
	}
	value, err := json.Marshal(protocol.ClockStart{Seconds: 6 * 3600})
	if err != nil {
		t.Fatal(err)
	}
	f.state.Changed(client.Update{Session: &protocol.Session{Topic: protocol.TopicClockStart, Value: value}}, f.ctx)
	if !f.has("state:time:06:00") {
		t.Fatal("another window's start time shows")
	}
	f.state.Draw(ebiten.NewImage(clockview.Width, clockview.Height), f.ctx.Docs)
}

// TestClockRunsThePreview shows the preview's time and sends its controls:
// an hour on and back (wrapping at midnight), a jump, the rate, and stopping
// and starting the clock; the preview's end returns to the next start.
func TestClockRunsThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID}}, f.ctx)
	f.preview(9*3600, 60, false, 1)
	if !f.has("state:running") || !f.has("state:time:09:00") {
		t.Fatal("the preview's time shows")
	}
	f.state.Draw(ebiten.NewImage(clockview.Width, clockview.Height), f.ctx.Docs)
	f.click("hour-on")
	if control := f.lastControl(); control.Action != protocol.ControlJump || control.Seconds != 10*3600 ||
		control.Run != runID {
		t.Fatalf("+1h jumps: %+v", control)
	}
	f.preview(30*60, 60, false, 2)
	f.click("hour-back")
	if control := f.lastControl(); control.Seconds != 23*3600+30*60 {
		t.Fatalf("−1h at 00:30 wraps to 23:30: %+v", control)
	}
	f.typeTime("6")
	if control := f.lastControl(); control.Action != protocol.ControlJump || control.Seconds != 6*3600 {
		t.Fatalf("typed 6 jumps to 06:00: %+v", control)
	}
	f.click("rate-600")
	if control := f.lastControl(); control.Action != protocol.ControlRate || control.Rate != 600 {
		t.Fatalf("rate: %+v", control)
	}
	f.click("stop-clock")
	if control := f.lastControl(); control.Action != protocol.ControlStopClock {
		t.Fatalf("stop: %+v", control)
	}
	f.preview(6*3600, 600, true, 3)
	f.click("stop-clock")
	if control := f.lastControl(); control.Action != protocol.ControlStartClock {
		t.Fatalf("start: %+v", control)
	}
	if len(f.requests.Published) != 0 {
		t.Fatal("during a preview the next start is left alone")
	}
	f.state.Changed(client.Update{PreviewStop: &protocol.PreviewStop{Run: runID}}, f.ctx)
	if f.has("state:running") || !f.has("state:time:08:00") {
		t.Fatal("after the preview the next start shows again")
	}
}
