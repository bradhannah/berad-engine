package workflowview_test

import (
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// answerCallers answers the latest question of who calls the workflow.
func (f *fixture) answerCallers(names ...string) {
	f.t.Helper()
	for index := len(f.requests.UsesOf) - 1; index >= 0; index-- {
		if f.requests.UsesOf[index].Item != workflowplugin.CallItem {
			continue
		}
		uses := make([]protocol.Use, 0, len(names))
		for _, name := range names {
			uses = append(uses, protocol.Use{Document: workflowID, Name: name, Format: "workflow",
				Label: "a Call workflow node", Item: workflowplugin.CallItem})
		}
		f.requests.Uses[index](uses, nil)
		f.typeText("")

		return
	}
	f.t.Fatal("nobody asked who calls the workflow")
}

// TestCalledByAndTheLock shows who calls a workflow and its ways out along
// the top of its canvas, and fixes an input's type while it is called but
// not a local's of its own.
func TestCalledByAndTheLock(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:return")
	f.answerCallers("Shopkeeper day", "Guard rounds")
	if !f.has("calledby=Called by 2: Shopkeeper day, Guard rounds · Ways out: finished, done") {
		t.Fatal("the strip names who calls it and its ways out")
	}
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-type-bool")
	if f.local().Type != worldstate.TypeBool {
		t.Fatal("a local of its own changes type while called")
	}
	f.click("ui:value-role-input")
	f.click("ui:value-type-text")
	if f.local().Type != worldstate.TypeBool {
		t.Fatal("an input's type is fixed while the workflow is called")
	}
	f.answerCallers()
	f.click("ui:value-type-text")
	if f.local().Type != worldstate.TypeText {
		t.Fatal("called by nothing, an input changes type")
	}
}
