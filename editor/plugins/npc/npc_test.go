package npc_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
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
	var routine ids.ULID
	routine[15] = 7
	document := npcplugin.NewDocument()
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{npcplugin.SetBehaviour{}, encode(t, npcplugin.SetBehaviourArgs{Behaviour: routine})},
		{npcplugin.SetNotes{}, encode(t, npcplugin.SetNotesArgs{Notes: "Grumpy before coffee."})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
		if _, err := check.kind.Apply(document, check.args); err != nil {
			t.Fatal(err)
		}
	}
	references := document.References()
	if len(references) != 1 || references[0].Document != routine {
		t.Fatalf("an NPC refers to its behaviour: %+v", references)
	}
	tooLong := encode(t, npcplugin.SetNotesArgs{Notes: strings.Repeat("a", npc.MaxNotes+1)})
	if _, err := (npcplugin.SetNotes{}).Apply(document, tooLong); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("notes too long: %v", err)
	}
}
