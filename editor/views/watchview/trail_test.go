package watchview_test

import (
	"image"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// sendTrail gives the window a preview state watching the clerk, with a
// trail and whom it is of.
func (f *fixture) sendTrail(tick uint64, watched ids.ULID, trail []protocol.PreviewTrail) {
	clerk := protocol.PreviewEntity{
		ID: placementID.String(), Floor: floorID, X: 4, Y: 2, Face: [2]int{0, 1}, NPC: clerkID, Workflow: routineID,
		Node: waitID, Activity: "Wait 12 min of 30 min (game time)",
	}
	state := protocol.PreviewState{
		Run: runID, Tick: tick, Entities: []protocol.PreviewEntity{clerk}, Clock: 9 * 3600, Rate: 60,
		Watched: watched, Trail: trail,
	}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// passed is a change on the trail from the start to the Wait, saying why.
func passed(seq uint64, why string) protocol.PreviewTrail {
	return protocol.PreviewTrail{Seq: seq, Clock: 9 * 3600, Change: protocol.PreviewChange{
		From: startID, Output: flow.OutputNext, To: waitID, Tick: seq, Why: why,
	}}
}

// TestTheStripShowsTheLogTheTrailOrTheValues shows the Log first, and the
// Trail or the Values when their tabs are pressed; the trail keeps each
// change once, and starts again when another NPC is watched (increment 46).
func TestTheStripShowsTheLogTheTrailOrTheValues(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.sendTrail(1, placementID, []protocol.PreviewTrail{passed(1, "Started again: the clock moved to 9:00")})
	f.sendTrail(2, placementID, []protocol.PreviewTrail{
		passed(1, "Started again: the clock moved to 9:00"), passed(2, "Waited 30 min"),
	})
	if !f.has("strip=log") || !f.has("trail-lines=2") || !f.has("trail:Waited 30 min") {
		t.Fatal("the Log shows first, and the trail keeps each change once")
	}
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.click("ui:strip-trail")
	if !f.has("strip=trail") {
		t.Fatal("the Trail tab shows the trail")
	}
	if _, found := f.state.Locate("ui:log-every", f.docs); found {
		t.Fatal("the log's own buttons go while the trail shows")
	}
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.click("ui:strip-values")
	if !f.has("strip=values") {
		t.Fatal("the Values tab shows the values")
	}
	f.sendTrail(3, frametest.ID(42), []protocol.PreviewTrail{passed(3, "Read 9:00: 9:00 to 17:00 holds")})
	if !f.has("trail-lines=1") || f.has("trail:Waited") {
		t.Fatal("watching another NPC starts the trail again")
	}
}

// TestTheValuesScrollWhenTheyDoNotFit shows as many values as fit in the
// strip's columns, and the wheel scrolls the rest into view (increment 46).
func TestTheValuesScrollWhenTheyDoNotFit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	values := make([]protocol.PreviewValue, 0, 40)
	for index := range 40 {
		values = append(values, protocol.PreviewValue{Path: "npc.v" + strconv.Itoa(index), Value: "1"})
	}
	state := protocol.PreviewState{Run: runID, Tick: 1, Clock: 9 * 3600, Rate: 60, Watched: placementID, Values: values}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.click("ui:strip-values")
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	wheel := ui.NewInput(image.Pt(400, 470))
	wheel.WheelY = -1
	f.frame(wheel)
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	if !f.has("first-value=2") {
		t.Fatal("the wheel scrolls the values a row")
	}
}
