package workflowview_test

import (
	"encoding/json"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	workflowID = frametest.ID(1)
	startID    = frametest.ID(2)
	floorID    = frametest.ID(3)
	deskID     = frametest.ID(4)
)

type fixture struct {
	t        *testing.T
	state    *workflowview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
	registry *plugin.Registry
	applied  int
	nextID   byte
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	registry, err := plugin.NewRegistry(workflowplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	data, err := workflowplugin.Prepare(workflowplugin.NewData(), func() (ids.ULID, error) { return startID, nil })
	if err != nil {
		t.Fatal(err)
	}
	document, err := workflowplugin.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	office, err := tilefloor.NewDocument(10, 10, "floor")
	if err != nil {
		t.Fatal(err)
	}
	if err := office.Floor.InsertAnchor(0, floor.Anchor{ID: deskID, Name: "Office desk 3", Col: 4, Row: 5}); err != nil {
		t.Fatal(err)
	}
	docs := frametest.NewDocs()
	docs.Put(workflowID, "Daily routine", document)
	docs.Put(floorID, "Office", office)
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: workflowID, Name: "Daily routine", Format: flow.FormatName},
		{ID: floorID, Name: "Office", Format: floor.FormatName},
	}))
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := workflowview.New(frametest.ID(99), workflowID, theme)
	state.Start(ctx)
	fixture := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx, registry: registry, applied: 0,
		nextID: 10}
	fixture.frame(ui.NewInput(image.Pt(0, 0)))

	return fixture
}

func (f *fixture) workflow() *flow.Workflow {
	var found *flow.Workflow
	f.docs.View(workflowID, func(document client.Document) {
		found = document.Model.(*workflowplugin.Document).Workflow
	})

	return found
}

// frame runs one frame, then carries out every new command as the
// orchestrator would: prepared, applied to the document and answered.
func (f *fixture) frame(input ui.Input) {
	f.t.Helper()
	f.state.Update(&input, f.ctx)
	for f.applied < len(f.requests.Commands) {
		command := f.requests.Commands[f.applied]
		f.applied++
		f.carryOut(command)
	}
}

func (f *fixture) carryOut(command frametest.Command) {
	f.t.Helper()
	kind, ok := f.registry.Edit(command.Command.Edit)
	if !ok {
		f.t.Fatalf("unknown edit %s", command.Command.Edit)
	}
	args := command.Command.Args
	var created ids.ULID
	f.docs.View(workflowID, func(document client.Document) {
		if preparer, prepares := kind.(model.Preparer); prepares {
			prepared, err := preparer.Prepare(document.Model, args, func() (ids.ULID, error) {
				created = frametest.ID(f.nextID)
				f.nextID++

				return created, nil
			})
			if err != nil {
				f.t.Fatal(err)
			}
			args = prepared
		}
		if _, err := kind.Apply(document.Model, args); err != nil {
			command.Done(protocol.Result{OK: false, Reason: err.Error(), Revision: 0, Created: ids.ULID{}}, err)
			created = ids.ULID{1}
		}
	})
	f.docs.Touch(workflowID)
	if created != (ids.ULID{1}) {
		command.Succeed(created)
	}
}

// at finds a named part of the window after an idle frame, as a running
// window would have drawn one since the last change.
func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	f.frame(ui.NewInput(image.Pt(0, 0)))
	point, ok := f.state.Locate(name, f.docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.docs)

	return ok
}

func (f *fixture) click(name string) {
	f.t.Helper()
	at := f.at(name)
	f.press(at)
	f.release(at)
}

func (f *fixture) press(at image.Point) {
	f.t.Helper()
	input := ui.NewInput(at)
	input.Pressed, input.Down = true, true
	f.frame(input)
}

func (f *fixture) hold(at image.Point) {
	f.t.Helper()
	input := ui.NewInput(at)
	input.Down = true
	f.frame(input)
}

