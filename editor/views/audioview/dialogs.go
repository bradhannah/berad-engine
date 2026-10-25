package audioview

import (
	"image"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// What the sound dialog does.
type dialogKind int

const (
	// dialogAdd copies a chosen file in as a new cue, or as another recording
	// of a cue.
	dialogAdd dialogKind = iota
	// dialogNewCue makes a cue with no sound yet.
	dialogNewCue
	// dialogReplace copies a chosen file in for a recording.
	dialogReplace
	// dialogAttribution changes a recording's attribution.
	dialogAttribution
)

// reminder is shown with the commercial checkbox.
func reminder() []string {
	return []string{
		"Some free licences forbid commercial use, such as CC BY-NC.",
		"Tick this only when the licence allows use in a commercial game.",
	}
}

// Dialog layout, in screen pixels.
const (
	dialogWidth  = 480
	dialogRadius = 10
	dialogShadow = 6
	actionWidth  = 110
	idHint       = "Lowercase letters, digits, _ and -, in parts joined by /"
)

// soundTarget is what a chosen file is for: a cue to add it to, a
// recording to replace, or neither for a new cue.
type soundTarget struct {
	cue, recording string
}

// soundDialog asks what a new sound or cue is, and who made the sound.
type soundDialog struct {
	kind     dialogKind
	path     string
	target   soundTarget
	category string
	// name and id describe a new cue; autoID is the ID last suggested, kept
	// in step with the name until the author changes the ID.
	name, id                 *ui.TextField
	autoID                   string
	creator, source, license *ui.TextField
	categories               []*ui.Button
	commercial               *ui.Toggle
	primary, cancel          *ui.Button
	focus                    ui.Focus
	problem                  string
	waiting                  bool
	rect                     image.Rectangle
	// problemTop is where a problem is written, under everything else.
	problemTop int
}

// newDialog returns the dialog for a kind of change.
func (s *State) newDialog(kind dialogKind, path string, target soundTarget, category string) *soundDialog {
	field := func(id, placeholder string) *ui.TextField {
		made := ui.NewTextField(id, textedit.NewField("", false, textBytes), false, s.theme)
		made.Placeholder, made.Unmarked = placeholder, true

		return made
	}
	dialog := &soundDialog{
		kind: kind, path: path, target: target, category: category,
		name: field("cue-name", "Door opens"), id: field("cue-id", "sfx/door_open"), autoID: "",
		creator: field("creator", "Who made it"), source: field("source", "Where it came from, such as a web address"),
		license: field("license", "Its licence, such as CC0-1.0 or CC BY 4.0"), categories: nil,
		commercial: &ui.Toggle{ID: "commercial", Label: "Allowed in a commercial game", Rect: ui.Zero(), On: false},
		primary:    ui.DialogButton("dialog-ok", primaryLabel(kind)), cancel: ui.DialogButton(ui.DialogCancel, "Cancel"),
		focus: ui.Focus{}, problem: "", waiting: false, rect: ui.Zero(), problemTop: 0,
	}
	dialog.primary.Primary = true
	for _, category := range categories() {
		dialog.categories = append(dialog.categories, ui.NewButton("dialog-"+category, categoryLabel(category)))
	}
	if dialog.newCue() {
		dialog.focus.Register(dialog.name, dialog.id)
		dialog.name.Field.Restore(nameFromFile(path))
		dialog.suggestID()
		dialog.focus.Set(dialog.name)
	}
	if dialog.attributes() {
		dialog.focus.Register(dialog.creator, dialog.source, dialog.license)
	}

	return dialog
}

func primaryLabel(kind dialogKind) string {
	switch kind {
	case dialogNewCue:
		return "Create"
	case dialogReplace:
		return "Replace"
	case dialogAttribution:
		return "Save"
	case dialogAdd:
	}

	return "Add"
}

// newCue reports whether the dialog makes a cue.
func (dlg *soundDialog) newCue() bool {
	return dlg.kind == dialogNewCue || dlg.kind == dialogAdd && dlg.target.cue == ""
}

// attributes reports whether the dialog asks who made a sound.
func (dlg *soundDialog) attributes() bool { return dlg.kind != dialogNewCue }

// title says what the dialog does.
func (dlg *soundDialog) title() string {
	file := filepath.Base(dlg.path)
	switch {
	case dlg.kind == dialogNewCue:
		return "New cue"
	case dlg.kind == dialogAttribution:
		return "Attribution of “" + dlg.target.recording + "”"
	case dlg.kind == dialogReplace:
		return "Replace the file of “" + dlg.target.recording + "” with “" + file + "”"
	case dlg.target.cue != "":
		return "Add “" + file + "” to “" + dlg.target.cue + "”"
	}

	return "Add “" + file + "”"
}

// nameFromFile suggests a cue's name from a file's: its words, the first
// capitalised.
func nameFromFile(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	words := strings.FieldsFunc(base, func(r rune) bool { return r == '_' || r == '-' || unicode.IsSpace(r) })
	name := strings.Join(words, " ")
	if name == "" || path == "" {
		return ""
	}
	first := []rune(name)

	return string(unicode.ToUpper(first[0])) + string(first[1:])
}

// slug turns a name into an ID part: lowercase letters and digits joined by _.
func slug(name string) string {
	parts := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})

	return strings.Join(parts, "_")
}

