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
// plays in turn, or the selected recording and the cues that play it. It
// scrolls when it is taller than the window.
type panel struct {
	play, addRecording, addLayer *ui.Button
	up, down, takeOut, replace   *ui.Button
	openRecording, openFile      *ui.Button
	// cues are the cues playing the selected recording, which select them,
	// and hearIn the Play beside each, by cue, which plays the recording as
	// that cue plays it.
	cues   []*ui.Button
	hearIn map[string]*ui.Button
	// listTop is where the recordings, or the cues, start; loudTop where a
	// cue's loudness rows start, and chainTop how it plays.
	listTop, loudTop, chainTop int
	// scroll is how far it is scrolled down, at most furthest.
	scroll, furthest int
}

func newPanel() *panel {
	button := ui.NewButton

	return &panel{
		play: button("play", labelPlayCue), addRecording: button("add-recording", "Add recording…"),
		addLayer: button("add-layer", "Add to soundscape"), up: button("recording-up", "Up"),
		down: button("recording-down", "Down"), takeOut: button("take-out", "Take out"),
		replace: button("replace-file", "Replace file…"), openRecording: button("open-recording", "Open"),
		openFile: button("open-file", "Open file"), cues: nil, hearIn: map[string]*ui.Button{}, listTop: 0, loudTop: 0,
		chainTop: 0, scroll: 0, furthest: 0,
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

// panelTop is where the panel's content starts, as far as it is scrolled.
func (s *State) panelTop() int { return s.panelArea().Min.Y + ui.Inset - s.panel.scroll }

// layoutPanel places the panel's buttons and list for what is selected, and
// keeps its scroll within its content.
func (s *State) layoutPanel() {
	pane, area := s.panel, s.panelArea()
	bottom := s.layoutPanelContent()
	pane.furthest = max(bottom+pane.scroll-area.Max.Y, 0)
	if pane.scroll > pane.furthest {
		pane.scroll = pane.furthest
		s.layoutPanelContent()
	}
}

// layoutPanelContent places what the panel shows, and returns where it ends.
func (s *State) layoutPanelContent() int {
	pane, area := s.panel, s.panelArea()
	left, right := area.Min.X+ui.Inset, area.Max.X-ui.Inset
	for _, button := range pane.all() {
		button.Rect = ui.Zero()
	}
	for _, hear := range pane.hearIn {
		hear.Label = labelPlay
	}
	s.played.Rect = ui.Zero()
	for _, button := range s.files.buttons() {
		button.Rect = ui.Zero()
	}
	s.files.wave.Rect = ui.Zero()
	if cue, ok := s.selectedCue(); ok {
		return s.layoutCue(cue, left, area)
	}
	if file, ok := s.selectedFile(); ok {
		top := s.layoutFile(file, left, right, s.panelTop()+headerLines(s.fileProblemsOf(file))*ui.LineHeight+ui.Gap)
		s.layoutTrim(left, right, top)

		return s.layoutAttribution(left, right, top)
	}
	recording, ok := s.selectedRecording()
	if !ok {
		s.layoutTrim(left, right, area.Min.Y)
		s.layoutAttribution(left, right, area.Min.Y)

		return area.Min.Y
	}
	top := s.panelTop() + headerLines(s.problems(recording))*ui.LineHeight + ui.Gap
	s.row(left, top, pane.replace)
	if recording.File != "" {
		s.row(pane.replace.Rect.Max.X+panelButtonGap, top, pane.openFile)
	}
	top = s.layoutTrim(left, right, top+ui.ButtonH+ui.Inset)
	pane.listTop = s.layoutAttribution(left, right, top) + ui.LineHeight
	pane.cues = pane.cues[:0]
	for index, cue := range s.usedBy(recording.ID) {
		hear := pane.hearButton(cue.ID)
		rowTop := pane.listTop + index*(ui.ButtonH+ui.Gap)
		s.row(left, rowTop, hear)
		button := ui.NewButton(cuePrefix+cue.ID, cue.Name)
		button.Rect = image.Rect(hear.Rect.Max.X+panelButtonGap, rowTop,
			hear.Rect.Max.X+panelButtonGap+s.theme.TextWidth(cue.Name)+ui.Half*ui.Inset, rowTop+ui.ButtonH)
		pane.cues = append(pane.cues, button)
	}

	return pane.listTop + max(len(pane.cues)*(ui.ButtonH+ui.Gap), ui.LineHeight) + ui.Inset
}

// layoutCue places a cue's buttons and its recordings, and returns where
// they end.
func (s *State) layoutCue(cue protocol.AudioCue, left int, area image.Rectangle) int {
	pane := s.panel
	top := s.panelTop() + headerLines(s.cueProblems(cue))*ui.LineHeight + ui.Gap
	s.row(left, top, pane.play, pane.addRecording, pane.addLayer)
	pane.addRecording.Disabled = cue.Ambient != nil
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
	s.row(left, s.played.Rect.Max.Y+ui.Gap, pane.up, pane.down, pane.takeOut, pane.replace, pane.openRecording)
	index, _ := s.playedIndex(cue)
	pane.up.Disabled, pane.down.Disabled = index <= 0, index < 0 || index >= len(cue.Recordings)-1
	pane.takeOut.Disabled = len(cue.Recordings) < minRecordings
	top = s.layoutTrim(left, area.Max.X-ui.Inset, pane.up.Rect.Max.Y+ui.Inset)
	top = s.layoutAttribution(left, area.Max.X-ui.Inset, top)
	pane.loudTop = top + ui.LineHeight
	pane.chainTop = pane.loudTop + len(cue.Recordings)*(ui.RowHeight+loudRowGap) + ui.Inset

	return pane.chainTop + chainHeight + ui.Inset
}

// row lays buttons out in a row, each as wide as its label.
func (s *State) row(left, top int, buttons ...*ui.Button) {
	for _, button := range buttons {
		width := s.theme.TextWidth(button.Label) + ui.Half*ui.Inset
		button.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		left += width + panelButtonGap
	}
}

// panelButtons counts the panel's own buttons, before the Plays beside cues.
const panelButtons = 9

func (pane *panel) all() []*ui.Button {
	buttons := make([]*ui.Button, 0, panelButtons+len(pane.hearIn))
	buttons = append(buttons, pane.play, pane.addRecording, pane.addLayer, pane.up, pane.down, pane.takeOut,
		pane.replace, pane.openRecording, pane.openFile)
	for _, hear := range pane.hearIn {
		buttons = append(buttons, hear)
	}

	return buttons
}

// hearButton is the Play beside a cue under Played by, kept from frame to
// frame so it reads Stop while it plays.
func (pane *panel) hearButton(cue string) *ui.Button {
	hear, found := pane.hearIn[cue]
	if !found {
		hear = ui.NewButton(hearInPrefix+cue, labelPlay)
		pane.hearIn[cue] = hear
	}

	return hear
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

// scrollPanel scrolls the panel for the wheel over it, but not over a list
// of recordings that scrolls itself.
func (s *State) scrollPanel(input *ui.Input) {
	pane := s.panel
	if input.WheelY == 0 || !input.Cursor.In(s.panelArea()) ||
		input.Cursor.In(s.played.Rect) && len(s.played.Rows) > playedRows {
		return
	}
	pane.scroll = min(max(pane.scroll-int(input.WheelY*ui.RowHeight), 0), pane.furthest)
	input.WheelY = 0
}

// updatePanel handles the panel's buttons and list. Away from the panel, its
// buttons, which may be scrolled out of it, see no presses; a waveform end
// being dragged follows the pointer anywhere.
func (s *State) updatePanel(input *ui.Input, ctx *frame.Context) {
	pane := s.panel
	if !input.Cursor.In(s.panelArea()) && !s.trim.wave.Dragging() {
		away := *input
		away.ClearPointer()
		input = &away
	}
	s.updateTrim(input, ctx)
	s.updateAttribution(input, ctx)
	if file, ok := s.selectedFile(); ok {
		s.updateFile(input, ctx, file)

		return
	}
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
	case pane.replace.Update(input):
		s.chooseSound(ctx, soundTarget{cue: "", recording: recording.ID})
	case pane.openFile.Update(input):
		s.choose(ctx, filePrefix+recording.File)
	}
	for _, cue := range s.usedBy(recording.ID) {
		if hear := pane.hearButton(cue.ID); hear.Update(input) {
			s.auditionAt(ctx, hear, appliedWindow(recording), recording.CalibrationDB+cueBoost(cue))
		}
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
	case pane.addLayer.Update(input):
		s.addToSoundscape(ctx, cue, pane.addLayer.Rect)
	case !chosen:
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
	case pane.openRecording.Update(input):
		s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), func() {
			s.mode, s.list.Selected = modeRecordings, recordingPrefix+order[index]
		})
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

// appliedWindow is the part of its file a recording plays, nil for all of
// it.
func appliedWindow(recording protocol.AudioRecording) *protocol.AudioWindow {
	if recording.EndMs <= recording.StartMs {
		return nil
	}

	return &protocol.AudioWindow{StartMs: recording.StartMs, EndMs: recording.EndMs}
}

// drawPanel draws what is selected, within the panel: a cue and its
// recordings, or a recording and the cues that play it, and how far it is
// scrolled.
func (s *State) drawPanel(window *ebiten.Image) {
	area := s.panelArea()
	screen, clipped := window.SubImage(area).(*ebiten.Image)
	if !clipped {
		return
	}
	s.drawPanelScroll(screen)
	left, top := area.Min.X+ui.Inset, s.panelTop()
	if cue, ok := s.selectedCue(); ok {
		s.panel.play.Label = labelPlayCue
		if s.sounds != nil && s.sounds.Playing(cue.ID) {
			s.panel.play.Label = labelStop
		}
		what := categoryLabel(cue.Category) + " · " + plays(len(cue.Recordings))
		s.drawHeader(screen, left, top, cue.Name, cue.ID, what, s.cueProblems(cue))
		label := "RECORDINGS · PLAY IN TURN"
		if cue.Ambient != nil {
			label = "RECORDING · ONE, AS IT IS HEARD FROM A DISTANCE"
		}
		s.theme.DrawLabel(screen, label, left, s.panel.listTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
		s.played.Draw(screen, s.theme)
		s.drawButtons(screen, s.panel.all()...)
		s.drawCueLoudness(screen, cue, left)
		s.drawTrim(screen)
		s.drawAttribution(screen)

		return
	}
	if file, ok := s.selectedFile(); ok {
		s.drawFilePanel(screen, file, left, top)

		return
	}
	recording, ok := s.selectedRecording()
	if !ok {
		s.drawEmpty(screen, left, top)

		return
	}
	s.drawRecordingPanel(screen, recording, left, top)
}

// drawRecordingPanel draws a recording: which piece of its file it is, its
// waveform and loudness, its attribution, and the cues that play it.
func (s *State) drawRecordingPanel(screen *ebiten.Image, recording protocol.AudioRecording, left, top int) {
	where := recording.File
	if where != "" {
		where = "a piece of " + recording.File + " · " + spanText(recording)
	}
	s.drawHeader(screen, left, top, recording.ID, where, recordingWhat(recording), s.problems(recording))
	s.drawButtons(screen, s.panel.replace, s.panel.openFile)
	s.drawTrim(screen)
	s.drawAttribution(screen)
	s.theme.DrawLabel(screen, "PLAYED BY · HOW LOUD IT PLAYS IS EACH CUE'S CHOICE", left,
		s.panel.listTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
	if len(s.panel.cues) == 0 {
		s.theme.DrawText(screen, "No cue plays this recording.", left, s.panel.listTop, ui.ColorSubdued())
	}
	s.drawButtons(screen, s.panel.cues...)
	for _, cue := range s.usedBy(recording.ID) {
		s.drawButtons(screen, s.panel.hearButton(cue.ID))
	}
	s.drawUsedLoudness(screen, recording)
}

// fileProblemsOf lists what is wrong with any piece of a file, each once.
func (s *State) fileProblemsOf(file string) []string {
	var found []string
	for _, piece := range s.pieces(file) {
		for _, problem := range s.problems(piece) {
			if !contains(found, problem) {
				found = append(found, problem)
			}
		}
	}

	return found
}

// drawFilePanel draws a file: what is wrong with any piece of it, its
// waveform with its pieces, and its attribution.
func (s *State) drawFilePanel(screen *ebiten.Image, file string, left, top int) {
	pieces := s.pieces(file)
	s.drawHeader(screen, left, top, path.Base(file), file, "A file · "+piecesText(len(pieces))+" cut from it",
		s.fileProblemsOf(file))
	s.drawFile(screen)
	s.drawAttribution(screen)
}

// thumbWidth is the width of the panel's scroll thumb, in screen pixels.
const thumbWidth = 4

// drawPanelScroll draws a thumb at the panel's right edge while it is
// taller than the window, showing which part is in view.
func (s *State) drawPanelScroll(screen *ebiten.Image) {
	pane, area := s.panel, s.panelArea()
	if pane.furthest == 0 {
		return
	}
	height := max(area.Dy()*area.Dy()/(area.Dy()+pane.furthest), ui.ButtonH)
	top := area.Min.Y + (area.Dy()-height)*pane.scroll/pane.furthest
	right := area.Max.X - ui.Gap/ui.Half
	ui.FillRound(screen, image.Rect(right-thumbWidth, top, right, top+height), ui.Half, ui.ColorRule())
}

// What the cue's Play reads, and every Play while it plays.
const (
	labelPlayCue = "Play cue"
	labelStop    = "Stop"
)

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