func (f *fixture) release(at image.Point) {
	f.t.Helper()
	input := ui.NewInput(at)
	input.Released = true
	f.frame(input)
}

func (f *fixture) drag(from, to image.Point) {
	f.t.Helper()
	f.press(from)
	f.hold(from.Add(to).Div(2))
	f.hold(to)
	f.release(to)
}

func (f *fixture) keys(names ...string) {
	f.t.Helper()
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) typeInto(field, text string) {
	f.t.Helper()
	f.click("ui:" + field)
	f.keys("Cmd+A")
	input := ui.NewInput(f.at("ui:" + field))
	input.Text = text
	f.frame(input)
	f.keys("Enter")
}

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

func (f *fixture) nth(kind string, number int) flow.Node {
	f.t.Helper()
	for _, node := range f.workflow().Nodes() {
		if node.Kind == kind {
			number--
			if number == 0 {
				return node
			}
		}
	}
	f.t.Fatalf("no %s number %d", kind, number)

	return flow.Node{}
}

// A palette click adds a node after the selected one, connected from its
// first free output, as one undo step; the new node is selected, so clicks
// build a sequence.
func TestPaletteClicksBuildASequence(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.click("palette:" + flow.KindWait)
	start, goTo, wait := f.nth(flow.KindStart, 1), f.nth(flow.KindGoTo, 1), f.nth(flow.KindWait, 1)
	if start.Next[flow.OutputNext] != goTo.ID || goTo.Next[flow.OutputNext] != wait.ID {
		t.Fatalf("start → go to → wait expected: %+v %+v", start.Next, goTo.Next)
	}
	if !f.has("selected:" + flow.KindWait) {
		t.Fatal("the newest node is selected")
	}
	if !slices.Contains(f.requests.Subscribed, project.IndexID()) {
		t.Fatal("the window subscribes to the project index, whose ID is zero, for names")
	}
	gestures := map[string]int{}
	for _, command := range f.requests.Commands {
		gestures[command.Command.Gesture]++
	}
	if len(gestures) != 2 {
		t.Fatalf("each added node and its link are one undo step: %v", gestures)
	}
	goToBox, _ := f.workflow().Box(goTo.ID)
	waitBox, _ := f.workflow().Box(wait.ID)
	if waitBox.X <= goToBox.X {
		t.Fatal("each node goes to the right of the one before")
	}
}

// Dragging from an output's port to a node connects them; dropping on empty
// canvas disconnects; links back to the start are refused with a reason.
func TestPortsConnectAndDisconnect(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:400,300"))
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:400,420"))
	f.drag(f.at("port:start:1:next"), f.at("node:wait:1"))
	if f.nth(flow.KindStart, 1).Next[flow.OutputNext] != f.nth(flow.KindWait, 1).ID {
		t.Fatal("start → wait 1 expected")
	}
	f.drag(f.at("port:wait:1:next"), f.at("node:start:1"))
	if !strings.Contains(f.status(), "back to the start") {
		t.Fatalf("links to the start are refused: %q", f.status())
	}
	f.drag(f.at("port:start:1:next"), f.at("node:wait:2"))
	if f.nth(flow.KindStart, 1).Next[flow.OutputNext] != f.nth(flow.KindWait, 2).ID {
		t.Fatal("dragging again moves the link")
	}
	f.drag(f.at("port:start:1:next"), f.at("canvas:900,40"))
	if len(f.nth(flow.KindStart, 1).Next) != 0 {
		t.Fatal("dropping on empty canvas disconnects")
	}
}

