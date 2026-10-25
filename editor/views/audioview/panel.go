package audioview

import (
	"image"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Row ID prefix of a cue's recordings in the panel.
const playedPrefix = "plays:"

// minRecordings is how many recordings a cue needs before one can be taken out.
const minRecordings = 2

// Panel layout, in screen pixels.
const (
	panelButtonGap = 6
	playedRows     = 4
)

// panel is the middle of the window: the selected cue and the recordings it
// plays in turn, or the selected recording and the cues that play it.
type panel struct {
	play, addRecording                                     *ui.Button
	playRecording, up, down, takeOut, replace, attribution *ui.Button
	// cues are the cues playing the selected recording, which select them.
	cues []*ui.Button
	// listTop is where the recordings, or the cues, start.
	listTop int
}

func newPanel() *panel {
	button := ui.NewButton

	return &panel{
		play: button("play", "Play"), addRecording: button("add-recording", "Add recording…"),
		playRecording: button("play-recording", "Play recording"), up: button("recording-up", "Up"),
		down: button("recording-down", "Down"), takeOut: button("take-out", "Take out"),
		replace: button("replace-file", "Replace file…"), attribution: button("attribution", "Attribution…"),
		cues: nil, listTop: 0,
	}
}

// area is the panel's part of the window.
func (s *State) panelArea() image.Rectangle {
	content := frame.Content(s.size)

	return image.Rect(listWidth, toolbarHeight, content.Max.X, content.Max.Y)
}

// headerLines counts the lines above the panel's buttons: the name, the ID,
// what it is, and its problems.
func headerLines(found []string) int {
	const fixed = 3

	return fixed + len(found)
}

// layoutPanel places the panel's buttons and list for what is selected.
func (s *State) layoutPanel() {
	pane, area := s.panel, s.panelArea()
	left := area.Min.X + ui.Inset
	for _, button := range pane.all() {
		button.Rect = ui.Zero()
	}
	s.played.Rect = ui.Zero()
	if cue, ok := s.selectedCue(); ok {
		s.layoutCue(cue, left, area)

		return
	}
	if recording, ok := s.selectedRecording(); ok {
		top := area.Min.Y + ui.Inset + headerLines(s.problems(recording))*ui.LineHeight + ui.Gap
		s.row(left, top, pane.play, pane.replace, pane.attribution)
		top = s.layoutTrim(left, area.Max.X-ui.Inset, top+ui.ButtonH+ui.Inset)
		pane.listTop = top + ui.LineHeight
		pane.cues = pane.cues[:0]
		for index, cue := range s.usedBy(recording.ID) {
			button := ui.NewButton(cuePrefix+cue.ID, cue.Name)
			rowTop := pane.listTop + index*(ui.ButtonH+ui.Gap)
			button.Rect = image.Rect(left, rowTop, left+s.theme.TextWidth(cue.Name)+ui.Half*ui.Inset, rowTop+ui.ButtonH)
			pane.cues = append(pane.cues, button)
		}
	}
}

// layoutCue places a cue's buttons and its recordings.
func (s *State) layoutCue(cue protocol.AudioCue, left int, area image.Rectangle) {
	pane := s.panel
	top := area.Min.Y + ui.Inset + headerLines(s.cueProblems(cue))*ui.LineHeight + ui.Gap
	s.row(left, top, pane.play, pane.addRecording)
	pane.listTop = top + ui.ButtonH + ui.Inset + ui.LineHeight
	rows := make([]ui.Row, 0, len(cue.Recordings))
	for index, recording := range cue.Recordings {
		row := ui.NewRow(playedPrefix+recording.ID, strconv.Itoa(index+1)+"  "+recording.ID)
		row.Aside = path.Base(recording.File)
		if recording.File == "" {
			row.Aside = ""
		}
		if found := s.problems(recording); len(found) > 0 {
			row.Detail = found[0]
		} else if recording.Status == protocol.RecordingNoFile {
			row.Detail = stateNoSound
		}
		rows = append(rows, row)
	}
	s.played.Rows = rows
	for _, row := range rows {
		if row.ID == s.playedNext {
			s.played.Selected, s.playedNext = s.playedNext, ""
		}
	}
	if _, held := s.playedIndex(cue); !held && len(rows) > 0 {
		s.played.Selected = rows[0].ID
	}
	shown := min(max(len(rows), 1), playedRows)
	s.played.Rect = image.Rect(left, pane.listTop, area.Max.X-ui.Inset, pane.listTop+shown*ui.RowHeight)
	s.row(left, s.played.Rect.Max.Y+ui.Gap, pane.playRecording, pane.up, pane.down, pane.takeOut, pane.replace,
		pane.attribution)
	index, _ := s.playedIndex(cue)
	pane.up.Disabled, pane.down.Disabled = index <= 0, index < 0 || index >= len(cue.Recordings)-1
	pane.takeOut.Disabled = len(cue.Recordings) < minRecordings
	s.layoutTrim(left, area.Max.X-ui.Inset, pane.playRecording.Rect.Max.Y+ui.Inset)
}

// row lays buttons out in a row, each as wide as its label.
func (s *State) row(left, top int, buttons ...*ui.Button) {
	for _, button := range buttons {
		width := s.theme.TextWidth(button.Label) + ui.Half*ui.Inset
		button.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		left += width + panelButtonGap
	}
}

func (pane *panel) all() []*ui.Button {
	return []*ui.Button{
		pane.play, pane.addRecording, pane.playRecording, pane.up, pane.down, pane.takeOut, pane.replace, pane.attribution,
	}
}

// playedIndex returns the position of the recording selected among a cue's
// recordings.
func (s *State) playedIndex(cue protocol.AudioCue) (int, bool) {
	id, ok := cutPrefix(s.played.Selected, playedPrefix)
	if !ok {
		return -1, false
	}
	for index, recording := range cue.Recordings {
		if recording.ID == id {
			return index, true
		}
	}

	return -1, false
}

// recordingIDs lists a cue's recordings in play order.
func recordingIDs(cue protocol.AudioCue) []string {
	ids := make([]string, 0, len(cue.Recordings))
	for _, recording := range cue.Recordings {
		ids = append(ids, recording.ID)
	}

	return ids
}

// updatePanel handles the panel's buttons and list.
func (s *State) updatePanel(input *ui.Input, ctx *frame.Context) {
	pane := s.panel
	s.updateTrim(input, ctx)
	if cue, ok := s.selectedCue(); ok {
		previous := s.played.Selected
		if s.played.Update(input).Changed {
			chosen := s.played.Selected
			s.played.Selected = previous
			s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), func() { s.played.Selected = chosen })
		}
		s.updateCue(input, ctx, cue)

		return
	}
	recording, ok := s.selectedRecording()
	if !ok {
		return
	}
	switch {
	case pane.play.Update(input):
		s.listenRecording(ctx, recording.ID)
	case pane.replace.Update(input):
		s.chooseSound(ctx, soundTarget{cue: "", recording: recording.ID})
	case pane.attribution.Update(input):
		s.openAttribution(recording)
	}
	for _, button := range pane.cues {
		if button.Update(input) {
			s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), func() {
				s.mode, s.list.Selected = modeCues, button.ID
			})
		}
	}
}

