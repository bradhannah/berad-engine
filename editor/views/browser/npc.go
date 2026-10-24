package browser

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// NPC inspector layout, in screen pixels: the notes are at most notesHeight
// tall, and never run into the save bar.
const (
	notesHeight = 160
	notesLeast  = 60
	panelGap    = 18
	noBehaviour = "none"
)

// npcPanel is the inspector's part for a selected NPC: its behaviour and its
// private notes.
type npcPanel struct {
	npc       ids.ULID
	notes     *ui.TextField
	choose    *ui.Button
	open      *ui.Button
	behaviour image.Rectangle
	top       int
}

func newNPCPanel(theme *ui.Theme) *npcPanel {
	notes := ui.NewTextField("notes", textedit.NewField("", true, npc.MaxNotes), true, theme)
	notes.Placeholder = "Private notes about this NPC. Players never see them."

	return &npcPanel{
		npc: ids.ULID{}, notes: notes, choose: ui.NewButton("choose-behaviour", "Choose…"),
		open: ui.NewButton("open-behaviour", "Open"), behaviour: ui.Zero(), top: 0,
	}
}

// selectedNPC returns the selected document's ID when it is an NPC.
func (s *State) selectedNPC(docs frame.Docs) (ids.ULID, bool) {
	entry, ok := s.selected(docs)

	return entry.ID, ok && entry.Format == npc.FormatName
}

// npcOf reads an NPC from the window's copy.
func (s *State) npcOf(docs frame.Docs, npcID ids.ULID) (npc.NPC, bool) {
	var found npc.NPC
	loaded := docs.View(npcID, func(document client.Document) {
		if npcDoc, ok := document.Model.(*npcplugin.Document); ok {
			found = npcDoc.NPC
		}
	})

	return found, loaded
}

// syncNPC keeps the NPC panel showing the selected NPC's saved notes. The
// guard has already asked about unsaved notes before the selection moved.
func (s *State) syncNPC(ctx *frame.Context) {
	npcID, ok := s.selectedNPC(ctx.Docs)
	if !ok {
		if !s.npc.npc.IsZero() {
			s.npc.notes.Field.Revert()
			s.npc.npc = ids.ULID{}
		}

		return
	}
	if npcID != s.npc.npc {
		s.npc.notes.Field.Revert()
		s.npc.npc = npcID
	}
	s.subscribe(npcID, ctx)
	if found, loaded := s.npcOf(ctx.Docs, npcID); loaded {
		s.npc.notes.Field.SetSaved(found.Notes)
		s.npc.open.Disabled = found.Behaviour.IsZero()
	}
}

// layoutNPC places the NPC panel from top down to limit, below the name.
func (s *State) layoutNPC(docs frame.Docs, left, width, top, limit int) {
	panel := s.npc
	panel.behaviour, panel.notes.Rect, panel.choose.Rect, panel.open.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	if _, ok := s.selectedNPC(docs); !ok {
		return
	}
	panel.top = top
	rowY := panel.top + ui.LineHeight
	panel.behaviour = image.Rect(left, rowY, left+width, rowY+ui.ButtonH)
	rowY += ui.ButtonH + ui.Gap*ui.Half
	half := (width - buttonGap) / ui.Half
	panel.choose.Rect = image.Rect(left, rowY, left+half, rowY+ui.ButtonH)
	panel.open.Rect = image.Rect(left+half+buttonGap, rowY, left+width, rowY+ui.ButtonH)
	rowY += ui.ButtonH + panelGap + ui.LineHeight
	panel.notes.Rect = image.Rect(left, rowY, left+width, rowY+max(min(notesHeight, limit-rowY), notesLeast))
}