// Dragging a node moves it, as one undo step; a container's contents move
// with it.
func TestDraggingMovesNodes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.drag(f.at("palette:"+flow.KindContainer), f.at("canvas:300,200"))
	f.click("palette:" + flow.KindWait)
	container, inner := f.nth(flow.KindContainer, 1), f.nth(flow.KindWait, 1)
	if inner.Parent != container.ID || container.Next[flow.OutputInside] != inner.ID {
		t.Fatal("a palette click with a container selected adds its first step inside it")
	}
	before, _ := f.workflow().Box(inner.ID)
	moves := len(f.requests.Of(workflowplugin.MoveEdit))
	from := f.at("node:container:1")
	f.drag(from, from.Add(image.Pt(60, 40)))
	after, _ := f.workflow().Box(inner.ID)
	if after.X != before.X+60 || after.Y != before.Y+40 {
		t.Fatalf("the contents moved with the container: %v → %v", before, after)
	}
	gestures := map[string]bool{}
	for _, command := range f.requests.Of(workflowplugin.MoveEdit)[moves:] {
		gestures[command.Command.Gesture] = true
	}
	if len(gestures) != 1 {
		t.Fatal("a drag is one undo step")
	}
}

// The inspector is built from the selected node's declarations; Enter saves
// a field and a wrong value is refused with a reason.
func TestInspectorEditsParameters(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	f.typeInto("param-duration", "45")
	if wait, _ := f.nth(flow.KindWait, 1).Duration("duration"); wait.Amount != 45 || wait.Unit != flow.UnitMinutes {
		t.Fatalf("wait %+v", wait)
	}
	f.click("ui:duration-seconds")
	f.click("ui:duration-real")
	if wait, _ := f.nth(flow.KindWait, 1).Duration("duration"); wait.Unit != flow.UnitSeconds || wait.Clock != flow.ClockReal {
		t.Fatalf("unit and clock %+v", wait)
	}
	replaces := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeInto("param-duration", "lots")
	if len(f.requests.Of(workflowplugin.ReplaceEdit)) != replaces || !strings.Contains(f.status(), "whole number") {
		t.Fatalf("a wrong value is refused: %q", f.status())
	}
	f.keys("Escape")
	f.click("node:start:1")
	f.click("palette:" + flow.KindTimeSwitch)
	f.click("ui:add-time")
	f.typeInto("ranges-2-from", "17:30")
	spans, _ := f.nth(flow.KindTimeSwitch, 1).TimeRanges("ranges")
	if len(spans) != 2 || spans[1].From != 17*60+30 || spans[1].Port == spans[0].Port {
		t.Fatalf("times %+v", spans)
	}
	f.click("palette:" + flow.KindWait)
	timeSwitch := f.nth(flow.KindTimeSwitch, 1)
	if timeSwitch.Next[spans[0].Port] == (ids.ULID{}) {
		t.Fatal("a click adds after the first free output")
	}
	f.click("node:time-switch:1")
	f.click("ui:ranges-1-remove")
	timeSwitch = f.nth(flow.KindTimeSwitch, 1)
	if remaining, _ := timeSwitch.TimeRanges("ranges"); len(remaining) != 1 || timeSwitch.Next[spans[0].Port] != (ids.ULID{}) {
		t.Fatalf("removing a time removes its output: %+v %+v", remaining, timeSwitch.Next)
	}
}

