package window

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// clockStandIn stands in for the Clock view: it says whether a preview runs
// and keeps what the bar asked of it.
type clockStandIn struct {
	running bool
	asked   []string
}

func (v *clockStandIn) Title(frame.Docs) string                       { return "Clock" }
func (v *clockStandIn) Update(*ui.Input, *frame.Context)              {}
func (v *clockStandIn) Changed(client.Update, *frame.Context)         {}
func (v *clockStandIn) Draw(*ebiten.Image, frame.Docs)                {}
func (v *clockStandIn) Focused() *ui.TextField                        { return nil }
func (v *clockStandIn) Locate(string, frame.Docs) (image.Point, bool) { return image.Pt(0, 0), false }
func (v *clockStandIn) Readout() (string, bool)                       { return "08:00", v.running }
func (v *clockStandIn) Ask(action string)                             { v.asked = append(v.asked, action) }
func (v *clockStandIn) Controls() (bool, bool)                        { return v.running, false }

// TestStartingAPreviewOpensNothing starts a preview: the clock's panel stays
// closed until its readout is pressed. The clock's hour steps work from the
// bar, and Stop only while a preview runs (increment 41).
func TestStartingAPreviewOpensNothing(t *testing.T) {
	t.Parallel()
	view := &clockStandIn{running: false, asked: nil}
	shown := &shell{size: image.Pt(1400, 900), clock: &pane{view: "clock", host: &host{view: view}}}
	press := func(id string) {
		for _, button := range shown.clockButtons() {
			if button.ID == id {
				shown.pressReadout(ui.Center(button.Rect))
			}
		}
	}
	press(clockview.AskStopStart)
	press(clockview.AskHourOn)
	if len(view.asked) != 1 || view.asked[0] != clockview.AskHourOn {
		t.Fatalf("with no preview, +1h works and Stop does nothing: %v", view.asked)
	}
	view.running = true
	shown.followClock()
	if shown.clockOpen {
		t.Fatal("a preview starting opens nothing")
	}
	press(clockview.AskStopStart)
	press(clockview.AskHourBack)
	if len(view.asked) != 3 || view.asked[1] != clockview.AskStopStart || view.asked[2] != clockview.AskHourBack {
		t.Fatalf("while a preview runs, Stop and −1h work from the bar: %v", view.asked)
	}
	shown.pressReadout(ui.Center(shown.readoutRect()))
	if !shown.clockOpen {
		t.Fatal("pressing the readout opens the panel")
	}
}
