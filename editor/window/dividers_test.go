package window

import (
	"encoding/json"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// panelView is a pickingView with a left panel whose width is dragged.
type panelView struct {
	pickingView

	left int
}

func (v *panelView) LeftEdge() (int, bool) {
	if v.left > 0 {
		return v.left, true
	}

	return 190, true
}

func (v *panelView) SetLeftWidth(width int) { v.left = width }

// panelPane is a pane of a panelView, laid out at 1200 by 800.
func panelPane(view string) (*pane, *panelView) {
	shown := titled(view, "Map: Town")
	made := &panelView{pickingView: pickingView{selected: ""}, left: 0}
	shown.host = &host{view: made, frame: frame.NewFrame(time.Now), visible: image.Pt(1200, 800)}
	shown.host.frame.ShowInspector.On, shown.host.frame.ShowLeft.On = true, true

	return shown, made
}

// drag drags from one point to another on a host's dividers, the window
// following the widths while the divider is held, as it does every frame.
func drag(shown *host, from, to image.Point, follow func()) {
	press := ui.NewInput(from)
	press.Pressed, press.Down = true, true
	shown.updateDividers(&press)
	hold := ui.NewInput(to)
	hold.Down = true
	shown.updateDividers(&hold)
	if follow != nil {
		follow()
	}
	release := ui.NewInput(to)
	shown.updateDividers(&release)
}

// TestDividersDrag drags the inspector's and the left panel's dividers,
// keeps the canvas's room and says what makes more, puts a panel back on a
// double-click, and gives every view of the kind the widths (increment 53).
// It sets the inspector's width every view reads, so it runs alone.
func TestDividersDrag(t *testing.T) { //nolint:paralleltest // It sets the width every view lays out with.
	first, view := panelPane("map")
	second, other := panelPane("map")
	shown := &shell{
		tabs: []*tab{{panes: []*pane{first}}, {panes: []*pane{second}}}, folded: map[string]bool{},
		leftFolded: map[string]bool{}, widths: map[string]kindWidths{}, size: image.Pt(1200, 800),
	}
	host := first.host
	host.useWidths()
	drag(host, ui.Center(host.inspectorDivider()), image.Pt(820, 300), shown.followInspector)
	if host.inspectorWidth != 380 || !host.arranged {
		t.Fatalf("the inspector's divider drags: %d", host.inspectorWidth)
	}
	shown.followInspector()
	if second.host.inspectorWidth != 380 {
		t.Fatal("every Map takes the width")
	}
	host.useWidths()
	drag(host, ui.Center(host.leftDivider()), image.Pt(260, 300), shown.followInspector)
	shown.followInspector()
	if view.left != 260 || other.left != 260 {
		t.Fatalf("the left panel's divider drags for every Map: %d %d", view.left, other.left)
	}
	host.useWidths()
	drag(host, ui.Center(host.inspectorDivider()), image.Pt(300, 300), shown.followInspector)
	if host.inspectorWidth != 1200-260-minCanvas || !strings.Contains(host.frame.Refusal(), "Cmd+Shift+I") {
		t.Fatalf("the canvas keeps its room, and the refusal says what makes more: %d %q",
			host.inspectorWidth, host.frame.Refusal())
	}
	double := ui.NewInput(ui.Center(host.inspectorDivider()))
	double.Pressed, double.Down, double.DoubleClick = true, true, true
	host.updateDividers(&double)
	shown.followInspector()
	if host.inspectorWidth != 0 || second.host.inspectorWidth != 0 {
		t.Fatal("a double-click puts the inspector back")
	}
	frame.SetInspectorWidth(0)
}

// TestTheArrangementIsKept puts back each kind's widths and folds the
// orchestrator kept for this size of screen (increment 53).
func TestTheArrangementIsKept(t *testing.T) {
	t.Parallel()
	first, view := panelPane("map")
	shown := &shell{
		tabs: []*tab{{panes: []*pane{first}}}, folded: map[string]bool{}, leftFolded: map[string]bool{},
		widths: map[string]kindWidths{}, screen: "1512x982",
	}
	kept, err := json.Marshal(protocol.Layout{Screens: map[string]protocol.Arrangement{
		"1512x982":  {Kinds: map[string]protocol.KindLayout{"map": {Inspector: 340, Left: 230, InspectorFolded: true}}},
		"2560x1440": {Kinds: map[string]protocol.KindLayout{"map": {Inspector: 500}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	shown.keepArrangement(kept)
	if first.host.inspectorWidth != 340 || view.left != 230 || first.host.frame.ShowInspector.On {
		t.Fatalf("this screen's arrangement is put back: %d %d %v", first.host.inspectorWidth, view.left,
			first.host.frame.ShowInspector.On)
	}
}
