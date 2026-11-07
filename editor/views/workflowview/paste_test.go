package workflowview_test

import (
	"strings"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// memoryClipboard is a clipboard shared by the windows of a test.
type memoryClipboard struct{ text string }

func (m *memoryClipboard) Read() (string, error) { return m.text, nil }

func (m *memoryClipboard) Write(text string) error {
	m.text = text

	return nil
}

// withLocal declares a local value in the fixture's workflow.
func (f *fixture) withLocal(seed byte, name string, kind worldstate.Type) ids.ULID {
	f.t.Helper()
	declared := worldstate.New(frametest.ID(seed), name, worldstate.ScopeLocal, kind)
	f.carryOutOn(workflowID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{declared}, Set: nil, Remove: nil,
	})

	return declared.ID
}

// copiedLog copies a Log of its workflow's local count, and returns the
// clipboard holding it.
func copiedLog(t *testing.T) *memoryClipboard {
	t.Helper()
	f := withNode(t, flow.KindLog)
	f.withLocal(54, "count", worldstate.TypeInteger)
	f.typeInto("param-message", "Count is {local.count}")
	clipboard := &memoryClipboard{text: ""}
	f.ctx.Clipboard = clipboard
	f.keys("Cmd+C")
	if !strings.Contains(clipboard.text, "berad.workflow.nodes") || !strings.Contains(f.status(), "Copied 1 node") {
		t.Fatalf("Cmd+C copies the Log: %q", f.status())
	}

	return clipboard
}

// pasteInto pastes a clipboard into a fixture's workflow with Cmd+V.
func (f *fixture) pasteInto(clipboard *memoryClipboard) {
	f.t.Helper()
	f.ctx.Clipboard = clipboard
	f.keys("Cmd+V")
}

// loggedLocal is the local the nth Log's message reads.
func (f *fixture) loggedLocal(number int) ids.ULID {
	f.t.Helper()
	segments, err := f.nth(flow.KindLog, number).Message()
	for _, segment := range segments {
		if segment.Ref != nil {
			return segment.Ref.Value
		}
	}
	f.t.Fatalf("the Log reads no value: %v %v", segments, err)

	return ids.ULID{}
}

// TestPasteDeclaresTheLocalsItNeeds pastes a Log of local.count into a
// workflow with no count: count is declared there with the Log, as one
// undo step, and the pasted Log is selected.
func TestPasteDeclaresTheLocalsItNeeds(t *testing.T) {
	t.Parallel()
	clipboard := copiedLog(t)
	f := newFixture(t)
	f.pasteInto(clipboard)
	declared, found := f.workflow().Locals().Value(frametest.ID(54))
	if !f.has("nodes:2") || !found || declared.Name != "count" || f.loggedLocal(1) != declared.ID {
		t.Fatalf("the Log is pasted and count declared for it: %+v %v", declared, found)
	}
	locals, adds := f.requests.Of(workflowplugin.ChangeLocalsEdit), f.requests.Of(workflowplugin.AddEdit)
	if len(locals) != 1 || len(adds) != 1 || locals[0].Command.Gesture != adds[0].Command.Gesture {
		t.Fatal("declaring count and adding the Log are one undo step")
	}
	if !f.has("selected:"+flow.KindLog) || !strings.Contains(f.status(), "Declared count") {
		t.Fatalf("the pasted Log is selected, and the status says what was declared: %q", f.status())
	}
}

// TestPasteUsesALocalAlreadyThere pastes the Log into a workflow with its
// own count of the same type, which the Log reads instead.
func TestPasteUsesALocalAlreadyThere(t *testing.T) {
	t.Parallel()
	clipboard := copiedLog(t)
	f := newFixture(t)
	own := f.withLocal(55, "count", worldstate.TypeInteger)
	f.pasteInto(clipboard)
	if !f.has("nodes:2") || f.loggedLocal(1) != own || f.workflow().Locals().Len() != 1 {
		t.Fatal("the pasted Log reads the count already there, and nothing is declared")
	}
}

// TestPasteNumbersALocalOfAnotherType pastes the Log into a workflow whose
// count is text: the Log's count is declared as count_2.
func TestPasteNumbersALocalOfAnotherType(t *testing.T) {
	t.Parallel()
	clipboard := copiedLog(t)
	f := newFixture(t)
	f.withLocal(55, "count", worldstate.TypeText)
	f.pasteInto(clipboard)
	declared, found := f.workflow().Locals().Value(frametest.ID(54))
	if !found || declared.Name != "count_2" || f.loggedLocal(1) != declared.ID {
		t.Fatalf("the Log's count is declared as count_2: %+v", declared)
	}
}

// TestPasteLeavesOtherTextAlone pastes text that is not a copy of nodes,
// and a copy whose node this workflow's purpose doesn't offer.
func TestPasteLeavesOtherTextAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.pasteInto(&memoryClipboard{text: "Good morning"})
	if !f.has("nodes:1") || len(f.requests.Of(workflowplugin.AddEdit)) != 0 {
		t.Fatal("text that is not a copy of nodes pastes nothing")
	}
}

// TestPasteInTheSameWorkflow copies a Wait and pastes it back: a copy with
// no new locals.
func TestPasteInTheSameWorkflow(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	clipboard := &memoryClipboard{text: ""}
	f.ctx.Clipboard = clipboard
	f.keys("Cmd+C")
	f.keys("Cmd+V")
	if !f.has("nodes:3") || len(f.requests.Of(workflowplugin.ChangeLocalsEdit)) != 0 {
		t.Fatal("a paste in the same workflow adds a copy")
	}
}

// TestPasteRefusesNodesTheWorkflowDoesNotOffer copies a Go to and pastes it
// into a zone loop, which has no body to move: nothing is pasted.
func TestPasteRefusesNodesTheWorkflowDoesNotOffer(t *testing.T) {
	t.Parallel()
	source := withNode(t, flow.KindGoTo)
	clipboard := &memoryClipboard{text: ""}
	source.ctx.Clipboard = clipboard
	source.keys("Cmd+C")
	f := newFixture(t)
	f.asZoneLoop()
	f.pasteInto(clipboard)
	if !f.has("nodes:1") || !strings.Contains(f.status(), "This workflow has no Go to") {
		t.Fatalf("a zone loop takes no Go to: %q", f.status())
	}
}

// TestRightClickDuplicatesAndCopies duplicates a Wait from its right-click
// menu, then copies it from there.
func TestRightClickDuplicatesAndCopies(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	clipboard := &memoryClipboard{text: ""}
	f.ctx.Clipboard = clipboard
	f.rightClick("node:" + flow.KindWait + ":1")
	f.click("menu:duplicate")
	if !f.has("nodes:3") || !f.has("selected:"+flow.KindWait) {
		t.Fatal("Duplicate copies the Wait and selects the copy")
	}
	f.rightClick("node:" + flow.KindWait + ":2")
	f.click("menu:copy")
	if !strings.Contains(clipboard.text, flow.KindWait) {
		t.Fatal("Copy puts the Wait on the clipboard")
	}
	f.rightClick("node:start:1")
	if !f.menuOffers("duplicate") {
		t.Fatal("the start's menu shows Duplicate, greyed")
	}
}
