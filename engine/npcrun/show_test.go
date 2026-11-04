package npcrun_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/show"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// showNode makes a Show node with its message and settings.
func showNode(t *testing.T, seed byte, kind, text string, settings map[string]any) flow.Node {
	t.Helper()
	params := map[string]any{}
	if text != "" {
		params[flow.ParamMessage] = []flow.Segment{textPart(text)}
	}
	for name, value := range settings {
		params[name] = value
	}

	return node(t, seed, kind, params)
}

// hour is a Wait that ends a test's workflow.
func hour(t *testing.T, seed byte) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)})
}

// TestSaying queues two lines over Ada's head, the second waited for: both
// go to the log at once, the first is said, then the second, and only then
// does Ada go on to her Wait.
func TestSaying(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		showNode(t, 2, flow.KindSay, "First", nil),
		showNode(t, 3, flow.KindSay, "Second", map[string]any{flow.ParamCarry: flow.CarryWhenDone}),
		hour(t, 4),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4))
	town := loggingTown(t)
	ada := town.place(100, image.Pt(2, 2), workflow)
	town.run(1)
	if line, saying := ada.Saying(); !saying || line.Text != "First" || len(ada.Lines) != 2 ||
		ada.Lines[1].Text != "Ada says: Second" {
		t.Fatalf("Ada says her first line, and both are in the log: %+v %+v", line, ada.Lines)
	}
	town.run(show.ReadingTicks("First"))
	if line, _ := ada.Saying(); line.Text != "Second" || ada.Node != id(3) {
		t.Fatalf("then her second, waiting on it: %+v %v", line, ada.Node)
	}
	town.run(show.ReadingTicks("Second"))
	if _, saying := ada.Saying(); saying || ada.Node != id(4) {
		t.Fatalf("once it is said she goes on to her Wait: %v", ada.Node)
	}
}

// TestBoxes shows Ada's box and then Bea's, one at a time in the order
// asked for; Ada waits for hers to go; then a box waits for the player to
// close it, holding them; and a shake moves the screen for its seconds.
func TestBoxes(t *testing.T) {
	t.Parallel()
	town := loggingTown(t)
	ada := town.place(100, image.Pt(2, 2), workflowOf(t, []flow.Node{
		showNode(t, 2, flow.KindShowMessage, "Ada's", map[string]any{
			flow.ParamCarry: flow.CarryWhenDone, flow.ParamBoxName: flow.BoxNameNPC,
		}),
		showNode(t, 3, flow.KindShowMessage, "Close me", map[string]any{
			flow.ParamAway: flow.AwayClosed, flow.ParamCarry: flow.CarryWhenDone,
		}),
		showNode(t, 4, flow.KindShake, "", map[string]any{flow.ParamStrength: flow.ShakeStrong, flow.ParamSeconds: 1}),
		hour(t, 5),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4), link(4, flow.OutputNext, 5)))
	town.place(110, image.Pt(4, 2), workflowOf(t, []flow.Node{
		showNode(t, 2, flow.KindShowMessage, "Bea's", nil), hour(t, 3),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3)))
	town.run(1)
	front, queued, shown := town.stage.Front()
	if !shown || front.Text != "Ada's" || front.Name != "Ada" || queued != 2 {
		t.Fatalf("Ada's box shows first, named for her, Bea's waiting: %+v %d", front, queued)
	}
	town.run(show.ReadingTicks("Ada's"))
	if front, _, _ := town.stage.Front(); front.Text != "Bea's" || ada.Node != id(3) {
		t.Fatalf("then Bea's, and Ada has asked for hers to be closed: %+v %v", front, ada.Node)
	}
	town.run(show.ReadingTicks("Bea's") + 1)
	if !town.stage.Holding() || ada.Node != id(3) {
		t.Fatal("the box to close holds the player, and Ada waits for it")
	}
	town.stage.Close()
	town.run(2)
	if !town.stage.Shaking() || ada.Node != id(4) {
		t.Fatal("closed, Ada shakes the screen")
	}
	if x, y := town.stage.Offset(); x == 0 && y == 0 {
		t.Fatal("the screen moves while it shakes")
	}
	town.run(show.SecondsTicks(1) + 1)
	if x, y := town.stage.Offset(); town.stage.Shaking() || x != 0 || y != 0 || ada.Node != id(5) {
		t.Fatalf("the shake ends after its second, and Ada goes on: %v %v %v", x, y, ada.Node)
	}
}
