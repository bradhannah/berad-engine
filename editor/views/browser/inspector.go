package browser

import (
	"fmt"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Inspector layout, in screen pixels, from the top of the panel: the type
// and name, a rule, the name field, the document's ID, its tags, its folder,
// then its actions.
const (
	headerRule   = ui.Inset + ui.LineHeight + ui.TitleSize + ui.Inset
	nameFieldTop = headerRule + ui.Inset + ui.LineHeight
	tagsFieldTop = nameFieldTop + ui.ButtonH + ui.Half*ui.LineHeight + ui.Half*ui.Gap
	footerGap    = 10
	moveWidth    = 96
	chipPad      = 8
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
	case art.SpriteFormatName:
		return kind{label: "Sprite", noun: "sprite", icon: ui.IconSprite, open: "Open sprite"}
	case art.PaletteFormatName:
		return kind{label: "Palette", noun: "palette", icon: ui.IconPalette, open: "Open in Palette Editor"}
	}

	return kind{label: "Document", noun: "document", icon: ui.IconNone, open: ""}
}

// layoutInspector places the inspector for what is selected: a document's
// fields and actions, or a folder's or the settings', the save bar and
// Delete at the foot. A document every project holds keeps its name and
// cannot be deleted.
func (s *State) layoutInspector(docs frame.Docs) {
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	for _, button := range s.inspectorButtons() {
		button.Rect = ui.Zero()
	}
	s.zonePalettes.Layout(docs, ids.ULID{}, 0, 0, 0, s.size)
	s.spritePalettes = nil
	s.tagBns, s.remove.Disabled, s.audio.open.Rect = nil, false, ui.Zero()
	s.name.Rect, s.tags.Rect, s.save.Rect, s.tileSize.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	if s.list.Selected == settingsRow {
		s.layoutSettings(docs, left, width)
		s.layoutNPC(docs, left, width, 0, 0)

		return
	}
	if _, isCue := s.selectedCue(); isCue {
		s.layoutCue(left, width)
		s.layoutNPC(docs, left, width, 0, 0)

		return
	}
	_, isFolder := s.selectedFolder()
	entry, isDocument := s.selected(docs)
	if !isFolder && !isDocument {
		s.layoutNPC(docs, left, width, 0, 0)

		return
	}
	s.name.Rect = image.Rect(left, nameFieldTop, left+width, nameFieldTop+ui.ButtonH)
	bottom := panel.Max.Y - ui.Inset
	s.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
	saveTop := s.remove.Rect.Min.Y - footerGap - ui.SaveBarHeight(width)
	s.save.Rect = image.Rect(left, saveTop, left+width, saveTop+ui.SaveBarHeight(width))
	s.save.Unsaved = s.Unsaved(docs)
	if isFolder {
		s.layoutFolder(docs, left, width)

		return
	}
	s.remove.Label = "Delete " + kindOf(entry.Format).noun + "…"
	if everyProjectHolds(entry.ID) {
		s.name.Rect, s.remove.Disabled = ui.Zero(), true
	}
	top := s.layoutTags(entry, left, width)
	top = s.layoutActions(docs, entry, left, width, top)
	top = s.layoutArt(docs, entry, left, width, top)
	s.layoutNPC(docs, left, width, top, saveTop-footerGap)
}

// inspectorButtons are the inspector's buttons outside the NPC panel.
func (s *State) inspectorButtons() []*ui.Button {
	return []*ui.Button{s.open, s.floorButton, s.up, s.down, s.remove, s.moveBn, s.newFolderBn}
}

// layoutTags places the tags field, a button for each tag that searches for
// it, and the folder and Move to… for a type with folders. It returns where
// the actions start.
func (s *State) layoutTags(entry project.Entry, left, width int) int {
	s.tags.Rect = image.Rect(left, tagsFieldTop, left+width, tagsFieldTop+ui.ButtonH)
	top := s.tags.Rect.Max.Y + ui.Gap
	chipLeft := left
	for _, tag := range entry.Tags {
		label := "#" + tag
		chipWidth := s.theme.TextWidth(label) + ui.Half*chipPad
		if chipLeft+chipWidth > left+width {
			break
		}
		chip := ui.NewButton("tag:"+tag, label)
		chip.Rect = image.Rect(chipLeft, top, chipLeft+chipWidth, top+ui.ButtonH)
		s.tagBns = append(s.tagBns, chip)
		chipLeft += chipWidth + ui.Gap
	}
	if len(s.tagBns) > 0 {
		top += ui.ButtonH + ui.Gap
	}
	top += ui.Inset
	if formatSection(entry.Format) == "" {
		return top
	}
	s.moveBn.Rect = image.Rect(left+width-moveWidth, top+ui.LineHeight, left+width, top+ui.LineHeight+ui.ButtonH)

	return s.moveBn.Rect.Max.Y + ui.Inset
}