// suggestID keeps the ID in step with the name and category, until the
// author changes it.
func (dlg *soundDialog) suggestID() {
	current := dlg.id.Field.Value()
	suggested := ""
	if part := slug(dlg.name.Field.Value()); part != "" {
		suggested = dlg.category + "/" + part
	}
	if current == dlg.autoID {
		dlg.id.Field.Restore(suggested)
	}
	dlg.autoID = suggested
}

// chooseSound asks for a file, then what it is for.
func (s *State) chooseSound(ctx *frame.Context, target soundTarget) {
	s.chooseSoundIn(ctx, target, protocol.CategorySFX)
}

func (s *State) chooseSoundIn(ctx *frame.Context, target soundTarget, category string) {
	ctx.Files.ChooseSound(func(path string, chosen bool) {
		if !chosen {
			return
		}
		kind := dialogAdd
		if target.recording != "" {
			kind = dialogReplace
		}
		s.dialog = s.newDialog(kind, path, target, category)
	})
}

// openAttribution edits a recording's attribution.
func (s *State) openAttribution(recording protocol.AudioRecording) {
	dialog := s.newDialog(dialogAttribution, "", soundTarget{cue: "", recording: recording.ID}, "")
	attribution := recording.Attribution
	for field, value := range map[*ui.TextField]string{
		dialog.creator: attribution.Creator, dialog.source: attribution.Source, dialog.license: attribution.License,
	} {
		if !unknown(value) {
			field.Field.Restore(value)
		}
	}
	dialog.commercial.On = attribution.Commercial
	s.dialog = dialog
}

// openNewCue makes a cue with no sound yet, in a category.
func (s *State) openNewCue(category string) {
	s.dialog = s.newDialog(dialogNewCue, "", soundTarget{cue: "", recording: ""}, category)
}

// layout places the dialog's parts, centred in the window: it places them
// once to learn the dialog's height, then again where they go.
func (dlg *soundDialog) layout(window image.Rectangle) {
	width := min(dialogWidth, window.Dx()-ui.Half*ui.Inset)
	left := window.Min.X + (window.Dx()-width)/ui.Half
	height := dlg.place(left, 0, width) + ui.Inset + ui.ButtonH + ui.Inset
	top := window.Min.Y + max((window.Dy()-height)/ui.Half, ui.Inset)
	dlg.place(left, top, width)
	dlg.rect = image.Rect(left, top, left+width, top+height)
	right := dlg.rect.Max.X - ui.Inset
	dlg.cancel.Rect = image.Rect(right-actionWidth, dlg.rect.Max.Y-ui.Inset-ui.ButtonH, right, dlg.rect.Max.Y-ui.Inset)
	dlg.primary.Rect = dlg.cancel.Rect.Sub(image.Pt(actionWidth+ui.Gap*ui.Half, 0))
}

