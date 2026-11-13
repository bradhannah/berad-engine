package mapview_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/play"
)

// TestThePreviewKeepsTheWatchedNPCsTrail sends the watched NPC's changes of
// node, numbered, at the game time, with what sent it on, and starts the
// trail again when another NPC is watched (increment 46).
func TestThePreviewKeepsTheWatchedNPCsTrail(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	f.startPreview("cell:3,3")
	f.idle(play.StepTicks + 2)
	f.click("live-npc:Ada Clerk")
	f.idle(2)
	if trail := f.lastState().Trail; len(trail) != 0 {
		t.Fatalf("the trail starts when the clerk is watched: %+v", trail)
	}
	f.clockControl(protocol.ControlJump, 0, 12*3600)
	f.idle(play.StepTicks + 2)
	trail := f.lastState().Trail
	whys := make([]string, 0, len(trail))
	for index, entry := range trail {
		whys = append(whys, entry.Change.Why)
		if entry.Seq != uint64(index+1) || entry.Clock < 12*3600 {
			t.Fatalf("each change is numbered, at the game time: %+v", entry)
		}
	}
	if !slices.Contains(whys, "Started again: the clock moved to 12:00, clearing what it was doing") ||
		!slices.Contains(whys, "Moved 1 tile right") {
		t.Fatalf("the trail says why the clerk started again and what it did: %q", whys)
	}
	f.state.Changed(client.Update{PreviewControl: &protocol.PreviewControl{
		Run: runID, Action: protocol.ControlWatch, Placement: frametest.ID(99),
	}}, f.ctx)
	f.idle(2)
	if trail := f.lastState().Trail; len(trail) != 0 {
		t.Fatalf("watching another NPC starts the trail again: %+v", trail)
	}
}
