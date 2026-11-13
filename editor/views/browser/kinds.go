package browser

import (
	"cmp"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// A zone's Zone loop box, and the row for no zone loop in its list.
const (
	noZoneLoop     = "no-zone-loop"
	chooseLoopID   = "choose-zone-loop"
	openLoopID     = "open-zone-loop"
	headingRowID   = "heading:"
	chooseLoopText = "Choose…"
)

// loopPanel is a selected zone's Zone loop box and its buttons.
type loopPanel struct {
	top    int
	box    image.Rectangle
	choose *ui.Button
	open   *ui.Button
	// flag says why the zone's loop doesn't fit it, or is "".
	flag string
}

func newLoopPanel() *loopPanel {
	return &loopPanel{
		top: 0, box: ui.Zero(), choose: ui.NewButton(chooseLoopID, chooseLoopText), open: ui.NewButton(openLoopID, "Open"),
		flag: "",
	}
}

// subscribeWorkflows keeps every workflow loaded, to know where each runs.
func (s *State) subscribeWorkflows(ctx *frame.Context) {
	for _, entry := range s.index(ctx.Docs).OfFormat(flow.FormatName) {
		s.subscribe(entry.ID, ctx)
	}
}

// runsInOf returns the zone a workflow runs in: zero for any zone, for a
// zone since deleted, or for a workflow not loaded yet.
func (s *State) runsInOf(docs frame.Docs, workflowID ids.ULID) ids.ULID {
	runs := ids.ULID{}
	docs.View(workflowID, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			runs = workflowDoc.Workflow.RunsIn()
		}
	})
	if _, exists := s.index(docs).Entry(runs); !exists {
		return ids.ULID{}
	}

	return runs
}

// headingRow is a list row that heads a group and cannot be chosen.
func headingRow(label string) ui.Row {
	row := ui.NewRow(headingRowID+label, label)
	row.Heading = true

	return row
}

// workflowRows lists the workflows of a kind for an NPC or a zone living in
// home, named who: those running there first, then those running in any
// zone, then, grey, those running in other zones, which it can't take.
// With no home, those running in one zone follow those running in any.
func (s *State) workflowRows(docs frame.Docs, purpose flow.Purpose, home ids.ULID, who string) []ui.Row {
	index := s.index(docs)
	icon := ui.IconWorkflow
	if purpose == flow.PurposeZoneLoop {
		icon = ui.IconZoneLoop
	}
	var here, anywhere, elsewhere []ui.Row
	for _, entry := range index.OfFormat(flow.FormatName) {
		if kind := cmp.Or(flow.Purpose(entry.Kind), flow.PurposeBehaviour); kind != purpose {
			continue
		}
		row := ui.NewRow(entry.ID.String(), entry.Name)
		row.Icon = icon
		switch runs := s.runsInOf(docs, entry.ID); {
		case runs.IsZero():
			anywhere = append(anywhere, row)
		case runs == home:
			here = append(here, row)
		default:
			row.Detail, row.Muted = index.Name(runs, ""), !home.IsZero()
			elsewhere = append(elsewhere, row)
		}
	}
	var rows []ui.Row
	group := func(label string, listed []ui.Row) {
		if len(listed) > 0 {
			rows = append(append(rows, headingRow(label)), listed...)
		}
	}
	homeName := index.Name(home, "its zone")
	if purpose == flow.PurposeZoneLoop {
		group("Run in "+homeName, here)
		group("Run in any zone", anywhere)
		group("Run in other zones: "+who+" can't run them", elsewhere)

		return rows
	}
	group("Run in "+homeName+", where "+who+" lives", here)
	group("Run in any zone", anywhere)
	if home.IsZero() {
		group("Run in one zone", elsewhere)
	} else {
		group("Run in other zones: "+who+" can't follow them", elsewhere)
	}

	return rows
}

// refusal says why a workflow chosen for an NPC, or a zone, living in home
// can't be taken, or returns "".
func (s *State) refusal(docs frame.Docs, workflowID, home ids.ULID, who string) string {
	runs := s.runsInOf(docs, workflowID)
	if runs.IsZero() || home.IsZero() || runs == home {
		return ""
	}
	index := s.index(docs)
	if workflowID == s.loopOf(docs, home) || index.Name(home, "") == who {
		return index.Name(workflowID, "That workflow") + " runs in " + index.Name(runs, "another zone") + ", not in " +
			who + ". Choose one that runs in " + who + " or in any zone."
	}

	return index.Name(workflowID, "That workflow") + " runs in " + index.Name(runs, "another zone") + ", and " + who +
		" lives in " + index.Name(home, "its zone") + ". Choose one that runs there or in any zone."
}

// loopOf returns a zone's loop, zero for none or a workflow since deleted.
func (s *State) loopOf(docs frame.Docs, zoneID ids.ULID) ids.ULID {
	loop := ids.ULID{}
	docs.View(zoneID, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			loop = zoneDoc.Zone.Loop
		}
	})
	if _, exists := s.index(docs).Entry(loop); !exists {
		return ids.ULID{}
	}

	return loop
}

// layoutZoneLoop places a selected zone's Zone loop box and its buttons,
// returning where what follows goes.
func (s *State) layoutZoneLoop(docs frame.Docs, left, width, top int) int {
	panel := s.loop
	panel.box, panel.choose.Rect, panel.open.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	entry, ok := s.selected(docs)
	if !ok || entry.Format != zone.FormatName {
		return top
	}
	panel.top = top + panelGap
	panel.flag = s.refusal(docs, s.loopOf(docs, entry.ID), entry.ID, entry.Name)

	return layoutChoice(panel.top, left, width, &panel.box, panel.choose, panel.open)
}

