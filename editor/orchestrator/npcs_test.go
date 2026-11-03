package orchestrator_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
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

// TestPlacedNPCsGetTheirZone opens a project saved before NPCs had zones:
// an NPC placed on a floor of a zone lives there from then on, and one
// placed nowhere still lives nowhere.
func TestPlacedNPCsGetTheirZone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	orch, err := orchestrator.New(baseConfig(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	market := zoneplugin.NewDocument()
	if err := market.Zone.Insert(0, floorID); err != nil {
		t.Fatal(err)
	}
	marketID, err := orch.CreateDocument("Market", market)
	if err != nil {
		t.Fatal(err)
	}
	clerk, err := orch.CreateDocument("Clerk", npcplugin.NewDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orch.CreateDocument("Wanderer", npcplugin.NewDocument()); err != nil {
		t.Fatal(err)
	}
	placed := floor.Placement{ID: ids.ULID{}, NPC: clerk, Col: 2, Row: 3}
	placed.ID[0], placed.ID[15] = 7, 7
	if err := orch.ChangeDocument(floorID, func(document model.Document) error {
		floorDoc, _ := document.(*tilefloor.Document) //nolint:errcheck // The project's own floor.

		return floorDoc.Floor.InsertPlacement(0, placed)
	}); err != nil {
		t.Fatal(err)
	}
	if err := orch.Close(); err != nil {
		t.Fatal(err)
	}
	if file := readEnvelopeFile(t, onlyFile(t, dir, "npcs/clerk--*.json")); strings.Contains(string(file.Data), "zone") {
		t.Fatalf("saved before it had a zone: %s", file.Data)
	}
	reopened, err := orchestrator.New(baseConfig(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if file := readEnvelopeFile(t, onlyFile(t, dir, "npcs/clerk--*.json")); !strings.Contains(string(file.Data),
		marketID.String()) {
		t.Fatalf("the placed NPC lives in its placement's zone: %s", file.Data)
	}
	if file := readEnvelopeFile(t, onlyFile(t, dir, "npcs/wanderer--*.json")); strings.Contains(string(file.Data), "zone") {
		t.Fatalf("an NPC placed nowhere lives nowhere: %s", file.Data)
	}
}
