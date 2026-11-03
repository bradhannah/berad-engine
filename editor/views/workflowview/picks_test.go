package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// startPick answers the latest question of which maps could answer, so
// the pick starts, and returns it.
func (f *fixture) startPick() protocol.PickStart {
	f.t.Helper()
	f.requests.PickAsks[len(f.requests.PickAsks)-1](
		[]protocol.PickMap{{Window: frametest.ID(50), Zone: frametest.ID(51), Shows: true}}, nil)

	return f.requests.PickStarts[len(f.requests.PickStarts)-1]
}

// TestPicksSayWhatTheyDo starts a pick for a place local's start, takes it
// over from a Call's place input, and answers it, the status naming the
// place; then the input's own button starts a pick and, clicked again,
// cancels it.
func TestPicksSayWhatTheyDo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCall()
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-type-place")
	f.click("ui:value-pick-place")
	first := f.startPick()
	if !f.has("picking") {
		t.Fatal("the local's start is being picked")
	}
	f.click("node:call-workflow:1")
	f.click("ui:input-place-pick")
	if len(f.requests.PickCancels) != 1 || f.requests.PickCancels[0].Pick != first.Pick ||
		f.requests.PickCancels[0].Reason != "another pick started" {
		t.Fatalf("the input's pick takes the local's over: %+v", f.requests.PickCancels)
	}
	second := f.startPick()
	f.state.Changed(client.Update{PickAnswer: &protocol.PickAnswer{
		Pick: second.Pick, Floor: floorID, Item: deskID.String(), Name: "Desk",
	}}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if got := f.status(); got != "Picked “Desk” for place." {
		t.Fatalf("the status names the place picked: %q", got)
	}
	f.click("ui:input-place-pick")
	if len(f.requests.PickAsks) != 3 || len(f.requests.PickCancels) != 1 {
		t.Fatalf("with no pick running, the button starts one: %d asks, %+v", len(f.requests.PickAsks),
			f.requests.PickCancels)
	}
	third := f.startPick()
	f.click("ui:input-place-pick")
	if len(f.requests.PickCancels) != 2 || f.requests.PickCancels[1].Pick != third.Pick || f.has("picking") {
		t.Fatalf("clicked again, it cancels its own pick: %+v", f.requests.PickCancels)
	}
}
