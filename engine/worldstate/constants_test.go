package worldstate_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestConstantsAreDeclared declares constants where values are, with text
// far longer than a value's, saves and reads them, and refuses a constant of
// another type, with limits, in a workflow, as a group, too long, or with a
// value's name beside it. A version 3 world state reads unchanged.
func TestConstantsAreDeclared(t *testing.T) {
	t.Parallel()
	state := worldstate.NewDocument()
	letter := worldstate.NewConstant(id(1), "letter", worldstate.ScopeWorld, worldstate.TypeText)
	letter.Default = worldstate.Value{Type: worldstate.TypeText, Text: strings.Repeat("a", 1000)}.Encode()
	wait := worldstate.NewConstant(id(2), "restock_minutes", worldstate.ScopeWorld, worldstate.TypeInteger)
	insert(t, state.Values, letter, wait)
	if !letter.IsConstant() || letter.IsValue() || !worldstate.New(id(3), "x", worldstate.ScopeWorld,
		worldstate.TypeText).IsValue() {
		t.Fatal("a constant is no value, and a value no constant")
	}
	if value := letter.Zero(); len(value.Text) != 1000 {
		t.Fatalf("a constant's text is longer than a value's: %d", len(value.Text))
	}
	data, err := state.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := worldstate.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if again, ok := read.Values.Value(id(1)); !ok || !again.IsConstant() || !again.Equal(letter) {
		t.Fatalf("read back: %+v", again)
	}
	tooLong := worldstate.NewConstant(id(9), "too_long", worldstate.ScopeWorld, worldstate.TypeText)
	tooLong.Default = worldstate.Value{Type: worldstate.TypeText, Text: strings.Repeat("a", 4001)}.Encode()
	limited := worldstate.NewConstant(id(9), "limited", worldstate.ScopeWorld, worldstate.TypeInteger)
	limited.Max = worldstate.Limit(3)
	group := worldstate.New(id(9), "group", worldstate.ScopeWorld, worldstate.TypeGroup)
	group.Constant = true
	for name, wrong := range map[string]worldstate.Declaration{
		"a yes or no":         worldstate.NewConstant(id(9), "flag", worldstate.ScopeWorld, worldstate.TypeBool),
		"with limits":         limited,
		"a group":             group,
		"longer than a page":  tooLong,
		"beside a value":      worldstate.New(id(9), "letter", worldstate.ScopeWorld, worldstate.TypeBool),
		"a workflow's own":    worldstate.NewConstant(id(9), "local", worldstate.ScopeLocal, worldstate.TypeText),
		"a choice of choices": worldstate.NewConstant(id(9), "pick", worldstate.ScopeWorld, worldstate.TypeChoice),
	} {
		if err := read.Values.Insert(wrong); !errors.Is(err, worldstate.ErrValue) {
			t.Errorf("a constant %s is refused: %v", name, err)
		}
	}
	locals := worldstate.NewDeclarations(worldstate.HoldsLocals)
	if err := locals.Insert(worldstate.NewConstant(id(9), "local", worldstate.ScopeLocal,
		worldstate.TypeText)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a workflow has no constants: %v", err)
	}
	registry := content.NewRegistry()
	if err := registry.Register(worldstate.Format()); err != nil {
		t.Fatal(err)
	}
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: worldstate.FormatName, Version: 3, ID: worldstate.ID(), Name: worldstate.Name, Revision: 1,
		Data: json.RawMessage(`{"values":[]}`),
	})
	if err != nil || !changed || upgraded.Version != worldstate.FormatVersion || worldstate.FormatVersion != 4 {
		t.Fatalf("upgrade %v %v", changed, err)
	}
}

// harbour is a world state with a shop template holding a greeting and a
// served value, a world constant, Marta and Sana with the template, Ivo
// without it, and George with a constant of his own.
type harbour struct {
	catalog                   *worldstate.Catalog
	marta, sana, ivo, george  ids.ULID
	greeting, restock, served worldstate.Declaration
	backstory                 worldstate.Declaration
}

func newHarbour(t *testing.T) harbour {
	t.Helper()
	world := worldstate.NewDocument().Values
	shopTemplate := worldstate.New(id(1), "shop", worldstate.ScopeNPC, worldstate.TypeTemplate)
	made := harbour{
		catalog: nil, marta: id(100), sana: id(101), ivo: id(102), george: id(103),
		greeting: in(worldstate.NewConstant(id(2), "greeting", worldstate.ScopeNPC, worldstate.TypeText), id(1)),
		restock:  worldstate.NewConstant(id(3), "restock_minutes", worldstate.ScopeWorld, worldstate.TypeInteger),
		served:   in(worldstate.New(id(4), "served", worldstate.ScopeNPC, worldstate.TypeBool), id(1)),
		backstory: worldstate.NewConstant(id(5), "backstory", worldstate.ScopeNPC,
			worldstate.TypeText),
	}
	made.greeting.Default = worldstate.Value{Type: worldstate.TypeText, Text: "Welcome in!"}.Encode()
	made.restock.Default = worldstate.Value{Type: worldstate.TypeInteger, Integer: 30}.Encode()
	made.backstory.Default = worldstate.Value{Type: worldstate.TypeText, Text: "Salt flats."}.Encode()
	insert(t, world, shopTemplate, made.greeting, made.served, made.restock)
	marta, sana, ivo, george := worldstate.NewOwned(worldstate.ScopeNPC), worldstate.NewOwned(worldstate.ScopeNPC),
		worldstate.NewOwned(worldstate.ScopeNPC), worldstate.NewOwned(worldstate.ScopeNPC)
	for _, owned := range []*worldstate.Owned{&marta, &sana} {
		if err := owned.Apply(id(1)); err != nil {
			t.Fatal(err)
		}
	}
	marta.SetConstant(made.greeting.ID, json.RawMessage(`"Fresh off the boat!"`))
	ivo.SetConstant(made.greeting.ID, json.RawMessage(`"Rope and tar."`))
	sana.SetConstant(made.restock.ID, json.RawMessage(`"not a number"`))
	insert(t, george.Values, made.backstory)
	made.catalog = worldstate.NewCatalog(world, map[ids.ULID]worldstate.Owner{
		made.marta:  {Scope: worldstate.ScopeNPC, Path: "marta", Name: "Marta", Owned: marta},
		made.sana:   {Scope: worldstate.ScopeNPC, Path: "sana", Name: "Sana", Owned: sana},
		made.ivo:    {Scope: worldstate.ScopeNPC, Path: "ivo", Name: "Ivo", Owned: ivo},
		made.george: {Scope: worldstate.ScopeNPC, Path: "george", Name: "George", Owned: george},
	})

	return made
}

