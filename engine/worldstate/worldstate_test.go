package worldstate_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func id(n byte) ids.ULID {
	var made ids.ULID
	made[0], made[15] = 1, n

	return made
}

// in returns a declaration inside a group or template.
func in(declared worldstate.Declaration, parent ids.ULID) worldstate.Declaration {
	declared.Parent = parent

	return declared
}

func insert(t *testing.T, list *worldstate.Declarations, declared ...worldstate.Declaration) {
	t.Helper()
	if err := list.InsertAll(declared); err != nil {
		t.Fatal(err)
	}
}

func TestNames(t *testing.T) {
	t.Parallel()
	for _, good := range []string{"mood", "gold_owed", "a1", "x"} {
		if err := worldstate.CheckName(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
	for _, bad := range []string{"", "george's", "Mood", "1st", "_x", "has space", "a.b", "abcdefghijklmnopqrstuvwxyzabcdefghijklmno"} {
		if err := worldstate.CheckName(bad); !errors.Is(err, worldstate.ErrValue) {
			t.Errorf("%q should be refused: %v", bad, err)
		}
	}
	if names, err := worldstate.SplitName("oof.doof"); err != nil || len(names) != 2 || names[1] != "doof" {
		t.Fatalf("a dotted name is a group and a name: %v %v", names, err)
	}
	for _, bad := range []string{"oof..doof", "oof.", "a.b.c.d.e.f.g.h"} {
		if _, err := worldstate.SplitName(bad); !errors.Is(err, worldstate.ErrValue) {
			t.Errorf("%q should be refused: %v", bad, err)
		}
	}
}

func TestValuesOfEachType(t *testing.T) {
	t.Parallel()
	mood := worldstate.New(id(1), "mood", worldstate.ScopeNPC, worldstate.TypeChoice)
	mood.Choices = []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}}
	if err := mood.Check(); err != nil {
		t.Fatal(err)
	}
	if got := mood.Label(mood.Zero()); got != "calm" {
		t.Fatalf("a list starts at its default: %q", got)
	}
	cross, err := mood.Parse(json.RawMessage(`"c2"`))
	if err != nil || mood.Label(cross) != "cross" {
		t.Fatalf("a choice reads by its ID: %v %v", cross, err)
	}
	if _, err := mood.Parse(json.RawMessage(`"cross"`)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a choice's name is not its ID: %v", err)
	}
	for _, check := range []struct {
		kind  worldstate.Type
		saved string
		label string
	}{
		{worldstate.TypeInteger, "-12", "-12"},
		{worldstate.TypeBool, "true", "true"},
		{worldstate.TypeText, `"Old Tom"`, `"Old Tom"`},
		{worldstate.TypeNPC, `""`, "none"},
	} {
		declared := worldstate.New(id(2), "value", worldstate.ScopeLocal, check.kind)
		value, err := declared.Parse(json.RawMessage(check.saved))
		if err != nil || declared.Label(value) != check.label || !bytes.Equal(value.Encode(), []byte(check.saved)) {
			t.Errorf("%s: %+v %v", check.kind, value, err)
		}
	}
	count := worldstate.New(id(3), "count", worldstate.ScopeWorld, worldstate.TypeInteger)
	if _, err := count.Parse(json.RawMessage(`2000000000000000`)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("integers are bounded: %v", err)
	}
	if _, err := count.Parse(json.RawMessage(`true`)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("an integer is not a bool: %v", err)
	}
}

