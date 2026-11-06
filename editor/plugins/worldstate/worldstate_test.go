package worldstate_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func encode(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(stateplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	if !registry.IsRequired(worldstate.ID()) || registry.IsReadOnly(worldstate.ID()) {
		t.Fatal("every project holds a world state it can change")
	}
	kind, ok := registry.Edit(stateplugin.ChangeEdit)
	if !ok {
		t.Fatal("no change edit")
	}
	preparer, ok := kind.(model.Preparer)
	if !ok {
		t.Fatal("a change prepares new IDs")
	}
	document := stateplugin.NewDocument()
	quests := worldstate.New(stateplugin.TempID(1), "quests", worldstate.ScopeWorld, worldstate.TypeGroup)
	slain := worldstate.New(stateplugin.TempID(2), "dragon_slain", worldstate.ScopeWorld, worldstate.TypeBool)
	slain.Parent = quests.ID
	var next ids.ULID
	next[0] = 1
	prepared, err := preparer.Prepare(document, encode(t, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{quests, slain}, Set: nil, Remove: nil,
	}), func() (ids.ULID, error) {
		next[15]++

		return next, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var change stateplugin.ChangeArgs
	if err := json.Unmarshal(prepared, &change); err != nil {
		t.Fatal(err)
	}
	if change.Add[0].ID[15] != 1 || change.Add[1].ID != next || change.Add[1].Parent != change.Add[0].ID {
		t.Fatalf("new IDs in order, the value last and inside its new group: %+v", change.Add)
	}
	again, err := preparer.Prepare(document, prepared, func() (ids.ULID, error) { return ids.ULID{}, errors.New("no more") })
	if err != nil || string(again) != string(prepared) {
		t.Fatalf("a prepared change prepares unchanged: %v", err)
	}
	moved := change.Add[1].Clone()
	moved.Parent, moved.Name = ids.ULID{}, "slain"
	for _, args := range []json.RawMessage{
		prepared,
		encode(t, stateplugin.ChangeArgs{Add: nil, Set: []worldstate.Declaration{moved}, Remove: nil}),
		encode(t, stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: []ids.ULID{change.Add[0].ID}}),
	} {
		if err := model.VerifyUndo(document, kind, args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", args, err)
		}
		if _, err := kind.Apply(document, args); err != nil {
			t.Fatalf("%s: %v", args, err)
		}
	}
	if paths := document.State.Values.Values(); len(paths) != 1 || document.State.Values.Path(paths[0].ID) != "world.slain" {
		t.Fatalf("the value left its group, which went: %+v", paths)
	}
	for name, bad := range map[string]worldstate.Declaration{
		"punctuation":   worldstate.New(next, "george's", worldstate.ScopeWorld, worldstate.TypeInteger),
		"a local value": worldstate.New(next, "customer", worldstate.ScopeLocal, worldstate.TypeNPC),
	} {
		args := encode(t, stateplugin.ChangeArgs{Add: []worldstate.Declaration{bad}, Set: nil, Remove: nil})
		if _, err := kind.Apply(document, args); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("%s is refused: %v", name, err)
		}
	}
	held := encode(t, stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: []ids.ULID{change.Add[1].ID, next}})
	if _, err := kind.Apply(document, held); !errors.Is(err, model.ErrArgs) || document.State.Values.Len() != 1 {
		t.Fatalf("a change is all or nothing: %v", err)
	}
}

