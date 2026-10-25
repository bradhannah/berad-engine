package audioview

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// inspectorButtons counts the inspector's buttons other than its categories.
const inspectorButtons = 8

// gainStep is how much the gain buttons change a cue's gain, which runs from
// 0 to 1.
const gainStep = 0.1

// Inspector layout, in screen pixels.
const (
	gainWidth  = 34
	labelLines = 1
)

// inspector is the right of the window: what can be changed about the
// selected cue or recording.
type inspector struct {
	name, how, terms, note *ui.TextField
	category               []*ui.Button
	quieter, louder        *ui.Button
	removeCue, removeRec   *ui.Button
	editAttribution        *ui.Button
	ambient                *ambientSection
	classes                *classifier
	// shown is the row the fields were filled from, and subject names it for
	// the unsaved changes guard; usesTop is where the cue's uses are listed.
	shown   string
	subject string
	usesTop int
	// uses are where the shown cue is used, once asked; usesFor is the cue
	// they belong to.
	uses    []protocol.Use
	usesFor string
}

func newInspector(theme *ui.Theme) *inspector {
	field := func(id, placeholder string, bytes int) *ui.TextField {
		made := ui.NewTextField(id, textedit.NewField("", false, bytes), false, theme)
		made.Placeholder = placeholder

		return made
	}
	made := &inspector{
		name: field("name", "Name", nameBytes), how: field("how", "How to hear it in the game", textBytes),
		terms: field("terms", "hinge, creak", termsBytes), note: field("note", "A note about this recording", textBytes),
		category: nil, quieter: ui.NewButton("quieter", "−"), louder: ui.NewButton("louder", "+"),
		removeCue: ui.NewButton("delete-cue", "Delete cue"), removeRec: ui.NewButton("delete-recording", "Delete recording"),
		editAttribution: ui.NewButton("edit-attribution", "Edit attribution…"),
		ambient:         newAmbientSection(theme), classes: newClassifier(),
		shown: "", subject: "", usesTop: 0, uses: nil, usesFor: "",
	}
	for _, category := range categories() {
		made.category = append(made.category, ui.NewButton("category-"+category, categoryLabel(category)))
	}
	made.removeCue.Danger, made.removeRec.Danger = true, true

	return made
}

func (insp *inspector) fields() []*ui.TextField {
	return append([]*ui.TextField{insp.name, insp.how, insp.terms, insp.note}, insp.ambient.fields()...)
}

func (insp *inspector) buttons() []*ui.Button {
	buttons := make([]*ui.Button, 0, len(insp.category)+inspectorButtons)
	buttons = append(buttons, insp.quieter, insp.louder, insp.removeCue, insp.removeRec, insp.editAttribution,
		insp.ambient.setDistance, insp.classes.profile, insp.classes.family)

	return append(buttons, insp.category...)
}

// sync fills the fields from what is selected when the selection changes,
// and asks where a newly shown cue is used.
func (insp *inspector) sync(view *State, ctx *frame.Context) {
	if view.list.Selected == insp.shown {
		return
	}
	insp.shown = view.list.Selected
	for _, field := range insp.fields() {
		field.Field.Revert()
	}
	view.focus.Set(nil)
	insp.listed(view)
	if cue, ok := view.selectedCue(); ok && insp.usesFor != cue.ID {
		insp.uses, insp.usesFor = nil, cue.ID
		view.whereUsed(ctx, cue.ID, func(uses []protocol.Use) {
			if insp.usesFor == cue.ID {
				insp.uses = uses
			}
		})
	}
}

// listed shows the saved values of what is selected, keeping typed drafts,
// and selects the item a change made once the library lists it.
func (insp *inspector) listed(view *State) {
	if view.selectNext != "" {
		if _, ok := view.rowOf(view.selectNext); ok {
			view.list.Selected, view.selectNext = view.selectNext, ""
		}
	}
	if cue, ok := view.selectedCue(); ok {
		insp.subject = cue.Name
		insp.name.Field.SetSaved(cue.Name)
		insp.how.Field.SetSaved(cue.HowToHear)
		insp.terms.Field.SetSaved(strings.Join(cue.SearchTerms, ", "))
		insp.ambient.listed(cue)
	}
	if recording, ok := view.selectedRecording(); ok {
		insp.subject = recording.ID
		insp.note.Field.SetSaved(recording.Note)
	}
}

