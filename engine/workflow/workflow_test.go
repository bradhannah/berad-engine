package workflow_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func id(seed byte) ids.ULID {
	var made ids.ULID
	made[15] = seed

	return made
}

func node(t *testing.T, seed byte, kind string, parent ids.ULID) workflow.Node {
	t.Helper()
	made, err := workflow.NewNode(kind)
	if err != nil {
		t.Fatal(err)
	}
	made.ID, made.Parent = id(seed), parent

	return made
}

// routine is start → time switch; 09–17 → the Work container (go to, then
// wait, inside); the switch's otherwise → wait.
func routine(t *testing.T) *workflow.Workflow {
	t.Helper()
	flow, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	nodes := []workflow.Node{
		node(t, 2, workflow.KindTimeSwitch, ids.ULID{}),
		node(t, 3, workflow.KindContainer, ids.ULID{}),
		node(t, 4, workflow.KindGoTo, id(3)),
		node(t, 5, workflow.KindWait, id(3)),
		node(t, 6, workflow.KindWait, ids.ULID{}),
		node(t, 7, workflow.KindNote, ids.ULID{}),
	}
	links := []workflow.Link{
		{From: id(1), Output: workflow.OutputNext, To: id(2)},
		{From: id(2), Output: "r1", To: id(3)},
		{From: id(2), Output: workflow.OutputOtherwise, To: id(6)},
		{From: id(3), Output: workflow.OutputInside, To: id(4)},
		{From: id(4), Output: workflow.OutputNext, To: id(5)},
	}
	boxes := map[ids.ULID]workflow.Box{id(2): {X: 120, Y: 0, W: 0, H: 0}, id(3): {X: 300, Y: 0, W: 400, H: 200}}
	if err := flow.Insert(nodes, boxes, links); err != nil {
		t.Fatal(err)
	}

	return flow
}

func TestARoutineRoundTrips(t *testing.T) {
	t.Parallel()
	flow := routine(t)
	logic, err := flow.Encode()
	if err != nil {
		t.Fatal(err)
	}
	layout, err := flow.EncodeLayout()
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(logic) && string(logic) == string(layout) {
		t.Fatal("logic and layout are saved apart")
	}
	decoded, err := workflow.Decode(logic)
	if err != nil {
		t.Fatal(err)
	}
	if err := decoded.DecodeLayout(layout); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(flow) {
		t.Fatal("round trip changed the workflow")
	}
	if children := flow.Children(id(3)); !slices.Equal(children, []ids.ULID{id(4), id(5)}) {
		t.Fatalf("children %v", children)
	}
}

func TestRemovingAndPuttingBackRestoresExactly(t *testing.T) {
	t.Parallel()
	flow := routine(t)
	before := flow.Clone()
	removed, boxes, links, err := flow.Remove([]ids.ULID{id(3)})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 3 || len(links) != 1 || links[0] != (workflow.Link{From: id(2), Output: "r1", To: id(3)}) {
		t.Fatalf("a container takes its contents, and links to it are cleared: %v %v", removed, links)
	}
	if _, ok := flow.Node(id(4)); ok {
		t.Fatal("the container's contents went with it")
	}
	if err := flow.Insert(removed, boxes, links); err != nil {
		t.Fatal(err)
	}
	if !flow.Equal(before) {
		t.Fatal("putting back did not restore the workflow")
	}
	if _, _, _, err := flow.Remove([]ids.ULID{id(1)}); !errors.Is(err, workflow.ErrNode) {
		t.Fatal("the start node stays")
	}
}