// TestConstantsForEachHolder reads a template's constant as each NPC with
// the template has it, its own or the default; keeps an NPC's value while
// the template is off; reads an NPC's own constant; and keeps constants out
// of what reads values.
func TestConstantsForEachHolder(t *testing.T) {
	t.Parallel()
	harbour := newHarbour(t)
	catalog := harbour.catalog
	for _, check := range []struct {
		name         string
		scope        worldstate.Scope
		owner, value ids.ULID
		found        bool
		reads        string
	}{
		{"Marta's own greeting", worldstate.ScopeNPC, harbour.marta, harbour.greeting.ID, true, `"Fresh off the boat!"`},
		{"Sana's default greeting", worldstate.ScopeNPC, harbour.sana, harbour.greeting.ID, true, `"Welcome in!"`},
		{"Ivo has no shop template", worldstate.ScopeNPC, harbour.ivo, harbour.greeting.ID, false, ""},
		{"George's own constant", worldstate.ScopeNPC, harbour.george, harbour.backstory.ID, true, `"Salt flats."`},
		{"a world constant", worldstate.ScopeWorld, ids.ULID{}, harbour.restock.ID, true, "30"},
		{"a value is no constant", worldstate.ScopeNPC, harbour.marta, harbour.served.ID, false, ""},
	} {
		declared, value, found := catalog.Constant(check.scope, check.owner, check.value)
		if found != check.found || (found && declared.Label(value) != check.reads) {
			t.Errorf("%s: found %v, reads %s", check.name, found, declared.Label(value))
		}
	}
	if _, found := catalog.Lookup(worldstate.ScopeNPC, harbour.marta, harbour.greeting.ID); found {
		t.Fatal("a constant is not looked up as a value")
	}
	if _, found := catalog.Lookup(worldstate.ScopeWorld, ids.ULID{}, harbour.restock.ID); found {
		t.Fatal("a world constant is not looked up as a value")
	}
	for _, held := range catalog.Held(worldstate.ScopeNPC, harbour.marta) {
		if held.Declaration.IsConstant() {
			t.Fatalf("an NPC's values hold no constant: %s", held.Path)
		}
	}
	var paths []string
	for _, held := range catalog.HeldConstants(worldstate.ScopeNPC, harbour.marta) {
		paths = append(paths, held.Path)
	}
	if len(paths) != 1 || paths[0] != "npc.shop.greeting" {
		t.Fatalf("Marta's constants: %v", paths)
	}
	if holders := catalog.Holders(id(1)); len(holders) != 2 || holders[0] != harbour.marta ||
		holders[1] != harbour.sana {
		t.Fatalf("the shop template is on Marta and Sana: %v", holders)
	}
	ivo, _ := catalog.Owner(harbour.ivo)
	if err := ivo.Owned.Apply(id(1)); err != nil {
		t.Fatal(err)
	}
	if own, has := ivo.Owned.ConstantValue(harbour.greeting); !has || own.Text != "Rope and tar." {
		t.Fatalf("Ivo's greeting was kept while the template was off: %+v", own)
	}
}

// TestOwnConstantValuesAreSaved saves an NPC's own constant values, reads
// them back, clears one, and refuses one with no constant.
func TestOwnConstantValuesAreSaved(t *testing.T) {
	t.Parallel()
	owned := worldstate.NewOwned(worldstate.ScopeNPC)
	owned.SetConstant(id(2), json.RawMessage(`"Fresh off the boat!"`))
	owned.SetConstant(id(3), json.RawMessage(`5`))
	wire, err := owned.Wire()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var again worldstate.WireOwned
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	read, err := worldstate.ReadOwned(worldstate.ScopeNPC, again)
	if err != nil || !read.Equal(owned) || read.Len() != 2 {
		t.Fatalf("read back %+v %v", read, err)
	}
	if raw, had := read.ClearConstant(id(3)); !had || string(raw) != "5" || read.Equal(owned) {
		t.Fatal("clearing a value gives the default back")
	}
	if _, err := worldstate.ReadOwned(worldstate.ScopeNPC, worldstate.WireOwned{
		Values: nil, Templates: nil, Constants: map[ids.ULID]json.RawMessage{{}: json.RawMessage(`1`)},
	}); !errors.Is(err, worldstate.ErrData) {
		t.Fatalf("a value for no constant is refused: %v", err)
	}
}