// place lays the dialog's fields out from a top edge, each under its label,
// and returns where the last part ends.
func (dlg *soundDialog) place(left, top, width int) int {
	column, right, row := left+ui.Inset, left+width-ui.Inset, top+ui.Inset+ui.LineHeight
	if dlg.path != "" {
		row += ui.LineHeight
	}
	field := func(field *ui.TextField) {
		row += ui.LineHeight
		field.Rect = image.Rect(column, row, right, row+ui.ButtonH)
		row += ui.ButtonH + ui.Gap
	}
	for _, unplaced := range []*ui.TextField{dlg.name, dlg.id, dlg.creator, dlg.source, dlg.license} {
		unplaced.Rect = ui.Zero()
	}
	if dlg.newCue() {
		field(dlg.name)
		field(dlg.id)
		row += ui.LineHeight + ui.LineHeight
		dlg.layoutCategories(column, right, row)
		row += ui.ButtonH + ui.Inset
	}
	if dlg.attributes() {
		row += ui.LineHeight
		field(dlg.creator)
		field(dlg.source)
		field(dlg.license)
		dlg.commercial.Rect = image.Rect(column, row, right, row+ui.ButtonH)
		row += ui.ButtonH + len(reminder())*ui.LineHeight
	}
	dlg.problemTop = row
	if dlg.problem != "" {
		row += ui.LineHeight
	}

	return row - top
}

func (dlg *soundDialog) layoutCategories(left, right, top int) {
	gaps := (len(dlg.categories) - 1) * ui.Gap
	for index, button := range dlg.categories {
		from := left + index*(right-left-gaps)/len(dlg.categories) + index*ui.Gap
		to := left + (index+1)*(right-left-gaps)/len(dlg.categories) + index*ui.Gap
		button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
		button.On = dlg.category == categories()[index]
	}
}

