package audioview

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/loudness"
)

// inspectorButtons counts the inspector's buttons other than its categories.
const inspectorButtons = 8

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
	// body is how far the fields are scrolled between the inspector's label
	// and Delete at its foot (increment 61).
	body ui.Scroll
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
		ambient: newAmbientSection(theme), classes: newClassifier(),
		shown: "", subject: "", usesTop: 0, uses: nil, usesFor: "", body: ui.NewScroll(),
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
	buttons = append(buttons, insp.quieter, insp.louder, insp.removeCue, insp.removeRec, insp.ambient.setDistance,
		insp.classes.profile, insp.classes.family)

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
	insp.body.For(s.list.Selected)
	insp.body.Body = ui.Zero()
	foot := area.Max.Y - ui.Inset - ui.ButtonH
	footed := image.Rect(left, foot, right, area.Max.Y-ui.Inset)
	insp.body.Body = image.Rect(area.Min.X, bodyTop(area), area.Max.X, max(bodyTop(area), foot-ui.Inset))
	if cue, ok := s.selectedCue(); ok {
		insp.removeCue.Rect = footed
		s.fitInspector(s.layoutCueFields(cue, left, right, bodyTop(area)-insp.body.Offset))

		return
	}
	if recording, ok := s.selectedRecording(); ok {
		insp.removeRec.Rect = footed
		insp.removeRec.Disabled = len(s.usedBy(recording.ID)) > 0
		if insp.removeRec.Disabled {
			insp.body.Body.Max.Y = max(insp.body.Body.Min.Y, foot-ui.LineHeight-ui.Inset)
		}
		top := factsTop(area) - insp.body.Offset + ui.LineHeight*recordingFacts + ui.Gap
		top = s.placeField(insp.note, left, right, top)
		s.fitInspector(s.layoutFamily(recording, left, right, top))
	}
}

// placeField places a field under its label from a top edge, and returns
// where the next part goes.
func (s *State) placeField(field *ui.TextField, left, right, top int) int {
	top += ui.LineHeight
	field.Rect = image.Rect(left, top, right, top+ui.ButtonH)

	return top + ui.ButtonH + ui.Inset
}

// layoutCueFields places a cue's fields and buttons from a top edge, and
// returns where its uses end.
func (s *State) layoutCueFields(cue protocol.AudioCue, left, right, top int) int {
	insp := s.inspector
	top = s.placeField(insp.name, left, right, top)
	top += ui.LineHeight + ui.Inset
	s.layoutCategories(cue, left, right, top+ui.LineHeight)
	top += ui.LineHeight + ui.ButtonH + ui.Inset + ui.LineHeight
	insp.quieter.Rect = image.Rect(left, top, left+gainWidth, top+ui.ButtonH)
	insp.louder.Rect = image.Rect(right-gainWidth, top, right, top+ui.ButtonH)
	_, quieter := s.nextStep(cue, -1)
	_, louder := s.nextStep(cue, 1)
	insp.quieter.Disabled, insp.louder.Disabled = !quieter, !louder
	top += ui.ButtonH + ui.LineHeight + ui.Inset
	if showsAmbient(cue) {
		top = s.layoutAmbient(cue, left, right, top)
	}
	top = s.placeField(insp.how, left, right, top)
	top = s.placeField(insp.terms, left, right, top)
	insp.usesTop = s.layoutProfile(cue, left, right, top)

	return insp.usesTop + (len(s.usesShown())+1)*ui.LineHeight + ui.Inset
}

// bodyTop is where the inspector's fields start, under its label.
func bodyTop(area image.Rectangle) int { return area.Min.Y + ui.Inset + ui.LineHeight }

// fitInspector keeps the inspector scrolled no further than its fields,
// ending at bottom, need, and lays it out again when that moved it.
func (s *State) fitInspector(bottom int) {
	if s.inspector.body.Fit(bottom) {
		s.layoutInspector()
	}
}

