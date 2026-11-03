package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// local returns the only local value.
func (f *fixture) local() worldstate.Declaration {
	f.t.Helper()
	values := f.workflow().Locals().Values()
	if len(values) != 1 {
		f.t.Fatalf("locals %+v", values)
	}

	return values[0]
}

// answerPick answers the latest pick on a map with the office desk.
func (f *fixture) answerPick() {
	f.t.Helper()
	f.requests.PickAsks[len(f.requests.PickAsks)-1](
		[]protocol.PickMap{{Window: frametest.ID(50), Zone: frametest.ID(51), Shows: true}}, nil)
	pick := f.requests.PickStarts[len(f.requests.PickStarts)-1]
	f.state.Changed(client.Update{PickAnswer: &protocol.PickAnswer{Pick: pick.Pick, Floor: floorID, Item: deskID.String()}},
		f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestRolesAndPlaces gives a local each role in turn, keeps its role
// through a change of type, and makes it a place that starts where it is
// picked on a map.
func TestRolesAndPlaces(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("Escape")
	f.click("ui:add-local")
	if f.local().Role != worldstate.RoleLocal {
		t.Fatal("a new local is a local of its own")
	}
	f.click("ui:value-role-input")
	if f.local().Role != worldstate.RoleInput {
		t.Fatalf("made an input: %+v", f.local())
	}
	f.click("ui:value-role-output")
	if f.local().Role != worldstate.RoleOutput {
		t.Fatalf("made an output: %+v", f.local())
	}
	f.click("ui:value-role-input")
	f.click("ui:value-type-place")
	if declared := f.local(); declared.Type != worldstate.TypePlace || declared.Role != worldstate.RoleInput ||
		!declared.Zero().Place.IsZero() {
		t.Fatalf("a place input with no place yet: %+v", declared)
	}
	f.click("ui:value-pick-place")
	f.answerPick()
	start := f.local().Zero()
	if start.Place != (worldstate.Place{Floor: floorID, Anchor: deskID}) {
		t.Fatalf("it starts at the desk picked: %+v", start)
	}
	f.click("ui:value-role-local")
	if f.local().Role != worldstate.RoleLocal {
		t.Fatal("back to a local of its own")
	}
}
