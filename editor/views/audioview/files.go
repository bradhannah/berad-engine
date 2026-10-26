package audioview

import (
	"fmt"
	"image"
	"path"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Files and their pieces: a recording is a piece of one file, and one file
// can be cut into many recordings, which share it and its attribution. The
// Recordings list groups recordings under their file, and a file opens a
// screen of its own: its waveform with every piece marked, the sounds Find
// sounds proposes that no piece covers yet, and its attribution.

// filePrefix starts a file's row ID; problemFileDisagrees is a recording
// whose attribution differs from another piece of its file.
const (
	filePrefix           = "file:"
	problemFileDisagrees = "attribution differs from its file's other pieces"
	sectionSlots         = sectionPrefix + "slots"
)

// fileScreen is what the window knows of the file shown: its waveform, the
// sounds found in it and the buttons that open its pieces.
type fileScreen struct {
	file     string
	analysis *protocol.AudioAnalysis
	asked    string
	found    []protocol.AudioWindow
	wave     *ui.Waveform
	play     *ui.Button
	find     *ui.Button
	makeAll  *ui.Button
	pieces   []*ui.Button
	top      int
}

func newFileScreen() *fileScreen {
	return &fileScreen{
		file: "", analysis: nil, asked: "", found: nil, wave: ui.NewWaveform("file-waveform"),
		play: ui.NewButton("play-file", "Play whole file"), find: ui.NewButton("find-sounds", "Find sounds"),
		makeAll: ui.NewButton("make-pieces", "Make recordings"), pieces: nil, top: 0,
	}
}

func (screen *fileScreen) buttons() []*ui.Button {
	return append([]*ui.Button{screen.play, screen.find, screen.makeAll}, screen.pieces...)
}

// pieces lists the recordings of a file, ordered by where they start.
func (s *State) pieces(file string) []protocol.AudioRecording {
	var found []protocol.AudioRecording
	if s.library == nil || file == "" {
		return found
	}
	seen := map[string]bool{}
	add := func(recording protocol.AudioRecording) {
		if recording.File == file && !seen[recording.ID] {
			seen[recording.ID] = true
			found = append(found, recording)
		}
	}
	for _, cue := range s.library.Cues {
		for _, recording := range cue.Recordings {
			add(recording)
		}
	}
	for _, recording := range s.library.Unused {
		add(recording)
	}
	slices.SortFunc(found, func(a, b protocol.AudioRecording) int {
		if a.StartMs != b.StartMs {
			return int(a.StartMs - b.StartMs)
		}
		if a.ID < b.ID {
			return -1
		}

		return 1
	})

	return found
}

// disagrees reports whether a recording's attribution differs from another
// piece of its file.
func (s *State) disagrees(recording protocol.AudioRecording) bool {
	for _, piece := range s.pieces(recording.File) {
		if piece.ID != recording.ID && !slices.Equal(attributionKey(piece), attributionKey(recording)) {
			return true
		}
	}

	return false
}

func attributionKey(recording protocol.AudioRecording) []string {
	attribution := recording.Attribution

	return []string{
		attribution.Creator, attribution.Source, attribution.SourceType, attribution.License,
		strconv.FormatBool(attribution.Commercial), processingText(attribution.Processing),
	}
}

// selectedFile returns the file selected in the list.
func (s *State) selectedFile() (string, bool) {
	file, ok := cutPrefix(s.list.Selected, filePrefix)
	if !ok || len(s.pieces(file)) == 0 {
		return "", false
	}

	return file, true
}

// fileRows lists the recordings under their files, a file's row first with
// how many pieces it has, then the recordings with no file.
func (s *State) fileRows() []ui.Row {
	files := map[string]bool{}
	var slots []protocol.AudioRecording
	for _, recording := range s.allRecordings() {
		if recording.File == "" {
			slots = append(slots, recording)
		} else {
			files[recording.File] = true
		}
	}
	var rows []ui.Row
	shownFiles := 0
	for _, file := range slices.Sorted(mapKeys(files)) {
		if pieces := s.shownRows(s.pieces(file), 1); len(pieces) > 0 {
			shownFiles++
			row := ui.NewRow(filePrefix+file, path.Base(file))
			row.Icon, row.Strong = ui.IconFolder, true
			rows = append(append(rows, row), pieces...)
		}
	}
	if s.closed[sectionUsed] && !s.searching() {
		rows = nil
	}
	rows = append([]ui.Row{s.section(sectionUsed, "By file", shownFiles, false)}, rows...)
	if shownSlots := s.shownRows(slots, 0); len(shownSlots) > 0 {
		rows = append(rows, s.section(sectionSlots, "Empty slots", len(shownSlots), false))
		if !s.closed[sectionSlots] || s.searching() {
			rows = append(rows, shownSlots...)
		}
	}

	return rows
}

// shownRows are the rows of the recordings the filter and search show.
func (s *State) shownRows(recordings []protocol.AudioRecording, indent int) []ui.Row {
	var rows []ui.Row
	for _, recording := range recordings {
		if s.shows(s.problems(recording)) && s.matches(recordingTexts(recording)...) {
			rows = append(rows, s.recordingRow(recording, indent))
		}
	}

	return rows
}

// recordingRow is a recording's row: its ID, indented under its file, and its
// first problem, or how many cues play it.
func (s *State) recordingRow(recording protocol.AudioRecording, indent int) ui.Row {
	row := ui.NewRow(recordingPrefix+recording.ID, recording.ID)
	row.Icon, row.Indent = ui.IconSound, indent
	switch found := s.problems(recording); {
	case len(found) > 0:
		row.Detail = found[0]
	case recording.Status == protocol.RecordingNoFile:
		row.Detail = stateNoSound
	default:
		row.Detail = cuesText(len(s.usedBy(recording.ID)))
	}

	return row
}

func piecesText(count int) string {
	if count == 1 {
		return "1 recording"
	}

	return strconv.Itoa(count) + " recordings"
}

func soundsText(count int) string {
	if count == 1 {
		return "1 sound"
	}

	return strconv.Itoa(count) + " sounds"
}

func cuesText(count int) string {
	switch count {
	case 0:
		return "not used"
	case 1:
		return "1 cue"
	}

	return strconv.Itoa(count) + " cues"
}

func mapKeys(values map[string]bool) func(func(string) bool) {
	return func(yield func(string) bool) {
		for key := range values {
			if !yield(key) {
				return
			}
		}
	}
}

// allRecordings lists every recording of the library, each once.
func (s *State) allRecordings() []protocol.AudioRecording {
	var all []protocol.AudioRecording
	seen := map[string]bool{}
	if s.library == nil {
		return all
	}
	for _, cue := range s.library.Cues {
		for _, recording := range cue.Recordings {
			if !seen[recording.ID] {
				seen[recording.ID] = true
				all = append(all, recording)
			}
		}
	}

	return append(all, s.library.Unused...)
}

// marksOf are the pieces of a file outlined on a waveform, all but one.
func marksOf(pieces []protocol.AudioRecording, except string) []ui.WaveMark {
	var marks []ui.WaveMark
	for _, piece := range pieces {
		if piece.ID != except && piece.EndMs > piece.StartMs {
			marks = append(marks, ui.WaveMark{Start: piece.StartMs, End: piece.EndMs, Found: false})
		}
	}

	return marks
}

// uncovered keeps the sounds found that no piece covers at least half of.
func uncovered(sounds []protocol.AudioWindow, pieces []protocol.AudioRecording) []protocol.AudioWindow {
	var kept []protocol.AudioWindow
	for _, sound := range sounds {
		covered := false
		for _, piece := range pieces {
			start, end := piece.StartMs, piece.EndMs
			if end <= start {
				covered = true

				break
			}
			overlap := min(end, sound.EndMs) - max(start, sound.StartMs)
			if overlap*ui.Half >= sound.EndMs-sound.StartMs {
				covered = true

				break
			}
		}
		if !covered {
			kept = append(kept, sound)
		}
	}

	return kept
}

// layoutFile places the file screen's parts from a top edge, and returns
// where they end.
func (s *State) layoutFile(file string, left, right, top int) int {
	screen := s.files
	if screen.file != file {
		screen.file, screen.analysis, screen.asked, screen.found = file, nil, "", nil
	}
	pieces := s.pieces(file)
	s.row(left, top, screen.play, screen.find, screen.makeAll)
	screen.makeAll.Label = "Make " + piecesText(len(screen.found))
	screen.makeAll.Disabled = len(screen.found) == 0
	screen.find.Disabled = screen.analysis == nil
	screen.top = top + ui.ButtonH + ui.Inset
	screen.wave.Rect = image.Rect(left, screen.top+ui.LineHeight, right, screen.top+ui.LineHeight+waveHeight)
	screen.wave.HasRegion = false
	screen.wave.Marks = marksOf(pieces, "")
	for _, sound := range screen.found {
		screen.wave.Marks = append(screen.wave.Marks, ui.WaveMark{Start: sound.StartMs, End: sound.EndMs, Found: true})
	}
	if screen.analysis != nil {
		screen.wave.Peaks, screen.wave.From, screen.wave.To = screen.analysis.Peaks, 0, screen.analysis.DurationMs
	} else {
		screen.wave.Peaks = nil
	}
	top = screen.wave.Rect.Max.Y + ui.Inset + ui.LineHeight
	screen.pieces = screen.pieces[:0]
	for index, piece := range pieces {
		label := piece.ID + " · " + spanText(piece)
		button := ui.NewButton(recordingPrefix+piece.ID, label)
		rowTop := top + index*(ui.ButtonH+ui.Gap)
		button.Rect = image.Rect(left, rowTop, left+s.theme.TextWidth(label)+ui.Half*ui.Inset, rowTop+ui.ButtonH)
		screen.pieces = append(screen.pieces, button)
	}

	return top + len(pieces)*(ui.ButtonH+ui.Gap) + ui.Inset
}

// spanText says which part of its file a recording plays.
func spanText(recording protocol.AudioRecording) string {
	if recording.EndMs <= recording.StartMs {
		return "all of it"
	}

	return seconds(recording.StartMs) + "–" + seconds(recording.EndMs) + " s"
}

// askFile asks for the waveform of the file shown and the sounds in it.
func (s *State) askFile(ctx *frame.Context) {
	screen := s.files
	file, ok := s.selectedFile()
	if !ok || screen.wave.Rect.Empty() {
		return
	}
	first := s.pieces(file)[0]
	key := fmt.Sprint(first.ID, first.Version, screen.wave.Rect.Dx())
	if key == screen.asked {
		return
	}
	screen.asked = key
	ctx.Requests.AudioAnalysis(protocol.AudioAnalysis{
		Recording: first.ID, From: 0, To: 0, Columns: max(screen.wave.Rect.Dx()/ui.Half, 1), Region: nil, Version: "",
		DurationMs: 0, Peaks: nil, Suggested: nil, Sounds: nil, Calibration: nil, Problem: "",
	}, func(answer protocol.AudioAnalysis, err error) {
		if screen.asked != key {
			return
		}
		if err != nil {
			ctx.Frame.Status("The file could not be drawn: " + err.Error())

			return
		}
		screen.analysis = &answer
	})
}

// updateFile handles the file screen's buttons: listening to it, finding its
// sounds, making recordings of those no piece covers, and opening a piece.
func (s *State) updateFile(input *ui.Input, ctx *frame.Context, file string) {
	screen := s.files
	pieces := s.pieces(file)
	switch {
	case screen.play.Update(input):
		s.playFile(ctx, file)
	case screen.find.Update(input) && screen.analysis != nil:
		screen.found = uncovered(screen.analysis.Sounds, pieces)
		if len(screen.found) == 0 {
			ctx.Frame.Status("Every sound in the file is already a recording.")
		} else {
			ctx.Frame.Status("Found " + soundsText(len(screen.found)) + " no recording covers yet, in orange.")
		}
	case screen.makeAll.Update(input) && len(screen.found) > 0:
		s.makePieces(ctx, pieces, screen.found)
	}
	for _, button := range screen.pieces {
		if button.Update(input) {
			s.choose(ctx, button.ID)
		}
	}
}

// playFile plays a whole file, as recorded, or stops it while it plays.
func (s *State) playFile(ctx *frame.Context, file string) {
	playing := auditioning(ctx.Sounds)
	ctx.Sounds.Stop()
	if playing {
		return
	}
	if pieces := s.pieces(file); len(pieces) > 0 {
		ctx.Sounds.Audition(pieces[0].ID, nil, 0, false)
	}
}

// makePieces makes a recording of each part found. They join the one cue
// that plays the file's other pieces, when one cue plays them all.
func (s *State) makePieces(ctx *frame.Context, pieces []protocol.AudioRecording, found []protocol.AudioWindow) {
	cue := ""
	var players []string
	for _, piece := range pieces {
		for _, user := range s.usedBy(piece.ID) {
			if !slices.Contains(players, user.ID) {
				players = append(players, user.ID)
			}
		}
	}
	if len(players) == 1 {
		cue = players[0]
	}
	regions := slices.Clone(found)
	s.edit(ctx, protocol.AudioMakePieces, protocol.MakePieces{Recording: pieces[0].ID, Regions: regions, Cue: cue},
		func(result protocol.Result) {
			s.files.found = nil
			said := "Made " + piecesText(len(regions)) + ", from " + result.Item + "."
			if cue != "" {
				said += " " + cue + " plays them in turn."
			}
			ctx.Frame.Status(said)
		})
}

// drawFile draws the file screen's waveform and pieces.
func (s *State) drawFile(screen *ebiten.Image) {
	files := s.files
	left := files.wave.Rect.Min.X
	label := "WAVEFORM · EVERY RECORDING CUT FROM IT IN BLUE"
	if len(files.found) > 0 {
		label += ", SOUNDS FOUND IN ORANGE"
	}
	if files.analysis != nil && files.analysis.Problem != "" {
		label = "WAVEFORM · " + files.analysis.Problem
	}
	s.theme.DrawLabel(screen, label, left, files.top+ui.Gap, ui.ColorMuted())
	files.wave.Draw(screen)
	s.theme.DrawLabel(screen, "RECORDINGS CUT FROM IT · SELECT ONE TO OPEN IT", left,
		files.wave.Rect.Max.Y+ui.Inset+ui.Gap, ui.ColorMuted())
	s.drawButtons(screen, files.buttons()...)
}
