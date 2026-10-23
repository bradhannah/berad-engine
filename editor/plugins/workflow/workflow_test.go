package workflow_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func id(seed byte) ids.ULID {
	var made ids.ULID
	made[15] = seed

	return made
}

// counter hands out IDs 10, 11, 12…, like the orchestrator's generator.
func counter() model.IDSource {
	next := byte(10)

	return func() (ids.ULID, error) {
		next++

		return id(next - 1), nil
	}
}

func encode(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func newNode(t *testing.T, kind string, seed byte, parent ids.ULID) flow.Node {
	t.Helper()
	made, err := flow.NewNode(kind)
	if err != nil {
		t.Fatal(err)
	}
	made.ID, made.Parent = id(seed), parent

	return made
}

// newDocument creates a workflow the way the orchestrator does: its start
// node gets an ID when it is prepared.
func newDocument(t *testing.T) *workflow.Document {
	t.Helper()
	data, err := workflow.Prepare(workflow.NewData(), func() (ids.ULID, error) { return id(1), nil })
	if err != nil {
		t.Fatal(err)
	}
	document, err := workflow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	workflowDoc, ok := document.(*workflow.Document)
	if !ok {
		t.Fatal("not a workflow document")
	}

	return workflowDoc
}

func apply(t *testing.T, document model.Document, kind model.EditKind, args json.RawMessage) {
	t.Helper()
	if _, err := kind.Apply(document, args); err != nil {
		t.Fatal(err)
	}
}

// TestEditsMeetUndoContract builds a small routine and checks every edit
// restores the workflow and its layout exactly when undone.
func TestEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(workflow.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := newDocument(t)
	if document.Workflow.Start() != id(1) {
		t.Fatalf("start %v", document.Workflow.Start())
	}
	container := newNode(t, flow.KindContainer, 3, ids.ULID{})
	inner := newNode(t, flow.KindWait, 4, id(3))
	container = container.WithLink(flow.OutputInside, id(4))
	add := encode(t, workflow.AddArgs{
		Nodes: []flow.Node{container, inner},
		Boxes: map[ids.ULID]flow.Box{id(3): {X: 200, Y: 0, W: 300, H: 160}, id(4): {X: 220, Y: 40, W: 0, H: 0}},
		Links: []flow.Link{{From: id(1), Output: flow.OutputNext, To: id(3)}},
	})
	checks := []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{workflow.Add{}, add},
	}
	for _, check := range checks {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatal(err)
		}
	}
	apply(t, document, workflow.Add{}, add)
	longer, err := inner.WithParam("minutes", 90)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{workflow.Replace{}, encode(t, workflow.ReplaceArgs{Node: longer})},
		{workflow.Move{}, encode(t, workflow.MoveArgs{Boxes: map[ids.ULID]flow.Box{
			id(3): {X: 5, Y: 5, W: 320, H: 180}, id(1): {X: 0, Y: 0, W: 0, H: 0},
		}})},
		{workflow.Remove{}, encode(t, workflow.RemoveArgs{Nodes: []ids.ULID{id(3)}})},
		{workflow.Remove{}, encode(t, workflow.RemoveArgs{Nodes: []ids.ULID{id(4)}})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
	}
}

func TestAddGivesNewNodesIDs(t *testing.T) {
	t.Parallel()
	document := newDocument(t)
	wait := newNode(t, flow.KindWait, 0, ids.ULID{})
	args := encode(t, workflow.AddArgs{
		Nodes: []flow.Node{wait}, Boxes: map[ids.ULID]flow.Box{{}: {X: 40, Y: 60, W: 0, H: 0}}, Links: nil,
	})
	prepared, err := (workflow.Add{}).Prepare(document, args, counter())
	if err != nil {
		t.Fatal(err)
	}
	again, err := (workflow.Add{}).Prepare(document, prepared, counter())
	if err != nil || string(again) != string(prepared) {
		t.Fatal("preparing a prepared edit leaves it unchanged")
	}
	apply(t, document, workflow.Add{}, prepared)
	if node, ok := document.Workflow.Node(id(10)); !ok || node.Kind != flow.KindWait {
		t.Fatal("the new node has the first assigned ID")
	}
	if box, ok := document.Workflow.Box(id(10)); !ok || box.Y != 60 {
		t.Fatalf("its box moved to its new ID: %v %v", box, ok)
	}
}

func TestEditsRefuseNodesThatDoNotFit(t *testing.T) {
	t.Parallel()
	document := newDocument(t)
	wait := newNode(t, flow.KindWait, 2, ids.ULID{})
	apply(t, document, workflow.Add{}, encode(t, workflow.AddArgs{Nodes: []flow.Node{wait}, Boxes: nil, Links: nil}))
	badMinutes, _ := wait.WithParam("minutes", -5)
	wrongOutput := wait.WithLink(flow.OutputYes, id(2))
	for name, args := range map[string]json.RawMessage{
		"bad parameter": encode(t, workflow.ReplaceArgs{Node: badMinutes}),
		"wrong output":  encode(t, workflow.ReplaceArgs{Node: wrongOutput}),
		"remove start":  encode(t, workflow.RemoveArgs{Nodes: []ids.ULID{id(1)}}),
	} {
		var kind model.EditKind = workflow.Replace{}
		if name == "remove start" {
			kind = workflow.Remove{}
		}
		if _, err := kind.Apply(document, args); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestReferencesListGoToAnchors(t *testing.T) {
	t.Parallel()
	document := newDocument(t)
	goTo := newNode(t, flow.KindGoTo, 2, ids.ULID{})
	goTo, err := goTo.WithParam("destination", tilefloor.AnchorRef{Floor: id(40), Anchor: id(41)})
	if err != nil {
		t.Fatal(err)
	}
	unset := newNode(t, flow.KindGoTo, 3, ids.ULID{})
	apply(t, document, workflow.Add{}, encode(t, workflow.AddArgs{Nodes: []flow.Node{goTo, unset}, Boxes: nil, Links: nil}))
	references := document.References()
	if len(references) != 1 || references[0].Document != id(40) || references[0].Item != id(41).String() {
		t.Fatalf("references %+v", references)
	}
}

func TestLayoutTravelsWithTheDocument(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(workflow.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := newDocument(t)
	apply(t, document, workflow.Move{}, encode(t, workflow.MoveArgs{Boxes: map[ids.ULID]flow.Box{id(1): {X: 7, Y: 9, W: 0, H: 0}}}))
	data, _ := document.Encode()
	layout, err := model.LayoutOf(document)
	if err != nil {
		t.Fatal(err)
	}
	documentType, ok := registry.Document(flow.FormatName)
	if !ok || documentType.Layout == nil || documentType.Layout.Name != flow.LayoutFormatName {
		t.Fatal("the workflow type declares its layout format")
	}
	restored, err := documentType.DecodeWithLayout(data, layout)
	if err != nil {
		t.Fatal(err)
	}
	box, placed := restored.(*workflow.Document).Workflow.Box(id(1))
	if !placed || box.X != 7 {
		t.Fatalf("box %v %v", box, placed)
	}
	if _, ok := registry.Formats().Lookup(flow.LayoutFormatName); !ok {
		t.Fatal("the layout format is registered so old layouts can be upgraded")
	}
}
