package workflow_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// returning gives a Return a way out.
func returning(t *testing.T, seed byte, parent ids.ULID, label, port string) workflow.Node {
	t.Helper()
	made := node(t, seed, workflow.KindReturn, parent)
	made, err := made.WithParam(workflow.ParamWayName, label)
	if err == nil {
		made, err = made.WithParam(workflow.ParamWayPort, port)
	}
	if err != nil {
		t.Fatal(err)
	}

	return made
}

func TestWaysOut(t *testing.T) {
	t.Parallel()
	module, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if port := module.NewPort(); port != "w1" {
		t.Fatalf("the first port is %q, want w1", port)
	}
	nodes := []workflow.Node{
		node(t, 2, workflow.KindContainer, ids.ULID{}),
		returning(t, 3, ids.ULID{}, "Arrived", "w1"),
		returning(t, 4, ids.ULID{}, "Closed", "w2"),
		returning(t, 5, ids.ULID{}, "Arrived", "w1"),
		returning(t, 6, id(2), "Inside", "w3"),
	}
	if err := module.Insert(nodes, nil, nil); err != nil {
		t.Fatal(err)
	}
	want := []workflow.WayOut{{Port: "w1", Label: "Arrived"}, {Port: "w2", Label: "Closed"}}
	if got := module.WaysOut(); !slices.Equal(got, want) {
		t.Fatalf("ways out %v, want %v: two Returns of one name are one, and one inside a container is none", got, want)
	}
	if port := module.NewPort(); port != "w4" {
		t.Fatalf("a new port is %q, want w4", port)
	}
	renamed, err := nodes[2].WithParam(workflow.ParamWayName, "Shut")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.Replace(renamed); err != nil {
		t.Fatal(err)
	}
	if got := module.WaysOut(); got[1] != (workflow.WayOut{Port: "w2", Label: "Shut"}) {
		t.Fatalf("a renamed way out is %v, want w2 named Shut", got[1])
	}
}

func TestACallsOutputsAreItsWaysOut(t *testing.T) {
	t.Parallel()
	call := node(t, 2, workflow.KindCall, ids.ULID{})
	if _, ok := call.Called(); ok {
		t.Fatal("a new Call names no workflow")
	}
	call, err := call.WithParam(workflow.ParamWays, []workflow.WayOut{{Port: "w1", Label: "Arrived"}})
	if err == nil {
		call, err = call.WithParam(workflow.ParamWorkflow, id(9).String())
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckParams(call); err != nil {
		t.Fatal(err)
	}
	if called, ok := call.Called(); !ok || called != id(9) {
		t.Fatalf("the Call names %v, want %v", called, id(9))
	}
	outputs := workflow.Outputs(call)
	want := []workflow.Output{{Name: "w1", Label: "Arrived"}, {Name: workflow.OutputDone, Label: "done"}}
	if !slices.Equal(outputs, want) {
		t.Fatalf("outputs %v, want %v", outputs, want)
	}
	for _, port := range []string{"", workflow.OutputDone} {
		bad, err := call.WithParam(workflow.ParamWays, []workflow.WayOut{{Port: port, Label: "x"}})
		if err != nil {
			t.Fatal(err)
		}
		if workflow.CheckParams(bad) == nil {
			t.Fatalf("a way out with port %q is refused", port)
		}
	}
}

func TestInputsAndOutputs(t *testing.T) {
	t.Parallel()
	module, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	place := worldstate.New(id(20), "place", worldstate.ScopeLocal, worldstate.TypePlace)
	place.Role = worldstate.RoleInput
	tries := worldstate.New(id(21), "tries", worldstate.ScopeLocal, worldstate.TypeInteger)
	tries.Role = worldstate.RoleOutput
	plain := worldstate.New(id(22), "plain", worldstate.ScopeLocal, worldstate.TypeBool)
	if err := module.Locals().InsertAll([]worldstate.Declaration{place, tries, plain}); err != nil {
		t.Fatal(err)
	}
	if inputs := module.Inputs(); len(inputs) != 1 || inputs[0].ID != id(20) {
		t.Fatalf("inputs %v, want place", inputs)
	}
	if outputs := module.Outputs(); len(outputs) != 1 || outputs[0].ID != id(21) {
		t.Fatalf("outputs %v, want tries", outputs)
	}
	logic, err := module.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := workflow.Decode(logic)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(module) {
		t.Fatal("roles and a place local survive saving")
	}
	world := worldstate.New(id(23), "flag", worldstate.ScopeWorld, worldstate.TypeBool)
	world.Role = worldstate.RoleInput
	if world.Check() == nil {
		t.Fatal("only a local value is an input")
	}
}
