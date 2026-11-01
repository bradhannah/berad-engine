package main

import (
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The clerk's day, the sample's behaviour workflow: it shows each kind of
// node at work. A time switch sends the clerk to the shop from 08:00 to
// 18:00 and to the door otherwise. Shop hours are a time-bound container:
// at the desk the clerk rings the chime and serves for 20 game minutes,
// then mostly tidies the shelf and now and then stretches. At night the
// clerk waits by the door an hour at a time. A note sums it up under the
// rest. It is placeholder behaviour for trying the editor, not game
// content.
const (
	routineName  = "Clerk's day"
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
	routineNote  = "The clerk's day: the shop from 08:00 to 18:00, mostly tidying, sometimes stretching; " +
		"the door at night."
)

// sampleAnchors are the anchors the clerk's day goes to, on the ground floor.
type sampleAnchors struct {
	floor, desk, shelf, door ids.ULID
}

// routineBuilder makes the clerk's day's nodes with new IDs.
type routineBuilder struct {
	orch  *orchestrator.Orchestrator
	nodes []flow.Node
	links []flow.Link
	err   error
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
		"destination": floor.AnchorRef{Floor: at.floor, Anchor: anchor},
	})
}

// wait is a Wait on the game clock.
func (b *routineBuilder) wait(parent ids.ULID, amount int, unit string) ids.ULID {
	return b.node(flow.KindWait, parent, map[string]any{
		"duration": flow.Duration{Amount: amount, Unit: unit, Clock: flow.ClockGame},
	})
}

// clerksDay builds the clerk's day from its start node.
func clerksDay(orch *orchestrator.Orchestrator, anchors sampleAnchors) (*flow.Workflow, error) {
	start, err := orch.NewID()
	if err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}
	made := &routineBuilder{orch: orch, nodes: nil, links: nil, err: nil}
	none := ids.ULID{}
	hours := made.node(flow.KindTimeSwitch, none, map[string]any{
		"ranges": []flow.TimeRange{{Port: "r1", From: openMinute, To: closeMinute}},
	})
	shop := made.node(flow.KindContainer, none, map[string]any{
		"name": "Shop hours", "bound": flow.TimeRange{Port: "", From: openMinute, To: closeMinute},
	})
	toDoor, sleep := made.goTo(none, anchors, anchors.door), made.wait(none, nightHours, flow.UnitHours)
	toDesk := made.goTo(shop, anchors, anchors.desk)
	chime := made.node(flow.KindPlaySound, shop, map[string]any{"cue": chimeCue, "loudness": flow.LoudnessAsCue})
	serve := made.wait(shop, serveMinutes, flow.UnitMinutes)
	choose := made.node(flow.KindRandom, shop, map[string]any{"options": []flow.Share{
		{Port: "o1", Label: "tidy the shelf", Share: tidyShare}, {Port: "o2", Label: "stretch", Share: stretchShare},
	}})
	toShelf, tidy := made.goTo(shop, anchors, anchors.shelf), made.wait(shop, tidyMinutes, flow.UnitMinutes)
	down := made.node(flow.KindMove, shop, map[string]any{"direction": flow.DirectionDown, "tiles": stretchCells})
	rise := made.node(flow.KindMove, shop, map[string]any{"direction": flow.DirectionUp, "tiles": stretchCells})
	note := made.node(flow.KindNote, none, map[string]any{"text": routineNote})
	for _, joined := range []struct {
		from   ids.ULID
		output string
		to     ids.ULID
	}{
		{start, flow.OutputNext, hours}, {hours, "r1", shop}, {hours, flow.OutputOtherwise, toDoor},
		{toDoor, flow.OutputNext, sleep}, {sleep, flow.OutputNext, hours},
		{shop, flow.OutputInside, toDesk}, {shop, flow.OutputExit, hours},
		{toDesk, flow.OutputNext, chime}, {chime, flow.OutputNext, serve}, {serve, flow.OutputNext, choose},
		{choose, "o1", toShelf}, {toShelf, flow.OutputNext, tidy}, {tidy, flow.OutputNext, toDesk},
		{choose, "o2", down}, {down, flow.OutputNext, rise}, {rise, flow.OutputNext, toDesk},
	} {
		made.link(joined.from, joined.output, joined.to)
	}
	if made.err != nil {
		return nil, made.err
	}

	return arrangedRoutine(start, made, note)
}

// arrangedRoutine puts the nodes in a workflow, laid out as Auto-arrange
// lays it out, with the note under everything else.
func arrangedRoutine(start ids.ULID, b *routineBuilder, note ids.ULID) (*flow.Workflow, error) {
	workflow, err := flow.New(start)
	if err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}
	if err := workflow.Insert(b.nodes, nil, b.links); err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}
	boxes := workflowview.Arrange(workflow, func(flow.Node, int) int { return noteHeight })
	bottom := 0
	for _, box := range boxes {
		bottom = max(bottom, box.Y+max(box.H, noteHeight))
	}
	if first, placed := boxes[start]; placed {
		boxes[note] = flow.Box{X: first.X, Y: bottom + noteHeight/2, W: 0, H: 0}
	}
	if _, err := workflow.Move(boxes); err != nil {
		return nil, fmt.Errorf("sample routine: %w", err)
	}

	return workflow, nil
}

// sampleRoutine makes the clerk's day and gives it to the clerk.
func sampleRoutine(orch *orchestrator.Orchestrator, clerk ids.ULID, at sampleAnchors) error {
	workflow, err := clerksDay(orch, at)
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

		return nil
	})
	if err != nil {
		return fmt.Errorf("sample routine: %w", err)
	}

	return nil
}