// misfitLabel is a chosen workflow's name in a box, with where it runs when
// that is not where it should: flagged in the warning ink.
func (s *State) misfitLabel(docs frame.Docs, workflowID ids.ULID, flagged bool) (string, color.RGBA) {
	name := s.index(docs).Name(workflowID, "a deleted workflow")
	if !flagged {
		return name, ui.ColorInk()
	}

	return name + " · runs in " + s.index(docs).Name(s.runsInOf(docs, workflowID), "another zone"), ui.ColorWarn()
}

// drawZoneLoop draws a selected zone's Zone loop box.
func (s *State) drawZoneLoop(screen *ebiten.Image, docs frame.Docs, left int) {
	panel := s.loop
	if panel.box.Empty() {
		return
	}
	s.theme.Heading(screen, "ZONE LOOP", left, panel.top)
	ui.Box(screen, panel.box, ui.ColorField())
	label, ink := "No zone loop", ui.ColorMuted()
	if entry, ok := s.selected(docs); ok {
		if loop := s.loopOf(docs, entry.ID); !loop.IsZero() {
			label, ink = s.misfitLabel(docs, loop, panel.flag != "")
		}
	}
	s.theme.DrawTextIn(screen, label, panel.box, ink)
	panel.choose.Draw(screen, s.theme)
	panel.open.Draw(screen, s.theme)
}

// updateZoneLoop handles a selected zone's Zone loop buttons, reporting
// whether one was clicked.
func (s *State) updateZoneLoop(input *ui.Input, ctx *frame.Context) bool {
	entry, ok := s.selected(ctx.Docs)
	if !ok || entry.Format != zone.FormatName {
		return false
	}
	s.subscribeWorkflows(ctx)
	switch {
	case s.loop.choose.Update(input):
		current := s.loopOf(ctx.Docs, entry.ID)
		selected := noZoneLoop
		if !current.IsZero() {
			selected = current.String()
		}
		list := ui.NewList("zone-loops", selected)
		list.Rows = append([]ui.Row{ui.NewRow(noZoneLoop, "No zone loop")},
			s.workflowRows(ctx.Docs, flow.PurposeZoneLoop, entry.ID, entry.Name)...)
		s.dialog = ui.NewDialog("Which zone loop does "+entry.Name+" run?", nil,
			ui.DialogButton(useID, "Use"), ui.DialogButton(ui.DialogCancel, "Cancel"))
		s.dialog.List, s.dialogKind, s.loopFor = list, dialogZoneLoop, entry.ID

		return true
	case s.loop.open.Update(input):
		if loop := s.loopOf(ctx.Docs, entry.ID); !loop.IsZero() {
			ctx.Requests.Open(workflowView, loop, false)
		}

		return true
	}

	return false
}

// setZoneLoop saves the zone loop chosen in the dialog, refusing one that
// runs in another zone.
func (s *State) setZoneLoop(ctx *frame.Context, chosen string) {
	loop, err := ids.Parse(chosen)
	if chosen == noZoneLoop || err != nil {
		loop = ids.ULID{}
	}
	zoneID := s.loopFor
	name := s.index(ctx.Docs).Name(zoneID, "The zone")
	if refused := s.refusal(ctx.Docs, loop, zoneID, name); refused != "" {
		ctx.Frame.Refuse(refused)

		return
	}
	s.command(ctx, zoneID, zoneplugin.SetLoopEdit, "", zoneplugin.SetLoopArgs{Loop: loop},
		func(protocol.Result, error) { ctx.Frame.Status("Zone loop set") }, nil)
}

// chooseBehaviour asks which NPC behaviour the NPC follows: those running
// where it lives first, then those running in any zone, then, grey, those
// it can't follow.
func (s *State) chooseBehaviour(ctx *frame.Context) {
	found, _ := s.npcOf(ctx.Docs, s.npc.npc)
	selected := noBehaviour
	if !found.Behaviour.IsZero() {
		selected = found.Behaviour.String()
	}
	name := s.index(ctx.Docs).Name(s.npc.npc, "this NPC")
	list := ui.NewList("workflows", selected)
	list.Rows = append([]ui.Row{ui.NewRow(noBehaviour, "No behaviour")},
		s.workflowRows(ctx.Docs, flow.PurposeBehaviour, s.zoneOfNPC(ctx.Docs, s.npc.npc), name)...)
	s.dialog = ui.NewDialog("Which workflow does "+name+" follow?", nil,
		ui.DialogButton(useID, "Use"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List, s.dialogKind = list, dialogBehaviour
}

// setBehaviour saves the behaviour chosen in the dialog, refusing one that
// runs where the NPC does not live.
func (s *State) setBehaviour(ctx *frame.Context, chosen string) {
	behaviour, err := ids.Parse(chosen)
	if chosen == noBehaviour || err != nil {
		behaviour = ids.ULID{}
	}
	npcID := s.npc.npc
	name := s.index(ctx.Docs).Name(npcID, "The NPC")
	if refused := s.refusal(ctx.Docs, behaviour, s.zoneOfNPC(ctx.Docs, npcID), name); refused != "" {
		ctx.Frame.Refuse(refused)

		return
	}
	s.command(ctx, npcID, npcplugin.SetBehaviourEdit, "", npcplugin.SetBehaviourArgs{Behaviour: behaviour},
		func(protocol.Result, error) { ctx.Frame.Status("Behaviour set") }, nil)
}
