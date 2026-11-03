package npcrun_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// names names a test's documents and places.
type names map[ids.ULID]string

func (n names) Document(documentID ids.ULID) string { return n[documentID] }

func (n names) Place(place worldstate.Place) string { return n[place.Anchor] }

// builtInRef refers to the workflow's own NPC's built-in value.
func builtInRef(value ids.ULID) flow.ValueRef {
	return flow.ValueRef{Scope: worldstate.ScopeNPC, Value: value, Owner: ids.ULID{}, Found: ids.ULID{}}
}

// TestBuiltInValues sets an NPC's own sprite, which its workflow can, and
// not its zone, which is set in its details; and sets a text value to its
// zone, as the zone's name.
func TestBuiltInValues(t *testing.T) {
	t.Parallel()
	owners := map[ids.ULID]worldstate.Owner{
		id(101): {
			Scope: worldstate.ScopeNPC, Path: "ada", Name: "Ada", Owned: worldstate.NewOwned(worldstate.ScopeNPC),
			Zone: id(zoneID), Behaviour: id(90), Sprite: id(91),
		},
		id(zoneID): {
			Scope: worldstate.ScopeZone, Path: "office", Name: "Office", Owned: worldstate.NewOwned(worldstate.ScopeZone),
			Zone: ids.ULID{}, Behaviour: ids.ULID{}, Sprite: ids.ULID{},
		},
	}
	state := worldstate.NewStore(worldstate.NewCatalog(worldstate.NewDeclarations(worldstate.HoldsWorldState), owners))
	quotedID := func(value ids.ULID) json.RawMessage {
		data, err := json.Marshal(value.String())
		if err != nil {
			t.Fatal(err)
		}

		return data
	}
	nodes := []flow.Node{
		node(t, 2, flow.KindSetValue, map[string]any{flow.ParamValue: builtInRef(worldstate.SpriteID()), "to": quotedID(id(92))}),
		node(t, 3, flow.KindSetValue, map[string]any{flow.ParamValue: builtInRef(worldstate.ZoneID()), "to": quotedID(id(93))}),
		node(t, 4, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeLocal, 72),
			"to": flow.RefOperand(builtInRef(worldstate.ZoneID()))}),
		node(t, 5, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}
	workflow := workflowOf(t, nodes, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3),
		link(3, flow.OutputNext, 4), link(4, flow.OutputNext, 5))
	withLocal(t, workflow, 72, "where", worldstate.TypeText)
	town := newTown(t, state)
	town.names = names{id(zoneID): "Office"}
	ada := town.place(100, image.Pt(2, 2), workflow)
	var refused []string
	for range 3 {
		town.run(1)
		for _, change := range ada.Taken {
			if change.Status != "" {
				refused = append(refused, change.Status)
			}
		}
	}
	sprite, _ := state.Catalog().Lookup(worldstate.ScopeNPC, id(101), worldstate.SpriteID())
	zone, _ := state.Catalog().Lookup(worldstate.ScopeNPC, id(101), worldstate.ZoneID())
	if state.Get(sprite, id(101)).Ref != id(92) {
		t.Fatal("a workflow sets its own NPC's sprite")
	}
	if state.Get(zone, id(101)).Ref != id(zoneID) || len(refused) != 1 || refused[0] != npcrun.StatusNoValue {
		t.Fatalf("a workflow cannot set its NPC's zone, and says so: %v", refused)
	}
	if where := ada.Locals()[id(72)]; where.Text != "Office" {
		t.Fatalf("a text value set to the zone holds its name: %+v", where)
	}
}