func TestStructureIsKept(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*workflow.Workflow) error{
		"link out of a container": func(flow *workflow.Workflow) error {
			inner, _ := flow.Node(id(5))
			_, err := flow.Replace(inner.WithLink(workflow.OutputNext, id(6)))

			return err
		},
		"first step outside": func(flow *workflow.Workflow) error {
			container, _ := flow.Node(id(3))
			_, err := flow.Replace(container.WithLink(workflow.OutputInside, id(6)))

			return err
		},
		"link to a note": func(flow *workflow.Workflow) error {
			wait, _ := flow.Node(id(6))
			_, err := flow.Replace(wait.WithLink(workflow.OutputNext, id(7)))

			return err
		},
		"link back to the start": func(flow *workflow.Workflow) error {
			wait, _ := flow.Node(id(6))
			_, err := flow.Replace(wait.WithLink(workflow.OutputNext, id(1)))

			return err
		},
		"nested container": func(flow *workflow.Workflow) error {
			return flow.Insert([]workflow.Node{node(t, 9, workflow.KindContainer, id(3))}, nil, nil)
		},
		"second start": func(flow *workflow.Workflow) error {
			return flow.Insert([]workflow.Node{node(t, 9, workflow.KindStart, ids.ULID{})}, nil, nil)
		},
		"repeated ID": func(flow *workflow.Workflow) error {
			return flow.Insert([]workflow.Node{node(t, 6, workflow.KindWait, ids.ULID{})}, nil, nil)
		},
		"change of kind": func(flow *workflow.Workflow) error {
			changed := node(t, 6, workflow.KindGoTo, ids.ULID{})
			_, err := flow.Replace(changed)

			return err
		},
		"link to nowhere": func(flow *workflow.Workflow) error {
			wait, _ := flow.Node(id(6))
			_, err := flow.Replace(wait.WithLink(workflow.OutputNext, id(99)))

			return err
		},
	} {
		flow := routine(t)
		before := flow.Clone()
		if err := change(flow); !errors.Is(err, workflow.ErrNode) {
			t.Fatalf("%s: %v", name, err)
		}
		if !flow.Equal(before) {
			t.Fatalf("%s: a refused change left the workflow changed", name)
		}
	}
}

