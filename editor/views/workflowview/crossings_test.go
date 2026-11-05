package workflowview_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func TestANodeDroppedOnALineIsLooseAndCrossed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindMove)
	f.drag(f.at("palette:"+flow.KindWait), f.at("line:start:1:next"))
	if !f.has("loose:wait:1") || !f.has("crossed:wait:1") {
		t.Fatal("the dropped wait is not linked, and the start's line runs under it")
	}
	if f.has("loose:move:1") || f.has("crossed:move:1") {
		t.Fatal("the move the line leads to is linked, and the line ends at it")
	}
}

func TestACheckFindsProblemsWithoutAWindow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindMove)
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:700,600"))
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	check := workflowview.NewCheck(workflowID, theme)
	check.Update(f.ctx)
	if families := check.Families(); !slices.Contains(families, workflowview.FamilyNotLinked) ||
		slices.Contains(families, workflowview.FamilyUnused) {
		t.Fatalf("the loose wait is not linked; who runs it is not known yet: %v", families)
	}
	answer := f.requests.RunnerAsks[len(f.requests.RunnerAsks)-1]
	answer([]protocol.Use{{Document: adaID, Name: "Ada Clerk", Format: npc.FormatName, Label: protocol.RunsAsBehaviour}}, nil)
	f.requests.Uses[len(f.requests.Uses)-1](nil, nil)
	if used := check.UsedBy(); used != "Ada Clerk" {
		t.Fatalf("Ada runs it: %q", used)
	}
	if slices.Contains(check.Families(), workflowview.FamilyUnused) {
		t.Fatal("a workflow an NPC runs is used")
	}
}
