package main

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The clerk's day, the sample's behaviour workflow: it shows each kind of
// node at work, with Fetch stock, a workflow it calls, for a Return. A time
// switch sends the clerk to the shop from 08:00 to 18:00 and to the door
// otherwise. Shop hours are a time-bound container: at the desk the clerk
// rings the chime and looks for a customer waiting at the counter, serving
// one for 20 game minutes, then mostly calls Fetch stock to fetch from the
// shelf, tidying it when there is stock, and now and then stretches. At
// night the clerk stops serving, if closing came mid-serve, and waits by
// the door an hour at a time. Whenever a customer rings the bell, the clerk
// says Just a moment! and goes back to what she was doing. A note sums it up
// under the rest. It is placeholder behaviour for trying the editor, not
// game content.
const (
	routineName  = "Clerk's day"
	fetchName    = "Fetch stock"
	stockShare   = 3
	noneShare    = 1
	openMinute   = 8 * 60
	closeMinute  = 18 * 60
	serveMinutes = 20
	tidyMinutes  = 10
	nightHours   = 1
	stretchCells = 2
	tidyShare    = 3
	stretchShare = 1
	noteHeight   = 96
	chimeCue     = "sample/chime"
	// paramDestination is a Go to's destination.
	paramDestination = "destination"
	routineNote      = "The clerk's day: the shop from 08:00 to 18:00, serving whoever waits at the counter, " +
		"mostly fetching stock and tidying, sometimes stretching; the door at night."
	// Fetch stock's ways out.
	portFetched = "w1"
	portNone    = "w2"
)

// sampleAnchors are the anchors the clerk's day goes to, on the ground floor.
type sampleAnchors struct {
	floor, desk, shelf, door, counter ids.ULID
}

// routineBuilder makes the clerk's day's nodes with new IDs.
type routineBuilder struct {
	orch   *orchestrator.Orchestrator
	nodes  []flow.Node
	links  []flow.Link
	locals []worldstate.Declaration
	err    error
}

// local declares a local value and returns its ID.
func (b *routineBuilder) local(name string, kind worldstate.Type) ids.ULID {
	if b.err != nil {
		return ids.ULID{}
	}
	made, err := b.orch.NewID()
	if err != nil {
		b.err = fmt.Errorf("sample routine: %w", err)

		return ids.ULID{}
	}
	b.locals = append(b.locals, worldstate.New(made, name, worldstate.ScopeLocal, kind))

	return made
}

// node adds a node of a kind inside a container, zero for none, with
// parameters set, and returns its ID.
func (b *routineBuilder) node(kind string, parent ids.ULID, params map[string]any) ids.ULID {
	if b.err != nil {
		return ids.ULID{}
	}
	made, err := flow.NewNode(kind)
	if err == nil {
		made.ID, err = b.orch.NewID()
	}
	for name, value := range params {
		if err == nil {
			made, err = made.WithParam(name, value)
		}
	}
	if err != nil {
		b.err = fmt.Errorf("sample routine: %w", err)

		return ids.ULID{}
	}
	made.Parent = parent
	b.nodes = append(b.nodes, made)

	return made.ID
}

// link joins an output of one node to another.
func (b *routineBuilder) link(from ids.ULID, output string, to ids.ULID) {
	b.links = append(b.links, flow.Link{From: from, Output: output, To: to})
}

// goTo is a Go to an anchor of the ground floor.
func (b *routineBuilder) goTo(parent ids.ULID, at sampleAnchors, anchor ids.ULID) ids.ULID {
	return b.node(flow.KindGoTo, parent, map[string]any{
		paramDestination: floor.AnchorRef{Floor: at.floor, Anchor: anchor},
	})
}

// wait is a Wait on the game clock.
func (b *routineBuilder) wait(parent ids.ULID, amount int, unit string) ids.ULID {
	return b.node(flow.KindWait, parent, map[string]any{
		"duration": flow.Duration{Amount: amount, Unit: unit, Clock: flow.ClockGame, Constant: flow.NoConstant()},
	})
}

// serveWait waits as long as serving takes: the shop's constant, its
// minutes typed as well.
func (b *routineBuilder) serveWait(parent ids.ULID, values shopValues) ids.ULID {
	return b.node(flow.KindWait, parent, map[string]any{
		"duration": flow.Duration{
			Amount: serveMinutes, Unit: flow.UnitMinutes, Clock: flow.ClockGame, Constant: worldValue(values.serveMinutes),
		},
	})
}