// Go to's destination is picked on a map (D13).
func TestDestinationIsPickedOnAMap(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.click("ui:pick")
	if len(f.requests.PickAsks) != 1 {
		t.Fatal("Pick on map asks which maps could answer")
	}
	f.requests.PickAsks[0]([]protocol.PickMap{{Window: frametest.ID(50), Zone: frametest.ID(51), Shows: true}}, nil)
	if len(f.requests.PickStarts) != 1 || !f.has("picking") {
		t.Fatal("with one map open, the pick starts there")
	}
	start := f.requests.PickStarts[0]
	f.state.Changed(client.Update{PickAnswer: &protocol.PickAnswer{Pick: start.Pick, Floor: floorID, Item: deskID.String()}}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	destination, _ := f.nth(flow.KindGoTo, 1).Anchor("destination")
	if destination.Floor != floorID || destination.Anchor != deskID {
		t.Fatalf("destination %+v", destination)
	}
}

// Delete removes the selected node; the start node stays. Auto-arrange
// places every node, as one undo step.
func TestDeleteAndArrange(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	f.click("palette:" + flow.KindWait)
	f.keys("Backspace")
	if !f.has("nodes:2") {
		t.Fatal("Backspace removes the selected node")
	}
	f.click("node:start:1")
	f.keys("Delete")
	if !f.has("nodes:2") {
		t.Fatal("the start node stays")
	}
	f.click("ui:arrange")
	moves := f.requests.Of(workflowplugin.MoveEdit)
	var arranged workflowplugin.MoveArgs
	if err := json.Unmarshal(moves[len(moves)-1].Command.Args, &arranged); err != nil {
		t.Fatal(err)
	}
	if len(arranged.Boxes) != 2 || arranged.Boxes[startID].X >= arranged.Boxes[f.nth(flow.KindWait, 1).ID].X {
		t.Fatalf("arranged %+v", arranged.Boxes)
	}
}

// TestRandomWalk builds the test routine "move up or down at random, wait
// two seconds, repeat": a random choice with two named options, a Move for
// each, both leading into one Wait, which then has an entry dot per link.
// "+ option" on the node adds a third option with an equal share.
func TestRandomWalk(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindRandom)
	f.typeInto("options-1-label", "up")
	f.typeInto("options-2-label", "down")
	f.click("palette:" + flow.KindMove)
	f.click("node:random:1")
	f.click("palette:" + flow.KindMove)
	f.click("ui:direction-down")
	f.click("node:move:1")
	f.click("palette:" + flow.KindWait)
	f.typeInto("param-duration", "2")
	f.click("ui:duration-seconds")
	if !f.has("entries:wait:1=1") {
		t.Fatal("the wait has one link in")
	}
	f.drag(f.at("port:move:2:next"), f.at("entry:wait:1"))
	if !f.has("entries:wait:1=2") {
		t.Fatal("a second link into the wait gets its own entry dot")
	}
	random, wait := f.nth(flow.KindRandom, 1), f.nth(flow.KindWait, 1)
	up, down := f.nth(flow.KindMove, 1), f.nth(flow.KindMove, 2)
	options, _ := random.Shares("options")
	direction, _ := down.Choice("direction")
	duration, _ := wait.Duration("duration")
	if len(options) != 2 || options[0].Label != "up" || options[1].Label != "down" ||
		random.Next[options[0].Port] != up.ID || random.Next[options[1].Port] != down.ID ||
		up.Next[flow.OutputNext] != wait.ID || down.Next[flow.OutputNext] != wait.ID || direction != flow.DirectionDown ||
		duration != (flow.Duration{Amount: 2, Unit: flow.UnitSeconds, Clock: flow.ClockGame}) {
		t.Fatalf("the routine is not built as asked: %+v", f.workflow().Nodes())
	}
	f.click("add-option:random:1")
	options, _ = f.nth(flow.KindRandom, 1).Shares("options")
	if len(options) != 3 || options[2].Share != options[1].Share || options[2].Label != "option 3" {
		t.Fatalf("+ option adds an equal option: %+v", options)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
}

// TestLinesCanBeSelectedAndDeleted lights a node's lines while it is
// selected, selects a line by clicking it and removes it with Backspace.
func TestLinesCanBeSelectedAndDeleted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	if !f.has("lit:start:1:next") {
		t.Fatal("the selected node's lines are lit")
	}
	f.click("line:start:1:next")
	if !f.has("line-selected") || !f.has("lit:start:1:next") {
		t.Fatal("clicking a line selects it")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.keys("Backspace")
	if len(f.nth(flow.KindStart, 1).Next) != 0 || f.has("line-selected") || !strings.Contains(f.status(), "Line removed") {
		t.Fatalf("Backspace removes the selected line: %+v %q", f.nth(flow.KindStart, 1).Next, f.status())
	}
	if len(f.nth(flow.KindWait, 1).Next) != 0 {
		t.Fatal("only the selected line goes")
	}
}

// TestProblemMarks marks a Go to with no destination and clears the mark
// once it has one; a node nothing links to is marked too, and so is a
// destination whose anchor was deleted.
func TestProblemMarks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	if !f.has("problem:go-to:1") || !f.has("problems:1") {
		t.Fatal("a Go to with no destination is marked")
	}
	f.click("ui:pick")
	f.requests.PickAsks[0]([]protocol.PickMap{{Window: frametest.ID(50), Zone: frametest.ID(51), Shows: true}}, nil)
	pick := f.requests.PickStarts[0]
	f.state.Changed(client.Update{PickAnswer: &protocol.PickAnswer{Pick: pick.Pick, Floor: floorID, Item: deskID.String()}}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("problems:0") {
		t.Fatal("a destination clears the mark")
	}
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:500,500"))
	if !f.has("problem:wait:1") {
		t.Fatal("a node nothing links to is marked")
	}
	office, err := tilefloor.NewDocument(10, 10, "floor")
	if err != nil {
		t.Fatal(err)
	}
	f.docs.Put(floorID, "Office", office)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("problem:go-to:1") {
		t.Fatal("a deleted destination anchor is marked")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
}

// TestLayoutIsNotRedoneWhenIdle lays the workflow out only when it changes.
func TestLayoutIsNotRedoneWhenIdle(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	laidOut := f.state.LayoutCount()
	for range 5 {
		f.frame(ui.NewInput(image.Pt(0, 0)))
	}
	if f.state.LayoutCount() != laidOut {
		t.Fatalf("laid out %d times while idle", f.state.LayoutCount()-laidOut)
	}
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.state.LayoutCount() == laidOut {
		t.Fatal("a change lays the workflow out again")
	}
}

// TestPickingUpALineByItsArrowEnd moves a line to another node by its entry
// dot, without moving the node, leaves it alone when dropped back where it
// was, refuses the start node, and removes it when dropped on empty canvas.
func TestPickingUpALineByItsArrowEnd(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:520,420"))
	first, second := f.nth(flow.KindWait, 1), f.nth(flow.KindWait, 2)
	firstBox, _ := f.workflow().Box(first.ID)
	from, onto := f.at("entry:wait:1:1"), f.at("node:wait:2")
	f.press(from)
	f.hold(onto)
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.release(onto)
	if f.nth(flow.KindStart, 1).Next[flow.OutputNext] != second.ID {
		t.Fatalf("the line moves to the second wait: %+v", f.nth(flow.KindStart, 1).Next)
	}
	if box, _ := f.workflow().Box(first.ID); box != firstBox {
		t.Fatal("picking up a line does not drag the node")
	}
	sent := len(f.requests.Commands)
	f.drag(f.at("entry:wait:2:1"), f.at("node:wait:2"))
	if len(f.requests.Commands) != sent {
		t.Fatal("dropping a line back where it was changes nothing")
	}
	f.drag(f.at("entry:wait:2:1"), f.at("node:start:1"))
	if f.nth(flow.KindStart, 1).Next[flow.OutputNext] != second.ID || !strings.Contains(f.status(), "back to the start") {
		t.Fatalf("a line cannot go into the start: %q", f.status())
	}
	f.drag(f.at("entry:wait:2:1"), f.at("canvas:900,40"))
	if len(f.nth(flow.KindStart, 1).Next) != 0 {
		t.Fatal("dropping a line on empty canvas removes it")
	}
}

// TestDroppingOnALineAddsASeparateNode drops a Wait from the palette on the
// line from the start to a Move: the Wait is added on its own and the line
// stays as it was.
func TestDroppingOnALineAddsASeparateNode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindMove)
	f.drag(f.at("palette:"+flow.KindWait), f.at("line:start:1:next"))
	wait, move := f.nth(flow.KindWait, 1), f.nth(flow.KindMove, 1)
	if f.nth(flow.KindStart, 1).Next[flow.OutputNext] != move.ID || len(wait.Next) != 0 {
		t.Fatalf("the line is unchanged and the wait stands alone: %+v", f.workflow().Nodes())
	}
}