func TestIntegerLimits(t *testing.T) {
	t.Parallel()
	health := worldstate.New(id(1), "health", worldstate.ScopeNPC, worldstate.TypeInteger)
	health.Min, health.Max = worldstate.Limit(1), worldstate.Limit(100)
	if err := health.Check(); !errors.Is(err, worldstate.ErrValue) || health.Zero().Integer != 1 {
		t.Fatalf("a default below the minimum is refused, and reads as the minimum: %v %+v", err, health.Zero())
	}
	health.Default = json.RawMessage("50")
	if err := health.Check(); err != nil {
		t.Fatal(err)
	}
	if health.Hold(150) != 100 || health.Hold(-3) != 1 || health.Hold(40) != 40 {
		t.Fatal("a change is held at the limits")
	}
	full := worldstate.Value{Type: worldstate.TypeInteger, Integer: 100, Bool: false, Choice: "", Text: "", NPC: ids.ULID{}}
	if !health.AtMost(full) || health.AtLeast(full) || health.Fits(worldstate.Value{
		Type: worldstate.TypeInteger, Integer: 101, Bool: false, Choice: "", Text: "", NPC: ids.ULID{},
	}) {
		t.Fatal("at its maximum, and 101 does not fit")
	}
	health.Default = json.RawMessage("150")
	if err := health.Check(); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a default must be within the limits: %v", err)
	}
	health.Default, health.Min = json.RawMessage("50"), worldstate.Limit(200)
	if err := health.Check(); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("the minimum cannot pass the maximum: %v", err)
	}
	flag := worldstate.New(id(2), "flag", worldstate.ScopeWorld, worldstate.TypeBool)
	flag.Max = worldstate.Limit(1)
	if err := flag.Check(); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("only an integer has limits: %v", err)
	}
	unlimited := worldstate.New(id(3), "gold", worldstate.ScopeWorld, worldstate.TypeInteger)
	if unlimited.AtMost(full) || unlimited.Hold(worldstate.MaxInteger+1) != worldstate.MaxInteger {
		t.Fatal("without limits, only the engine's bound holds")
	}
}

func TestGroupsAndTemplates(t *testing.T) {
	t.Parallel()
	state := worldstate.NewDocument().Values
	quests := worldstate.New(id(1), "quests", worldstate.ScopeWorld, worldstate.TypeGroup)
	slain := in(worldstate.New(id(2), "dragon_slain", worldstate.ScopeWorld, worldstate.TypeBool), quests.ID)
	guards := worldstate.New(id(3), "guards", worldstate.ScopeNPC, worldstate.TypeTemplate)
	arrests := in(worldstate.New(id(4), "how_many_arrested", worldstate.ScopeNPC, worldstate.TypeInteger), guards.ID)
	insert(t, state, quests, slain, guards, arrests)
	if state.Path(slain.ID) != "world.quests.dragon_slain" || state.Path(arrests.ID) != "npc.guards.how_many_arrested" {
		t.Fatalf("paths: %s %s", state.Path(slain.ID), state.Path(arrests.ID))
	}
	if template, ok := state.Template(arrests.ID); !ok || template != guards.ID {
		t.Fatal("a template's value knows its template")
	}
	if children := state.Children(worldstate.ScopeWorld, ids.ULID{}); len(children) != 1 || children[0].ID != quests.ID {
		t.Fatalf("the world's top holds the group: %+v", children)
	}
	if found, ok := state.Find(worldstate.ScopeWorld, quests.ID, "dragon_slain"); !ok || found.ID != slain.ID {
		t.Fatal("a group's value is found by name")
	}
	for name, bad := range map[string]worldstate.Declaration{
		"an NPC value outside a template": worldstate.New(id(10), "loose", worldstate.ScopeNPC, worldstate.TypeBool),
		"a value in a value":              in(worldstate.New(id(11), "inner", worldstate.ScopeWorld, worldstate.TypeBool), slain.ID),
		"a template in a group":           in(worldstate.New(id(12), "inner", worldstate.ScopeWorld, worldstate.TypeTemplate), quests.ID),
		"a world template":                worldstate.New(id(13), "world_set", worldstate.ScopeWorld, worldstate.TypeTemplate),
		"a name twice in a group":         in(worldstate.New(id(14), "dragon_slain", worldstate.ScopeWorld, worldstate.TypeInteger), quests.ID),
		"another scope's group":           in(worldstate.New(id(15), "stray", worldstate.ScopeZone, worldstate.TypeBool), quests.ID),
	} {
		if err := state.Insert(bad); !errors.Is(err, worldstate.ErrValue) {
			t.Errorf("%s should be refused: %v", name, err)
		}
	}
	if err := state.Insert(worldstate.New(id(16), "dragon_slain", worldstate.ScopeWorld, worldstate.TypeBool)); err != nil {
		t.Fatalf("the same name outside the group is another value: %v", err)
	}
	if _, err := state.Remove(quests.ID); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a group holding values is not removed alone: %v", err)
	}
	within := state.Within(quests.ID)
	removed, err := state.RemoveAll(append(within, quests.ID))
	if err != nil || len(removed) != 2 || state.Len() != 3 {
		t.Fatalf("a group goes with what it holds: %v %d", err, state.Len())
	}
	if err := state.InsertAll(removed); err != nil || state.Path(slain.ID) != "world.quests.dragon_slain" {
		t.Fatalf("and comes back with it: %v", err)
	}
	deep := ids.ULID{}
	for depth := range worldstate.MaxDepth + 2 {
		group := in(worldstate.New(id(byte(30+depth)), "deep", worldstate.ScopeWorld, worldstate.TypeGroup), deep)
		err := state.Insert(group)
		if depth <= worldstate.MaxDepth && err != nil || depth > worldstate.MaxDepth && !errors.Is(err, worldstate.ErrValue) {
			t.Fatalf("anything is at most %d groups deep: %d %v", worldstate.MaxDepth, depth, err)
		}
		deep = group.ID
	}
	data, err := worldstate.NewDocument().Encode()
	if err != nil {
		t.Fatal(err)
	}
	document := &worldstate.Document{Values: state}
	if data, err = document.Encode(); err != nil {
		t.Fatal(err)
	}
	decoded, err := worldstate.Decode(data)
	if err != nil || !decoded.Values.Equal(state) {
		t.Fatalf("saved and read unchanged: %v", err)
	}
}

