package valuetree_test

import (
	"errors"
	"testing"

	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func id(n byte) ids.ULID {
	var made ids.ULID
	made[0], made[15] = 1, n

	return made
}

// apply makes a change as the orchestrator would: permanent IDs for
// temporary ones, in order, then the change.
func apply(t *testing.T, list *worldstate.Declarations, change stateplugin.ChangeArgs, next *byte) {
	t.Helper()
	permanent := map[ids.ULID]ids.ULID{}
	for index, added := range change.Add {
		*next++
		permanent[added.ID] = id(*next)
		change.Add[index].ID = id(*next)
	}
	for _, each := range [][]worldstate.Declaration{change.Add, change.Set} {
		for index := range each {
			if made, ok := permanent[each[index].Parent]; ok {
				each[index].Parent = made
			}
		}
	}
	if err := list.InsertAll(change.Add); err != nil {
		t.Fatal(err)
	}
	for _, set := range change.Set {
		if _, err := list.Replace(set); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeclareAndMove(t *testing.T) {
	t.Parallel()
	list := worldstate.NewDeclarations(worldstate.HoldsWorldState)
	next := byte(0)
	change, err := valuetree.Declare(list, worldstate.ScopeWorld, ids.ULID{}, "quests.dragon_slain", worldstate.TypeBool)
	if err != nil || len(change.Add) != 2 || change.Add[1].Parent != change.Add[0].ID {
		t.Fatalf("a dotted name declares its group first: %+v %v", change, err)
	}
	apply(t, list, change, &next)
	slain := id(next)
	if list.Path(slain) != "world.quests.dragon_slain" || valuetree.Dotted(list, slain) != "quests.dragon_slain" {
		t.Fatalf("declared at %s", list.Path(slain))
	}
	change, err = valuetree.Declare(list, worldstate.ScopeWorld, ids.ULID{}, "quests.ring_found", worldstate.TypeBool)
	if err != nil || len(change.Add) != 1 {
		t.Fatalf("an existing group is used: %+v %v", change, err)
	}
	apply(t, list, change, &next)
	if _, err := valuetree.Declare(list, worldstate.ScopeWorld, ids.ULID{}, "quests.ring_found",
		worldstate.TypeInteger); !errors.Is(err, valuetree.ErrName) {
		t.Fatalf("a taken name is refused: %v", err)
	}
	if _, err := valuetree.Declare(list, worldstate.ScopeWorld, ids.ULID{}, "Quests.x", worldstate.TypeBool); !errors.Is(
		err, valuetree.ErrName) {
		t.Fatalf("a bad name is refused: %v", err)
	}
	declared, _ := list.Value(slain)
	change, err = valuetree.Move(list, declared, ids.ULID{}, "slain")
	if err != nil || len(change.Add) != 0 {
		t.Fatalf("move to the top: %+v %v", change, err)
	}
	apply(t, list, change, &next)
	if list.Path(slain) != "world.slain" {
		t.Fatalf("moved to %s", list.Path(slain))
	}
	quests, _ := list.Find(worldstate.ScopeWorld, ids.ULID{}, "quests")
	if _, err := valuetree.Move(list, quests, ids.ULID{}, "quests.inner"); !errors.Is(err, valuetree.ErrName) {
		t.Fatalf("a group cannot go inside itself: %v", err)
	}
	declared, _ = list.Value(slain)
	change, err = valuetree.Move(list, declared, ids.ULID{}, "story.dragon.slain")
	if err != nil || len(change.Add) != 2 {
		t.Fatalf("a move declares new groups: %+v %v", change, err)
	}
	apply(t, list, change, &next)
	if list.Path(slain) != "world.story.dragon.slain" {
		t.Fatalf("moved to %s", list.Path(slain))
	}
	story, _ := list.Find(worldstate.ScopeWorld, ids.ULID{}, "story")
	if removing, values := valuetree.Removal(list, story.ID); len(removing) != 3 || values != 1 {
		t.Fatalf("deleting story takes its group and value: %v %d", removing, values)
	}
}

func TestTemplatesNameFromTheTemplate(t *testing.T) {
	t.Parallel()
	list := worldstate.NewDeclarations(worldstate.HoldsWorldState)
	next := byte(0)
	change, err := valuetree.Declare(list, worldstate.ScopeNPC, ids.ULID{}, "guards", worldstate.TypeTemplate)
	if err != nil {
		t.Fatal(err)
	}
	apply(t, list, change, &next)
	guards := id(next)
	if change, err = valuetree.Declare(list, worldstate.ScopeNPC, guards, "combat.arrests", worldstate.TypeInteger); err != nil {
		t.Fatal(err)
	}
	apply(t, list, change, &next)
	if valuetree.Root(list, id(next)) != guards || valuetree.Dotted(list, id(next)) != "combat.arrests" ||
		list.Path(id(next)) != "npc.guards.combat.arrests" {
		t.Fatalf("a template's value is named from the template: %s", list.Path(id(next)))
	}
	if valuetree.Dotted(list, guards) != "guards" || !valuetree.Root(list, guards).IsZero() {
		t.Fatal("a template is named from the top")
	}
}
