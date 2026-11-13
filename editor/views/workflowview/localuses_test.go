package workflowview_test

import (
	"encoding/json"
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestALocalInUseIsRefused adds a local value and a Set value node that sets
// it: deleting the local is refused, as a world value in use is, and the
// nodes using it are listed (increment 58).
func TestALocalInUseIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("Escape")
	f.click("ui:add-local")
	local := f.workflow().Locals().Values()[0]
	node, err := flow.NewNode(flow.KindSetValue)
	if err != nil {
		t.Fatal(err)
	}
	node.ID = frametest.ID(77)
	ref, err := json.Marshal(flow.ValueRef{Scope: worldstate.ScopeLocal, Value: local.ID})
	if err != nil {
		t.Fatal(err)
	}
	node.Params[flow.ParamValue] = ref
	if err := f.workflow().Insert([]flow.Node{node}, map[ids.ULID]flow.Box{node.ID: {X: 400, Y: 200}}, nil); err != nil {
		t.Fatal(err)
	}
	f.docs.Touch(workflowID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:value-delete")
	if f.workflow().Locals().Len() != 1 {
		t.Fatal("a local in use is not deleted")
	}
	if got := f.status(); !strings.Contains(got, "local.value is used by 1 node") {
		t.Fatalf("the refusal says why: %q", got)
	}
	if !f.has("dialog-row=Set value") {
		t.Fatal("the node using it is listed")
	}
}