func TestListsHoldTheirOwn(t *testing.T) {
	t.Parallel()
	npc := worldstate.NewOwned(worldstate.ScopeNPC)
	secret := worldstate.New(id(1), "secret", worldstate.ScopeNPC, worldstate.TypeBool)
	combat := worldstate.New(id(2), "combat", worldstate.ScopeNPC, worldstate.TypeGroup)
	insert(t, npc.Values, secret, combat, in(worldstate.New(id(3), "health", worldstate.ScopeNPC, worldstate.TypeInteger), combat.ID))
	for name, bad := range map[*worldstate.Declarations]worldstate.Declaration{
		npc.Values: worldstate.New(id(4), "set", worldstate.ScopeNPC, worldstate.TypeTemplate),
		worldstate.NewOwned(worldstate.ScopeZone).Values:   worldstate.New(id(5), "x", worldstate.ScopeNPC, worldstate.TypeBool),
		worldstate.NewDeclarations(worldstate.HoldsLocals): worldstate.New(id(6), "g", worldstate.ScopeLocal, worldstate.TypeGroup),
	} {
		if err := name.Insert(bad); !errors.Is(err, worldstate.ErrValue) {
			t.Errorf("%+v should be refused: %v", bad, err)
		}
	}
	if err := worldstate.NewDeclarations(worldstate.HoldsLocals).Insert(
		worldstate.New(id(7), "customer", worldstate.ScopeLocal, worldstate.TypeNPC)); err != nil {
		t.Fatalf("a local can be an NPC: %v", err)
	}
	if err := npc.Values.Insert(worldstate.New(id(8), "customer", worldstate.ScopeNPC, worldstate.TypeNPC)); !errors.Is(
		err, worldstate.ErrValue) {
		t.Fatalf("an NPC's value cannot: %v", err)
	}
	if err := npc.Apply(id(20)); err != nil || npc.Apply(id(20)) == nil {
		t.Fatalf("a template is applied once: %v", err)
	}
	wire, err := npc.Wire()
	if err != nil {
		t.Fatal(err)
	}
	read, err := worldstate.ReadOwned(worldstate.ScopeNPC, wire)
	if err != nil || !read.Equal(npc) {
		t.Fatalf("what an NPC owns is saved and read unchanged: %v", err)
	}
	if index, err := read.Unapply(id(20)); err != nil || index != 0 || len(read.Templates) != 0 {
		t.Fatalf("unapply: %d %v", index, err)
	}
	if empty, err := worldstate.NewOwned(worldstate.ScopeZone).Wire(); err != nil || empty.Values != nil || empty.Templates != nil {
		t.Fatalf("nothing owned saves nothing: %+v %v", empty, err)
	}
}

