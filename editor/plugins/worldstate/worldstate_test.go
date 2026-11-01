package worldstate_test

import (
	"encoding/json"
	"errors"
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
	edit := func(name string) model.EditKind {
		kind, ok := registry.Edit(name)
		if !ok {
			t.Fatalf("no edit %s", name)
		}

		return kind
	}
	document := stateplugin.NewDocument()
	mood := worldstate.New(ids.ULID{}, "mood", worldstate.ScopeNPC, worldstate.TypeChoice)
	preparer, ok := edit(stateplugin.AddValueEdit).(model.Preparer)
	if !ok {
		t.Fatal("adding a value prepares its ID")
	}
	var next ids.ULID
	next[0], next[15] = 1, 9
	prepared, err := preparer.Prepare(document, encode(t, stateplugin.ValueArgs{Value: mood}), func() (ids.ULID, error) {
		return next, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mood.ID = next
	renamed := mood.Clone()
	renamed.Name, renamed.Description = "temper", "How the NPC feels."
	for _, check := range []struct {
		edit string
		args json.RawMessage
	}{
		{stateplugin.AddValueEdit, prepared},
		{stateplugin.SetValueEdit, encode(t, stateplugin.ValueArgs{Value: renamed})},
		{stateplugin.RemoveValueEdit, encode(t, stateplugin.RemoveValueArgs{ID: next})},
	} {
		kind := edit(check.edit)
		if err := model.VerifyUndo(document, kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.edit, err)
		}
		if _, err := kind.Apply(document, check.args); err != nil {
			t.Fatalf("%s: %v", check.edit, err)
		}
	}
	bad := worldstate.New(next, "george's", worldstate.ScopeNPC, worldstate.TypeNumber)
	if _, err := edit(stateplugin.AddValueEdit).Apply(document, encode(t, stateplugin.ValueArgs{Value: bad})); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a name with punctuation is refused: %v", err)
	}
	local := worldstate.New(next, "customer", worldstate.ScopeLocal, worldstate.TypeNPC)
	if _, err := edit(stateplugin.AddValueEdit).Apply(document, encode(t, stateplugin.ValueArgs{Value: local})); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("local values are declared on their workflow: %v", err)
	}
}
