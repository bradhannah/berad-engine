package orchestrator_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bradhannah/berad-engine/editor/orchestrator"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// goingTo makes a workflow document whose one Go to goes to an anchor on a
// floor.
func goingTo(t *testing.T, floorID ids.ULID) *workflow.Document {
	t.Helper()
	made, err := flow.New(ids.MustParse("01J00000000000000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	goTo, err := flow.NewNode(flow.KindGoTo)
	if err != nil {
		t.Fatal(err)
	}
	goTo.ID = ids.MustParse("01J00000000000000000000002")
	goTo, err = goTo.WithParam("destination", floor.AnchorRef{Floor: floorID, Anchor: ids.MustParse("01J00000000000000000000003")})
	if err != nil {
		t.Fatal(err)
	}
	if err := made.Insert([]flow.Node{goTo}, nil, nil); err != nil {
		t.Fatal(err)
	}

	return &workflow.Document{Workflow: made}
}

// asBeforeZones rewrites a saved workflow as version 7 wrote it, before
// workflows had kinds and zones.
func asBeforeZones(t *testing.T, path string) {
	t.Helper()
	envelope := readEnvelopeFile(t, path)
	var data map[string]any
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatal(err)
	}
	delete(data, "purpose")
	delete(data, "zone")
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	envelope.Version, envelope.Data = 7, raw
	encoded, err := content.Encode(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestOlderWorkflowsGetTheirZone reopens a project saved before workflows
// had zones: a behaviour gets the zone its follower lives in, a workflow
// nobody follows the zone of its places, and one naming no place runs in
// any zone, as does a workflow saved since that runs in any zone by
// choice.
func TestOlderWorkflowsGetTheirZone(t *testing.T) {
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
	harbourID, err := orch.CreateDocument("Harbour", zoneplugin.NewDocument())
	if err != nil {
		t.Fatal(err)
	}
	made := map[string]ids.ULID{}
	for name, document := range map[string]*workflow.Document{
		"Shop": goingTo(t, floorID), "Errand": goingTo(t, floorID), "Roam": goingTo(t, floorID),
		"Idle": {Workflow: mustNew(t)},
	} {
		if made[name], err = orch.CreateDocument(name, document); err != nil {
			t.Fatal(err)
		}
	}
	clerk := npcplugin.NewDocument()
	clerk.NPC.Zone, clerk.NPC.Behaviour = harbourID, made["Shop"]
	if _, err := orch.CreateDocument("Clerk", clerk); err != nil {
		t.Fatal(err)
	}
	if err := orch.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shop", "errand", "idle"} {
		asBeforeZones(t, onlyFile(t, dir, "workflows/"+name+"--????????.json"))
	}
	reopened, err := orchestrator.New(baseConfig(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]ids.ULID{
		"shop": harbourID, "errand": marketID, "idle": {}, "roam": {},
	} {
		saved := readEnvelopeFile(t, onlyFile(t, dir, "workflows/"+name+"--????????.json"))
		read, err := flow.Decode(saved.Data)
		if err != nil || saved.Version != flow.FormatVersion || read.RunsIn() != want ||
			read.Purpose() != flow.PurposeBehaviour {
			t.Fatalf("%s runs in %v, wanting %v: %v", name, read.RunsIn(), want, err)
		}
	}
}

func mustNew(t *testing.T) *flow.Workflow {
	t.Helper()
	made, err := flow.New(ids.MustParse("01J00000000000000000000001"))
	if err != nil {
		t.Fatal(err)
	}

	return made
}