// shop is a world state with a guards template and a world group, a guard
// with the template and George with a secret of his own.
type shop struct {
	catalog                         *worldstate.Catalog
	guard, george, market           ids.ULID
	arrests, secret, slain, crowded worldstate.Declaration
}

func newShop(t *testing.T) shop {
	t.Helper()
	state := worldstate.NewDocument().Values
	quests := worldstate.New(id(1), "quests", worldstate.ScopeWorld, worldstate.TypeGroup)
	made := shop{
		catalog: nil, guard: id(100), george: id(101), market: id(102),
		arrests: in(worldstate.New(id(4), "how_many_arrested", worldstate.ScopeNPC, worldstate.TypeInteger), id(3)),
		secret:  worldstate.New(id(5), "secret", worldstate.ScopeNPC, worldstate.TypeBool),
		slain:   in(worldstate.New(id(2), "dragon_slain", worldstate.ScopeWorld, worldstate.TypeBool), quests.ID),
		crowded: worldstate.New(id(6), "crowded", worldstate.ScopeZone, worldstate.TypeBool),
	}
	insert(t, state, quests, made.slain, worldstate.New(id(3), "guards", worldstate.ScopeNPC, worldstate.TypeTemplate),
		made.arrests)
	guard, george, market := worldstate.NewOwned(worldstate.ScopeNPC), worldstate.NewOwned(worldstate.ScopeNPC),
		worldstate.NewOwned(worldstate.ScopeZone)
	if err := guard.Apply(id(3)); err != nil {
		t.Fatal(err)
	}
	insert(t, george.Values, made.secret)
	insert(t, market.Values, made.crowded)
	made.catalog = worldstate.NewCatalog(state, map[ids.ULID]worldstate.Owner{
		made.guard:  {Scope: worldstate.ScopeNPC, Path: "guard1", Name: "Guard 1", Owned: guard},
		made.george: {Scope: worldstate.ScopeNPC, Path: "george", Name: "George", Owned: george},
		made.market: {Scope: worldstate.ScopeZone, Path: "market", Name: "Market", Owned: market},
	})

	return made
}

func TestCatalog(t *testing.T) {
	t.Parallel()
	shop := newShop(t)
	catalog := shop.catalog
	for _, check := range []struct {
		name         string
		scope        worldstate.Scope
		owner, value ids.ULID
		found        bool
	}{
		{"a world value", worldstate.ScopeWorld, ids.ULID{}, shop.slain.ID, true},
		{"a world group is no value", worldstate.ScopeWorld, ids.ULID{}, id(1), false},
		{"the guard's template value", worldstate.ScopeNPC, shop.guard, shop.arrests.ID, true},
		{"George has no guards template", worldstate.ScopeNPC, shop.george, shop.arrests.ID, false},
		{"George's own secret", worldstate.ScopeNPC, shop.george, shop.secret.ID, true},
		{"the guard has no secret", worldstate.ScopeNPC, shop.guard, shop.secret.ID, false},
		{"every NPC has a name", worldstate.ScopeNPC, shop.guard, worldstate.NameID(), true},
		{"a zone has no name", worldstate.ScopeZone, shop.market, worldstate.NameID(), false},
		{"the market's own value", worldstate.ScopeZone, shop.market, shop.crowded.ID, true},
		{"George is no zone", worldstate.ScopeZone, shop.george, shop.crowded.ID, false},
	} {
		if _, found := catalog.Lookup(check.scope, check.owner, check.value); found != check.found {
			t.Errorf("%s: found %v", check.name, found)
		}
	}
	if name, ok := catalog.Lookup(worldstate.ScopeNPC, shop.george, worldstate.NameID()); !ok ||
		name.Label(name.Zero()) != `"George"` {
		t.Fatal("an NPC's name starts as its display name")
	}
	if george, ok := catalog.Named(worldstate.ScopeNPC, "george"); !ok || george != shop.george {
		t.Fatal("an NPC is found by its path name")
	}
	if _, path, ok := catalog.Declared(shop.arrests.ID); !ok || path != "npc.guards.how_many_arrested" ||
		worldstate.NamedPath(path, "guard1") != "npc.guard1.guards.how_many_arrested" {
		t.Fatalf("a template's value is named %s", path)
	}
	if _, path, ok := catalog.Declared(shop.secret.ID); !ok || path != "npc.secret" {
		t.Fatalf("an NPC's own value is named %s", path)
	}
	var paths []string
	for _, held := range catalog.Held(worldstate.ScopeNPC, shop.guard) {
		paths = append(paths, held.Path)
	}
	if len(paths) != 2 || paths[0] != "npc.name" || paths[1] != "npc.guards.how_many_arrested" {
		t.Fatalf("the guard holds %v", paths)
	}
}

