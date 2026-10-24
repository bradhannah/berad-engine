package browser

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Inspector layout, in screen pixels, from the top of the panel: the type
// and name, a rule, the name field, the document's ID, then its actions.
const (
	headerRule   = ui.Inset + ui.LineHeight + ui.TitleSize + ui.Inset
	nameFieldTop = headerRule + ui.Inset + ui.LineHeight
	actionsTop   = nameFieldTop + ui.ButtonH + ui.LineHeight + ui.Inset
	footerGap    = 10
)

// kind is what the inspector says about a document type: its name, its icon
// and what its Open button says, if it has one.
type kind struct {
	label string
	noun  string
	icon  ui.Icon
	open  string
}

func kindOf(format string) kind {
	switch format {
	case zone.FormatName:
		return kind{label: "Zone", noun: "zone", icon: ui.IconZone, open: "Open map"}
	case floor.FormatName:
		return kind{label: "Floor", noun: "floor", icon: ui.IconFloor, open: "Open map"}
	case flow.FormatName:
		return kind{label: "Workflow", noun: "workflow", icon: ui.IconWorkflow, open: "Open workflow"}
	case npc.FormatName:
		return kind{label: npcLabel, noun: npcLabel, icon: ui.IconNPC, open: ""}
	case textdoc.FormatName:
		return kind{label: "Text document", noun: "text document", icon: ui.IconText, open: "Open text"}
	}

	return kind{label: "Document", noun: "document", icon: ui.IconNone, open: ""}
}

// layoutInspector places the name field, the selected type's actions, the
// save bar and Delete at the foot.
func (s *State) layoutInspector(docs frame.Docs) {
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	half := (width - buttonGap) / ui.Half
	for _, button := range []*ui.Button{s.open, s.floorButton, s.up, s.down, s.remove} {
		button.Rect = ui.Zero()
	}
	s.name.Rect, s.save.Rect = ui.Zero(), ui.Zero()
	entry, ok := s.selected(docs)
	if !ok {
		s.layoutNPC(docs, left, width, 0, 0)

		return
	}
	s.name.Rect = image.Rect(left, nameFieldTop, left+width, nameFieldTop+ui.ButtonH)
	bottom := panel.Max.Y - ui.Inset
	s.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
	s.remove.Label = "Delete " + kindOf(entry.Format).noun + "…"
	saveTop := s.remove.Rect.Min.Y - footerGap - ui.SaveBarHeight(width)
	s.save.Rect = image.Rect(left, saveTop, left+width, saveTop+ui.SaveBarHeight(width))
	s.save.Unsaved = s.Unsaved(docs)
	top := actionsTop
	s.open.Label, s.open.Disabled = kindOf(entry.Format).open, false
	switch entry.Format {
	case zone.FormatName:
		s.open.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
		s.floorButton.Rect = image.Rect(left+half+buttonGap, top, left+width, top+ui.ButtonH)
	case floor.FormatName:
		s.open.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + buttonGap
		s.up.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
		s.down.Rect = image.Rect(left+half+buttonGap, top, left+width, top+ui.ButtonH)
		_, inZone := s.zoneOf(docs, entry.ID)
		s.open.Disabled, s.up.Disabled, s.down.Disabled = !inZone, !inZone, !inZone
	case flow.FormatName, textdoc.FormatName:
		s.open.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	}
	s.layoutNPC(docs, left, width, top, saveTop-footerGap)
}

// syncName keeps the name field showing the selected document's name. The
// guard has already asked about unsaved changes before the selection moved.
func (s *State) syncName(docs frame.Docs) {
	entry, ok := s.selected(docs)
	if !ok {
		if !s.named.IsZero() {
			s.focus.Set(nil)
			s.name.Field.Revert()
			s.named = ids.ULID{}
		}

		return
	}
	if entry.ID != s.named {
		s.focus.Set(nil)
		s.name.Field.Revert()
		s.named = entry.ID
	}
	s.name.Field.SetSaved(entry.Name)
}