// updateCue handles a cue's buttons: listening to it and its recordings,
// adding one, moving one, taking one out and replacing one's file.
func (s *State) updateCue(input *ui.Input, ctx *frame.Context, cue protocol.AudioCue) {
	pane := s.panel
	index, chosen := s.playedIndex(cue)
	order := recordingIDs(cue)
	switch {
	case pane.play.Update(input):
		s.listen(ctx, cue.ID)
	case pane.addRecording.Update(input):
		s.chooseSound(ctx, soundTarget{cue: cue.ID, recording: ""})
	case !chosen:
	case pane.playRecording.Update(input):
		s.listenRecording(ctx, order[index])
	case pane.up.Update(input):
		order[index-1], order[index] = order[index], order[index-1]
		s.reorder(ctx, cue, order)
	case pane.down.Update(input):
		order[index+1], order[index] = order[index], order[index+1]
		s.reorder(ctx, cue, order)
	case pane.takeOut.Update(input):
		s.reorder(ctx, cue, slices.Delete(order, index, index+1))
	case pane.replace.Update(input):
		s.chooseSound(ctx, soundTarget{cue: "", recording: order[index]})
	case pane.attribution.Update(input):
		if recording, found := s.recording(order[index]); found {
			s.openAttribution(recording)
		}
	}
}

// reorder sets the recordings a cue plays in turn.
func (s *State) reorder(ctx *frame.Context, cue protocol.AudioCue, order []string) {
	s.edit(ctx, protocol.AudioSetRecordings, protocol.CueRecordings{Cue: cue.ID, Recordings: order}, nil)
}

// listen plays a cue, or stops it while it plays.
func (s *State) listen(ctx *frame.Context, cue string) {
	if ctx.Sounds.Playing(cue) {
		ctx.Sounds.Stop()

		return
	}
	ctx.Sounds.Stop()
	ctx.Sounds.Play(cue)
}