// updateDialog handles a frame of the sound dialog, which takes all input.
func (s *State) updateDialog(input *ui.Input, ctx *frame.Context) {
	dlg := s.dialog
	dlg.layout(image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	defer input.ClearAll()
	field, outcome := dlg.focus.Update(input, ctx.Clipboard)
	if field == dlg.name {
		dlg.suggestID()
	}
	for index, button := range dlg.categories {
		if !button.Rect.Empty() && button.Update(input) {
			dlg.category = categories()[index]
			dlg.suggestID()
		}
	}
	dlg.commercial.Update(input)
	dlg.primary.Disabled = dlg.waiting
	s.closeOrSubmit(input, ctx, field, outcome)
}

// closeOrSubmit closes the dialog for Cancel or Escape, and sends its change
// for its button or Enter.
func (s *State) closeOrSubmit(input *ui.Input, ctx *frame.Context, field *ui.TextField, outcome textedit.Outcome) {
	dlg := s.dialog
	escape, enter := input.Has(ui.KeyEscape, ui.Plain()), input.Has(ui.KeyEnter, ui.Plain())
	switch {
	case dlg.cancel.Update(input) || escape && (field == nil || dlg.focus.Current() == nil):
		s.dialog = nil
	case dlg.primary.Update(input) || outcome == textedit.OutcomeCommit || field == nil && enter:
		if !dlg.waiting {
			s.submit(ctx)
		}
	}
}

// submit sends the dialog's change, and closes it once it is made.
func (s *State) submit(ctx *frame.Context) {
	dlg := s.dialog
	name, cueID := strings.TrimSpace(dlg.name.Field.Value()), strings.TrimSpace(dlg.id.Field.Value())
	if dlg.newCue() && name == "" {
		dlg.problem = "Give the cue a name."

		return
	}
	if dlg.newCue() && !validID(cueID) {
		dlg.problem = "The ID uses lowercase letters, digits, _ and -, in parts joined by /."

		return
	}
	attribution := protocol.Attribution{
		Creator: dlg.creator.Field.Value(), Source: dlg.source.Field.Value(), License: dlg.license.Field.Value(),
		Commercial: dlg.commercial.On,
	}
	cue := &protocol.NewCue{ID: cueID, Name: name, Category: dlg.category, HowToHear: ""}
	edit, args, selectNext := s.dialogEdit(dlg, cue, attribution)
	dlg.waiting, dlg.problem = true, ""
	if dlg.path != "" {
		ctx.Frame.Status("Copying “" + filepath.Base(dlg.path) + "” into the project…")
	}
	s.editIn(ctx, edit, args, func(protocol.Result) {
		if s.dialog == dlg {
			s.dialog = nil
		}
		if selectNext != "" {
			s.selectNext = selectNext
			s.inspector.listed(s)
		}
	}, func() {
		dlg.waiting, dlg.problem = false, "It could not be done; the status bar says why."
	})
}

// dialogEdit is the edit a dialog makes, and what to select once it is made.
func (s *State) dialogEdit(
	dlg *soundDialog, cue *protocol.NewCue, attribution protocol.Attribution,
) (string, any, string) {
	switch {
	case dlg.kind == dialogNewCue:
		return protocol.AudioCreateCue, *cue, cuePrefix + cue.ID
	case dlg.kind == dialogReplace:
		return protocol.AudioReplaceFile, protocol.ReplaceFile{
			Recording: dlg.target.recording, Path: dlg.path, Attribution: attribution,
		}, ""
	case dlg.kind == dialogAttribution:
		return protocol.AudioSetAttribution, protocol.AttributionChange{
			Recording: dlg.target.recording, Attribution: attribution,
		}, ""
	case dlg.target.cue != "":
		return protocol.AudioAddSound, protocol.AddSound{
			Cue: dlg.target.cue, NewCue: nil, Path: dlg.path, Attribution: attribution,
		}, ""
	}

	return protocol.AudioAddSound, protocol.AddSound{Cue: "", NewCue: cue, Path: dlg.path, Attribution: attribution},
		cuePrefix + cue.ID
}

// validID reports whether a cue ID is lowercase letters, digits, _ and -, in
// parts joined by /, as the orchestrator requires of new cues.
func validID(id string) bool {
	if id == "" {
		return false
	}
	for part := range strings.SplitSeq(id, "/") {
		if part == "" || strings.Trim(part, "abcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
			return false
		}
	}

	return true
}

// drawDialog draws the sound dialog over a shaded window.
func (s *State) drawDialog(screen *ebiten.Image) {
	dlg := s.dialog
	dlg.layout(screen.Bounds())
	ui.FillRect(screen, screen.Bounds(), ui.ColorShade())
	ui.FillRound(screen, dlg.rect.Add(image.Pt(0, dialogShadow)), dialogRadius, ui.ColorShade())
	ui.FillRound(screen, dlg.rect, dialogRadius, ui.ColorBackground())
	ui.StrokeRound(screen, dlg.rect, dialogRadius, ui.Border, ui.ColorInk())
	column, row := dlg.rect.Min.X+ui.Inset, dlg.rect.Min.Y+ui.Inset
	width := dlg.rect.Dx() - ui.Half*ui.Inset
	s.theme.DrawStrong(screen, s.theme.FitText(dlg.title(), width), column, row, ui.ColorInk())
	if dlg.path != "" {
		row += ui.LineHeight
		s.theme.DrawMono(screen, s.theme.FitText(dlg.path, width), column, row+ui.Gap, ui.ColorSubdued())
	}
	label := func(field *ui.TextField, text string) {
		s.theme.DrawLabel(screen, text, field.Rect.Min.X, field.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
		field.Draw(screen)
	}
	if dlg.newCue() {
		label(dlg.name, "NAME")
		label(dlg.id, "ID · CANNOT CHANGE LATER")
		s.theme.DrawText(screen, idHint, column, dlg.id.Rect.Max.Y+ui.Gap, ui.ColorSubdued())
		s.theme.DrawLabel(screen, "CATEGORY", column, dlg.categories[0].Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
		for _, button := range dlg.categories {
			button.Draw(screen, s.theme)
		}
	}
	if dlg.attributes() {
		heading := dlg.creator.Rect.Min.Y - ui.LineHeight*ui.Half + ui.Gap
		s.theme.DrawLabel(screen, "WHERE IT CAME FROM", column, heading, ui.ColorInk())
		label(dlg.creator, "CREATOR")
		label(dlg.source, "SOURCE")
		label(dlg.license, "LICENCE")
		dlg.commercial.Draw(screen, s.theme)
		for index, line := range reminder() {
			s.theme.DrawText(screen, line, column, dlg.commercial.Rect.Max.Y+index*ui.LineHeight, ui.ColorMuted())
		}
	}
	if dlg.problem != "" {
		s.theme.DrawText(screen, dlg.problem, column, dlg.problemTop, ui.ColorWarn())
	}
	dlg.primary.Draw(screen, s.theme)
	dlg.cancel.Draw(screen, s.theme)
}

// locateInDialog finds a dialog's field, button or the checkbox, for input
// scripts.
func (dlg *soundDialog) locate(name string) (image.Point, bool) {
	for _, field := range []*ui.TextField{dlg.name, dlg.id, dlg.creator, dlg.source, dlg.license} {
		if name == field.ID && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	buttons := append([]*ui.Button{dlg.primary, dlg.cancel}, dlg.categories...)
	for _, button := range buttons {
		if name == button.ID && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}
	if name == dlg.commercial.ID {
		return image.Pt(dlg.commercial.Rect.Min.X+ui.Inset/ui.Half, ui.Center(dlg.commercial.Rect).Y), true
	}

	return image.Pt(0, 0), false
}

// confirmDeleteCue asks where a cue is used, then whether to delete it.
func (s *State) confirmDeleteCue(ctx *frame.Context, cue protocol.AudioCue) {
	s.whereUsed(ctx, cue.ID, func(uses []protocol.Use) {
		lines := []string{"Its recordings stay in the library, unused."}
		if len(uses) > 0 {
			lines = append(lines, "These play it, and will play nothing:")
			for _, use := range uses {
				lines = append(lines, "  "+use.Name+" ("+use.Label+")")
			}
		}
		lines = append(lines, "You can undo this.")
		s.confirm = ui.NewDialog("Delete “"+cue.Name+"”?", lines, ui.DialogButton("delete", "Delete"),
			ui.DialogButton(ui.DialogCancel, "Cancel"))
		s.deleting = cuePrefix + cue.ID
	})
}

// confirmDeleteRecording asks whether to delete a recording no cue plays.
func (s *State) confirmDeleteRecording(recording protocol.AudioRecording) {
	s.confirm = ui.NewDialog("Delete “"+recording.ID+"”?", []string{
		"Its file stays in the project's audio folder.", "You can undo this.",
	}, ui.DialogButton("delete", "Delete"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.deleting = recordingPrefix + recording.ID
}

// updateConfirm handles the delete dialog.
func (s *State) updateConfirm(input *ui.Input, ctx *frame.Context) {
	choice := s.confirm.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	if choice == "" {
		return
	}
	s.confirm = nil
	if choice != "delete" {
		return
	}
	if id, ok := cutPrefix(s.deleting, cuePrefix); ok {
		s.edit(ctx, protocol.AudioDeleteCue, protocol.AudioItem{ID: id}, func(protocol.Result) {
			ctx.Frame.Status("Deleted the cue. Undo brings it back.")
		})
	}
	if id, ok := cutPrefix(s.deleting, recordingPrefix); ok {
		s.edit(ctx, protocol.AudioDeleteRecording, protocol.AudioItem{ID: id}, func(protocol.Result) {
			ctx.Frame.Status("Deleted the recording. Undo brings it back.")
		})
	}
	s.list.Selected = ""
}
