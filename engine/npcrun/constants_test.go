package npcrun_test

import (
	"encoding/json"
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Constants: the shop template's greeting and serving time, which NPC 101
// has its own of, and a world motto as long as a page.
const (
	greetingID = 80
	serveID    = 81
	mottoID    = 82
	noteID     = 83
)

// constantsCatalog is catalog's shop with constants: NPC 101's own greeting
// and serving time, and NPC 111's the defaults; and a world motto and note.
func constantsCatalog(t *testing.T) *worldstate.Catalog {
	t.Helper()
	base := catalog(t)
	world := base.World().Clone()
	greeting := worldstate.NewConstant(id(greetingID), "greeting", worldstate.ScopeNPC, worldstate.TypeText)
	greeting.Parent, greeting.Default = id(shopID), json.RawMessage(`"Welcome [b]in[/b]!"`)
	serve := worldstate.NewConstant(id(serveID), "serve_seconds", worldstate.ScopeNPC, worldstate.TypeInteger)
	serve.Parent, serve.Default = id(shopID), json.RawMessage(`1`)
	motto := worldstate.NewConstant(id(mottoID), "motto", worldstate.ScopeWorld, worldstate.TypeText)
	motto.Default = worldstate.Value{Type: worldstate.TypeText, Text: strings.Repeat("Fair winds. ", 30)}.Encode()
	note := worldstate.New(id(noteID), "note", worldstate.ScopeWorld, worldstate.TypeText)
	if err := world.InsertAll([]worldstate.Declaration{greeting, serve, motto, note}); err != nil {
		t.Fatal(err)
	}
	owners := map[ids.ULID]worldstate.Owner{}
	for _, seed := range []byte{101, 111, 121, 131, loner + 1, zoneID} {
		owner, _ := base.Owner(id(seed))
		if seed == 101 {
			owner.Owned.SetConstant(id(greetingID), json.RawMessage(`"Fresh off the boat!"`))
			owner.Owned.SetConstant(id(serveID), json.RawMessage(`5`))
		}
		owners[id(seed)] = owner
	}

	return worldstate.NewCatalog(world, owners)
}

// TestConstantsAsTheGameRuns runs two shopkeepers on one workflow: each
// says the greeting as it has it, own or default, with its markup, logged
// without it; checks it; waits its serving time; and copies a page-long
// motto into a value, cut to 200 characters.
func TestConstantsAsTheGameRuns(t *testing.T) {
	t.Parallel()
	greeting := ref(worldstate.ScopeNPC, greetingID)
	serving := flow.Duration{
		Amount: 9, Unit: flow.UnitSeconds, Clock: flow.ClockGame, Constant: ref(worldstate.ScopeNPC, serveID),
	}
	shop := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindSay, map[string]any{flow.ParamMessage: []flow.Segment{{Text: "", Ref: &greeting}}}),
		node(t, 3, flow.KindCheckValue, map[string]any{flow.ParamValue: greeting,
			"against": json.RawMessage(`"Welcome [b]in[/b]!"`)}),
		node(t, 4, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, waitingID),
			"to": json.RawMessage("true")}),
		node(t, 5, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeWorld, noteID),
			"to": flow.RefOperand(ref(worldstate.ScopeWorld, mottoID))}),
		node(t, 6, flow.KindWait, map[string]any{"duration": serving}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputYes, 4),
		link(3, flow.OutputOtherwise, 5), link(4, flow.OutputNext, 5), link(5, flow.OutputNext, 6))
	town := newTown(t, worldstate.NewStore(constantsCatalog(t)))
	marta, sana := town.place(100, image.Pt(2, 2), shop), town.place(110, image.Pt(5, 5), shop)
	town.run(1)
	said, saying := marta.Saying()
	if !saying || said.Text != "Fresh off the boat!" {
		t.Fatalf("Marta says her own greeting: %+v", said)
	}
	if said, saying = sana.Saying(); !saying || said.Text != "Welcome [b]in[/b]!" {
		t.Fatalf("Sana says the default, its markup kept for the screen: %+v", said)
	}
	if len(sana.Lines) == 0 || !strings.HasSuffix(sana.Lines[0].Text, "says: Welcome in!") {
		t.Fatalf("the log shows it without its markup: %+v", sana.Lines)
	}
	if town.value(t, waitingID, marta.NPC).Bool || !town.value(t, waitingID, sana.NPC).Bool {
		t.Fatal("a check reads each NPC's own greeting: only the default matches")
	}
	if note := town.value(t, noteID, ids.ULID{}); len([]rune(note.Text)) != worldstate.MaxText {
		t.Fatalf("a page-long constant is cut to a value's 200 characters: %d", len([]rune(note.Text)))
	}
	loops := map[ids.ULID]int{}
	for range 30 {
		town.run(1)
		for _, each := range []*npcrun.NPC{marta, sana} {
			for _, line := range each.Lines {
				if strings.Contains(line.Text, "says:") {
					loops[each.NPC]++
				}
			}
		}
	}
	if loops[sana.NPC] <= 2*loops[marta.NPC] {
		t.Fatalf("Sana waits her one second a loop, Marta her own five: %d and %d loops", loops[sana.NPC],
			loops[marta.NPC])
	}
}