// listenRecording plays one recording on its own, or stops it while it plays.
func (s *State) listenRecording(ctx *frame.Context, recording string) {
	if ctx.Sounds.PlayingRecording(recording) {
		ctx.Sounds.Stop()

		return
	}
	ctx.Sounds.Stop()
	ctx.Sounds.PlayRecording(recording)
}

// drawPanel draws what is selected: a cue and its recordings, or a recording
// and the cues that play it.
func (s *State) drawPanel(screen *ebiten.Image) {
	area := s.panelArea()
	left, top := area.Min.X+ui.Inset, area.Min.Y+ui.Inset
	if cue, ok := s.selectedCue(); ok {
		s.panel.play.Label = playLabel(s.sounds != nil && s.sounds.Playing(cue.ID))
		if index, chosen := s.playedIndex(cue); chosen && s.sounds != nil &&
			s.sounds.PlayingRecording(cue.Recordings[index].ID) {
			s.panel.playRecording.Label = "Stop recording"
		} else {
			s.panel.playRecording.Label = "Play recording"
		}
		what := categoryLabel(cue.Category) + " · " + plays(len(cue.Recordings))
		s.drawHeader(screen, left, top, cue.Name, cue.ID, what, s.cueProblems(cue))
		s.theme.DrawLabel(screen, "RECORDINGS · PLAY IN TURN", left, s.panel.listTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
		s.played.Draw(screen, s.theme)
		s.drawButtons(screen, s.panel.all()...)
		s.drawTrim(screen)

		return
	}
	recording, ok := s.selectedRecording()
	if !ok {
		s.drawEmpty(screen, left, top)

		return
	}
	s.panel.play.Label = playLabel(s.sounds != nil && s.sounds.PlayingRecording(recording.ID))
	s.drawHeader(screen, left, top, recording.ID, recording.File, recordingWhat(recording), s.problems(recording))
	s.drawButtons(screen, s.panel.play, s.panel.replace, s.panel.attribution)
	s.drawTrim(screen)
	s.theme.DrawLabel(screen, "PLAYED BY", left, s.panel.listTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
	if len(s.panel.cues) == 0 {
		s.theme.DrawText(screen, "No cue plays this recording.", left, s.panel.listTop, ui.ColorSubdued())
	}
	s.drawButtons(screen, s.panel.cues...)
}

// labelStop is what a play button reads while what it plays is playing.
const labelStop = "Stop"

func playLabel(playing bool) string {
	if playing {
		return labelStop
	}

	return "Play"
}

func plays(count int) string {
	if count == 1 {
		return "plays 1 recording"
	}

	return "plays " + strconv.Itoa(count) + " recordings in turn"
}

// recordingWhat says what state a recording is in.
func recordingWhat(recording protocol.AudioRecording) string {
	switch recording.Status {
	case protocol.RecordingNoFile:
		return "An empty slot: no sound yet"
	case protocol.RecordingReady:
		return "Ready · " + strconv.FormatInt(recording.Size/bytesPerKB, 10) + " KB"
	}

	return "The file is " + recording.Status
}

const bytesPerKB = 1024

// drawHeader draws a name, its ID or file, what it is, and its problems.
func (s *State) drawHeader(screen *ebiten.Image, left, top int, name, id, what string, found []string) {
	s.theme.DrawTitle(screen, name, left, top, ui.ColorInk())
	top += ui.LineHeight
	s.theme.DrawMono(screen, id, left, top+ui.Gap, ui.ColorSubdued())
	top += ui.LineHeight
	s.theme.DrawText(screen, what, left, top, ui.ColorMuted())
	for _, problem := range found {
		top += ui.LineHeight
		s.theme.DrawText(screen, "! "+sentence(problem), left, top, ui.ColorWarn())
	}
}

// sentence starts a problem with a capital letter.
func sentence(value string) string {
	if value == "" {
		return value
	}

	return strings.ToUpper(value[:1]) + value[1:]
}

func (s *State) drawButtons(screen *ebiten.Image, buttons ...*ui.Button) {
	for _, button := range buttons {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
}

// drawEmpty says what to do when nothing is selected, or why the library
// cannot be shown.
func (s *State) drawEmpty(screen *ebiten.Image, left, top int) {
	lines := []string{"Select a cue or a recording.", "Add sound… copies a file in, and asks who made it."}
	switch {
	case s.library == nil:
		lines = []string{"Listing the audio library…"}
	case s.library.Problem != "":
		lines = []string{s.library.Problem}
	}
	for _, line := range lines {
		s.theme.DrawText(screen, line, left, top, ui.ColorMuted())
		top += ui.LineHeight
	}
}