// updateInspectorFoot scrolls the inspector's fields with the wheel over it,
// and gives a press beside them, on Delete or the label, to those alone,
// never to a field scrolled behind them (increment 61). It reports whether
// the press did something.
func (s *State) updateInspectorFoot(input *ui.Input, ctx *frame.Context) bool {
	insp := s.inspector
	if s.soundscape.shown || s.loud.shown {
		return false
	}
	if insp.body.Wheel(input, s.inspectorArea()) {
		s.layout()
	}
	if !insp.body.Hidden(input.Cursor) {
		return false
	}
	defer input.ClearPointer()
	if cue, ok := s.selectedCue(); ok && insp.removeCue.Update(input) {
		s.confirmDeleteCue(ctx, cue)

		return true
	}
	if recording, ok := s.selectedRecording(); ok && insp.removeRec.Update(input) {
		s.confirmDeleteRecording(recording)

		return true
	}

	return false
}

// usesShown are the uses the inspector lists, at most usesListed; the rest
// it counts.
func (s *State) usesShown() []protocol.Use {
	uses := s.inspector.uses
	if len(uses) > usesListed {
		uses = uses[:usesListed-1]
	}

	return uses
}

// usesListed is how many uses the inspector lists before counting the rest.
const usesListed = 8

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

// recordingFacts counts the lines about a recording's file above its note:
// a label and a value. Its attribution is in the panel.
const recordingFacts = 2

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
			s.stepCue(ctx, cue, -1)
		case insp.louder.Update(input):
			s.stepCue(ctx, cue, 1)
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
		case insp.classes.family.Update(input):
			s.openFamilies()
		case insp.removeRec.Update(input):
			s.confirmDeleteRecording(recording)
		}
	}
}

// nextStep is the step quieter or louder than the one a cue plays at, by
// direction; from between steps, the nearest that way.
func (s *State) nextStep(cue protocol.AudioCue, direction int) (loudness.Step, bool) {
	_, now, _ := s.cueStep(cue)
	steps := loudness.Steps()
	if direction < 0 {
		for index := len(steps) - 1; index >= 0; index-- {
			if steps[index].DB < now-evenTolerance/ui.Half {
				return steps[index], true
			}
		}

		return steps[0], false
	}
	for _, step := range steps {
		if step.DB > now+evenTolerance/ui.Half {
			return step, true
		}
	}

	return steps[len(steps)-1], false
}

// stepCue sets how loud a cue is meant to sound, a step quieter or louder.
func (s *State) stepCue(ctx *frame.Context, cue protocol.AudioCue, direction int) {
	step, found := s.nextStep(cue, direction)
	if !found {
		return
	}
	s.edit(ctx, protocol.AudioSetLoudness, protocol.LoudnessChange{Cue: cue.ID, Step: step.ID},
		func(protocol.Result) {
			ctx.Frame.Status(cue.Name + " is meant to sound " + step.Label + ". Undo puts it back.")
		})
}

const percent = 100

