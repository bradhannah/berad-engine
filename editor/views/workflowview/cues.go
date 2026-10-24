package workflowview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A cue parameter shows the chosen cue's name and ID with Play beside it;
// clicking the cue opens the chooser under it: a search box and the audio
// library's cues grouped by category, each with a button to hear it.
const (
	listenWidth  = 64
	rowPlayWidth = 22
	rowPlayInset = 3
	glyphSize    = 8
	caretWidth   = 10
	maxCueSearch = 100
	// Row IDs of the chooser's category headings start with headingPrefix.
	headingPrefix = "category:"
	// locateCue and locatePlay name a chooser row and its play button for
	// input scripts.
	locateCue  = "cue"
	locatePlay = "play"
)

// Problems a Play sound can have.
const (
	problemNoCue      = "No cue chosen yet."
	problemMissingCue = "Its cue is not in the audio library. Choose another, or add it with the terminal audio editor."
)

// cueCategories are the chooser's groups, in order.
func cueCategories() []flow.Choice {
	return []flow.Choice{
		{Value: protocol.CategoryMusic, Label: "Music"}, {Value: protocol.CategorySFX, Label: "SFX"},
		{Value: protocol.CategoryAmbient, Label: "Ambient"},
	}
}

// askAudio asks for the project's audio library, unless a question is out.
func (s *State) askAudio(ctx *frame.Context) {
	if s.audioAsking {
		return
	}
	s.audioAsking = true
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		s.audioAsking = false
		if err != nil {
			ctx.Frame.Status("The audio library could not be listed: " + err.Error())

			return
		}
		s.audio = &library
		s.audioRevision++
	})
}

// playsSounds reports whether the workflow has a node with a cue.
func (s *State) playsSounds() bool {
	for _, node := range s.workflow.Nodes() {
		if node.Kind == flow.KindPlaySound {
			return true
		}
	}

	return false
}

// listedCue finds a cue in the audio library as last listed.
func (s *State) listedCue(id string) (protocol.AudioCue, bool) {
	if s.audio != nil {
		for _, cue := range s.audio.Cues {
			if cue.ID == id {
				return cue, true
			}
		}
	}

	var none protocol.AudioCue

	return none, false
}

// chosenCue returns a cue parameter's cue, or "" for none.
func chosenCue(node flow.Node, name string) string {
	cue, err := node.Cue(name)
	if err != nil {
		return ""
	}

	return cue
}

// cueLabel names a cue parameter's value and gives its ID, or says what is
// wrong with it.
func (s *State) cueLabel(cueID string) (string, string) {
	if cueID == "" {
		return "No cue yet", ""
	}
	if s.audio == nil {
		return "…", cueID
	}
	if cue, found := s.listedCue(cueID); found {
		return cue.Name, cueID
	}

	return "Missing cue", cueID
}

// cueProblem says what is wrong with a Play sound's cue. A library not
// listed yet is not a problem until it arrives.
func (s *State) cueProblem(node flow.Node) string {
	cueID, err := node.Cue("cue")
	switch {
	case err != nil || cueID == "":
		return problemNoCue
	case s.audio == nil:
		return ""
	case s.audio.Problem != "":
		return s.audio.Problem
	}
	if _, found := s.listedCue(cueID); !found {
		return problemMissingCue
	}

	return ""
}

// cueState says why a cue cannot be heard: "" when it can, "silent" for an
// intentional silent slot, or a recording's status.
func cueState(cue protocol.AudioCue) string {
	state := "silent"
	for _, recording := range cue.Recordings {
		switch recording.Status {
		case protocol.RecordingReady:
			state = ""
		case protocol.RecordingNoFile:
		default:
			return recording.Status
		}
	}

	return state
}

// audible reports whether a cue can be heard.
func (s *State) audible(id string) bool {
	cue, found := s.listedCue(id)

	return found && cueState(cue) == ""
}

// newCueControl makes a cue parameter's widgets; the search box takes part
// in focus only while the chooser shows it.
func (s *State) newCueControl(made *control) {
	made.listen = ui.NewButton("listen", "Play")
	made.search = ui.NewTextField("cue-search", textedit.NewField("", false, maxCueSearch), false, s.theme)
	made.search.Placeholder, made.search.Unmarked = "Find a cue by name or ID", true
	made.list = &ui.List{ID: "cues", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil}
	s.focus.Register(made.search)
}