// sayValue says what a value or a constant holds.
func (b *routineBuilder) sayValue(parent ids.ULID, ref flow.ValueRef) ids.ULID {
	return b.node(flow.KindSay, parent, map[string]any{flow.ParamMessage: []flow.Segment{{Text: "", Ref: &ref}}})
}

// serving sets whether the clerk is serving.
func (b *routineBuilder) serving(parent ids.ULID, values shopValues, serving string) ids.ULID {
	return b.node(flow.KindSetValue, parent, setTo(npcValue(values.serving), serving))
}

// shutNotice is the box the clerk shows on shutting the shop.
const shutNotice = "The shop is shut for the night."

// show adds a Say or a Show message of a line of text, carrying on at once.
func (b *routineBuilder) show(parent ids.ULID, kind, text string) ids.ULID {
	return b.node(kind, parent, map[string]any{flow.ParamMessage: []flow.Segment{{Text: text, Ref: nil}}})
}

// logServing writes to Watch whom the clerk serves.
func (b *routineBuilder) logServing(parent ids.ULID, customer flow.ValueRef) ids.ULID {
	return b.node(flow.KindLog, parent, map[string]any{flow.ParamMessage: []flow.Segment{
		{Text: "Serving ", Ref: nil}, {Text: "", Ref: &customer},
	}})
}

// fetchStock is the workflow the clerk's day calls: the clerk goes to the
// place it is given, the shelf to start with, and finds stock there three
// times in four, lifting the crate with a shake and a chime together,
// going back by Fetched; or telling everyone in the zone the shelf is empty
// and going back by Out of stock.
func fetchStock(
	orch *orchestrator.Orchestrator, anchors sampleAnchors, emptyShelf ids.ULID,
) (*flow.Workflow, ids.ULID, error) {
	start, err := orch.NewID()
	if err != nil {
		return nil, ids.ULID{}, fmt.Errorf("sample routine: %w", err)
	}
	made := &routineBuilder{orch: orch, nodes: nil, links: nil, locals: nil, err: nil}
	place := made.local("place", worldstate.TypePlace)
	none := ids.ULID{}
	walk := made.node(flow.KindGoTo, none, map[string]any{
		flow.ParamGoes: flow.GoesToValue, flow.ParamPlace: localValue(place),
	})
	stock := made.node(flow.KindRandom, none, map[string]any{"options": []flow.Share{
		share("o1", "in stock", stockShare), share("o2", "out of stock", noneShare),
	}})
	fetched := made.wayOut("Fetched", portFetched)
	out := made.wayOut("Out of stock", portNone)
	lift := made.node(flow.KindTogether, none, map[string]any{"name": "Lift the crate", flow.ParamLanes: []flow.Lane{
		{Port: "l1", Label: "Shake"}, {Port: "l2", Label: "Chime"},
	}})
	crate := made.node(flow.KindShake, lift, map[string]any{flow.ParamStrength: flow.ShakeGentle, flow.ParamSeconds: 1})
	thud := made.node(flow.KindPlaySound, lift, map[string]any{"cue": chimeCue, flow.ParamCarry: flow.CarryWhenDone})
	made.link(start, flow.OutputNext, walk)
	made.link(walk, flow.OutputNext, stock)
	made.link(stock, "o1", lift)
	made.link(lift, "l1", crate)
	made.link(lift, "l2", thud)
	made.link(lift, flow.OutputDone, fetched)
	empty := made.node(flow.KindSendSignal, none, map[string]any{
		flow.ParamSignal: emptyShelf.String(), flow.ParamSendTo: flow.ToZone,
	})
	made.link(stock, "o2", empty)
	made.link(empty, flow.OutputNext, out)
	if made.err != nil {
		return nil, ids.ULID{}, made.err
	}
	shelf := worldstate.NoValue()
	shelf.Type, shelf.Place = worldstate.TypePlace, worldstate.Place{Floor: anchors.floor, Anchor: anchors.shelf}
	made.locals[0].Role, made.locals[0].Default = worldstate.RoleInput, shelf.Encode()
	workflow, err := arrangedRoutine(start, made, ids.ULID{})

	return workflow, place, err
}

