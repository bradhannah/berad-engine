package workflowview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// withApron adds a sprite, Apron, to the project.
func (f *fixture) withApron() {
	f.t.Helper()
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(append(entries, project.Entry{
		ID: frametest.ID(97), Name: "Apron", Format: art.SpriteFormatName, Folder: "", Tags: nil, Kind: "",
	})))
}

// TestCheckingAZone offers a Check value its NPC's built-in values, read
// only, and checks the zone it lives in against Town, chosen with the zone
// picker.
func TestCheckingAZone(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.withTown()
	f.click("ui:pick-value")
	for _, path := range []string{"npc.name", "npc.zone", "npc.behaviour", "npc.sprite"} {
		if _, ok := f.state.Locate("valuerow:"+path, f.docs); !ok {
			t.Fatalf("a Check value reads %s", path)
		}
	}
	f.click("valuerow:npc.zone")
	if !f.has("value:value=npc.zone") {
		t.Fatal("chose its NPC's zone")
	}
	f.click("ui:pick-against")
	if !f.has("zone-picker") {
		t.Fatal("a zone is chosen with the zone picker")
	}
	f.click("zone:Town")
	f.keys("Enter")
	if !f.has("holds:against=Town") || f.has("problem:"+flow.KindCheckValue+":1") {
		t.Fatal("it checks against Town")
	}
}

// TestSettingASprite offers a Set value its NPC's sprite and not its zone
// or behaviour, sets the sprite to Apron, chosen from a list, and flags a
// Set value on its zone.
func TestSettingASprite(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSetValue)
	f.withApron()
	f.click("ui:pick-value")
	if _, ok := f.state.Locate("valuerow:npc.sprite", f.docs); !ok {
		t.Fatal("a Set value sets its NPC's sprite")
	}
	for _, path := range []string{"npc.zone", "npc.behaviour"} {
		if _, ok := f.state.Locate("valuerow:"+path, f.docs); ok {
			t.Fatalf("a Set value never sets %s", path)
		}
	}
	f.click("valuerow:npc.sprite")
	f.click("ui:pick-to")
	if !f.has("dialog-row=Apron") {
		t.Fatal("the sprites are listed to choose from")
	}
	f.click("row:Apron")
	f.click("ui:use")
	if !f.has("holds:to=Apron") || f.has("problem:"+flow.KindSetValue+":1") {
		t.Fatal("it sets the sprite to Apron")
	}
	zone, err := f.nth(flow.KindSetValue, 1).WithParam(flow.ParamValue, flow.ValueRef{
		Scope: worldstate.ScopeNPC, Value: worldstate.ZoneID(), Owner: ids.ULID{}, Found: ids.ULID{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.workflow().Replace(zone); err != nil {
		t.Fatal(err)
	}
	f.docs.Touch(workflowID)
	f.frame(noInput())
	if !f.has("problem:" + flow.KindSetValue + ":1=An NPC's zone and behaviour are set in its details, not by a workflow.") {
		t.Fatal("a Set value on its NPC's zone is flagged")
	}
}

// TestTextFromAnyValue sets a text local to its NPC's zone: Another value
// offers values of every type, as text.
func TestTextFromAnyValue(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSetValue)
	where := worldstate.New(frametest.ID(54), "where", worldstate.ScopeLocal, worldstate.TypeText)
	f.carryOutOn(workflowID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{where}, Set: nil, Remove: nil,
	})
	f.click("ui:pick-value")
	f.click("valuerow:local.where")
	f.click("ui:source-value")
	f.click("ui:pick-to")
	f.click("valuerow:npc.zone")
	if !f.has("refers:to=npc.zone") || f.has("problem:"+flow.KindSetValue+":1") {
		t.Fatal("a text value is set to its NPC's zone, as text")
	}
}
