package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/show"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// shown makes a Show node with a message and settings.
func shown(t *testing.T, seed byte, kind, text string, settings map[string]any) flow.Node {
	t.Helper()
	made, err := flow.NewNode(kind)
	if err == nil && text != "" {
		made, err = made.WithParam(flow.ParamMessage, []flow.Segment{{Text: text, Ref: nil}})
	}
	for name, value := range settings {
		if err == nil {
			made, err = made.WithParam(name, value)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	made.ID = frametest.ID(seed)

	return made
}

// TestShowingInThePreview previews the clerk, whose workflow says a line,
// shows a box the player must close, then shakes the screen: the line is
// over the clerk, the box holds the player until a click closes it, and the
// screen then shakes.
func TestShowingInThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	f.docs.Put(worldstate.ID(), worldstate.Name, stateplugin.NewDocument())
	say := shown(t, 90, flow.KindSay, "Fresh bread!", nil)
	box := shown(t, 91, flow.KindShowMessage, "Mind the step", map[string]any{
		flow.ParamAway: flow.AwayClosed, flow.ParamCarry: flow.CarryWhenDone,
	})
	shake := shown(t, 92, flow.KindShake, "", map[string]any{flow.ParamSeconds: 1})
	f.docs.View(routineID, func(document client.Document) {
		routine := document.Model.(*workflowplugin.Document).Workflow
		first, _ := routine.Node(frametest.ID(80))
		if err := routine.Insert([]flow.Node{
			say.WithLink(flow.OutputNext, box.ID), box.WithLink(flow.OutputNext, shake.ID),
			shake.WithLink(flow.OutputNext, first.Next[flow.OutputNext]),
		}, nil, []flow.Link{{From: frametest.ID(80), Output: flow.OutputNext, To: say.ID}}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(routineID)
	f.startPreview("cell:3,3")
	f.idle(3)
	if !f.has("speaking:Ada Clerk=Fresh bread!") || !f.has("box=Mind the step") || !f.has("boxes=1") {
		t.Fatal("the clerk says her line, and the box shows")
	}
	if _, found := f.state.Locate("speech:Ada Clerk", f.docs); !found {
		t.Fatal("her line is drawn over her head")
	}
	if box := f.lastState().Box; box == nil || !box.Closing {
		t.Fatalf("the preview's state carries the box, waiting to be closed: %+v", box)
	}
	f.idle(show.ReadingTicks("Mind the step") + 2)
	if !f.has("box=Mind the step") || f.has("shaking") {
		t.Fatal("the box stays until the player closes it, and the clerk waits")
	}
	f.click("box")
	f.idle(3)
	if f.has("box=Mind the step") || !f.has("shaking") {
		t.Fatal("a click closes it, and the clerk shakes the screen")
	}
}
