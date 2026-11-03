package worldstate_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// namer names a test's documents, places and NPCs.
type namer map[ids.ULID]string

func (n namer) Document(id ids.ULID) string { return n[id] }

func (n namer) Place(place worldstate.Place) string { return n[place.Anchor] }

func (n namer) NPC(id ids.ULID) string { return n[id] }

// TestEveryNPCsBuiltIns finds an NPC's zone, behaviour and sprite as values
// starting as its details have them, beside its name, by the same IDs for
// every NPC; the zone and behaviour are read only and the sprite can be set.
func TestEveryNPCsBuiltIns(t *testing.T) {
	t.Parallel()
	ada := worldstate.Owner{
		Scope: worldstate.ScopeNPC, Path: "ada", Name: "Ada", Owned: worldstate.NewOwned(worldstate.ScopeNPC),
		Zone: id(30), Behaviour: id(31), Sprite: id(32),
	}
	catalog := worldstate.NewCatalog(worldstate.NewDeclarations(worldstate.HoldsWorldState),
		map[ids.ULID]worldstate.Owner{id(20): ada})
	store := worldstate.NewStore(catalog)
	for builtIn, want := range map[ids.ULID]ids.ULID{
		worldstate.ZoneID(): id(30), worldstate.BehaviourID(): id(31), worldstate.SpriteID(): id(32),
	} {
		declared, ok := catalog.Lookup(worldstate.ScopeNPC, id(20), builtIn)
		if !ok || !worldstate.IsBuiltIn(builtIn) || store.Get(declared, id(20)).Ref != want {
			t.Fatalf("Ada's %s starts as her details have it", declared.Name)
		}
		if _, path, found := catalog.Declared(builtIn); !found || path != "npc."+declared.Name {
			t.Fatalf("every NPC's %s is npc.%s: %q", declared.Name, declared.Name, path)
		}
	}
	if !worldstate.ReadOnly(worldstate.ZoneID()) || !worldstate.ReadOnly(worldstate.BehaviourID()) ||
		worldstate.ReadOnly(worldstate.SpriteID()) || worldstate.ReadOnly(worldstate.NameID()) {
		t.Fatal("the zone and behaviour are read only; the name and sprite are not")
	}
	sprite, _ := catalog.Lookup(worldstate.ScopeNPC, id(20), worldstate.SpriteID())
	changed := sprite.Empty()
	changed.Ref = id(33)
	if err := store.Set(sprite, id(20), changed); err != nil || store.Get(sprite, id(20)).Ref != id(33) {
		t.Fatalf("a sprite can be set: %v", err)
	}
	held := catalog.Held(worldstate.ScopeNPC, id(20))
	if len(held) != 4 || held[0].Path != "npc.name" || held[1].Path != "npc.zone" || held[2].Path != "npc.behaviour" ||
		held[3].Path != "npc.sprite" {
		t.Fatalf("an NPC holds its name, zone, behaviour and sprite: %+v", held)
	}
	encoded := changed.Encode()
	if read, err := sprite.Parse(encoded); err != nil || read != changed {
		t.Fatalf("a sprite value saves and reads back: %s %v", encoded, err)
	}
}

// TestTextForms reads every type as text, names and all.
func TestTextForms(t *testing.T) {
	t.Parallel()
	names := namer{id(30): "Market", id(31): "Shopkeeping", id(40): "Counter, Ground floor, Market", id(50): "Ada"}
	value := func(kind worldstate.Type, set func(*worldstate.Value)) (worldstate.Declaration, worldstate.Value) {
		declared := worldstate.New(id(60), "value", worldstate.ScopeLocal, kind)
		made := declared.Empty()
		set(&made)

		return declared, made
	}
	for want, each := range map[string]func() (worldstate.Declaration, worldstate.Value){
		"12": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeInteger, func(v *worldstate.Value) { v.Integer = 12 })
		},
		"true": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeBool, func(v *worldstate.Value) { v.Bool = true })
		},
		"Hello": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeText, func(v *worldstate.Value) { v.Text = "Hello" })
		},
		"Ada": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeNPC, func(v *worldstate.Value) { v.NPC = id(50) })
		},
		"Counter, Ground floor, Market": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypePlace, func(v *worldstate.Value) { v.Place = worldstate.Place{Floor: id(41), Anchor: id(40)} })
		},
		"Market": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeZone, func(v *worldstate.Value) { v.Ref = id(30) })
		},
		"no sprite": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeSprite, func(*worldstate.Value) {})
		},
		"none": func() (worldstate.Declaration, worldstate.Value) {
			return value(worldstate.TypeNPC, func(*worldstate.Value) {})
		},
	} {
		declared, made := each()
		if got := worldstate.Text(declared, made, names); got != want {
			t.Fatalf("%s reads %q, not %q", declared.Type, got, want)
		}
	}
	behaviour := worldstate.BuiltIns(worldstate.Owner{
		Scope: worldstate.ScopeNPC, Path: "", Name: "", Owned: worldstate.NewOwned(worldstate.ScopeNPC), Zone: ids.ULID{},
		Behaviour: ids.ULID{}, Sprite: ids.ULID{},
	})[2]
	if got := worldstate.Text(behaviour, behaviour.Zero(), names); got != "no behaviour" {
		t.Fatalf("no behaviour reads %q", got)
	}
}