// rowOf finds an item among the library's cues and recordings.
func (s *State) rowOf(item string) (string, bool) {
	if id, ok := cutPrefix(item, cuePrefix); ok {
		_, found := s.cue(id)

		return item, found
	}
	if id, ok := cutPrefix(item, recordingPrefix); ok {
		_, found := s.recording(id)

		return item, found
	}

	return "", false
}

// area is the inspector's part of the window.
func (s *State) inspectorArea() image.Rectangle { return frame.Inspector(s.size) }

// layoutInspector places the inspector's fields and buttons for what is
// selected, and leaves the others with no place.
func (s *State) layoutInspector() {
	insp, area := s.inspector, s.inspectorArea()
	insp.hide()
	if s.soundscape.shown {
		s.layoutSoundscape()

		return
	}
	s.soundscape.hide()
	left, right := area.Min.X+ui.Inset, area.Max.X-ui.Inset
	top := area.Min.Y + ui.Inset + ui.LineHeight
	place := func(field *ui.TextField) {
		top += ui.LineHeight
		field.Rect = image.Rect(left, top, right, top+ui.ButtonH)
		top += ui.ButtonH + ui.Inset
	}
	if cue, ok := s.selectedCue(); ok {
		place(insp.name)
		top += ui.LineHeight + ui.Inset
		s.layoutCategories(cue, left, right, top+ui.LineHeight)
		top += ui.LineHeight + ui.ButtonH + ui.Inset + ui.LineHeight
		insp.quieter.Rect = image.Rect(left, top, left+gainWidth, top+ui.ButtonH)
		insp.louder.Rect = image.Rect(right-gainWidth, top, right, top+ui.ButtonH)
		insp.quieter.Disabled, insp.louder.Disabled = cue.Gain <= 0, cue.Gain >= 1
		top += ui.ButtonH + ui.Inset
		if showsAmbient(cue) {
			top = s.layoutAmbient(cue, left, right, top)
		}
		place(insp.how)
		place(insp.terms)
		insp.usesTop = s.layoutProfile(cue, left, right, top)
		insp.removeCue.Rect = image.Rect(left, area.Max.Y-ui.Inset-ui.ButtonH, right, area.Max.Y-ui.Inset)

		return
	}
	if recording, ok := s.selectedRecording(); ok {
		top = factsTop(area) + ui.LineHeight*recordingFacts + ui.Gap
		insp.editAttribution.Rect = image.Rect(left, top, right, top+ui.ButtonH)
		top += ui.ButtonH + ui.Inset
		place(insp.note)
		s.layoutFamily(recording, left, right, top)
		insp.removeRec.Rect = image.Rect(left, area.Max.Y-ui.Inset-ui.ButtonH, right, area.Max.Y-ui.Inset)
		insp.removeRec.Disabled = len(s.usedBy(recording.ID)) > 0
	}
}

// hide leaves the inspector's fields and buttons with no place.
func (insp *inspector) hide() {
	for _, field := range insp.fields() {
		field.Rect = ui.Zero()
	}
	for _, button := range insp.buttons() {
		button.Rect = ui.Zero()
	}
	insp.ambient.curve = ui.Zero()
}

// recordingFacts counts the lines of facts about a recording above its
// attribution button: its file, original file, creator, source, licence and
// clearance, each a label and a value.
const recordingFacts = 12

// factsTop is where a recording's facts start, below the inspector's label.
func factsTop(area image.Rectangle) int { return area.Min.Y + ui.Inset + ui.LineHeight + ui.Inset }

// layoutCategories places the category buttons in a row, the cue's own on.
func (s *State) layoutCategories(cue protocol.AudioCue, left, right, top int) {
	buttons := s.inspector.category
	gaps := (len(buttons) - 1) * ui.Gap
	for index, button := range buttons {
		from := left + index*(right-left-gaps)/len(buttons) + index*ui.Gap
		to := left + (index+1)*(right-left-gaps)/len(buttons) + index*ui.Gap
		button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
		button.On = cue.Category == categories()[index]
	}
}

