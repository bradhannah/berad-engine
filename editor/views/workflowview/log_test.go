package workflowview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestWritingALine types a Log's message, inserts a value where the caret
// was, reads how the line would read for Ada, who follows the workflow and
// lives in Town, types a value by its path, and refuses one there is none
// of.
func TestWritingALine(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindLog)
	f.withTown()
	f.docs.View(adaID, func(document client.Document) {
		document.Model.(*npcplugin.Document).NPC.Zone = frametest.ID(87)
	})
	f.docs.Touch(adaID)
	count := worldstate.New(frametest.ID(54), "count", worldstate.ScopeLocal, worldstate.TypeInteger)
	f.carryOutOn(workflowID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{count}, Set: nil, Remove: nil,
	})
	f.typeInto("param-message", "Count is ")
	f.click("ui:insert-value")
	f.click("valuerow:local.count")
	if !f.has("line=Count is {local.count}") || !f.has("reads=Count is 0") || !f.has("unsaved=1") {
		t.Fatal("a value inserted after the text reads as its text form, and waits for Save")
	}
	f.click("ui:save")
	if !f.has("message=Count is {local.count}") {
		t.Fatal("Save keeps it")
	}
	f.typeInto("param-message", "At {npc.zone} with {local.count}")
	if !f.has("message=At {var.npc.zone} with {local.count}") || !f.has("reads=At Town with 0") {
		t.Fatal("a value typed by its path is kept, and reads as it would for Ada")
	}
	if !f.has("title:log:1=Log · At {var.npc.zone} with {local.count}") || f.has("problem:log:1") {
		t.Fatal("the node shows its message")
	}
	f.typeInto("param-message", "{local.nope}")
	if !strings.HasPrefix(f.status(), "No value is called local.nope.") ||
		!f.has("message=At {var.npc.zone} with {local.count}") {
		t.Fatalf("a value there is none of is refused: %q", f.status())
	}
}

// TestOpeningAtANode selects the node a Watch line names, as the window
// opens at it.
func TestOpeningAtANode(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindLog)
	logged := f.nth(flow.KindLog, 1)
	f.click("node:start:1")
	if f.has("selected:" + flow.KindLog) {
		t.Fatal("the start is selected")
	}
	f.state.Changed(client.Update{Attention: true, Item: workflowview.NodeItem + logged.ID.String()}, f.ctx)
	f.frame(noInput())
	if !f.has("selected:" + flow.KindLog) {
		t.Fatal("opening at the Log selects it")
	}
}

// TestABraceInALine types a line with a brace written as two: it is kept as
// text, reads with one brace, and shows with two again, while a value beside
// it still reads as its value (increment 60).
func TestABraceInALine(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindLog)
	count := worldstate.New(frametest.ID(54), "count", worldstate.ScopeLocal, worldstate.TypeInteger)
	f.carryOutOn(workflowID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{count}, Set: nil, Remove: nil,
	})
	f.typeInto("param-message", "Use the {{red} door, {local.count}")
	if !f.has("message=Use the {{red} door, {local.count}") || !f.has("reads=Use the {red} door, 0") {
		t.Fatalf("two braces are one of text: %q", f.status())
	}
	segments, err := f.nth(flow.KindLog, 1).Message()
	if err != nil || len(segments) != 2 || segments[0].Text != "Use the {red} door, " {
		t.Fatalf("the line holds one brace as text: %+v %v", segments, err)
	}
}
