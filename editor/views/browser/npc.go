package browser

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// NPC inspector layout, in screen pixels: the notes are at most notesHeight
// tall, and never run into the save bar.
const (
	notesHeight = 160
	notesLeast  = 60
	panelGap    = 18
	noBehaviour = "none"
	noSprite    = "none"
)

// npcPanel is the inspector's part for a selected NPC: its behaviour, the
// sprite it shows, its path name and its private notes.
type npcPanel struct {
	npc          ids.ULID
	path         *ui.TextField
	notes        *ui.TextField
	choose       *ui.Button
	open         *ui.Button
	chooseSprite *ui.Button
	openSprite   *ui.Button
	behaviour    image.Rectangle
	sprite       image.Rectangle
	top          int
	spriteTop    int
}

func newNPCPanel(theme *ui.Theme) *npcPanel {
	notes := ui.NewTextField("notes", textedit.NewField("", true, npc.MaxNotes), true, theme)
	notes.Placeholder = "Private notes about this NPC. Players never see them."
	path := ui.NewTextField("npc-path", textedit.NewField("", false, worldstate.MaxName), false, theme)
	path.Placeholder = "such as george"

	return &npcPanel{
		npc: ids.ULID{}, path: path, notes: notes, choose: ui.NewButton("choose-behaviour", "Choose…"),
		open: ui.NewButton("open-behaviour", "Open"), chooseSprite: ui.NewButton("choose-sprite", "Choose…"),
		openSprite: ui.NewButton("open-sprite", "Open"), behaviour: ui.Zero(), sprite: ui.Zero(), top: 0,
		spriteTop: 0,
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
			s.npc.path.Field.Revert()
			s.npc.notes.Field.Revert()
			s.npc.npc = ids.ULID{}
		}

		return
	}
	if npcID != s.npc.npc {
		s.npc.path.Field.Revert()
		s.npc.notes.Field.Revert()
		s.npc.npc = npcID
	}
	s.subscribe(npcID, ctx)
	if found, loaded := s.npcOf(ctx.Docs, npcID); loaded {
		s.npc.path.Field.SetSaved(found.Path)
		s.npc.notes.Field.SetSaved(found.Notes)
		s.npc.open.Disabled = found.Behaviour.IsZero()
		s.npc.openSprite.Disabled = found.Sprite.IsZero()
	}
}

// layoutNPC places the NPC panel from top down to limit, below the name.
func (s *State) layoutNPC(docs frame.Docs, left, width, top, limit int) {
	panel := s.npc
	panel.behaviour, panel.notes.Rect, panel.choose.Rect, panel.open.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	panel.sprite, panel.chooseSprite.Rect, panel.openSprite.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	panel.path.Rect = ui.Zero()
	if _, ok := s.selectedNPC(docs); !ok {
		return
	}
	panel.top = top
	rowY := layoutChoice(panel.top, left, width, &panel.behaviour, panel.choose, panel.open)
	panel.spriteTop = rowY
	rowY = layoutChoice(rowY, left, width, &panel.sprite, panel.chooseSprite, panel.openSprite) + ui.LineHeight
	panel.path.Rect = image.Rect(left, rowY, left+width, rowY+ui.ButtonH)
	rowY += ui.ButtonH + ui.Gap*ui.Half + ui.LineHeight + panelGap + ui.LineHeight
	panel.notes.Rect = image.Rect(left, rowY, left+width, rowY+max(min(notesHeight, limit-rowY), notesLeast))
}

// layoutChoice places a heading at top, the box naming what is chosen, and
// Choose… and Open below it, returning where the next heading goes.
func layoutChoice(top, left, width int, box *image.Rectangle, choose, open *ui.Button) int {
	rowY := top + ui.LineHeight
	*box = image.Rect(left, rowY, left+width, rowY+ui.ButtonH)
	rowY += ui.ButtonH + ui.Gap*ui.Half
	half := (width - buttonGap) / ui.Half
	choose.Rect = image.Rect(left, rowY, left+half, rowY+ui.ButtonH)
	open.Rect = image.Rect(left+half+buttonGap, rowY, left+width, rowY+ui.ButtonH)

	return rowY + ui.ButtonH + panelGap
}

