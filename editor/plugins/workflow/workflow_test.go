package workflow_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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
	longer, err := inner.WithParam("duration", flow.Duration{Amount: 90, Unit: flow.UnitMinutes, Clock: flow.ClockGame})
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
	key, depth := flow.LineKey(id(1), flow.OutputNext), -60
	bend := encode(t, workflow.MoveArgs{
		Boxes: map[ids.ULID]flow.Box{}, Lines: map[string]flow.Bend{key: {To: id(3), Lane: nil, Depth: &depth, End: &depth}},
	})
	if err := model.VerifyUndo(document, workflow.Move{}, bend, registry.Edit); err != nil {
		t.Fatalf("moving a line: %v", err)
	}
	tint := encode(t, workflow.TintArgs{Tints: flow.Tints{
		Nodes: map[ids.ULID]string{id(3): "green"}, Outcomes: map[string]string{key: "amber"},
		Lines: map[string]string{key: flow.TintNone},
	}})
	if err := model.VerifyUndo(document, workflow.Tint{}, tint, registry.Edit); err != nil {
		t.Fatalf("colouring: %v", err)
	}
	apply(t, document, workflow.Tint{}, tint)
	if err := model.VerifyUndo(document, workflow.Remove{}, encode(t, workflow.RemoveArgs{Nodes: []ids.ULID{id(3)}}),
		registry.Edit); err != nil {
		t.Fatalf("removing a coloured node: %v", err)
	}
	apply(t, document, workflow.Move{}, bend)
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{workflow.Move{}, encode(t, workflow.MoveArgs{
			Boxes: map[ids.ULID]flow.Box{}, Lines: map[string]flow.Bend{key: {To: ids.ULID{}, Lane: nil, Depth: nil, End: nil}},
		})},
		{workflow.Remove{}, encode(t, workflow.RemoveArgs{Nodes: []ids.ULID{id(3)}})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s with a moved line: %v", check.kind.Name(), err)
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
	badMinutes, _ := wait.WithParam("duration", flow.Duration{Amount: -5, Unit: flow.UnitMinutes, Clock: flow.ClockGame})
	wrongOutput := wait.WithLink(flow.OutputOtherwise, id(2))
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

func TestLocalValueEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(workflow.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := newDocument(t)
	customer := worldstate.New(id(9), "customer", worldstate.ScopeLocal, worldstate.TypeNPC)
	renamed := customer.Clone()
	renamed.Name = "shopper"
	edit := func(name string) model.EditKind {
		kind, ok := registry.Edit(name)
		if !ok {
			t.Fatalf("no edit %s", name)
		}

		return kind
	}
	change := edit(workflow.ChangeLocalsEdit)
	for _, args := range []stateplugin.ChangeArgs{
		{Add: []worldstate.Declaration{customer}, Set: nil, Remove: nil},
		{Add: nil, Set: []worldstate.Declaration{renamed}, Remove: nil},
		{Add: nil, Set: nil, Remove: []ids.ULID{id(9)}},
	} {
		if err := model.VerifyUndo(document, change, encode(t, args), registry.Edit); err != nil {
			t.Fatalf("%+v: %v", args, err)
		}
		apply(t, document, change, encode(t, args))
	}
	world := worldstate.New(id(10), "robbed", worldstate.ScopeWorld, worldstate.TypeBool)
	if _, err := change.Apply(document, encode(t, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{world}, Set: nil, Remove: nil,
	})); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a workflow declares only local values: %v", err)
	}
}

func TestReferencesListValuesAndNPCs(t *testing.T) {
	t.Parallel()
	document := newDocument(t)
	check := newNode(t, flow.KindCheckValue, 3, ids.ULID{})
	ref := flow.ValueRef{Scope: worldstate.ScopeNPC, Value: id(20), Owner: id(21), Found: ids.ULID{}}
	check, err := check.WithParam(flow.ParamValue, ref)
	if err != nil {
		t.Fatal(err)
	}
	set := newNode(t, flow.KindSetValue, 4, ids.ULID{})
	local := flow.ValueRef{Scope: worldstate.ScopeLocal, Value: id(22), Owner: ids.ULID{}, Found: ids.ULID{}}
	if set, err = set.WithParam(flow.ParamValue, local); err != nil {
		t.Fatal(err)
	}
	apply(t, document, workflow.Add{}, encode(t, workflow.AddArgs{Nodes: []flow.Node{check, set}, Boxes: nil, Links: nil}))
	references := document.References()
	if len(references) != 2 || references[0].Document != worldstate.ID() || references[0].Item != id(20).String() ||
		references[1].Document != id(21) {
		t.Fatalf("a value node refers to its world-state value and the NPC it names, not its locals: %+v", references)
	}
}

