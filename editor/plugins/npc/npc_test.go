package npc_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
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
	registry, err := plugin.NewRegistry(npcplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	var routine, secret, guards, armed ids.ULID
	routine[15], secret[0], secret[15], guards[0], guards[15], armed[0], armed[15] = 7, 1, 8, 1, 9, 1, 10
	edit := func(name string) model.EditKind {
		kind, ok := registry.Edit(name)
		if !ok {
			t.Fatalf("no edit %s", name)
		}

		return kind
	}
	document := npcplugin.NewDocument()
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{npcplugin.SetBehaviour{}, encode(t, npcplugin.SetBehaviourArgs{Behaviour: routine})},
		{npcplugin.SetNotes{}, encode(t, npcplugin.SetNotesArgs{Notes: "Grumpy before coffee."})},
		{npcplugin.SetPath{}, encode(t, npcplugin.SetPathArgs{Path: "george"})},
		{edit(npcplugin.ChangeValuesEdit), encode(t, stateplugin.ChangeArgs{
			Add: []worldstate.Declaration{worldstate.New(secret, "secret", worldstate.ScopeNPC, worldstate.TypeBool)},
			Set: nil, Remove: nil,
		})},
		{edit(npcplugin.ApplyTemplateEdit), encode(t, stateplugin.TemplateArgs{Template: guards, Index: 9})},
		{edit(npcplugin.ApplyTemplateEdit), encode(t, stateplugin.TemplateArgs{Template: armed, Index: 0})},
		{edit(npcplugin.UnapplyTemplateEdit), encode(t, stateplugin.TemplateArgs{Template: guards, Index: 0})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
		if _, err := check.kind.Apply(document, check.args); err != nil {
			t.Fatal(err)
		}
	}
	references := document.References()
	if len(references) != 2 || references[0].Item != armed.String() || references[1].Document != routine {
		t.Fatalf("an NPC refers to its templates and its behaviour: %+v", references)
	}
	if document.NPC.Owned.Values.Len() != 1 {
		t.Fatal("the NPC owns its secret")
	}
	npcScope := encode(t, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{worldstate.New(guards, "mood", worldstate.ScopeWorld, worldstate.TypeBool)},
		Set: nil, Remove: nil,
	})
	if _, err := edit(npcplugin.ChangeValuesEdit).Apply(document, npcScope); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("an NPC owns only NPC values: %v", err)
	}
	if document.NPC.Path != "george" {
		t.Fatalf("path name set: %+v", document.NPC)
	}
	badPath := encode(t, npcplugin.SetPathArgs{Path: "george's"})
	if _, err := (npcplugin.SetPath{}).Apply(document, badPath); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a path name with punctuation is refused: %v", err)
	}
	tooLong := encode(t, npcplugin.SetNotesArgs{Notes: strings.Repeat("a", npc.MaxNotes+1)})
	if _, err := (npcplugin.SetNotes{}).Apply(document, tooLong); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("notes too long: %v", err)
	}
}