// guardSelection asks about unsaved changes before the inspector moves to
// another document: the selection goes back until the author decides, and
// moves on after Save or Don't save.
func (s *State) guardSelection(ctx *frame.Context) {
	if s.named.IsZero() || s.list.Selected == s.named.String() {
		return
	}
	unsaved := s.Unsaved(ctx.Docs)
	entry, exists := s.index(ctx.Docs).Entry(s.named)
	if unsaved == 0 || !exists {
		return
	}
	wanted := s.list.Selected
	s.list.Selected = s.named.String()
	s.focus.Set(nil)
	s.guard.Ask(unsaved, entry.Name, func() { s.list.Selected = wanted })
}

// Unsaved counts the inspector's typed changes not saved yet: the name, and
// a selected NPC's notes.
func (s *State) Unsaved(frame.Docs) int {
	count := frame.UnsavedFields(s.name)
	if !s.npc.npc.IsZero() {
		count += frame.UnsavedFields(s.npc.notes)
	}

	return count
}

// Save saves the name and an NPC's notes. They belong to different
// documents, so saving both is two undo steps.
func (s *State) Save(ctx *frame.Context) bool {
	if s.name.Field.Unsaved() {
		s.commitName(ctx)
	}
	if !s.npc.npc.IsZero() && s.npc.notes.Field.Unsaved() {
		s.commitNotes(ctx)
	}

	return true
}

// Leave asks about unsaved changes before then, such as closing the window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.index(ctx.Docs).Name(s.named, "this document"), then)
}

// discard drops the inspector's typed changes.
func (s *State) discard() {
	s.focus.Set(nil)
	s.name.Field.Revert()
	s.npc.notes.Field.Revert()
}

func (s *State) commitName(ctx *frame.Context) {
	name, changed := s.name.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	name = strings.TrimSpace(name)
	rename := project.RenameArgs{ID: s.named, Name: name}
	s.command(ctx, project.IndexID(), project.RenameEdit, "", rename,
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Renamed to “" + name + "”")
		},
		func() { s.name.Field.Failed() })
}

// drawInspector draws what is selected: its type and name, its name field
// and ID, its actions, its NPC panel, the save bar and Delete.
func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	left := panel.Min.X + ui.Inset
	entry, ok := s.selected(docs)
	if !ok {
		s.theme.Heading(screen, "INSPECTOR", left, panel.Min.Y+ui.Inset)
		s.theme.DrawText(screen, "Select something in the list.", left, panel.Min.Y+ui.Inset+ui.LineHeight, ui.ColorMuted())
		s.theme.DrawText(screen, "Double-click or Enter opens it.", left, panel.Min.Y+ui.Inset+ui.Half*ui.LineHeight,
			ui.ColorMuted())

		return
	}
	about := kindOf(entry.Format)
	ui.DrawIcon(screen, about.icon, image.Pt(left, panel.Min.Y+ui.Inset-1))
	s.theme.DrawLabel(screen, about.label, left+ui.IconSize+ui.Gap*ui.Half, panel.Min.Y+ui.Inset, ui.ColorMuted())
	s.theme.DrawTitle(screen, entry.Name, left, panel.Min.Y+ui.Inset+ui.LineHeight, ui.ColorInk())
	ui.Line(screen, image.Pt(left, headerRule), image.Pt(panel.Max.X-ui.Inset, headerRule), ui.Hairline, ui.ColorRule())
	s.theme.Heading(screen, "NAME", left, nameFieldTop-ui.LineHeight)
	s.name.Draw(screen)
	s.theme.DrawText(screen, "ID "+entry.ID.String(), left, s.name.Rect.Max.Y+ui.Gap, ui.ColorSubdued())
	for _, button := range []*ui.Button{s.open, s.floorButton, s.up, s.down, s.remove} {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
	s.drawNPC(screen, docs, left)
	s.save.Draw(screen, s.theme)
}
