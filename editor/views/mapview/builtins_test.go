package mapview_test

import (
	"encoding/json"
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

// TestASpriteSetInThePreview previews the clerk, whose workflow sets its
// sprite to Apron: the preview's state and the Map show Apron.
func TestASpriteSetInThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	apron := frametest.ID(95)
	f.docs.Put(worldstate.ID(), worldstate.Name, stateplugin.NewDocument())
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(append(entries, project.Entry{
		ID: apron, Name: "Apron", Format: art.SpriteFormatName, Folder: "", Tags: nil, Kind: "",
	})))
	to, err := json.Marshal(apron.String())
	if err != nil {
		t.Fatal(err)
	}
	dress, err := flow.NewNode(flow.KindSetValue)
	if err == nil {
		dress, err = dress.WithParam(flow.ParamValue, flow.ValueRef{
			Scope: worldstate.ScopeNPC, Value: worldstate.SpriteID(), Owner: ids.ULID{}, Found: ids.ULID{},
		})
	}
	if err == nil {
		dress, err = dress.WithParam("to", json.RawMessage(to))
	}
	if err != nil {
		t.Fatal(err)
	}
	dress.ID = frametest.ID(96)
	f.docs.View(routineID, func(document client.Document) {
		routine := document.Model.(*workflowplugin.Document).Workflow
		first, _ := routine.Node(frametest.ID(80))
		if err := routine.Insert([]flow.Node{dress.WithLink(flow.OutputNext, first.Next[flow.OutputNext])}, nil,
			[]flow.Link{{From: frametest.ID(80), Output: flow.OutputNext, To: dress.ID}}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(routineID)
	if f.has("npc-sprite:Ada Clerk=Apron") {
		t.Fatal("the clerk has no sprite before the preview")
	}
	f.startPreview("cell:3,3")
	f.idle(3)
	if entities := f.lastState().Entities; entities[1].Sprite != apron || !f.has("npc-sprite:Ada Clerk=Apron") {
		t.Fatalf("the clerk shows Apron once its workflow sets it: %+v", entities[1])
	}
}