// wayOut is a Return at the top level.
func (b *routineBuilder) wayOut(label, port string) ids.ULID {
	return b.node(flow.KindReturn, ids.ULID{}, map[string]any{flow.ParamWayName: label, flow.ParamWayPort: port})
}

// fetchCall is the clerk's day's Call of Fetch stock, giving it the shelf.
func (b *routineBuilder) fetchCall(parent, fetch, place ids.ULID, at sampleAnchors) ids.ULID {
	shelf := worldstate.NoValue()
	shelf.Type, shelf.Place = worldstate.TypePlace, worldstate.Place{Floor: at.floor, Anchor: at.shelf}

	return b.node(flow.KindCall, parent, map[string]any{
		flow.ParamWorkflow: fetch.String(), flow.ParamInputs: map[ids.ULID]json.RawMessage{place: shelf.Encode()},
		flow.ParamWays: []flow.WayOut{{Port: portFetched, Label: "Fetched"}, {Port: portNone, Label: "Out of stock"}},
	})
}

// clerksDay builds the clerk's day from its start node, calling Fetch
// stock, whose place input is place.
func clerksDay(
	orch *orchestrator.Orchestrator, anchors sampleAnchors, values shopValues, fetch, place ids.ULID,
) (*flow.Workflow, error) {
	start, err := orch.NewID()
	if err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}
	made := &routineBuilder{orch: orch, nodes: nil, links: nil, locals: nil, err: nil}
	none := ids.ULID{}
	hours := made.node(flow.KindTimeSwitch, none, map[string]any{
		"ranges": []flow.TimeRange{{Port: "r1", From: openMinute, To: closeMinute}},
	})
	shop := made.node(flow.KindContainer, none, map[string]any{
		"name": "Shop hours", "bound": flow.TimeRange{Port: "", From: openMinute, To: closeMinute},
	})
	closing := made.node(flow.KindCheckValue, none, map[string]any{
		flow.ParamValue: npcValue(values.serving), "against": json.RawMessage(shopYes),
	})
	shut, notice := made.serving(none, values, shopNo), made.show(none, flow.KindShowMessage, shutNotice)
	toDoor, sleep := made.goTo(none, anchors, anchors.door), made.wait(none, nightHours, flow.UnitHours)
	toDesk := made.goTo(shop, anchors, anchors.desk)
	chime := made.node(flow.KindPlaySound, shop, map[string]any{"cue": chimeCue, "loudness": flow.LoudnessAsCue})
	greet := made.sayValue(shop, npcValue(values.greeting))
	customer := localValue(made.local("customer", worldstate.TypeNPC))
	find := made.node(flow.KindFindNPC, shop, map[string]any{
		"within": shopWithin, "where": npcValue(values.waiting), "is": json.RawMessage(shopYes), "into": customer,
	})
	serving, open := made.logServing(shop, customer), made.serving(shop, values, shopYes)
	serve, done := made.serveWait(shop, values), made.serving(shop, values, shopNo)
	choose := made.node(flow.KindRandom, shop, map[string]any{"options": []flow.Share{
		share("o1", "tidy the shelf", tidyShare), share("o2", "stretch", stretchShare),
	}})
	toShelf, tidy := made.fetchCall(shop, fetch, place, anchors), made.wait(shop, tidyMinutes, flow.UnitMinutes)
	down := made.node(flow.KindMove, shop, map[string]any{"direction": flow.DirectionDown, "tiles": stretchCells})
	rise := made.node(flow.KindMove, shop, map[string]any{"direction": flow.DirectionUp, "tiles": stretchCells})
	note := made.node(flow.KindNote, none, map[string]any{"text": routineNote})
	made.answerBell(values.bell)
	for _, joined := range []struct {
		from   ids.ULID
		output string
		to     ids.ULID
	}{
		{start, flow.OutputNext, hours}, {hours, "r1", shop}, {hours, flow.OutputOtherwise, closing},
		{closing, flow.OutputYes, shut}, {closing, flow.OutputOtherwise, toDoor}, {shut, flow.OutputNext, notice},
		{notice, flow.OutputNext, toDoor},
		{toDoor, flow.OutputNext, sleep}, {sleep, flow.OutputNext, hours},
		{shop, flow.OutputInside, toDesk}, {shop, flow.OutputExit, hours},
		{toDesk, flow.OutputNext, chime}, {chime, flow.OutputNext, greet}, {greet, flow.OutputNext, find},
		{find, flow.OutputFound, serving}, {serving, flow.OutputNext, open}, {find, flow.OutputNone, choose},
		{open, flow.OutputNext, serve}, {serve, flow.OutputNext, done}, {done, flow.OutputNext, choose},
		{choose, "o1", toShelf}, {toShelf, portFetched, tidy}, {toShelf, portNone, toDesk},
		{toShelf, flow.OutputDone, toDesk}, {tidy, flow.OutputNext, toDesk},
		{choose, "o2", down}, {down, flow.OutputNext, rise}, {rise, flow.OutputNext, toDesk},
	} {
		made.link(joined.from, joined.output, joined.to)
	}
	if made.err != nil {
		return nil, made.err
	}

	return arrangedRoutine(start, made, note)
}

