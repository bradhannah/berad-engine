package audioview

import (
	"image"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Window layout, in screen pixels.
const (
	toolbarHeight = 44
	listWidth     = 320
	toolbarGap    = 8
)

// layout places the toolbar, the list's controls, the list, the panel and
// the inspector.
func (s *State) layout() {
	content := frame.Content(s.size)
	top := (toolbarHeight - ui.ButtonH) / ui.Half
	right := content.Max.X - ui.Inset
	for _, button := range []*ui.Button{s.stopAll, s.checkFiles, s.newCue, s.addSound} {
		width := s.theme.TextWidth(button.Label) + ui.Half*ui.Inset
		button.Rect = image.Rect(right-width, top, right, top+ui.ButtonH)
		right -= width + toolbarGap
	}
	left, row := ui.Inset, toolbarHeight+ui.Inset/ui.Half
	s.spread(s.tabs, left, listWidth-ui.Inset, row)
	s.tabs[0].Label = "Cues · " + itoa(s.count(modeCues))
	s.tabs[1].Label = "Recordings · " + itoa(s.count(modeRecordings))
	for index, mode := range []string{modeCues, modeRecordings} {
		s.tabs[index].On = s.mode == mode
	}
	row += ui.ButtonH + ui.Gap*ui.Half
	s.spread(s.filters, left, listWidth-ui.Inset, row)
	for index, filter := range []string{filterAll, filterProblems, filterUncleared} {
		s.filters[index].On = s.filter == filter
	}
	row += ui.ButtonH + ui.Gap*ui.Half
	s.search.Rect = image.Rect(left, row, listWidth-ui.Inset, row+ui.ButtonH)
	row += ui.ButtonH + ui.Gap
	s.list.Rect = image.Rect(0, row, listWidth, content.Max.Y)
	s.list.Rows = s.rows()
	s.layoutPanel()
	s.layoutInspector()
}

// spread lays buttons out side by side across a width.
func (s *State) spread(buttons []*ui.Button, left, right, top int) {
	gaps := (len(buttons) - 1) * ui.Gap
	for index, button := range buttons {
		from := left + index*(right-left-gaps)/len(buttons) + index*ui.Gap
		to := left + (index+1)*(right-left-gaps)/len(buttons) + index*ui.Gap
		button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
	}
}

// Update handles one frame of input. The unsaved changes guard, a dialog or
// the delete question, while showing, takes all of it.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size, s.sounds = ctx.Size, ctx.Sounds
	s.layout()
	switch {
	case s.guard.Active():
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discard)

		return
	case s.dialog != nil:
		s.updateDialog(input, ctx)

		return
	case s.confirm != nil:
		s.updateConfirm(input, ctx)

		return
	case s.trim.dialog != nil:
		s.updateLevelDialog(input, ctx)

		return
	}
	s.inspector.sync(s, ctx)
	if s.syncTrim() {
		s.layout()
	}
	s.askTrim(ctx)
	s.syncRegionFields()
	s.updateFields(input, ctx)
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	s.updateToolbar(input, ctx)
	s.updateList(input, ctx)
	s.layout()
	s.updatePanel(input, ctx)
	s.updateInspector(input, ctx)
}

// updateFields handles the text fields: the search filters the list as it is
// typed and Escape clears it; Enter saves an inspector field.
func (s *State) updateFields(input *ui.Input, ctx *frame.Context) {
	field, outcome := s.focus.Update(input, ctx.Clipboard)
	switch {
	case field == s.search && outcome == textedit.OutcomeCommit:
		value, _ := s.search.Field.Commit()
		s.search.Field.SetSaved(value)
	case field == s.search && outcome == textedit.OutcomeBlur:
		s.search.Field.SetSaved("")
	case field != nil && field != s.search && outcome == textedit.OutcomeCommit:
		if !s.commitRegionField(field) {
			s.commitField(ctx, field)
		}
	}
}

// keys handles the window's keys: Cmd+F searches, Up and Down move the
// selection, and Space plays or stops what is selected.
func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	plain := ui.Plain()
	switch {
	case input.Consume(ui.KeyF, ui.Cmd()):
		s.focus.Set(s.search)
	case input.Consume(ui.KeyUp, plain):
		s.step(ctx, -1)
	case input.Consume(ui.KeyDown, plain):
		s.step(ctx, 1)
	case input.Consume(ui.KeySpace, plain):
		s.listenSelected(ctx)
	}
}

// listenSelected plays or stops the selected cue or recording.
func (s *State) listenSelected(ctx *frame.Context) {
	if cue, ok := s.selectedCue(); ok {
		s.listen(ctx, cue.ID)
	}
	if recording, ok := s.selectedRecording(); ok {
		s.listenRecording(ctx, recording.ID)
	}
}

// step moves the selection to the previous or next row.
func (s *State) step(ctx *frame.Context, delta int) {
	var selectable []string
	current := -1
	for _, row := range s.list.Rows {
		if row.Heading {
			continue
		}
		if row.ID == s.list.Selected {
			current = len(selectable)
		}
		selectable = append(selectable, row.ID)
	}
	next := current + delta
	if current < 0 {
		next = 0
	}
	if next >= 0 && next < len(selectable) {
		s.choose(ctx, selectable[next])
	}
}

// choose selects a row, first asking about typed changes to what was
// selected.
func (s *State) choose(ctx *frame.Context, row string) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), func() {
		s.list.Selected = row
		s.focus.Set(nil)
	})
}

// updateToolbar handles the toolbar and the list's tabs and filters.
func (s *State) updateToolbar(input *ui.Input, ctx *frame.Context) {
	switch {
	case s.addSound.Update(input):
		s.chooseSoundIn(ctx, soundTarget{cue: "", recording: ""}, s.categoryOfSelection())
	case s.newCue.Update(input):
		s.openNewCue(s.categoryOfSelection())
	case s.checkFiles.Update(input):
		s.check(ctx)
	case s.stopAll.Update(input):
		ctx.Sounds.Stop()
	}
	for index, mode := range []string{modeCues, modeRecordings} {
		if s.tabs[index].Update(input) && s.mode != mode {
			s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), func() {
				s.mode, s.list.Selected = mode, ""
			})
		}
	}
	for index, filter := range []string{filterAll, filterProblems, filterUncleared} {
		if s.filters[index].Update(input) {
			s.filter = filter
		}
	}
}

// categoryOfSelection is the category of the selected cue, which a new cue
// starts in, or SFX.
func (s *State) categoryOfSelection() string {
	if cue, ok := s.selectedCue(); ok {
		return cue.Category
	}

	return categories()[1]
}

// updateList handles the list: selecting, opening and closing sections, a
// category's [+], and a double click, which plays.
func (s *State) updateList(input *ui.Input, ctx *frame.Context) {
	previous := s.list.Selected
	result := s.list.Update(input)
	if result.Changed {
		chosen := s.list.Selected
		s.list.Selected = previous
		s.choose(ctx, chosen)
	}
	if result.Toggled != "" {
		s.closed[result.Toggled] = !s.closed[result.Toggled]
	}
	if category, ok := strings.CutPrefix(result.Added, categorySectionOf); ok {
		s.openNewCue(category)
	}
	if result.Activated {
		s.listenSelected(ctx)
	}
}
