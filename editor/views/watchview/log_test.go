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
	"github.com/bradhannah/berad-engine/editor/views/watchview"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// line is a line the clerk's Wait wrote, or another NPC's.
func line(seq uint64, text string, clerk bool) protocol.PreviewLine {
	entity, npcID := frametest.ID(9).String(), frametest.ID(10)
	if clerk {
		entity, npcID = placementID.String(), clerkID
	}

	return protocol.PreviewLine{
		Seq: seq, Clock: 9 * 3600, Entity: entity, NPC: npcID, Loop: ids.ULID{}, Workflow: routineID, Node: waitID,
		Text: text,
	}
}

// sendLines gives the window a preview state watching the clerk, holding
// the latest lines.
func (f *fixture) sendLines(tick uint64, lines ...protocol.PreviewLine) {
	f.send(tick, false)
	state := *f.state.StateNow()
	state.Tick++
	state.Log = lines
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestTheLog keeps the lines a preview's Logs write, each once, shows the
// watched clerk's or every NPC's, opens a line's workflow at its node,
// clears, keeps MaxLog lines and starts afresh with the next preview.
func TestTheLog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.sendLines(5, line(1, "Opening up", true), line(2, "Waiting for Ada", false), line(3, "Serving Bea", true))
	if !f.has("log-lines=2") || !f.has("log-line=Serving Bea") || f.has("log-line=Waiting for Ada") {
		t.Fatal("the log shows the watched clerk's lines")
	}
	f.click("ui:log-every")
	f.sendLines(7, line(2, "Waiting for Ada", false), line(3, "Serving Bea", true), line(4, "Closing", true))
	if !f.has("log-lines=4") || !f.has("log-line=Waiting for Ada") {
		t.Fatal("every NPC's lines show, each once")
	}
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.click("log-line:1")
	opened := f.requests.Opened
	if len(opened) != 1 || opened[0].View != workflowview.ViewName || opened[0].Target != routineID ||
		opened[0].Item != workflowview.NodeItem+waitID.String() {
		t.Fatalf("a line opens the workflow at the node that wrote it: %+v", opened)
	}
	f.click("ui:log-clear")
	if !f.has("log-lines=0") {
		t.Fatal("Clear empties the log")
	}
	var many []protocol.PreviewLine
	for seq := uint64(5); seq < 5+watchview.MaxLog+100; seq++ {
		many = append(many, line(seq, "Line "+strconv.FormatUint(seq, 10), true))
	}
	f.sendLines(9, many...)
	if !f.has("log-lines="+strconv.Itoa(watchview.MaxLog)) || f.has("log-line=Line 5") {
		t.Fatal("the log keeps the latest MaxLog lines")
	}
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("log-lines=0") {
		t.Fatal("a new preview starts the log afresh")
	}
}
