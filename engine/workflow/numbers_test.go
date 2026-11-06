package workflow_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestNumbersTakeConstants saves a Move's tiles, a Wait's amount and a
// random choice's share with constants standing in, keeping the numbers
// typed; lists the constants; resolves each, held to its limits; and
// refuses a workflow's own value standing in.
func TestNumbersTakeConstants(t *testing.T) {
	t.Parallel()
	steps := workflow.ValueRef{Scope: worldstate.ScopeNPC, Value: id(40), Owner: ids.ULID{}, Found: ids.ULID{}}
	restock := workflow.ValueRef{Scope: worldstate.ScopeWorld, Value: id(41), Owner: ids.ULID{}, Found: ids.ULID{}}
	move, err := node(t, 2, workflow.KindMove, ids.ULID{}).WithParam("tiles", workflow.Amount{Number: 3, Constant: steps})
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckParams(move); err != nil {
		t.Fatal(err)
	}
	if tiles, err := move.Number("tiles"); err != nil || tiles != 3 {
		t.Fatalf("the number typed is kept: %d %v", tiles, err)
	}
	wait, err := node(t, 3, workflow.KindWait, ids.ULID{}).WithParam("duration", workflow.Duration{
		Amount: 30, Unit: workflow.UnitMinutes, Clock: workflow.ClockGame, Constant: restock,
	})
	if err != nil || workflow.CheckParams(wait) != nil {
		t.Fatalf("a wait for a constant: %v", err)
	}
	for _, check := range []struct {
		node workflow.Node
		want workflow.ValueRef
	}{{move, steps}, {wait, restock}} {
		standing := check.node.NumberConstants()
		if len(standing) != 1 || standing[0].Constant != check.want {
			t.Fatalf("%s lists its constant: %+v", check.node.Kind, standing)
		}
	}
	amount, _ := move.Amount("tiles")
	low, high := workflow.Limits(workflow.TypeTiles)
	reads := func(number int) func(workflow.ValueRef) (int, bool) {
		return func(workflow.ValueRef) (int, bool) { return number, true }
	}
	if amount.Resolve(reads(6), low, high) != 6 || amount.Resolve(reads(80), low, high) != workflow.MaxTiles ||
		amount.Resolve(func(workflow.ValueRef) (int, bool) { return 0, false }, low, high) != 3 {
		t.Fatal("a constant's number is held to the limits, or the number typed stands")
	}
	local := workflow.ValueRef{Scope: worldstate.ScopeLocal, Value: id(42), Owner: ids.ULID{}, Found: ids.ULID{}}
	bad, err := node(t, 4, workflow.KindMove, ids.ULID{}).WithParam("tiles", workflow.Amount{Number: 1, Constant: local})
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckParams(bad); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a workflow's own value is no constant: %v", err)
	}
}

// TestVersion9ReadsUnchanged upgrades a version 9 workflow, whose numbers
// are all typed, unchanged.
func TestVersion9ReadsUnchanged(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	old := `{"start":"` + id(1).String() + `","nodes":[` +
		`{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` + id(2).String() + `"}},` +
		`{"id":"` + id(2).String() + `","kind":"move","params":{"direction":"up","tiles":4}}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 9, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(old),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion || workflow.FormatVersion != 10 {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	decoded, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	move, _ := decoded.Node(id(2))
	if tiles, err := move.Amount("tiles"); err != nil || tiles.Number != 4 || !tiles.Constant.IsZero() {
		t.Fatalf("its tiles read as typed: %+v %v", tiles, err)
	}
}