// updateNPC handles the NPC panel's buttons.
func (s *State) updateNPC(input *ui.Input, ctx *frame.Context) {
	npcID, ok := s.selectedNPC(ctx.Docs)
	if !ok {
		return
	}
	found, loaded := s.npcOf(ctx.Docs, npcID)
	switch {
	case s.npc.choose.Update(input):
		s.chooseBehaviour(ctx)
	case s.npc.open.Update(input):
		if loaded && !found.Behaviour.IsZero() {
			ctx.Requests.Open(workflowView, found.Behaviour, false)
		}
	case s.npc.chooseSprite.Update(input):
		s.chooseSprite(ctx)
	case s.npc.openSprite.Update(input):
		if loaded && !found.Sprite.IsZero() {
			ctx.Requests.Open(spriteView, found.Sprite, false)
		}
	}
}

// chooseSprite asks which sprite the NPC shows.
func (s *State) chooseSprite(ctx *frame.Context) {
	found, _ := s.npcOf(ctx.Docs, s.npc.npc)
	s.choose(ctx, npcChoice{
		title: "Which sprite does this NPC show?", list: "sprites", none: noSprite,
		noneLabel: "No sprite", format: art.SpriteFormatName, icon: ui.IconSprite, kind: dialogSprite,
	}, found.Sprite)
}

// setSprite saves the sprite chosen in the dialog.
func (s *State) setSprite(ctx *frame.Context, chosen string) {
	sprite, err := ids.Parse(chosen)
	if chosen == noSprite || err != nil {
		sprite = ids.ULID{}
	}
	s.command(ctx, s.npc.npc, npcplugin.SetSpriteEdit, "", npcplugin.SetSpriteArgs{Sprite: sprite},
		func(protocol.Result, error) { ctx.Frame.Status("Sprite set") }, nil)
}

// npcChoice is a dialog that chooses a document for the NPC: its title, the
// list's ID, the row for none, and the documents offered.
type npcChoice struct {
	title, list, none, noneLabel, format string
	icon                                 ui.Icon
	kind                                 dialogKind
}