func TestStore(t *testing.T) {
	t.Parallel()
	shop := newShop(t)
	store := worldstate.NewStore(shop.catalog)
	if got := store.Get(shop.arrests, shop.guard); got.Integer != 0 {
		t.Fatalf("a value never set is its default: %+v", got)
	}
	three := worldstate.Value{Type: worldstate.TypeInteger, Integer: 3, Bool: false, Choice: "", Text: "", NPC: ids.ULID{}}
	if err := store.Set(shop.arrests, shop.guard, three); err != nil {
		t.Fatal(err)
	}
	if store.Get(shop.arrests, shop.guard) != three || store.Get(shop.arrests, id(200)).Integer != 0 {
		t.Fatal("each NPC with the template has its own")
	}
	if err := store.Set(shop.slain, ids.ULID{}, three); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a value of the wrong type is refused: %v", err)
	}
	shown := store.Show(worldstate.ScopeNPC, shop.guard)
	if len(shown) != 2 || shown[1].Path != "npc.guards.how_many_arrested" || shown[1].Value != "3" {
		t.Fatalf("shown %+v", shown)
	}
	if world := store.Show(worldstate.ScopeWorld, ids.ULID{}); len(world) != 1 || world[0].Value != "false" {
		t.Fatalf("the world shows %+v", world)
	}
	capped := shop.arrests.Clone()
	capped.Max = worldstate.Limit(2)
	if got := store.Get(capped, shop.guard); got.Integer != 0 {
		t.Fatalf("a value its declaration no longer fits is its default: %+v", got)
	}
}

func TestUpgradeFrom1(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(worldstate.Format()); err != nil {
		t.Fatal(err)
	}
	old := `{"values":[` +
		`{"id":"01000000000000000000000001","name":"count","scope":"world","type":"number","default":3},` +
		`{"id":"01000000000000000000000002","name":"waiting","scope":"npc","type":"yes-no","default":false},` +
		`{"id":"01000000000000000000000003","name":"busy","scope":"zone","type":"yes-no","default":true}]}`
	envelope := content.Envelope{Format: worldstate.FormatName, Version: 1, ID: worldstate.ID(), Data: json.RawMessage(old)}
	upgraded, changed, err := registry.Upgrade(envelope)
	if err != nil || !changed {
		t.Fatalf("upgrade: %v", err)
	}
	decoded, err := worldstate.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, declared := range decoded.Values.Values() {
		if declared.Type.IsValue() {
			paths = append(paths, decoded.Values.Path(declared.ID)+":"+string(declared.Type))
		}
	}
	want := []string{"world.count:integer", "npc.shared.waiting:bool", "zone.shared.busy:bool"}
	if len(paths) != len(want) {
		t.Fatalf("upgraded to %v", paths)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("upgraded to %v", paths)
		}
	}
}