// answerBell adds the clerk's receiver for the bell: she says Just a
// moment!, and once it is said resumes what she was doing.
func (b *routineBuilder) answerBell(bell ids.ULID) {
	none := ids.ULID{}
	ring := b.node(flow.KindOnSignal, none, map[string]any{flow.ParamSignal: bell.String()})
	coming := b.node(flow.KindSay, none, map[string]any{
		flow.ParamMessage: []flow.Segment{{Text: "Just a moment!", Ref: nil}}, flow.ParamCarry: flow.CarryWhenDone,
	})
	back := b.node(flow.KindResume, none, nil)
	b.link(ring, flow.OutputNext, coming)
	b.link(coming, flow.OutputNext, back)
}

// arrangedRoutine puts the nodes in a workflow, laid out as Auto-arrange
// lays it out, with the note under everything else.
func arrangedRoutine(start ids.ULID, made *routineBuilder, note ids.ULID) (*flow.Workflow, error) {
	workflow, err := flow.New(start)
	if err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}
	for _, local := range made.locals {
		if err := workflow.Locals().Insert(local); err != nil {
			return nil, fmt.Errorf("sample routine: %w", err)
		}
	}
	if err := workflow.Insert(made.nodes, nil, made.links); err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}
	boxes := workflowview.Arrange(workflow, func(node flow.Node, _ int) int {
		if node.Kind == flow.KindNote {
			return noteHeight
		}

		return 0
	})
	bottom := 0
	for _, box := range boxes {
		bottom = max(bottom, box.Y+max(box.H, noteHeight))
	}
	if first, placed := boxes[start]; placed && !note.IsZero() {
		boxes[note] = flow.Box{X: first.X, Y: bottom + noteHeight/2, W: 0, H: 0}
	}
	if _, err := workflow.Move(boxes); err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}

	return workflow, nil
}

// sampleRoutine makes Fetch stock and the clerk's day, which calls it, and
// gives the day to the clerk, with the shop's values.
func sampleRoutine(orch *orchestrator.Orchestrator, clerk ids.ULID, anchors sampleAnchors, values shopValues) error {
	fetch, place, err := fetchStock(orch, anchors, values.emptyShelf)
	if err != nil {
		return err
	}
	fetchID, err := orch.CreateDocument(fetchName, &workflowplugin.Document{Workflow: fetch})
	if err != nil {
		return fmt.Errorf("sample routine: %w", err)
	}
	workflow, err := clerksDay(orch, anchors, values, fetchID, place)
	if err != nil {
		return err
	}
	routineID, err := orch.CreateDocument(routineName, &workflowplugin.Document{Workflow: workflow})
	if err != nil {
		return fmt.Errorf("sample routine: %w", err)
	}
	err = orch.ChangeDocument(clerk, func(document model.Document) error {
		npcDoc, ok := document.(*npcplugin.Document)
		if !ok {
			return fmt.Errorf("%w: the clerk", errSample)
		}
		npcDoc.NPC.Behaviour = routineID

		return npcDoc.NPC.Owned.Apply(values.template)
	})
	if err != nil {
		return fmt.Errorf("sample routine: %w", err)
	}

	return nil
}

// share is a random choice's option, its share typed.
func share(port, label string, share int) flow.Share {
	return flow.Share{Port: port, Label: label, Share: share, Constant: flow.NoConstant()}
}