// choose asks which document the NPC uses, with current selected.
func (s *State) choose(ctx *frame.Context, choice npcChoice, current ids.ULID) {
	selected := choice.none
	if !current.IsZero() {
		selected = current.String()
	}
	list := &ui.List{ID: choice.list, Rect: ui.Zero(), Rows: nil, Selected: selected, CanDrop: nil}
	list.Rows = append(list.Rows, ui.NewRow(choice.none, choice.noneLabel))
	for _, entry := range s.index(ctx.Docs).OfFormat(choice.format) {
		row := ui.NewRow(entry.ID.String(), entry.Name)
		row.Icon = choice.icon
		list.Rows = append(list.Rows, row)
	}
	s.dialog = ui.NewDialog(choice.title, nil,
		ui.DialogButton("use", "Use"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogKind = choice.kind
}

// chooseBehaviour asks which workflow the NPC follows.
func (s *State) chooseBehaviour(ctx *frame.Context) {
	found, _ := s.npcOf(ctx.Docs, s.npc.npc)
	s.choose(ctx, npcChoice{
		title: "Which workflow does this NPC follow?", list: "workflows", none: noBehaviour,
		noneLabel: "No behaviour", format: flow.FormatName, icon: ui.IconWorkflow, kind: dialogBehaviour,
	}, found.Behaviour)
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

// commitPath saves the NPC's path name, from Save or Cmd+S, or says what is
// wrong with it.
func (s *State) commitPath(ctx *frame.Context) {
	if err := npc.CheckPath(strings.TrimSpace(s.npc.path.Field.Value())); err != nil {
		s.focus.Set(s.npc.path)
		ctx.Frame.Status("A path name is lower-case letters, digits and underscores, starting with a letter")

		return
	}
	path, changed := s.npc.path.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	s.command(ctx, s.npc.npc, npcplugin.SetPathEdit, "", npcplugin.SetPathArgs{Path: strings.TrimSpace(path)},
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Path name saved")
		},
		func() { s.npc.path.Field.Failed() })
}

// paths maps each NPC's path name to the NPCs that use it, as far as the
// window has them.
func (s *State) paths(docs frame.Docs) map[string][]ids.ULID {
	used := map[string][]ids.ULID{}
	for _, entry := range s.entries(docs, npc.FormatName) {
		if found, loaded := s.npcOf(docs, entry.ID); loaded && found.Path != "" {
			used[found.Path] = append(used[found.Path], entry.ID)
		}
	}

	return used
}

// pathNote says what is worth knowing about the NPC's path name: that
// another NPC uses it too, or a suggestion when it has none.
func (s *State) pathNote(docs frame.Docs, npcID ids.ULID) (string, bool) {
	found, loaded := s.npcOf(docs, npcID)
	if !loaded {
		return "", false
	}
	if found.Path == "" {
		return "None yet, so workflows cannot name it. Suggested: " +
			npc.UniquePath(npc.SuggestPath(s.index(docs).Name(npcID, "")), s.takenPaths(docs)), false
	}
	for _, other := range s.paths(docs)[found.Path] {
		if other != npcID {
			return "Also the path name of " + s.index(docs).Name(other, "another NPC") + ". Give each its own.", true
		}
	}

	return "Workflows read its values as npc." + found.Path + ".…", false
}

// takenPaths lists the path names the window's NPCs use.
func (s *State) takenPaths(docs frame.Docs) map[string]bool {
	taken := map[string]bool{}
	for path := range s.paths(docs) {
		taken[path] = true
	}

	return taken
}

// createNPC creates an NPC with no behaviour yet, and a path name suggested
// from its name that no other NPC uses.
func (s *State) createNPC(ctx *frame.Context) {
	name := uniqueName("NPC", names(s.entries(ctx.Docs, npc.FormatName)))
	document := npcplugin.NewDocument()
	document.NPC.Path = npc.UniquePath(npc.SuggestPath(name), s.takenPaths(ctx.Docs))
	data, err := document.Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	args := newDocument(npc.FormatName, name, ids.ULID{}, data)
	args.Folder = s.targetFolder(ctx.Docs, npc.FormatName)
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
	s.theme.Heading(screen, "SPRITE", left, panel.spriteTop)
	ui.Box(screen, panel.sprite, ui.ColorBackground())
	label, ink = "Its token", ui.ColorMuted()
	if found, loaded := s.npcOf(docs, npcID); loaded && !found.Sprite.IsZero() {
		label, ink = s.index(docs).Name(found.Sprite, "Broken: its sprite was deleted"), ui.ColorInk()
	}
	s.theme.DrawTextIn(screen, label, panel.sprite, ink)
	panel.chooseSprite.Draw(screen, s.theme)
	panel.openSprite.Draw(screen, s.theme)
	s.theme.Heading(screen, "PATH NAME", left, panel.path.Rect.Min.Y-ui.LineHeight)
	panel.path.Draw(screen)
	if note, warn := s.pathNote(docs, npcID); note != "" {
		noteInk := ui.ColorMuted()
		if warn {
			noteInk = ui.ColorWarn()
		}
		s.theme.DrawText(screen, s.theme.FitText(note, panel.path.Rect.Dx()), left,
			panel.path.Rect.Max.Y+ui.Gap*ui.Half, noteInk)
	}
	s.theme.Heading(screen, "NOTES", left, panel.notes.Rect.Min.Y-ui.LineHeight)
	panel.notes.Draw(screen)
	if panel.notes.Field.Conflict() {
		s.theme.DrawText(screen, "Changed elsewhere. Saving keeps yours.", left, panel.notes.Rect.Max.Y+ui.Gap,
			ui.ColorWarn())
	}
}
