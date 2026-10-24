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
	} {
		if err := workflow.CheckParams(candidate); !errors.Is(err, workflow.ErrNode) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := workflow.CheckParams(with(wait, "duration", duration(168, workflow.UnitHours, workflow.ClockReal))); err != nil {
		t.Fatalf("a week on a real clock is allowed: %v", err)
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
	depth := -40
	previous, err := flow.SetBends(map[string]workflow.Bend{key: {To: id(2), Lane: nil, Depth: &depth}})
	if err != nil || !previous[key].To.IsZero() {
		t.Fatalf("set bends: %v %v", previous, err)
	}
	if bend, ok := flow.Bend(id(1), workflow.OutputNext); !ok || *bend.Depth != -40 {
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
	if _, err := flow.SetBends(map[string]workflow.Bend{"nonsense": {To: id(2), Lane: nil, Depth: nil}}); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a key that names no line is refused: %v", err)
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