// updateNPC handles the NPC panel's buttons.
func (s *State) updateNPC(input *ui.Input, ctx *frame.Context) {
	npcID, ok := s.selectedNPC(ctx.Docs)
	if !ok {
		return
	}
	switch {
	case s.npc.choose.Update(input):
		s.chooseBehaviour(ctx)
	case s.npc.open.Update(input):
		if found, loaded := s.npcOf(ctx.Docs, npcID); loaded && !found.Behaviour.IsZero() {
			ctx.Requests.Open(workflowView, found.Behaviour, false)
		}
	}
}

// chooseBehaviour asks which workflow the NPC follows.
func (s *State) chooseBehaviour(ctx *frame.Context) {
	current := noBehaviour
	if found, loaded := s.npcOf(ctx.Docs, s.npc.npc); loaded && !found.Behaviour.IsZero() {
		current = found.Behaviour.String()
	}
	list := &ui.List{ID: "workflows", Rect: ui.Zero(), Rows: nil, Selected: current}
	list.Rows = append(list.Rows, ui.NewRow(noBehaviour, "No behaviour"))
	for _, entry := range s.index(ctx.Docs).OfFormat(flow.FormatName) {
		row := ui.NewRow(entry.ID.String(), entry.Name)
		row.Icon = ui.IconWorkflow
		list.Rows = append(list.Rows, row)
	}
	s.dialog = ui.NewDialog("Which workflow does this NPC follow?", nil,
		ui.DialogButton("use", "Use"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogKind = dialogBehaviour
}

// setBehaviour saves the behaviour chosen in the dialog.
func (s *State) setBehaviour(ctx *frame.Context, chosen string) {
	behaviour, err := ids.Parse(chosen)
	if chosen == noBehaviour || err != nil {
		behaviour = ids.ULID{}
	}
	npcID := s.npc.npc
	s.command(ctx, npcID, npcplugin.SetBehaviourEdit, "", npcplugin.SetBehaviourArgs{Behaviour: behaviour},
		func(protocol.Result, error) { ctx.Frame.Status("Behaviour set") }, nil)
}

// commitNotes saves the NPC's notes, from Save or Cmd+S.
func (s *State) commitNotes(ctx *frame.Context) {
	notes, changed := s.npc.notes.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	s.command(ctx, s.npc.npc, npcplugin.SetNotesEdit, "", npcplugin.SetNotesArgs{Notes: notes},
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Notes saved")
		},
		func() { s.npc.notes.Field.Failed() })
}

// createNPC creates an NPC with no behaviour yet.
func (s *State) createNPC(ctx *frame.Context) {
	name := uniqueName("NPC", names(s.entries(ctx.Docs, npc.FormatName)))
	data, err := npcplugin.NewDocument().Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	args := newDocument(npc.FormatName, name, ids.ULID{}, data)
	s.command(ctx, project.IndexID(), project.CreateEdit, s.gesture(), args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
	}, nil)
}

// drawNPC draws the NPC panel.
func (s *State) drawNPC(screen *ebiten.Image, docs frame.Docs, left int) {
	npcID, ok := s.selectedNPC(docs)
	if !ok {
		return
	}
	panel := s.npc
	s.theme.Heading(screen, "BEHAVIOUR", left, panel.top)
	ui.Box(screen, panel.behaviour, ui.ColorBackground())
	label, ink := "No behaviour yet", ui.ColorMuted()
	if found, loaded := s.npcOf(docs, npcID); loaded && !found.Behaviour.IsZero() {
		label, ink = s.index(docs).Name(found.Behaviour, "Broken: its workflow was deleted"), ui.ColorInk()
	}
	s.theme.DrawTextIn(screen, label, panel.behaviour, ink)
	panel.choose.Draw(screen, s.theme)
	panel.open.Draw(screen, s.theme)
	s.theme.Heading(screen, "NOTES", left, panel.notes.Rect.Min.Y-ui.LineHeight)
	panel.notes.Draw(screen)
	if panel.notes.Field.Conflict() {
		s.theme.DrawText(screen, "Changed elsewhere. Saving keeps yours.", left, panel.notes.Rect.Max.Y+ui.Gap,
			ui.ColorWarn())
	}
}