func TestMoveReturnsWhatItReplaced(t *testing.T) {
	t.Parallel()
	flow := routine(t)
	previous, err := flow.Move(map[ids.ULID]workflow.Box{id(2): {X: 5, Y: 5, W: 0, H: 0}, id(6): {X: 9, Y: 9, W: 0, H: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if previous[id(2)].X != 120 || previous[id(6)].W != workflow.Unplaced {
		t.Fatalf("previous %v", previous)
	}
	if _, err := flow.Move(previous); err != nil {
		t.Fatal(err)
	}
	if _, placed := flow.Box(id(6)); placed {
		t.Fatal("moving back removes a box the node never had")
	}
	if box, _ := flow.Box(id(2)); box.X != 120 {
		t.Fatalf("box %v", box)
	}
}

func TestOutputsFollowTimeRanges(t *testing.T) {
	t.Parallel()
	timeSwitch := node(t, 2, workflow.KindTimeSwitch, ids.ULID{})
	ranges := []workflow.TimeRange{{Port: "r1", From: 360, To: 540}, {Port: "r2", From: 1320, To: 360}}
	timeSwitch, err := timeSwitch.WithParam("ranges", ranges)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, output := range workflow.Outputs(timeSwitch) {
		names = append(names, output.Name+" "+output.Label)
	}
	want := []string{"r1 06:00–09:00", "r2 22:00–06:00", "otherwise otherwise"}
	if !slices.Equal(names, want) {
		t.Fatalf("outputs %v", names)
	}
	if !ranges[1].Contains(30) || ranges[1].Contains(600) || !ranges[0].Contains(360) || ranges[0].Contains(540) {
		t.Fatal("ranges include their start, exclude their end and may run past midnight")
	}
}

func TestParamsAreChecked(t *testing.T) {
	t.Parallel()
	for _, kind := range workflow.Kinds() {
		made, err := workflow.NewNode(kind.Name)
		if err != nil {
			t.Fatal(err)
		}
		if err := workflow.CheckParams(made); err != nil {
			t.Fatalf("%s defaults: %v", kind.Name, err)
		}
	}
	wait := node(t, 6, workflow.KindWait, ids.ULID{})
	with := func(base workflow.Node, name string, value any) workflow.Node {
		t.Helper()
		changed, err := base.WithParam(name, value)
		if err != nil {
			t.Fatal(err)
		}

		return changed
	}
	duration := func(amount int, unit, clock string) workflow.Duration {
		return workflow.Duration{Amount: amount, Unit: unit, Clock: clock}
	}
	random := node(t, 8, workflow.KindRandom, ids.ULID{})
	move := node(t, 9, workflow.KindMove, ids.ULID{})
	timeSwitch := node(t, 2, workflow.KindTimeSwitch, ids.ULID{})
	sound := node(t, 10, workflow.KindPlaySound, ids.ULID{})
	extra := wait.Clone()
	extra.Params["colour"] = json.RawMessage(`"red"`)
	share := func(port string, amount int) workflow.Share {
		return workflow.Share{Port: port, Label: port, Share: amount}
	}
	for name, candidate := range map[string]workflow.Node{
		"wait too long":     with(wait, "duration", duration(169, workflow.UnitHours, workflow.ClockGame)),
		"negative wait":     with(wait, "duration", duration(-1, workflow.UnitSeconds, workflow.ClockGame)),
		"unknown unit":      with(wait, "duration", duration(1, "days", workflow.ClockGame)),
		"unknown clock":     with(wait, "duration", duration(1, workflow.UnitSeconds, "sundial")),
		"no options":        with(random, "options", []workflow.Share{}),
		"no shares":         with(random, "options", []workflow.Share{share("a", 0), share("b", 0)}),
		"share over 100":    with(random, "options", []workflow.Share{share("a", 101)}),
		"shared option":     with(random, "options", []workflow.Share{share("a", 1), share("a", 1)}),
		"unknown direction": with(move, "direction", "north"),
		"no tiles":          with(move, "tiles", 0),
		"too many tiles":    with(move, "tiles", workflow.MaxTiles+1),
		"shared output": with(timeSwitch, "ranges",
			[]workflow.TimeRange{{Port: "a", From: 1, To: 2}, {Port: "a", From: 3, To: 4}}),
		"empty range":       with(timeSwitch, "ranges", []workflow.TimeRange{{Port: "a", From: 5, To: 5}}),
		"unknown parameter": extra,
		"cue not text":      with(sound, "cue", 7),
		"cue too long":      with(sound, "cue", strings.Repeat("c", workflow.MaxCue+1)),
	} {
		if err := workflow.CheckParams(candidate); !errors.Is(err, workflow.ErrNode) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := workflow.CheckParams(with(wait, "duration", duration(168, workflow.UnitHours, workflow.ClockReal))); err != nil {
		t.Fatalf("a week on a real clock is allowed: %v", err)
	}
	chosen := with(sound, "cue", "door/open")
	if cue, err := chosen.Cue("cue"); err != nil || cue != "door/open" || workflow.CheckParams(chosen) != nil {
		t.Fatalf("a Play sound keeps its cue's ID: %q %v", cue, err)
	}
	if cue, err := sound.Cue("cue"); err != nil || cue != "" {
		t.Fatalf("a new Play sound has no cue yet: %q %v", cue, err)
	}
}

// TestRandomChoiceOptions gives a random choice one output per option and
// works out each option's share as a percentage.
func TestRandomChoiceOptions(t *testing.T) {
	t.Parallel()
	options := []workflow.Share{
		{Port: "o1", Label: "up", Share: 1}, {Port: "o2", Label: "down", Share: 1}, {Port: "o3", Label: "stay", Share: 2},
	}
	random, err := node(t, 8, workflow.KindRandom, ids.ULID{}).WithParam("options", options)
	if err != nil {
		t.Fatal(err)
	}
	outputs := workflow.Outputs(random)
	if len(outputs) != 3 || outputs[0].Name != "o1" || outputs[2].Label != "stay" {
		t.Fatalf("outputs %+v", outputs)
	}
	if got := workflow.Percentages(options); got[0] != 25 || got[1] != 25 || got[2] != 50 {
		t.Fatalf("percentages %v", got)
	}
	if got := workflow.Percentages(options[:1]); got[0] != 100 {
		t.Fatalf("one option is taken every time: %v", got)
	}
	if seconds := (workflow.Duration{Amount: 2, Unit: workflow.UnitHours, Clock: workflow.ClockGame}).Seconds(); seconds != 7200 {
		t.Fatalf("two hours are %d seconds", seconds)
	}
}

// TestUpgradeFromVersion1 turns a chance into two options that keep their
// outputs and links, and minutes into a wait on the game clock.
func TestUpgradeFromVersion1(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	old := `{"start":"` + id(1).String() + `","nodes":[` +
		`{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` + id(2).String() + `"}},` +
		`{"id":"` + id(2).String() + `","kind":"random","params":{"chance":30},"next":{"yes":"` + id(3).String() +
		`","otherwise":"` + id(3).String() + `"}},` +
		`{"id":"` + id(3).String() + `","kind":"wait","params":{"minutes":45}}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 1, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(old),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	decoded, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	random, _ := decoded.Node(id(2))
	options, err := random.Shares("options")
	if err != nil || len(options) != 2 || options[0].Share != 30 || options[1].Share != 70 ||
		random.Next["yes"] != id(3) || random.Next["otherwise"] != id(3) {
		t.Fatalf("random choice %+v %+v %v", options, random.Next, err)
	}
	wait, _ := decoded.Node(id(3))
	duration, err := wait.Duration("duration")
	if err != nil || duration != (workflow.Duration{Amount: 45, Unit: workflow.UnitMinutes, Clock: workflow.ClockGame}) {
		t.Fatalf("wait %+v %v", duration, err)
	}
	for _, upgradedNode := range decoded.Nodes() {
		if err := workflow.CheckParams(upgradedNode); err != nil {
			t.Fatalf("an upgraded %s does not fit its kind: %v", upgradedNode.Kind, err)
		}
	}
}

// TestUpgradeFromVersion2 gives a Play sound its cue's own loudness, as it
// played before.
func TestUpgradeFromVersion2(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	old := `{"start":"` + id(1).String() + `","nodes":[` +
		`{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` + id(2).String() + `"}},` +
		`{"id":"` + id(2).String() + `","kind":"play-sound","params":{"cue":"door/knock"}}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 2, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(old),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	decoded, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	played, _ := decoded.Node(id(2))
	loudness, err := played.Choice("loudness")
	if err != nil || loudness != workflow.LoudnessAsCue {
		t.Fatalf("loudness %q %v", loudness, err)
	}
	if err := workflow.CheckParams(played); err != nil {
		t.Fatalf("an upgraded Play sound does not fit its kind: %v", err)
	}
}

func TestDecodeRefusesBrokenWorkflows(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"not json": `{`,
		"no start": `{"start":"` + id(1).String() + `","nodes":[]}`,
		"dangling": `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() +
			`","kind":"start","next":{"next":"` + id(2).String() + `"}}]}`,
	} {
		if _, err := workflow.Decode(json.RawMessage(data)); !errors.Is(err, workflow.ErrData) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestLayoutDropsBoxesOfMissingNodes(t *testing.T) {
	t.Parallel()
	flow := routine(t)
	layout := `{"boxes":{"` + id(2).String() + `":{"x":1,"y":2},"` + id(42).String() + `":{"x":3,"y":4}}}`
	if err := flow.DecodeLayout(json.RawMessage(layout)); err != nil {
		t.Fatal(err)
	}
	if _, ok := flow.Box(id(42)); ok {
		t.Fatal("a box for a node not in the workflow is dropped")
	}
	if box, ok := flow.Box(id(2)); !ok || box.Y != 2 {
		t.Fatalf("box %v %v", box, ok)
	}
}

// TestBends moves a line by hand: the bend is saved with the layout, stops
// applying when the line is relinked and is left out of the saved layout,
// applies again when the line leads back, and setting the previous bends
// undoes it.
func TestBends(t *testing.T) {
	t.Parallel()
	flow, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Insert([]workflow.Node{node(t, 2, workflow.KindWait, ids.ULID{}), node(t, 3, workflow.KindWait, ids.ULID{})},
		nil, []workflow.Link{{From: id(1), Output: workflow.OutputNext, To: id(2)}}); err != nil {
		t.Fatal(err)
	}
	key := workflow.LineKey(id(1), workflow.OutputNext)
	depth, end := -40, -30
	previous, err := flow.SetBends(map[string]workflow.Bend{key: {To: id(2), Lane: nil, Depth: &depth, End: &end}})
	if err != nil || !previous[key].To.IsZero() {
		t.Fatalf("set bends: %v %v", previous, err)
	}
	if bend, ok := flow.Bend(id(1), workflow.OutputNext); !ok || *bend.Depth != -40 || *bend.End != -30 {
		t.Fatalf("the bend applies: %+v %v", bend, ok)
	}
	layout, err := flow.EncodeLayout()
	if err != nil || !strings.Contains(string(layout), `"lines"`) {
		t.Fatalf("the layout holds the bend: %s %v", layout, err)
	}
	reread := flow.Clone()
	if err := reread.DecodeLayout(layout); err != nil || !reread.Equal(flow) {
		t.Fatalf("the bend round-trips: %v", err)
	}
	start, _ := flow.Node(id(1))
	if _, err := flow.Replace(start.WithLink(workflow.OutputNext, id(3))); err != nil {
		t.Fatal(err)
	}
	if _, ok := flow.Bend(id(1), workflow.OutputNext); ok {
		t.Fatal("a relinked line loses its bend")
	}
	if layout, _ := flow.EncodeLayout(); strings.Contains(string(layout), `"lines"`) {
		t.Fatalf("a bend that no longer applies is not saved: %s", layout)
	}
	if _, err := flow.Replace(start); err != nil {
		t.Fatal(err)
	}
	if _, ok := flow.Bend(id(1), workflow.OutputNext); !ok {
		t.Fatal("leading back again brings the bend back")
	}
	if _, err := flow.SetBends(previous); err != nil {
		t.Fatal(err)
	}
	if len(flow.Bends()) != 0 {
		t.Fatal("setting the previous bends undoes them")
	}
	if _, err := flow.SetBends(map[string]workflow.Bend{"nonsense": {To: id(2), Lane: nil, Depth: nil, End: nil}}); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a key that names no line is refused: %v", err)
	}
}

// TestComparisonsByType lists what each type can be compared with, keeps
// is for a choice checked before any of existed, and accepts a list and a
// reference as operands while refusing a list within a list.
func TestComparisonsByType(t *testing.T) {
	t.Parallel()
	if got := workflow.Comparisons(worldstate.TypeInteger); len(got) != 9 || got[0] != workflow.CompareIs ||
		got[6] != workflow.CompareBetween {
		t.Fatalf("an integer's comparisons: %v", got)
	}
	if got := workflow.Comparisons(worldstate.TypeBool); len(got) != 1 || got[0] != workflow.CompareIs {
		t.Fatalf("a bool only is: %v", got)
	}
	if got := workflow.Comparisons(worldstate.TypeChoice); len(got) != 2 || got[0] != workflow.CompareAnyOf {
		t.Fatalf("a choice is any of or none of: %v", got)
	}
	if got := workflow.Comparisons(worldstate.TypeText); len(got) != 2 || got[1] != workflow.CompareNot {
		t.Fatalf("text is or is not: %v", got)
	}
	if workflow.Comparisons(worldstate.TypeNPC) != nil || workflow.CanCompare(workflow.CompareIs, worldstate.TypeNPC) {
		t.Fatal("an NPC compares with nothing")
	}
	if !workflow.CanCompare(workflow.CompareIs, worldstate.TypeChoice) ||
		workflow.CanCompare(workflow.CompareLeast, worldstate.TypeBool) {
		t.Fatal("is still fits a choice; at least never fits a bool")
	}
	check, err := workflow.NewNode(workflow.KindCheckValue)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := check.WithParam(workflow.ParamAmong, []string{"happy", "calm"})
	if err != nil || workflow.CheckParams(listed) != nil {
		t.Fatalf("a list of choices is an operand: %v", err)
	}
	if items := workflow.Operands(mustOperand(t, listed, workflow.ParamAmong)); len(items) != 2 {
		t.Fatalf("the list reads back as items: %v", items)
	}
	nested, _ := check.WithParam(workflow.ParamAmong, [][]string{{"happy"}})
	if err := workflow.CheckParams(nested); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a list within a list is refused: %v", err)
	}
	referring, err := check.WithParam(workflow.ParamAgainst, workflow.RefOperand(workflow.ValueRef{
		Scope: worldstate.ScopeWorld, Value: id(7), Owner: ids.ULID{}, Found: ids.ULID{},
	}))
	if err != nil || workflow.CheckParams(referring) != nil {
		t.Fatalf("a reference is an operand: %v", err)
	}
	if ref, refers := workflow.OperandRef(mustOperand(t, referring, workflow.ParamAgainst)); !refers || ref.Value != id(7) {
		t.Fatalf("the reference reads back: %+v %v", ref, refers)
	}
	if _, refers := workflow.OperandRef(json.RawMessage(`3`)); refers {
		t.Fatal("a number is no reference")
	}
	wrong, _ := check.WithParam(workflow.ParamAgainst, map[string]int{"foo": 1})
	if err := workflow.CheckParams(wrong); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("an object that is no reference is refused: %v", err)
	}
	if len(workflow.Operands(workflow.NoOperand())) != 0 || len(workflow.Operands(json.RawMessage(`"calm"`))) != 1 {
		t.Fatal("no operand is no items and one value is one item")
	}
}

func mustOperand(t *testing.T, node workflow.Node, name string) json.RawMessage {
	t.Helper()
	raw, err := node.Operand(name)
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

// TestUpgradeFromVersion5 reads a version 5 Check value, from before
// comparisons by type, unchanged: its comparison and operand keep their
// meaning.
func TestUpgradeFromVersion5(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	five := `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` +
		id(2).String() + `"}},{"id":"` + id(2).String() + `","kind":"check-value","params":{"value":{"scope":"local","value":"` +
		id(20).String() + `"},"compare":"at-least","against":3}}],"locals":[{"id":"` + id(20).String() +
		`","name":"tally","scope":"local","type":"integer","default":0}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 5, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(five),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	read, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	checked, _ := read.Node(id(2))
	compare, err := checked.Choice(workflow.ParamCompare)
	if err != nil || compare != workflow.CompareLeast || string(mustOperand(t, checked, workflow.ParamAgainst)) != "3" {
		t.Fatalf("version 5 reads unchanged: %q %v", compare, err)
	}
}

// TestLayoutUpgradeFromVersion2 reads a layout from before a back line's
// drop and end legs could be moved: its bends read as they were.
func TestLayoutUpgradeFromVersion2(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.LayoutFormat()); err != nil {
		t.Fatal(err)
	}
	key := workflow.LineKey(id(1), workflow.OutputNext)
	old := `{"boxes":{},"lines":{"` + key + `":{"to":"` + id(2).String() + `","depth":-40}}}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.LayoutFormatName, Version: 2, ID: id(9), Name: "Old", Revision: 0, Data: json.RawMessage(old),
	})
	if err != nil || !changed || upgraded.Version != workflow.LayoutFormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	flow, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Insert([]workflow.Node{node(t, 2, workflow.KindWait, ids.ULID{})}, nil,
		[]workflow.Link{{From: id(1), Output: workflow.OutputNext, To: id(2)}}); err != nil {
		t.Fatal(err)
	}
	if err := flow.DecodeLayout(upgraded.Data); err != nil {
		t.Fatal(err)
	}
	bend, ok := flow.Bend(id(1), workflow.OutputNext)
	if !ok || bend.Depth == nil || *bend.Depth != -40 || bend.Lane != nil || bend.End != nil {
		t.Fatalf("the bend reads as it was: %+v %v", bend, ok)
	}
}

// TestTints colours a node, an outcome and a line: the tints are saved with
// the layout and round-trip, go when their node goes, are taken away by an
// empty name, and setting the previous tints undoes a change.
func TestTints(t *testing.T) {
	t.Parallel()
	flow, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Insert([]workflow.Node{node(t, 2, workflow.KindWait, ids.ULID{})}, nil,
		[]workflow.Link{{From: id(1), Output: workflow.OutputNext, To: id(2)}}); err != nil {
		t.Fatal(err)
	}
	if layout, _ := flow.EncodeLayout(); strings.Contains(string(layout), `"tints"`) {
		t.Fatalf("no tints are saved until one is set: %s", layout)
	}
	key := workflow.LineKey(id(1), workflow.OutputNext)
	previous, err := flow.SetTints(workflow.Tints{
		Nodes: map[ids.ULID]string{id(2): "green"}, Outcomes: map[string]string{key: "amber"},
		Lines: map[string]string{key: workflow.TintNone},
	})
	if err != nil || previous.Nodes[id(2)] != "" || previous.Outcomes[key] != "" || previous.Lines[key] != "" {
		t.Fatalf("set tints: %+v %v", previous, err)
	}
	tints := flow.Tints()
	if tints.Nodes[id(2)] != "green" || tints.Outcomes[key] != "amber" || tints.Lines[key] != workflow.TintNone {
		t.Fatalf("the tints apply: %+v", tints)
	}
	layout, err := flow.EncodeLayout()
	if err != nil || !strings.Contains(string(layout), `"tints"`) {
		t.Fatalf("the layout holds the tints: %s %v", layout, err)
	}
	reread := flow.Clone()
	if err := reread.DecodeLayout(layout); err != nil || !reread.Equal(flow) {
		t.Fatalf("the tints round-trip: %v", err)
	}
	if _, err := flow.SetTints(workflow.Tints{Nodes: map[ids.ULID]string{id(9): "green"}, Outcomes: nil, Lines: nil}); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a node that does not exist is refused: %v", err)
	}
	if _, err := flow.SetTints(workflow.Tints{Nodes: nil, Outcomes: nil, Lines: map[string]string{"nonsense": "green"}}); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a key that names no line is refused: %v", err)
	}
	if _, err := flow.SetTints(previous); err != nil {
		t.Fatal(err)
	}
	if !flow.Tints().Empty() {
		t.Fatalf("setting the previous tints undoes them: %+v", flow.Tints())
	}
	if _, err := flow.SetTints(workflow.Tints{Nodes: map[ids.ULID]string{id(2): "teal"}, Outcomes: nil, Lines: nil}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := flow.Remove([]ids.ULID{id(2)}); err != nil {
		t.Fatal(err)
	}
	if !flow.Tints().Empty() {
		t.Fatalf("a removed node's tint goes with it: %+v", flow.Tints())
	}
}

// TestLayoutUpgradeFromVersion1 reads a layout from before lines could be
// moved by hand.
func TestLayoutUpgradeFromVersion1(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.LayoutFormat()); err != nil {
		t.Fatal(err)
	}
	old := `{"boxes":{"` + id(2).String() + `":{"x":10,"y":20}}}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.LayoutFormatName, Version: 1, ID: id(9), Name: "Old", Revision: 0, Data: json.RawMessage(old),
	})
	if err != nil || !changed || upgraded.Version != workflow.LayoutFormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	flow := routine(t)
	if err := flow.DecodeLayout(upgraded.Data); err != nil {
		t.Fatal(err)
	}
	if box, ok := flow.Box(id(2)); !ok || box.Y != 20 || len(flow.Bends()) != 0 {
		t.Fatalf("box %v %v, bends %v", box, ok, flow.Bends())
	}
}

func TestLocalValues(t *testing.T) {
	t.Parallel()
	flowDoc := routine(t)
	data, err := flowDoc.Encode()
	if err != nil || strings.Contains(string(data), "locals") {
		t.Fatalf("a workflow with no local values saves none: %s %v", data, err)
	}
	customer := worldstate.New(id(20), "customer", worldstate.ScopeLocal, worldstate.TypeNPC)
	if err := flowDoc.Locals().Insert(customer); err != nil {
		t.Fatal(err)
	}
	world := worldstate.New(id(21), "robbed", worldstate.ScopeWorld, worldstate.TypeBool)
	if err := flowDoc.Locals().Insert(world); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a workflow declares only local values: %v", err)
	}
	copied := flowDoc.Clone()
	if !copied.Equal(flowDoc) {
		t.Fatal("a copy keeps the local values")
	}
	data, err = flowDoc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := workflow.Decode(data)
	if err != nil || !decoded.Locals().Equal(flowDoc.Locals()) {
		t.Fatalf("local values saved and read unchanged: %v", err)
	}
	if _, err := copied.Locals().Remove(id(20)); err != nil || copied.Equal(flowDoc) {
		t.Fatalf("a copy's local values are its own: %v", err)
	}
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	old := `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() + `","kind":"start"}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 3, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(old),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	if read, err := workflow.Decode(upgraded.Data); err != nil || read.Locals().Len() != 0 {
		t.Fatalf("a version 3 workflow has no local values: %v", err)
	}
	four := `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` +
		id(2).String() + `"}},{"id":"` + id(2).String() + `","kind":"check-value","params":{"value":{"scope":"npc","value":"` +
		id(5).String() + `","npc":"` + id(6).String() + `"}}}],"locals":[{"id":"` + id(20).String() +
		`","name":"tally","scope":"local","type":"number","default":0}]}`
	upgraded, _, err = registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 4, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(four),
	})
	if err != nil {
		t.Fatal(err)
	}
	read, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	tally, _ := read.Locals().Value(id(20))
	checked, _ := read.Node(id(2))
	ref, err := checked.Ref(workflow.ParamValue)
	if err != nil || tally.Type != worldstate.TypeInteger || ref.Owner != id(6) {
		t.Fatalf("version 4 upgrades to integer locals and owners: %+v %+v %v", tally, ref, err)
	}
}

func TestValueNodes(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{workflow.KindSetValue, workflow.KindCheckValue, workflow.KindFindNPC} {
		made, err := workflow.NewNode(kind)
		if err != nil || workflow.CheckParams(made) != nil {
			t.Fatalf("%s: a new node fits its kind: %v", kind, err)
		}
	}
	check, err := workflow.NewNode(workflow.KindCheckValue)
	if err != nil {
		t.Fatal(err)
	}
	if outputs := workflow.Outputs(check); len(outputs) != 2 || outputs[0].Name != workflow.OutputYes {
		t.Fatalf("check value outputs %+v", outputs)
	}
	for name, ref := range map[string]workflow.ValueRef{
		"another NPC's local": {Scope: worldstate.ScopeLocal, Value: id(5), Owner: id(6), Found: ids.ULID{}},
		"two NPCs at once":    {Scope: worldstate.ScopeNPC, Value: id(5), Owner: id(6), Found: id(7)},
		"a zone found nearby": {Scope: worldstate.ScopeZone, Value: id(5), Owner: ids.ULID{}, Found: id(7)},
		"no such scope":       {Scope: "party", Value: id(5), Owner: ids.ULID{}, Found: ids.ULID{}},
	} {
		bad, err := check.WithParam(workflow.ParamValue, ref)
		if err != nil {
			t.Fatal(err)
		}
		if err := workflow.CheckParams(bad); !errors.Is(err, workflow.ErrNode) {
			t.Errorf("%s is refused: %v", name, err)
		}
	}
	found := workflow.ValueRef{Scope: worldstate.ScopeNPC, Value: id(5), Owner: ids.ULID{}, Found: id(7)}
	market := workflow.ValueRef{Scope: worldstate.ScopeZone, Value: id(5), Owner: id(8), Found: ids.ULID{}}
	if found.Writable() || !found.Others() || market.Writable() {
		t.Fatal("a found NPC's value, and a named zone's, are read only")
	}
	if named, err := check.WithParam(workflow.ParamValue, market); err != nil || workflow.CheckParams(named) != nil {
		t.Fatalf("a named zone's value can be read: %v", err)
	}
	for _, operand := range []string{`null`, `12`, `true`, `"c2"`, `"Old Tom"`} {
		good, err := check.WithParam("against", json.RawMessage(operand))
		if err != nil || workflow.CheckParams(good) != nil {
			t.Errorf("operand %s fits: %v", operand, err)
		}
	}
	bad, err := check.WithParam("against", json.RawMessage(`{"x":1}`))
	if err != nil || !errors.Is(workflow.CheckParams(bad), workflow.ErrNode) {
		t.Fatalf("an object is not an operand: %v", err)
	}
}

// TestUpgradeFromVersion6 gives a Go to from before place values its
// anchor as where it goes, and no place value.
func TestUpgradeFromVersion6(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	six := `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` +
		id(2).String() + `"}},{"id":"` + id(2).String() + `","kind":"go-to","params":{"destination":{"floor":"` +
		id(30).String() + `","anchor":"` + id(31).String() + `"}}}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 6, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(six),
	})
	if err != nil || !changed {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	read, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	goTo, _ := read.Node(id(2))
	goes, err := goTo.Choice(workflow.ParamGoes)
	destination, anchorErr := goTo.Anchor("destination")
	place, refErr := goTo.Ref(workflow.ParamPlace)
	if err != nil || anchorErr != nil || refErr != nil || goes != workflow.GoesToMap || destination.Anchor != id(31) ||
		!place.IsZero() {
		t.Fatalf("a Go to keeps its anchor and has no place value: %q %v %v", goes, destination, place)
	}
}