// layoutActions places the actions a document type has, from top down. It
// returns where the NPC panel starts.
func (s *State) layoutActions(docs frame.Docs, entry project.Entry, left, width, top int) int {
	half := (width - buttonGap) / ui.Half
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
	case flow.FormatName, textdoc.FormatName, art.SpriteFormatName, art.PaletteFormatName:
		s.open.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	}

	return top
}

// layoutFolder places a folder's buttons: a new folder inside, Move to…,
// and Delete, which only an empty folder allows.
func (s *State) layoutFolder(docs frame.Docs, left, width int) {
	folder, _ := s.selectedFolder()
	half := (width - buttonGap) / ui.Half
	top := s.name.Rect.Max.Y + ui.Half*ui.LineHeight + ui.Inset
	s.newFolderBn.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
	s.moveBn.Rect = image.Rect(left+half+buttonGap, top, left+width, top+ui.ButtonH)
	index := s.index(docs)
	s.remove.Label = "Delete folder"
	s.remove.Disabled = hasChildren(index.Folders(folder.Format), index.OfFormat(folder.Format), folder.Path)
}

// syncName keeps the name and tags fields showing what is selected. The
// guard has already asked about unsaved changes before the selection moved.
func (s *State) syncName(docs frame.Docs) {
	if s.list.Selected != s.named {
		s.focus.Set(nil)
		s.name.Field.Revert()
		s.tags.Field.Revert()
		s.tileSize.Field.Revert()
		s.named = s.list.Selected
	}
	if folder, ok := s.selectedFolder(); ok {
		s.name.Field.SetSaved(project.FolderName(folder.Path))
		s.tags.Field.SetSaved("")

		return
	}
	entry, ok := s.selected(docs)
	if !ok {
		return
	}
	s.name.Field.SetSaved(entry.Name)
	s.tags.Field.SetSaved(strings.Join(entry.Tags, ", "))
}

// guardSelection asks about unsaved changes before the inspector moves to
// another document or folder: the selection goes back until the author
// decides, and moves on after Save or Don't save.
func (s *State) guardSelection(ctx *frame.Context) {
	if s.named == "" || s.list.Selected == s.named {
		return
	}
	unsaved := s.Unsaved(ctx.Docs)
	if unsaved == 0 {
		return
	}
	name := s.UnsavedName(ctx.Docs)
	wanted := s.list.Selected
	s.list.Selected = s.named
	s.focus.Set(nil)
	s.guard.Ask(unsaved, name, func() { s.list.Selected = wanted })
}

// Unsaved counts the inspector's typed changes not saved yet: the name, a
// document's tags, a selected NPC's notes and the tile size.
func (s *State) Unsaved(frame.Docs) int {
	count := frame.UnsavedFields(s.name, s.tags, s.tileSize)
	if !s.npc.npc.IsZero() {
		count += frame.UnsavedFields(s.npc.notes)
	}

	return count
}

// Save saves the name, the tags, an NPC's notes and the tile size. They are
// separate changes, so saving several is several undo steps.
func (s *State) Save(ctx *frame.Context) bool {
	if s.tileSize.Field.Unsaved() {
		s.commitTileSize(ctx)
	}
	if s.name.Field.Unsaved() {
		s.commitName(ctx)
	}
	if s.tags.Field.Unsaved() {
		s.commitTags(ctx)
	}
	if !s.npc.npc.IsZero() && s.npc.notes.Field.Unsaved() {
		s.commitNotes(ctx)
	}

	return true
}

// UnsavedName names the document, folder or settings whose changes are
// unsaved.
func (s *State) UnsavedName(frame.Docs) string {
	if s.named == settingsRow {
		return art.SettingsName
	}
	if name := s.name.Field.Saved(); name != "" {
		return name
	}

	return "this document"
}

// Leave asks about unsaved changes before then, such as closing the window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

// discard drops the inspector's typed changes.
func (s *State) discard() {
	s.focus.Set(nil)
	s.name.Field.Revert()
	s.tags.Field.Revert()
	s.npc.notes.Field.Revert()
	s.tileSize.Field.Revert()
}

// commitName renames the selected document or folder.
func (s *State) commitName(ctx *frame.Context) {
	name, changed := s.name.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	name = strings.TrimSpace(name)
	if folder, ok := parseFolderRow(s.named); ok {
		s.renameFolder(ctx, folder, name)

		return
	}
	documentID, err := ids.Parse(s.named)
	if err != nil {
		return
	}
	rename := project.RenameArgs{ID: documentID, Name: name}
	s.command(ctx, project.IndexID(), project.RenameEdit, "", rename,
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Renamed to “" + name + "”")
		},
		func() { s.name.Field.Failed() })
}