// drawMeantToSound draws the step a cue plays at between its buttons, and
// its category's adjustment under them.
func (s *State) drawMeantToSound(screen *ebiten.Image, cue protocol.AudioCue) {
	insp := s.inspector
	left := insp.quieter.Rect.Min.X
	s.theme.DrawLabel(screen, "MEANT TO SOUND", left, insp.quieter.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
	between := image.Rect(insp.quieter.Rect.Max.X, insp.quieter.Rect.Min.Y, insp.louder.Rect.Min.X, insp.louder.Rect.Max.Y)
	label := s.stepLabel(cue)
	step, stepDB, found := s.cueStep(cue)
	if found && step.DB != 0 {
		label += " · " + decibels(step.DB)
	}
	if !found {
		step = loudness.Step{ID: "", Label: "", DB: stepDB}
	}
	const pipsWidth = 24
	width := pipsWidth + ui.Gap + s.theme.TextWidth(label)
	start := between.Min.X + max((between.Dx()-width)/ui.Half, ui.Gap)
	drawStepPips(screen, start, between.Min.Y, step, ui.ColorSelect())
	s.theme.DrawText(screen, s.theme.FitText(label, between.Max.X-start-pipsWidth-ui.Gap), start+pipsWidth+ui.Gap,
		between.Min.Y+ui.Gap, ui.ColorInk())
	note := categoryLabel(cue.Category) + " adjustment " + signedDB(s.adjustment(cue.Category)) + " · in Loudness…"
	s.theme.DrawText(screen, s.theme.FitText(note, s.inspectorArea().Dx()-ui.Half*ui.Inset), left,
		insp.quieter.Rect.Max.Y+ui.Gap, ui.ColorMuted())
}

// noAdjustmentDB is how near 0 dB an adjustment shows as none.
const noAdjustmentDB = 0.05

// signedDB writes an adjustment, ±0 dB when there is none.
func signedDB(value float64) string {
	if math.Abs(value) < noAdjustmentDB {
		return "±0 dB"
	}

	return decibels(value)
}

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
		if body, ok := insp.body.Clip(screen); ok {
			s.drawCueBody(body, cue, left)
		}
		s.drawButtons(screen, insp.removeCue)
		insp.body.DrawThumb(screen)
	case isRecording:
		s.theme.DrawLabel(screen, "INSPECTOR · RECORDING", left, top, ui.ColorMuted())
		if body, ok := insp.body.Clip(screen); ok {
			s.drawFacts(body, recording, left, factsTop(s.inspectorArea())-insp.body.Offset)
			s.drawFieldLabel(body, insp.note, "NOTE")
			s.drawFields(body, insp.note)
			s.drawFamily(body)
		}
		if insp.removeRec.Disabled {
			s.theme.DrawText(screen, "Only a recording no cue plays can be deleted.", left,
				insp.removeRec.Rect.Min.Y-ui.LineHeight, ui.ColorMuted())
		}
		s.drawButtons(screen, insp.removeRec)
		insp.body.DrawThumb(screen)
	default:
		s.theme.DrawLabel(screen, "INSPECTOR", left, top, ui.ColorMuted())
	}
}

// drawCueBody draws a cue's fields, which scroll between the inspector's
// label and Delete cue.
func (s *State) drawCueBody(body *ebiten.Image, cue protocol.AudioCue, left int) {
	insp := s.inspector
	s.drawFieldLabel(body, insp.name, "NAME")
	s.theme.DrawLabel(body, "ID", left, insp.name.Rect.Max.Y+ui.Inset, ui.ColorMuted())
	s.theme.DrawMono(body, cue.ID, left+ui.Half*ui.Inset, insp.name.Rect.Max.Y+ui.Inset, ui.ColorSubdued())
	s.theme.DrawLabel(body, "CATEGORY", left, insp.category[0].Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
	s.drawMeantToSound(body, cue)
	s.drawFieldLabel(body, insp.how, "HOW TO HEAR IT")
	s.drawFieldLabel(body, insp.terms, "SEARCH TERMS")
	s.drawUses(body, left, insp.usesTop)
	s.drawButtons(body, insp.quieter, insp.louder)
	s.drawButtons(body, insp.category...)
	s.drawFields(body, insp.name, insp.how, insp.terms)
	if showsAmbient(cue) {
		s.drawAmbient(body, cue, left)
	}
	s.drawProfile(body, cue)
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
func (s *State) drawUses(screen *ebiten.Image, left, top int) {
	s.theme.DrawLabel(screen, "USED BY", left, top, ui.ColorMuted())
	top += ui.LineHeight
	uses := s.usesShown()
	if len(uses) == 0 {
		s.theme.DrawText(screen, "Nothing uses this cue yet.", left, top, ui.ColorSubdued())

		return
	}
	width := s.inspectorArea().Dx() - ui.Half*ui.Inset
	for _, use := range uses {
		s.theme.DrawText(screen, s.theme.FitText(use.Name+" · "+use.Label, width), left, top, ui.ColorInk())
		top += ui.LineHeight
	}
	if more := len(s.inspector.uses) - len(uses); more > 0 {
		s.theme.DrawText(screen, "and "+itoa(more)+" more", left, top, ui.ColorMuted())
	}
}

// drawFacts shows a recording's file.
func (s *State) drawFacts(screen *ebiten.Image, recording protocol.AudioRecording, left, top int) {
	width := s.inspectorArea().Dx() - ui.Half*ui.Inset
	facts := []struct {
		label, value string
		ink          color.RGBA
	}{
		{"FILE", orNone(recording.File), ui.ColorMuted()},
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