// updateInspector handles the inspector's buttons; its fields are handled
// with the window's other fields.
func (s *State) updateInspector(input *ui.Input, ctx *frame.Context) {
	insp := s.inspector
	if cue, ok := s.selectedCue(); ok {
		for index, button := range insp.category {
			if button.Update(input) && !button.On {
				s.edit(ctx, protocol.AudioSetCategory, protocol.CategoryChange{Cue: cue.ID, Category: categories()[index]}, nil)
			}
		}
		switch {
		case insp.quieter.Update(input):
			s.setGain(ctx, cue, cue.Gain-gainStep)
		case insp.louder.Update(input):
			s.setGain(ctx, cue, cue.Gain+gainStep)
		case insp.ambient.setDistance.Update(input):
			s.giveDistance(ctx, cue)
		case insp.classes.profile.Update(input):
			s.openProfiles()
		case insp.removeCue.Update(input):
			s.confirmDeleteCue(ctx, cue)
		}

		return
	}
	if recording, ok := s.selectedRecording(); ok {
		switch {
		case insp.editAttribution.Update(input):
			s.openAttribution(recording)
		case insp.classes.family.Update(input):
			s.openFamilies()
		case insp.removeRec.Update(input):
			s.confirmDeleteRecording(recording)
		}
	}
}

// setGain sets a cue's gain, rounded to the step and kept between 0 and 1.
func (s *State) setGain(ctx *frame.Context, cue protocol.AudioCue, gain float64) {
	gain = math.Round(min(max(gain, 0), 1)/gainStep) * gainStep
	gain = math.Round(gain*percent) / percent
	s.edit(ctx, protocol.AudioSetCue, protocol.CueChange{
		Cue: cue.ID, Name: nil, Gain: &gain, HowToHear: nil, SearchTerms: nil,
	}, nil)
}

const percent = 100

// commitField saves a field of the inspector after Enter.
func (s *State) commitField(ctx *frame.Context, field *ui.TextField) {
	if s.commitAmbient(ctx, field) {
		return
	}
	value, changed := field.Field.Commit()
	if !changed {
		return
	}
	failed := field.Field.Failed
	if cue, ok := s.selectedCue(); ok {
		change := protocol.CueChange{Cue: cue.ID, Name: nil, Gain: nil, HowToHear: nil, SearchTerms: nil}
		switch field {
		case s.inspector.name:
			change.Name = &value
		case s.inspector.how:
			change.HowToHear = &value
		case s.inspector.terms:
			terms := splitTerms(value)
			change.SearchTerms = &terms
		}
		s.editIn(ctx, protocol.AudioSetCue, change, nil, failed)

		return
	}
	if recording, ok := s.selectedRecording(); ok && field == s.inspector.note {
		s.editIn(ctx, protocol.AudioSetRecording, protocol.RecordingChange{
			Recording: recording.ID, Note: &value, SearchTerms: nil,
		}, nil, failed)
	}
}

// splitTerms splits comma-separated search terms.
func splitTerms(value string) []string {
	terms := []string{}
	for term := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(term); trimmed != "" {
			terms = append(terms, trimmed)
		}
	}

	return terms
}

// saveAll saves every typed change of the inspector, for Cmd+S and the
// unsaved changes guard.
func (insp *inspector) saveAll(view *State, ctx *frame.Context) bool {
	for _, field := range insp.fields() {
		if field.Field.Unsaved() {
			view.commitField(ctx, field)
		}
	}

	return true
}