// TestSignalsChange adds a signal, giving it an identity, renames it, and
// refuses a second of the same name; the add's inverse removes it again.
func TestSignalsChange(t *testing.T) {
	t.Parallel()
	document := stateplugin.NewDocument()
	edit := stateplugin.Signals{}
	add := encode(t, stateplugin.SignalsArgs{Add: []worldstate.Signal{{Name: "bread_called"}}})
	prepared, err := edit.Prepare(document, add, func() (ids.ULID, error) { return ids.ULID{9}, nil })
	if err != nil {
		t.Fatal(err)
	}
	applied, err := edit.Apply(document, prepared)
	if err != nil || len(document.State.Signals) != 1 || document.State.Signals[0].ID != (ids.ULID{9}) {
		t.Fatalf("added with an identity: %+v %v", document.State.Signals, err)
	}
	rename := encode(t, stateplugin.SignalsArgs{Set: []worldstate.Signal{{ID: ids.ULID{9}, Name: "bread_ready"}}})
	if _, err := edit.Apply(document, rename); err != nil || document.State.Signals[0].Name != "bread_ready" {
		t.Fatalf("renamed: %+v %v", document.State.Signals, err)
	}
	twin := encode(t, stateplugin.SignalsArgs{Add: []worldstate.Signal{{ID: ids.ULID{8}, Name: "bread_ready"}}})
	if _, err := edit.Apply(document, twin); !errors.Is(err, model.ErrArgs) || len(document.State.Signals) != 1 {
		t.Fatalf("two of one name are refused: %v", err)
	}
	if _, err := edit.Apply(document, applied.Inverse); err != nil || len(document.State.Signals) != 0 {
		t.Fatalf("the add's inverse removes it: %+v %v", document.State.Signals, err)
	}
}

// TestConstantsAreEdited adds a world constant with a page of text and a
// constant in a template, renames, retypes and deletes them, each undone,
// and finds an NPC's own value of a deleted constant counting again once
// Undo brings the constant back.
func TestConstantsAreEdited(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(stateplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	kind, ok := registry.Edit(stateplugin.ChangeEdit)
	if !ok {
		t.Fatal("no change edit")
	}
	var shop, greeting, letter, marta ids.ULID
	shop[0], shop[15], greeting[0], greeting[15], letter[0], letter[15], marta[0], marta[15] = 1, 1, 1, 2, 1, 3, 2, 1
	document := stateplugin.NewDocument()
	template := worldstate.New(shop, "shop", worldstate.ScopeNPC, worldstate.TypeTemplate)
	greets := worldstate.NewConstant(greeting, "greeting", worldstate.ScopeNPC, worldstate.TypeText)
	greets.Parent = shop
	page := worldstate.NewConstant(letter, "letter", worldstate.ScopeWorld, worldstate.TypeText)
	page.Default = worldstate.Value{Type: worldstate.TypeText, Text: strings.Repeat("Dear Marta. ", 300)}.Encode()
	renamed, retyped := greets.Clone(), page.Clone()
	renamed.Name = "hello"
	retyped.Type, retyped.Default = worldstate.TypeInteger, json.RawMessage(`30`)
	for _, args := range []json.RawMessage{
		encode(t, stateplugin.ChangeArgs{Add: []worldstate.Declaration{template, greets, page}, Set: nil, Remove: nil}),
		encode(t, stateplugin.ChangeArgs{Add: nil, Set: []worldstate.Declaration{renamed, retyped}, Remove: nil}),
		encode(t, stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: []ids.ULID{letter}}),
	} {
		if err := model.VerifyUndo(document, kind, args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", args, err)
		}
		if _, err := kind.Apply(document, args); err != nil {
			t.Fatalf("%s: %v", args, err)
		}
	}
	owned := worldstate.NewOwned(worldstate.ScopeNPC)
	if err := owned.Apply(shop); err != nil {
		t.Fatal(err)
	}
	owned.SetConstant(greeting, json.RawMessage(`"Fresh off the boat!"`))
	removing := encode(t, stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: []ids.ULID{greeting}})
	applied, err := kind.Apply(document, removing)
	if err != nil {
		t.Fatal(err)
	}
	read := func() (worldstate.Value, bool) {
		catalog := worldstate.NewCatalog(document.State.Values, map[ids.ULID]worldstate.Owner{
			marta: {Scope: worldstate.ScopeNPC, Path: "marta", Name: "Marta", Owned: owned},
		})
		_, value, found := catalog.Constant(worldstate.ScopeNPC, marta, greeting)

		return value, found
	}
	if _, found := read(); found {
		t.Fatal("a deleted constant is gone")
	}
	undo := kind
	if applied.InverseEdit != "" {
		if undo, ok = registry.Edit(applied.InverseEdit); !ok {
			t.Fatal("no undo")
		}
	}
	if _, err := undo.Apply(document, applied.Inverse); err != nil {
		t.Fatal(err)
	}
	if value, found := read(); !found || value.Text != "Fresh off the boat!" {
		t.Fatalf("Undo brings back Marta's own greeting: %+v", value)
	}
}
