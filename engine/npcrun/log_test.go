package npcrun_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// textPart and valuePart are parts of a Log's message.
func textPart(text string) flow.Segment { return flow.Segment{Text: text, Ref: nil} }

func valuePart(ref flow.ValueRef) flow.Segment { return flow.Segment{Text: "", Ref: &ref} }

// loggingTown is a town whose one NPC, Ada, lives in the Office, follows
// Errands and looks like Ada walking.
func loggingTown(t *testing.T) *town {
	t.Helper()
	owners := map[ids.ULID]worldstate.Owner{
		id(101): {
			Scope: worldstate.ScopeNPC, Path: "ada", Name: "Ada", Owned: worldstate.NewOwned(worldstate.ScopeNPC),
			Zone: id(zoneID), Behaviour: id(90), Sprite: id(91),
		},
		id(zoneID): {
			Scope: worldstate.ScopeZone, Path: "office", Name: "Office", Owned: worldstate.NewOwned(worldstate.ScopeZone),
			Zone: ids.ULID{}, Behaviour: ids.ULID{}, Sprite: ids.ULID{},
		},
	}
	made := newTown(t, worldstate.NewStore(worldstate.NewCatalog(
		worldstate.NewDeclarations(worldstate.HoldsWorldState), owners)))
	made.names, made.behaviour = names{id(zoneID): "Office", id(90): "Errands", id(91): "Ada walking"}, id(90)

	return made
}

// TestLogLines writes a line holding a value of each type, each as its text
// form, a value it can't read as such, and names the node and workflow that
// wrote it.
func TestLogLines(t *testing.T) {
	t.Parallel()
	message := []flow.Segment{
		valuePart(builtInRef(worldstate.NameID())), textPart(" in "), valuePart(builtInRef(worldstate.ZoneID())),
		textPart(" follows "), valuePart(builtInRef(worldstate.BehaviourID())), textPart(" as "),
		valuePart(builtInRef(worldstate.SpriteID())), textPart(": "), valuePart(ref(worldstate.ScopeLocal, 80)),
		textPart(" "), valuePart(ref(worldstate.ScopeLocal, 81)), textPart(" "),
		valuePart(ref(worldstate.ScopeLocal, 82)), textPart(" "), valuePart(ref(worldstate.ScopeLocal, 83)),
		textPart(" "), valuePart(ref(worldstate.ScopeLocal, 84)),
	}
	nodes := []flow.Node{
		node(t, 2, flow.KindLog, map[string]any{flow.ParamMessage: message}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}
	workflow := workflowOf(t, nodes, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	withLocal(t, workflow, 80, "count", worldstate.TypeInteger)
	withLocal(t, workflow, 81, "done", worldstate.TypeBool)
	withLocal(t, workflow, 82, "spot", worldstate.TypePlace)
	withLocal(t, workflow, 83, "other", worldstate.TypeNPC)
	town := loggingTown(t)
	ada := town.place(100, image.Pt(2, 2), workflow)
	town.run(1)
	want := "Ada in Office follows Errands as Ada walking: 0 false no place none " + npcrun.Unreadable
	if len(ada.Lines) != 1 || ada.Lines[0].Text != want {
		t.Fatalf("a Log writes its line with each value as text: %+v", ada.Lines)
	}
	if ada.Lines[0].Node != id(2) || ada.Lines[0].Workflow != id(90) {
		t.Fatalf("a line names the node and workflow that wrote it: %+v", ada.Lines[0])
	}
	town.run(1)
	if len(ada.Lines) != 0 {
		t.Fatalf("lines last until the next step: %+v", ada.Lines)
	}
}

// TestLongAndLoopingLines cuts a line at MaxLine characters, and writes no
// lines for an NPC looping without waiting.
func TestLongAndLoopingLines(t *testing.T) {
	t.Parallel()
	long := []flow.Segment{
		textPart(strings.Repeat("a", flow.MaxMessageText)), valuePart(builtInRef(worldstate.NameID())),
	}
	once := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindLog, map[string]any{flow.ParamMessage: long}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	looping := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindLog, map[string]any{flow.ParamMessage: []flow.Segment{textPart("again")}}),
	}, link(1, flow.OutputNext, 2))
	town := loggingTown(t)
	ada := town.place(100, image.Pt(2, 2), once)
	loops := town.place(110, image.Pt(4, 2), looping)
	town.run(1)
	if len(ada.Lines) != 1 || len([]rune(ada.Lines[0].Text)) != npcrun.MaxLine {
		t.Fatalf("a long line is cut at MaxLine: %+v", ada.Lines)
	}
	if loops.Status != npcrun.StatusLooping || len(loops.Lines) != 0 {
		t.Fatalf("an NPC looping without waiting writes nothing: %q %+v", loops.Status, loops.Lines)
	}
}