// drawInspector draws what is selected: a document's type and name, its
// name field and ID, its tags and folder, its actions, its NPC panel, the
// save bar and Delete; or a folder's, a cue's or the settings'.
func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	left := panel.Min.X + ui.Inset
	if s.list.Selected == settingsRow {
		s.drawSettings(screen, left)

		return
	}
	if folder, ok := s.selectedFolder(); ok {
		s.drawFolder(screen, docs, folder, left)

		return
	}
	if cue, ok := s.selectedCue(); ok {
		s.drawCue(screen, cue, left)

		return
	}
	entry, ok := s.selected(docs)
	if !ok {
		s.theme.Heading(screen, "INSPECTOR", left, panel.Min.Y+ui.Inset)
		s.theme.DrawText(screen, "Select something in the list.", left, panel.Min.Y+ui.Inset+ui.LineHeight, ui.ColorMuted())
		s.theme.DrawText(screen, "Double-click or Enter opens it.", left, panel.Min.Y+ui.Inset+ui.Half*ui.LineHeight,
			ui.ColorMuted())

		return
	}
	about := kindOf(entry.Format)
	s.drawHeader(screen, about.icon, about.label, entry.Name, left)
	s.theme.Heading(screen, "NAME", left, nameFieldTop-ui.LineHeight)
	if s.name.Rect.Empty() {
		s.theme.DrawText(screen, "Every project holds it, so it keeps its name.", left,
			nameFieldTop+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
	} else {
		s.name.Draw(screen)
	}
	s.theme.DrawText(screen, "ID "+entry.ID.String(), left, nameFieldTop+ui.ButtonH+ui.Gap, ui.ColorSubdued())
	s.theme.Heading(screen, "TAGS", left, tagsFieldTop-ui.LineHeight)
	s.tags.Draw(screen)
	for _, chip := range s.tagBns {
		chip.Draw(screen, s.theme)
	}
	if !s.moveBn.Rect.Empty() {
		s.theme.Heading(screen, "FOLDER", left, s.moveBn.Rect.Min.Y-ui.LineHeight)
		where := showPath(formatSection(entry.Format), entry.Folder)
		s.theme.DrawText(screen, s.theme.FitText(where, s.moveBn.Rect.Min.X-ui.Gap-left), left,
			s.moveBn.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorInk())
	}
	for _, button := range s.inspectorButtons() {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
	s.drawNPC(screen, docs, left)
	s.drawArt(screen, left)
	s.save.Draw(screen, s.theme)
	s.moveMenu.Draw(screen, s.theme)
	s.zonePalettes.DrawMenu(screen)
}

// drawHeader draws the inspector's header: an icon, a type and a name.
func (s *State) drawHeader(screen *ebiten.Image, icon ui.Icon, label, name string, left int) {
	panel := frame.Inspector(s.size)
	ui.DrawIcon(screen, icon, image.Pt(left, panel.Min.Y+ui.Inset-1))
	s.theme.DrawLabel(screen, label, left+ui.IconSize+ui.Gap*ui.Half, panel.Min.Y+ui.Inset, ui.ColorMuted())
	s.theme.DrawTitle(screen, name, left, panel.Min.Y+ui.Inset+ui.LineHeight, ui.ColorInk())
	ui.Line(screen, image.Pt(left, headerRule), image.Pt(panel.Max.X-ui.Inset, headerRule), ui.Hairline, ui.ColorRule())
}

// drawFolder draws a folder's inspector: its name, what it holds, and its
// buttons.
func (s *State) drawFolder(screen *ebiten.Image, docs frame.Docs, folder project.Folder, left int) {
	section := formatSection(folder.Format)
	s.drawHeader(screen, ui.IconFolder, "Folder in "+strings.TrimPrefix(section, sectionPrefix),
		project.FolderName(folder.Path), left)
	s.theme.Heading(screen, "NAME", left, nameFieldTop-ui.LineHeight)
	s.name.Draw(screen)
	index := s.index(docs)
	held := countWithin(index.OfFormat(folder.Format), folder.Path)
	about := "Empty"
	if held > 0 {
		noun := strings.ToLower(kindOf(folder.Format).noun)
		about = fmt.Sprintf("Holds %d %s", held, plural(held, noun, noun+"s"))
	}
	s.theme.DrawText(screen, about, left, s.name.Rect.Max.Y+ui.Gap, ui.ColorSubdued())
	if s.remove.Disabled {
		s.theme.DrawText(screen, "Only an empty folder can be deleted.", left, s.name.Rect.Max.Y+ui.Gap+ui.LineHeight,
			ui.ColorMuted())
	}
	for _, button := range s.inspectorButtons() {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
	s.save.Draw(screen, s.theme)
	s.moveMenu.Draw(screen, s.theme)
}