// nodeInset is how far inside a node's top-left corner "node:<kind>:<n>"
// points.
const nodeInset = 18

// loopFixture builds start → random choice → Wait, with a line from the Wait
// back to the random choice, and auto-arranges it.
func loopFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindRandom)
	f.click("palette:" + flow.KindWait)
	f.drag(f.at("port:wait:1:next"), f.at("node:random:1"))
	f.click("ui:arrange")

	return f
}

// lastLines returns the line bends the last move edit sent.
func (f *fixture) lastLines() map[string]flow.Bend {
	f.t.Helper()
	moves := f.requests.Of(workflowplugin.MoveEdit)
	if len(moves) == 0 {
		f.t.Fatal("no move sent")
	}
	var args workflowplugin.MoveArgs
	if err := moves[len(moves)-1].Decode(&args); err != nil {
		f.t.Fatal(err)
	}

	return args.Lines
}

// TestMovingALineByHand drags the loop's run up above the nodes, moves the
// Wait and sees the run follow, then resets the line, bends it again and
// auto-arranges, which resets it too.
func TestMovingALineByHand(t *testing.T) {
	t.Parallel()
	f := loopFixture(t)
	key := flow.LineKey(f.nth(flow.KindWait, 1).ID, flow.OutputNext)
	grip := f.at("bend:wait:1:next")
	top := f.at("node:random:1").Y - nodeInset
	f.drag(grip, image.Pt(grip.X, top-30))
	if !f.has("bent:wait:1:next") || !f.has("line-selected") {
		t.Fatal("dragging the middle bends the line and selects it")
	}
	if bend := f.lastLines()[key]; bend.Depth == nil || *bend.Depth >= 0 {
		t.Fatalf("the run moves above the wait's output: %+v", bend)
	}
	run := f.at("bend:wait:1:next")
	if run.Y >= top {
		t.Fatalf("the loop now runs above the nodes: %d, nodes start at %d", run.Y, top)
	}
	wait := f.at("node:wait:1")
	f.drag(wait, wait.Add(image.Pt(0, 40)))
	if moved := f.at("bend:wait:1:next"); moved.Y != run.Y+40 {
		t.Fatalf("the run follows its node: %d, was %d", moved.Y, run.Y)
	}
	f.click("bend:wait:1:next")
	f.click("ui:reset-line")
	if f.has("bent:wait:1:next") {
		t.Fatal("Reset line puts the line back")
	}
	grip = f.at("bend:wait:1:next")
	f.drag(grip, image.Pt(grip.X, top-30))
	f.click("ui:arrange")
	if f.has("bent:wait:1:next") {
		t.Fatal("auto-arrange resets lines moved by hand")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
}

// TestMovingAForwardLineSideways drags the upright part of a line going
// forward.
func TestMovingAForwardLineSideways(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindRandom)
	f.click("palette:" + flow.KindWait)
	f.click("node:random:1")
	f.click("palette:" + flow.KindWait)
	f.click("ui:arrange")
	grip := f.at("bend:random:1:o2")
	f.drag(grip, grip.Add(image.Pt(20, 0)))
	if !f.has("bent:random:1:o2") {
		t.Fatal("dragging the upright part bends the line")
	}
	if moved := f.at("bend:random:1:o2"); moved.X != grip.X+20 {
		t.Fatalf("the upright part follows the pointer: %d, was %d", moved.X, grip.X)
	}
}

// TestLinesRunOverNotes puts a note on top of a line: clicking the line
// there selects the line, not the note.
func TestLinesRunOverNotes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	f.drag(f.at("palette:"+flow.KindNote), f.at("line:start:1:next"))
	f.click("line:start:1:next")
	if !f.has("line-selected") || f.has("selected:note") {
		t.Fatal("a line over a note is clicked, not the note")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
}