// drawInspector draws what can be changed about the selection.
func (s *State) drawInspector(screen *ebiten.Image) {
	insp, area := s.inspector, s.inspectorArea()
	frame.DrawInspector(screen)
	left, top := area.Min.X+ui.Inset, area.Min.Y+ui.Inset
	cue, isCue := s.selectedCue()
	recording, isRecording := s.selectedRecording()
	switch {
	case isCue:
		s.theme.DrawLabel(screen, "INSPECTOR · CUE", left, top, ui.ColorMuted())
		s.drawFieldLabel(screen, insp.name, "NAME")
		s.theme.DrawLabel(screen, "ID", left, insp.name.Rect.Max.Y+ui.Inset, ui.ColorMuted())
		s.theme.DrawMono(screen, cue.ID, left+ui.Half*ui.Inset, insp.name.Rect.Max.Y+ui.Inset, ui.ColorSubdued())
		s.theme.DrawLabel(screen, "CATEGORY", left, insp.category[0].Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
		s.theme.DrawLabel(screen, "CUE GAIN", left, insp.quieter.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
		gain := strconv.Itoa(int(math.Round(cue.Gain*percent))) + " %"
		s.theme.DrawTextIn(screen, gain, image.Rect(insp.quieter.Rect.Max.X, insp.quieter.Rect.Min.Y, insp.louder.Rect.Min.X,
			insp.louder.Rect.Max.Y), ui.ColorInk())
		s.drawFieldLabel(screen, insp.how, "HOW TO HEAR IT")
		s.drawFieldLabel(screen, insp.terms, "SEARCH TERMS")
		s.drawUses(screen, left, insp.usesTop, insp.removeCue.Rect.Min.Y-ui.Inset)
		s.drawButtons(screen, insp.quieter, insp.louder, insp.removeCue)
		s.drawButtons(screen, insp.category...)
		s.drawFields(screen, insp.name, insp.how, insp.terms)
		if showsAmbient(cue) {
			s.drawAmbient(screen, cue, left)
		}
		s.drawProfile(screen, cue)
	case isRecording:
		s.theme.DrawLabel(screen, "INSPECTOR · RECORDING", left, top, ui.ColorMuted())
		s.drawFacts(screen, recording, left, factsTop(s.inspectorArea()))
		s.drawFieldLabel(screen, insp.note, "NOTE")
		if insp.removeRec.Disabled {
			s.theme.DrawText(screen, "Only a recording no cue plays can be deleted.", left,
				insp.removeRec.Rect.Min.Y-ui.LineHeight, ui.ColorMuted())
		}
		s.drawButtons(screen, insp.editAttribution, insp.removeRec)
		s.drawFields(screen, insp.note)
		s.drawFamily(screen)
	default:
		s.theme.DrawLabel(screen, "INSPECTOR", left, top, ui.ColorMuted())
	}
}

func (s *State) drawFieldLabel(screen *ebiten.Image, field *ui.TextField, label string) {
	s.theme.DrawLabel(screen, label, field.Rect.Min.X, field.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
}

func (s *State) drawFields(screen *ebiten.Image, fields ...*ui.TextField) {
	for _, field := range fields {
		field.Draw(screen)
	}
}

// drawUses lists the workflows that play the cue, as many as fit above
// bottom, then how many more there are.
func (s *State) drawUses(screen *ebiten.Image, left, top, bottom int) {
	s.theme.DrawLabel(screen, "USED BY", left, top, ui.ColorMuted())
	top += ui.LineHeight
	uses := s.inspector.uses
	if len(uses) == 0 {
		s.theme.DrawText(screen, "Nothing uses this cue yet.", left, top, ui.ColorSubdued())

		return
	}
	width := s.inspectorArea().Dx() - ui.Half*ui.Inset
	fit := max((bottom-top)/ui.LineHeight, 1)
	if len(uses) > fit {
		uses = uses[:fit-1]
	}
	for _, use := range uses {
		s.theme.DrawText(screen, s.theme.FitText(use.Name+" · "+use.Label, width), left, top, ui.ColorInk())
		top += ui.LineHeight
	}
	if more := len(s.inspector.uses) - len(uses); more > 0 {
		s.theme.DrawText(screen, "and "+itoa(more)+" more", left, top, ui.ColorMuted())
	}
}

// drawFacts shows a recording's file and where it came from.
func (s *State) drawFacts(screen *ebiten.Image, recording protocol.AudioRecording, left, top int) {
	attribution := recording.Attribution
	cleared, ink := "Not cleared", ui.ColorWarn()
	if attribution.Commercial {
		cleared, ink = "Cleared", ui.ColorInk()
	}
	width := s.inspectorArea().Dx() - ui.Half*ui.Inset
	facts := []struct {
		label, value string
		ink          color.RGBA
	}{
		{"FILE", orNone(recording.File), ui.ColorMuted()},
		{"COPIED FROM", orNone(recording.OriginalFile), ui.ColorMuted()},
		{"CREATOR", attribution.Creator, ui.ColorInk()},
		{"SOURCE", attribution.Source, ui.ColorInk()},
		{"LICENCE", attribution.License, ui.ColorInk()},
		{"COMMERCIAL GAME", cleared, ink},
	}
	for _, fact := range facts {
		s.theme.DrawLabel(screen, fact.label, left, top, ui.ColorMuted())
		top += ui.LineHeight - ui.Gap
		ink := fact.ink
		if unknown(fact.value) {
			ink = ui.ColorSubdued()
		}
		s.theme.DrawText(screen, s.theme.FitText(fact.value, width), left, top, ink)
		top += ui.LineHeight + ui.Gap
	}
}

func orNone(value string) string {
	if value == "" {
		return "none"
	}

	return value
}