// layoutCue places the cue, Play and, when open, the chooser, which takes
// the inspector's height down to bottom, where the problem and hint start.
// It returns where the next control goes.
func (s *State) layoutCue(made *control, left, width, top, bottom int) int {
	made.cueBox = image.Rect(left, top, left+width-listenWidth-rowSpace, top+ui.ButtonH)
	made.listen.Rect = image.Rect(left+width-listenWidth, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + rowSpace
	made.search.Rect, made.list.Rect = ui.Zero(), ui.Zero()
	if !made.open {
		return top + fieldSpace
	}
	made.search.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + rowSpace
	made.list.Rows = s.cueRows(made.search.Field.Value())
	made.list.Rect = image.Rect(left, top, left+width, max(top+ui.RowHeight, bottom-ui.Gap))

	return made.list.Rect.Max.Y + fieldSpace
}

// cueRows lists the cues a search matches, by name, ID or search term,
// under their category's heading. A cue that cannot be heard says why.
func (s *State) cueRows(query string) []ui.Row {
	if s.audio == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var rows []ui.Row
	for _, category := range cueCategories() {
		var matching []ui.Row
		for _, cue := range s.audio.Cues {
			if cue.Category != category.Value || !matchesCue(cue, query) {
				continue
			}
			row := ui.NewRow(cue.ID, cue.Name)
			row.Detail = cue.ID
			if state := cueState(cue); state != "" {
				row.Detail += " · " + state
			}
			matching = append(matching, row)
		}
		if len(matching) > 0 {
			rows = append(append(rows, ui.NewHeading(headingPrefix+category.Value, category.Label)), matching...)
		}
	}

	return rows
}

func matchesCue(cue protocol.AudioCue, query string) bool {
	if query == "" || strings.Contains(strings.ToLower(cue.Name), query) ||
		strings.Contains(strings.ToLower(cue.ID), query) {
		return true
	}
	for _, term := range cue.SearchTerms {
		if strings.Contains(strings.ToLower(term), query) {
			return true
		}
	}

	return false
}

// rowPlayRect is the button that plays a cue's row, at the row's right end.
func rowPlayRect(row image.Rectangle) image.Rectangle {
	return image.Rect(row.Max.X-rowPlayInset-rowPlayWidth, row.Min.Y+rowPlayInset, row.Max.X-rowPlayInset,
		row.Max.Y-rowPlayInset)
}

// rowAt returns the cue row under a point, or "".
func rowAt(list *ui.List, point image.Point) (string, image.Rectangle) {
	if !point.In(list.Rect) {
		return "", ui.Zero()
	}
	for index, row := range list.Rows {
		if rect := list.RowRect(index); !row.Heading && point.In(rect) {
			return row.ID, rect
		}
	}

	return "", ui.Zero()
}

// updateCue plays or stops the chosen cue, opens and closes the chooser,
// plays a row's cue from its button, and chooses the cue clicked.
func (s *State) updateCue(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	chosen := chosenCue(node, made.param.Name)
	made.listen.Label = "Play"
	if chosen != "" && ctx.Sounds.Playing(chosen) {
		made.listen.Label = "Stop"
	}
	made.listen.Disabled = !s.audible(chosen)
	if made.listen.Update(input) {
		s.listen(ctx, chosen)

		return
	}
	if input.Pressed && input.Cursor.In(made.cueBox) {
		input.ClearPointer()
		if made.open {
			s.closeChooser(made)

			return
		}
		made.open, made.list.Selected = true, chosen
		s.focus.Set(made.search)
		s.askAudio(ctx)

		return
	}
	if !made.open {
		return
	}
	row, rect := rowAt(made.list, input.Cursor)
	if input.Pressed && row != "" && s.audible(row) && input.Cursor.In(rowPlayRect(rect)) {
		input.ClearPointer()
		s.listen(ctx, row)

		return
	}
	pressed := input.Pressed
	made.list.Update(input)
	if pressed && row != "" {
		s.closeChooser(made)
		if row != chosen {
			s.saveParam(ctx, node, made.param.Name, row)
			name, _ := s.cueLabel(row)
			ctx.Frame.Status("This Play sound now plays “" + name + "”")
		}
	}
}

// listen plays a cue, stopping whatever this window plays, or stops it when
// it is playing.
func (s *State) listen(ctx *frame.Context, cue string) {
	playing := ctx.Sounds.Playing(cue)
	ctx.Sounds.Stop()
	if !playing {
		ctx.Sounds.Play(cue)
	}
}

// closeChooser closes the chooser and leaves its search box.
func (s *State) closeChooser(made *control) {
	made.open = false
	if s.focus.Current() == made.search {
		s.focus.Set(nil)
	}
	made.search.Field.Revert()
	made.search.Field.SetSaved("")
}

// openChooser returns the cue control whose chooser is open.
func (s *State) openChooser() (*control, bool) {
	if s.form == nil {
		return nil, false
	}
	for index := range s.form.controls {
		if made := &s.form.controls[index]; made.param.Type == flow.TypeCue && made.open {
			return made, true
		}
	}

	return nil, false
}

// drawCue draws the chosen cue with its ID, subdued (D4), Play, and the
// chooser when open.
func (s *State) drawCue(screen *ebiten.Image, made *control, node flow.Node) {
	chosen := chosenCue(node, made.param.Name)
	name, shownID := s.cueLabel(chosen)
	ink := ui.ColorInk()
	switch {
	case s.audio != nil && chosen != "" && !s.isListed(chosen):
		ink = ui.ColorWarn()
	case !s.isListed(chosen):
		ink = ui.ColorMuted()
	}
	ui.Box(screen, made.cueBox, ui.ColorBackground())
	if made.open {
		ui.StrokeRound(screen, made.cueBox, ui.Radius, ui.Border, ui.ColorSelect())
	}
	textTop := made.cueBox.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	caret := image.Rect(made.cueBox.Max.X-textPad-caretWidth, made.cueBox.Min.Y, made.cueBox.Max.X-textPad,
		made.cueBox.Max.Y)
	ui.DrawTriangle(screen, caret, true, ui.ColorMuted())
	idRight := caret.Min.X - textPad
	idWidth := s.theme.MonoWidth(shownID)
	room := idRight - idWidth - textPad - made.cueBox.Min.X - textPad
	s.theme.DrawText(screen, s.clip(name, room), made.cueBox.Min.X+textPad, textTop, ink)
	s.theme.DrawMono(screen, shownID, idRight-idWidth, textTop, ui.ColorSubdued())
	made.listen.Draw(screen, s.theme)
	if !made.open {
		return
	}
	made.search.Draw(screen)
	list := made.list.Rect
	switch {
	case s.audio == nil:
		s.theme.DrawText(screen, "Listing the cues…", list.Min.X, list.Min.Y, ui.ColorMuted())
	case s.audio.Problem != "":
		for index, line := range s.wrap(s.audio.Problem, list.Dx()) {
			s.theme.DrawText(screen, line, list.Min.X, list.Min.Y+index*ui.LineHeight, ui.ColorMuted())
		}
	case len(made.list.Rows) == 0:
		s.theme.DrawText(screen, "No cue matches", list.Min.X, list.Min.Y, ui.ColorMuted())
	default:
		made.list.Draw(screen, s.theme)
		s.drawRowPlays(screen, made.list)
	}
}

// isListed reports whether a cue is in the library as last listed.
func (s *State) isListed(id string) bool {
	_, found := s.listedCue(id)

	return found
}

// playing reports whether this window plays a cue, as last seen in Update.
func (s *State) playing(cue string) bool {
	return s.sounds != nil && cue != "" && s.sounds.Playing(cue)
}

// drawRowPlays draws each audible row's play button: a triangle, or a
// square while its cue plays.
func (s *State) drawRowPlays(screen *ebiten.Image, list *ui.List) {
	for index, row := range list.Rows {
		rect := list.RowRect(index)
		if row.Heading || !rect.In(list.Rect) || !s.audible(row.ID) {
			continue
		}
		button := rowPlayRect(rect)
		ui.Box(screen, button, ui.ColorBackground())
		if s.playing(row.ID) {
			centre := ui.Center(button)
			ui.FillRect(screen, image.Rect(centre.X-glyphSize/ui.Half, centre.Y-glyphSize/ui.Half,
				centre.X+glyphSize/ui.Half, centre.Y+glyphSize/ui.Half), ui.ColorLive())

			continue
		}
		ui.DrawTriangle(screen, button, false, ui.ColorInk())
	}
}

// hasCueState answers "chooser" while a cue chooser is open, "cues" once
// the audio library is listed, and "cue=<id>" when the selected node plays
// that cue; and whether the state is one of them.
func (s *State) hasCueState(state string) (bool, bool) {
	switch {
	case state == "chooser":
		_, open := s.openChooser()

		return open, true
	case state == "cues":
		return s.audio != nil, true
	case strings.HasPrefix(state, "cue="):
		node, ok := s.selectedNode()

		return ok && chosenCue(node, "cue") == strings.TrimPrefix(state, "cue="), true
	}

	return false, false
}

// locateCueRow finds an open chooser's row for a cue, "cue:<id>", or the
// button that plays it, "play:<id>", which only a cue that can be heard
// has, for input scripts.
func (s *State) locateCueRow(kind, cue string) (image.Point, bool) {
	made, open := s.openChooser()
	if !open || (kind == locatePlay && !s.audible(cue)) {
		return image.Pt(0, 0), false
	}
	for index, row := range made.list.Rows {
		rect := made.list.RowRect(index)
		if row.ID != cue || row.Heading || !rect.In(made.list.Rect) {
			continue
		}
		if kind == locatePlay {
			return ui.Center(rowPlayRect(rect)), true
		}

		return image.Pt(rect.Min.X+textPad, ui.Center(rect).Y), true
	}

	return image.Pt(0, 0), false
}
