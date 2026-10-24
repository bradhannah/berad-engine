package orchestrator_test

import (
	"context"
	"strings"
	"testing"
	"time"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestNPCsAndPlacements gives an NPC a behaviour and places it on a floor:
// where-used finds the floor from the NPC and the NPC from its workflow, and
// both are saved, the placement in the floor's own file.
func TestNPCsAndPlacements(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID(), floorID)
	routine := createDocument(t, browser, flow.FormatName, "Clerk day", ids.ULID{}, workflow.NewData(), "")
	clerk := createDocument(t, browser, npc.FormatName, "Ada Clerk", ids.ULID{},
		encodeDocument(t, npcplugin.NewDocument()), "")
	mustCommand(t, browser, clerk, npcplugin.SetBehaviourEdit, "", npcplugin.SetBehaviourArgs{Behaviour: routine})
	placed := mustCommand(t, browser, floorID, tilefloor.AddPlacementEdit, "", tilefloor.AddPlacementArgs{
		Placement: floor.Placement{ID: ids.ULID{}, NPC: clerk, Col: 3, Row: 4}, Index: 0,
	})
	if placed.Created.IsZero() {
		t.Fatal("the placement got no ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	uses, err := browser.WhereUsed(ctx, clerk, "")
	if err != nil || len(uses) != 1 || uses[0].Document != floorID || uses[0].Label != "placed at 3, 4" {
		t.Fatalf("uses of the NPC %+v %v", uses, err)
	}
	uses, err = browser.WhereUsed(ctx, routine, "")
	if err != nil || len(uses) != 1 || uses[0].Document != clerk || uses[0].Label != "behaviour" {
		t.Fatalf("uses of the workflow %+v %v", uses, err)
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	saved := readFloorFile(t, dir)
	if saved.Version != floor.FormatVersion || !strings.Contains(string(saved.Data), placed.Created.String()) {
		t.Fatalf("the floor file lacks the placement: v%d %s", saved.Version, saved.Data)
	}
	npcFile := readEnvelopeFile(t, onlyFile(t, dir, "npcs/ada-clerk--*.json"))
	if !strings.Contains(string(npcFile.Data), routine.String()) {
		t.Fatalf("the NPC file lacks its behaviour: %s", npcFile.Data)
	}
}