// TestReferencesListCalls lists the workflow a Call calls, as the call
// item, and the world-state values it gives and keeps, not its locals.
func TestReferencesListCalls(t *testing.T) {
	t.Parallel()
	document := newDocument(t)
	call := newNode(t, flow.KindCall, 3, ids.ULID{})
	world := flow.ValueRef{Scope: worldstate.ScopeWorld, Value: id(20), Owner: ids.ULID{}, Found: ids.ULID{}}
	local := flow.ValueRef{Scope: worldstate.ScopeLocal, Value: id(22), Owner: ids.ULID{}, Found: ids.ULID{}}
	call, err := call.WithParam(flow.ParamWorkflow, id(30).String())
	if err == nil {
		call, err = call.WithParam(flow.ParamInputs, map[ids.ULID]json.RawMessage{id(31): flow.RefOperand(world)})
	}
	if err == nil {
		call, err = call.WithParam(flow.ParamKeep, map[ids.ULID]flow.ValueRef{id(32): local})
	}
	if err != nil {
		t.Fatal(err)
	}
	apply(t, document, workflow.Add{}, encode(t, workflow.AddArgs{Nodes: []flow.Node{call}, Boxes: nil, Links: nil}))
	references := document.References()
	if len(references) != 2 || references[0].Document != id(30) || references[0].Item != workflow.CallItem ||
		references[1].Document != worldstate.ID() || references[1].Item != id(20).String() {
		t.Fatalf("a Call refers to what it calls and the world value it gives: %+v", references)
	}
}

// TestReferencesListLogValues lists the world-state values a Log writes,
// and the NPC it names, not its locals.
func TestReferencesListLogValues(t *testing.T) {
	t.Parallel()
	document := newDocument(t)
	named := flow.ValueRef{Scope: worldstate.ScopeNPC, Value: id(20), Owner: id(21), Found: ids.ULID{}}
	local := flow.ValueRef{Scope: worldstate.ScopeLocal, Value: id(22), Owner: ids.ULID{}, Found: ids.ULID{}}
	logged, err := newNode(t, flow.KindLog, 3, ids.ULID{}).WithParam(flow.ParamMessage, []flow.Segment{
		{Text: "", Ref: &named}, {Text: " has ", Ref: nil}, {Text: "", Ref: &local},
	})
	if err != nil {
		t.Fatal(err)
	}
	apply(t, document, workflow.Add{}, encode(t, workflow.AddArgs{Nodes: []flow.Node{logged}, Boxes: nil, Links: nil}))
	references := document.References()
	if len(references) != 2 || references[0].Document != worldstate.ID() || references[0].Item != id(20).String() ||
		references[1].Document != id(21) || references[1].Label != "a Log node" {
		t.Fatalf("a Log refers to the world-state value it writes and its NPC: %+v", references)
	}
}

// TestAZoneLoopAndWhereItRuns makes a zone loop as the orchestrator does,
// keeping its kind through preparing, refuses a Go to in it, and sets the
// zone it runs in, which it then refers to, undoably.
func TestAZoneLoopAndWhereItRuns(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(workflow.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	data, err := workflow.Prepare(workflow.NewDataFor(flow.PurposeZoneLoop), func() (ids.ULID, error) { return id(1), nil })
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := workflow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	document, _ := decoded.(*workflow.Document)
	if document.Workflow.Purpose() != flow.PurposeZoneLoop || !document.Workflow.RunsIn().IsZero() {
		t.Fatal("a new zone loop keeps its kind and runs in any zone")
	}
	goTo := newNode(t, flow.KindGoTo, 2, ids.ULID{})
	if _, err := (workflow.Add{}).Apply(document, encode(t, workflow.AddArgs{
		Nodes: []flow.Node{goTo}, Boxes: nil, Links: nil,
	})); !errors.Is(err, model.ErrArgs) {
		t.Fatal("a zone loop refuses a Go to")
	}
	set := encode(t, workflow.SetRunsInArgs{Zone: id(40)})
	if err := model.VerifyUndo(document, workflow.SetRunsIn{}, set, registry.Edit); err != nil {
		t.Fatal(err)
	}
	apply(t, document, workflow.SetRunsIn{}, set)
	references := document.References()
	if len(references) != 1 || references[0].Document != id(40) || references[0].Label != workflow.RunsHere {
		t.Fatalf("a workflow refers to the zone it runs in: %+v", references)
	}
}
