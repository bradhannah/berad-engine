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
	var routine, secret, guards, armed, look, market ids.ULID
	market[0], market[15] = 3, 1
	routine[15], secret[0], secret[15], guards[0], guards[15], armed[0], armed[15] = 7, 1, 8, 1, 9, 1, 10
	look[0], look[15] = 2, 8
	var greeting ids.ULID
	greeting[0], greeting[15] = 1, 11
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
		{npcplugin.SetZone{}, encode(t, npcplugin.SetZoneArgs{Zone: market})},
		{npcplugin.SetBehaviour{}, encode(t, npcplugin.SetBehaviourArgs{Behaviour: routine})},
		{npcplugin.SetSprite{}, encode(t, npcplugin.SetSpriteArgs{Sprite: look})},
		{npcplugin.SetNotes{}, encode(t, npcplugin.SetNotesArgs{Notes: "Grumpy before coffee."})},
		{npcplugin.SetPath{}, encode(t, npcplugin.SetPathArgs{Path: "george"})},
		{edit(npcplugin.ChangeValuesEdit), encode(t, stateplugin.ChangeArgs{
			Add: []worldstate.Declaration{worldstate.New(secret, "secret", worldstate.ScopeNPC, worldstate.TypeBool)},
			Set: nil, Remove: nil,
		})},
		{edit(npcplugin.ApplyTemplateEdit), encode(t, stateplugin.TemplateArgs{Template: guards, Index: 9})},
		{edit(npcplugin.ApplyTemplateEdit), encode(t, stateplugin.TemplateArgs{Template: armed, Index: 0})},
		{edit(npcplugin.UnapplyTemplateEdit), encode(t, stateplugin.TemplateArgs{Template: guards, Index: 0})},
		{edit(npcplugin.ChangeValuesEdit), encode(t, stateplugin.ChangeArgs{
			Add: []worldstate.Declaration{worldstate.NewConstant(look, "backstory", worldstate.ScopeNPC,
				worldstate.TypeText)},
			Set: nil, Remove: nil,
		})},
		{edit(npcplugin.SetConstantEdit), encode(t, stateplugin.ConstantArgs{
			Constant: greeting, Value: json.RawMessage(`"Fresh off the boat!"`),
		})},
		{edit(npcplugin.SetConstantEdit), encode(t, stateplugin.ConstantArgs{
			Constant: greeting, Value: json.RawMessage(`"Mackerel today."`),
		})},
		{edit(npcplugin.SetConstantEdit), encode(t, stateplugin.ConstantArgs{Constant: greeting, Value: nil})},
		{edit(npcplugin.SetConstantEdit), encode(t, stateplugin.ConstantArgs{
			Constant: market, Value: json.RawMessage(`5`),
		})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
		if _, err := check.kind.Apply(document, check.args); err != nil {
			t.Fatal(err)
		}
	}
	references := document.References()
	if len(references) != 4 || references[0].Item != armed.String() || references[1].Document != market ||
		references[1].Label != npcplugin.LivesHere || references[2].Document != routine ||
		references[3].Document != look {
		t.Fatalf("an NPC refers to its templates, its zone, its behaviour and its sprite: %+v", references)
	}
	if document.NPC.Owned.Values.Len() != 2 {
		t.Fatal("the NPC owns its secret and its backstory")
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
	if own, has := document.NPC.Owned.ConstantValue(worldstate.NewConstant(market, "x",
		worldstate.ScopeNPC, worldstate.TypeInteger)); !has || own.Integer != 5 {
		t.Fatalf("the NPC keeps its own value of a constant: %+v", own)
	}
	for name, args := range map[string]json.RawMessage{
		"no constant": encode(t, stateplugin.ConstantArgs{Constant: ids.ULID{}, Value: json.RawMessage(`1`)}),
		"not JSON":    json.RawMessage(`{"constant":"` + greeting.String() + `","value":nope}`),
	} {
		if _, err := edit(npcplugin.SetConstantEdit).Apply(document, args); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("%s is refused: %v", name, err)
		}
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
