package audioview

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Trimming and levels: the waveform of the recording in focus, the region of
// it that plays, which stays a draft until Apply, and its level.

// Trim layout, in screen pixels, and its limits.
const (
	waveHeight  = 80
	fieldWidth  = 84
	loopWidth   = 64
	zoomWidth   = 34
	minViewMs   = 100
	minRegionMs = 10
	secondMs    = 1000.0
	fieldBytes  = 16
	regionLabel = "REGION · A DRAFT UNTIL APPLY"
)

// trimmer is what the window knows of the recording in focus: its region as
// saved and as drafted, the part of it shown, and its latest analysis.
type trimmer struct {
	recording, version string
	saved, draft       *protocol.AudioWindow
	// whole says the waveform shows all of the recording; otherwise it
	// shows the part that the from and until fields bound.
	whole       bool
	from, until int64
	analysis    *protocol.AudioAnalysis
	asked       string
	top         int
	// auditioned is the button whose part of the recording is playing.
	auditioned *ui.Button

	wave                                   *ui.Waveform
	start, end                             *ui.TextField
	zoomIn, zoomOut, fit                   *ui.Button
	playRegion, playWhole                  *ui.Button
	loop                                   *ui.Toggle
	suggest, all, apply, cancel, duplicate *ui.Button
	useSuggestion, setLevel, setTarget     *ui.Button
}

func newTrimmer(theme *ui.Theme) *trimmer {
	button := ui.NewButton
	field := func(id string) *ui.TextField {
		made := ui.NewTextField(id, textedit.NewField("", false, fieldBytes), false, theme)
		made.Unmarked = true

		return made
	}

	return &trimmer{
		recording: "", version: "", saved: nil, draft: nil, whole: true, from: 0, until: 0, analysis: nil, asked: "",
		top: 0, auditioned: nil, wave: ui.NewWaveform("waveform"), start: field("region-start"), end: field("region-end"),
		zoomIn: button("zoom-in", "+"), zoomOut: button("zoom-out", "−"), fit: button("zoom-fit", "Fit"),
		playRegion: button("play-region", "Play region"), playWhole: button("play-whole", "Play whole file"),
		loop:    &ui.Toggle{ID: "loop", Label: "Loop", Rect: ui.Zero(), On: false},
		suggest: button("suggest-region", "Suggest"), all: button("whole-region", "Whole file"),
		apply: button("apply-region", "Apply"), cancel: button("cancel-region", "Cancel"),
		duplicate:     button("duplicate-region", "Duplicate region"),
		useSuggestion: button("use-suggestion", "Use suggested level"), setLevel: button("set-level", "Set level…"),
		setTarget: button("set-target", "Target…"),
	}
}

func (t *trimmer) buttons() []*ui.Button {
	return []*ui.Button{
		t.zoomIn, t.zoomOut, t.fit, t.playRegion, t.playWhole, t.suggest, t.all, t.apply, t.cancel, t.duplicate,
		t.useSuggestion, t.setLevel, t.setTarget,
	}
}

// shows reports whether a recording is in focus with a waveform to show.
func (t *trimmer) shows() bool { return t.recording != "" }

// dirty reports whether the drafted region differs from the saved one.
func (t *trimmer) dirty() bool { return t.shows() && !sameWindow(t.saved, t.draft) }

// duration is the recording's length once analysed.
func (t *trimmer) duration() int64 {
	if t.analysis == nil {
		return 0
	}

	return t.analysis.DurationMs
}

// view is the part of the recording the waveform shows.
func (t *trimmer) view() (int64, int64) {
	if t.whole {
		return 0, t.duration()
	}

	return t.from, t.until
}

// region is the drafted region, all of the recording when there is none.
func (t *trimmer) region() (int64, int64) {
	if t.draft == nil {
		return 0, t.duration()
	}

	return t.draft.StartMs, t.draft.EndMs
}

func sameWindow(first, second *protocol.AudioWindow) bool {
	if first == nil || second == nil {
		return first == second
	}

	return *first == *second
}

func copyWindow(window *protocol.AudioWindow) *protocol.AudioWindow {
	if window == nil {
		return nil
	}
	copied := *window

	return &copied
}

// savedRegion is a recording's own region, nil for all of it.
func savedRegion(recording protocol.AudioRecording) *protocol.AudioWindow {
	if recording.EndMs <= recording.StartMs {
		return nil
	}

	return &protocol.AudioWindow{StartMs: recording.StartMs, EndMs: recording.EndMs}
}

// focused returns the recording whose waveform the panel shows: the one
// chosen among the selected cue's, or the selected recording.
func (s *State) focused() (protocol.AudioRecording, bool) {
	var none protocol.AudioRecording
	if cue, ok := s.selectedCue(); ok {
		if index, chosen := s.playedIndex(cue); chosen {
			return cue.Recordings[index], true
		}

		return none, false
	}

	return s.selectedRecording()
}

// syncTrim follows the recording in focus: a new one, or a new version of
// its file, starts afresh; a change to its saved region is followed by a
// draft that was not changed. It reports whether what the panel shows
// changed, so it is laid out again.
func (s *State) syncTrim() bool {
	trim := s.trim
	recording, ok := s.focused()
	if !ok || recording.Status != protocol.RecordingReady {
		shown := trim.shows()
		trim.recording, trim.analysis, trim.asked = "", nil, ""

		return shown
	}
	saved := savedRegion(recording)
	switch {
	case recording.ID != trim.recording || recording.Version != trim.version:
		trim.recording, trim.version, trim.analysis, trim.whole = recording.ID, recording.Version, nil, true
		trim.saved, trim.draft = saved, copyWindow(saved)

		return true
	case !sameWindow(saved, trim.saved):
		if !trim.dirty() {
			trim.draft = copyWindow(saved)
		}
		trim.saved = saved
	}

	return false
}

// askTrim asks for the waveform of what is shown and a suggestion for the
// drafted region, once each time they change; not while a region end is
// being dragged.
func (s *State) askTrim(ctx *frame.Context) {
	trim := s.trim
	if trim.wave.Dragging() || trim.wave.Rect.Empty() {
		return
	}
	asked := protocol.AudioAnalysis{
		Recording: trim.recording, From: 0, To: 0, Columns: max(trim.wave.Rect.Dx()/ui.Half, 1),
		Region: copyWindow(trim.draft), Version: "", DurationMs: 0, Peaks: nil, Suggested: nil, Calibration: nil,
		Problem: "",
	}
	if !trim.whole {
		asked.From, asked.To = trim.from, trim.until
	}
	key := fmt.Sprint(trim.version, asked.Recording, asked.From, asked.To, asked.Columns, asked.Region)
	if key == trim.asked {
		return
	}
	trim.asked = key
	ctx.Requests.AudioAnalysis(asked, func(answer protocol.AudioAnalysis, err error) {
		if trim.asked != key {
			return
		}
		if err != nil {
			ctx.Frame.Status("The waveform could not be drawn: " + err.Error())

			return
		}
		trim.analysis = &answer
	})
}

// layoutTrim places the waveform and its controls from a top edge.
func (s *State) layoutTrim(left, right, top int) int {
	trim := s.trim
	for _, button := range trim.buttons() {
		button.Rect = ui.Zero()
	}
	trim.wave.Rect, trim.start.Rect, trim.end.Rect, trim.loop.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	trim.top = top
	if !trim.shows() {
		return top
	}
	trim.playRegion.Label, trim.playWhole.Label = "Play region", "Play whole file"
	if s.sounds == nil || !auditioning(s.sounds) {
		trim.auditioned = nil
	} else if trim.auditioned != nil {
		trim.auditioned.Label = labelStop
	}
	trim.fit.Rect = image.Rect(right-zoomWidth*ui.Half, top, right, top+ui.ButtonH)
	trim.zoomIn.Rect = image.Rect(trim.fit.Rect.Min.X-ui.Gap-zoomWidth, top, trim.fit.Rect.Min.X-ui.Gap, top+ui.ButtonH)
	trim.zoomOut.Rect = trim.zoomIn.Rect.Sub(image.Pt(zoomWidth+ui.Gap, 0))
	top += ui.ButtonH + ui.Gap
	trim.wave.Rect = image.Rect(left, top, right, top+waveHeight)
	top += waveHeight + ui.Gap*ui.Half
	s.row(left, top, trim.playRegion, trim.playWhole)
	trim.loop.Rect = image.Rect(trim.playWhole.Rect.Max.X+ui.Inset, top, trim.playWhole.Rect.Max.X+ui.Inset+loopWidth,
		top+ui.ButtonH)
	s.row(trim.loop.Rect.Max.X+ui.Inset, top, trim.duplicate)
	top += ui.ButtonH + ui.Inset + ui.LineHeight
	trim.start.Rect = image.Rect(left, top, left+fieldWidth, top+ui.ButtonH)
	trim.end.Rect = image.Rect(trim.start.Rect.Max.X+ui.Gap, top, trim.start.Rect.Max.X+ui.Gap+fieldWidth, top+ui.ButtonH)
	s.row(trim.end.Rect.Max.X+ui.Inset, top, trim.suggest, trim.all, trim.apply, trim.cancel)
	top += ui.ButtonH + ui.LineHeight + ui.Inset + ui.LineHeight*levelLines
	s.row(left, top, trim.useSuggestion, trim.setLevel, trim.setTarget)
	trim.layoutWave()
	trim.apply.Disabled, trim.cancel.Disabled = !trim.dirty(), !trim.dirty()
	trim.suggest.Disabled = trim.analysis == nil || trim.analysis.Suggested == nil
	trim.useSuggestion.Disabled = trim.dirty() || trim.analysis == nil || trim.analysis.Calibration == nil

	return top + ui.ButtonH + ui.Inset
}

// levelLines counts the lines about a recording's level above its buttons:
// its label, how its level is set, and the suggestion.
const levelLines = 3

// layoutWave gives the waveform what it shows.
func (t *trimmer) layoutWave() {
	if t.analysis == nil {
		t.wave.Peaks, t.wave.HasRegion = nil, false

		return
	}
	t.wave.Peaks = t.analysis.Peaks
	t.wave.From, t.wave.To = t.view()
	if !t.wave.Dragging() {
		t.wave.Start, t.wave.End = t.region()
	}
	t.wave.HasRegion = true
}

// updateTrim handles the waveform and its controls.
func (s *State) updateTrim(input *ui.Input, ctx *frame.Context) {
	trim := s.trim
	if !trim.shows() {
		return
	}
	if changed, _ := trim.wave.Update(input); changed {
		trim.setDraft(trim.wave.Start, trim.wave.End)
	}
	trim.loop.Update(input)
	switch {
	case trim.zoomIn.Update(input):
		trim.zoom(ui.Half)
	case trim.zoomOut.Update(input):
		trim.zoomOutOnce()
	case trim.fit.Update(input):
		trim.whole = true
	case trim.playRegion.Update(input):
		s.audition(ctx, trim.playRegion, copyWindow(trim.draft))
	case trim.playWhole.Update(input):
		s.audition(ctx, trim.playWhole, nil)
	default:
		s.updateRegion(input, ctx)
	}
}

// updateRegion handles the region's and the level's buttons.
func (s *State) updateRegion(input *ui.Input, ctx *frame.Context) {
	trim := s.trim
	switch {
	case trim.suggest.Update(input) && trim.analysis != nil && trim.analysis.Suggested != nil:
		trim.draft = copyWindow(trim.analysis.Suggested)
	case trim.all.Update(input):
		trim.draft = nil
	case trim.cancel.Update(input):
		trim.draft = copyWindow(trim.saved)
	case trim.apply.Update(input):
		s.applyRegion(ctx)
	case trim.duplicate.Update(input):
		s.duplicateRegion(ctx)
	case trim.useSuggestion.Update(input):
		s.edit(ctx, protocol.AudioAcceptCalibration, protocol.AudioItem{ID: trim.recording}, nil)
	case trim.setLevel.Update(input):
		s.prompt = s.levelDialog(levelByHand, trim.recording, s.currentLevel())
	case trim.setTarget.Update(input):
		s.prompt = s.levelDialog(levelTarget, trim.recording, s.currentTarget())
	}
}

// setDraft drafts a region, none when it is all of the recording.
func (t *trimmer) setDraft(start, end int64) {
	if start <= 0 && end >= t.duration() {
		t.draft = nil

		return
	}
	t.draft = &protocol.AudioWindow{StartMs: start, EndMs: end}
}

// zoom shows a part of the recording smaller by a factor, round the region.
func (t *trimmer) zoom(factor int64) {
	from, until := t.view()
	start, end := t.region()
	centre := (start + end) / ui.Half
	span := max((until-from)/factor, minViewMs)
	t.from, t.until, t.whole = max(centre-span/ui.Half, 0), min(centre+span/ui.Half, t.duration()), false
}

// zoomOutOnce shows twice as much, or all of it.
func (t *trimmer) zoomOutOnce() {
	from, until := t.view()
	span := (until - from) * ui.Half
	if span >= t.duration() {
		t.whole = true

		return
	}
	centre := (from + until) / ui.Half
	t.from, t.until = max(centre-span/ui.Half, 0), min(centre+span/ui.Half, t.duration())
}

// auditioning reports whether part of a recording is playing.
func auditioning(sounds frame.Sounds) bool {
	_, playing := sounds.AuditionAt()

	return playing
}

// audition plays part of the recording, at the gain it would play at: for a
// measured recording, the gain suggested for the drafted region. Pressed
// again while it plays, its button stops it.
func (s *State) audition(ctx *frame.Context, button *ui.Button, window *protocol.AudioWindow) {
	trim := s.trim
	if auditioning(ctx.Sounds) {
		ctx.Sounds.Stop()
		stopped := trim.auditioned == button
		trim.auditioned = nil
		if stopped {
			return
		}
	}
	trim.auditioned = button
	recording, ok := s.focused()
	if !ok {
		return
	}
	gain := recording.CalibrationDB
	if recording.CalibrationMode == protocol.CalibrationAuto && trim.analysis != nil && trim.analysis.Calibration != nil &&
		window != nil {
		gain = trim.analysis.Calibration.GainDB
	}
	ctx.Sounds.Audition(recording.ID, window, gain, trim.loop.On)
}

// applyRegion saves the drafted region.
func (s *State) applyRegion(ctx *frame.Context) {
	trim := s.trim
	s.edit(ctx, protocol.AudioSetRegion, protocol.RegionChange{Recording: trim.recording, Region: copyWindow(trim.draft)},
		func(protocol.Result) { ctx.Frame.Status("Region applied. Undo puts it back.") })
}

// duplicateRegion makes a new recording of the drafted region, played in turn
// by the selected cue, and shows it.
func (s *State) duplicateRegion(ctx *frame.Context) {
	trim := s.trim
	cue := ""
	if selected, ok := s.selectedCue(); ok {
		cue = selected.ID
	}
	s.edit(ctx, protocol.AudioDuplicateRegion, protocol.DuplicateRegion{
		Recording: trim.recording, Region: copyWindow(trim.draft), Cue: cue,
	}, func(result protocol.Result) {
		if cue != "" {
			s.playedNext = playedPrefix + result.Item
		} else {
			s.selectNext = recordingPrefix + result.Item
		}
		ctx.Frame.Status("Made " + result.Item + " from the region.")
	})
}

// currentLevel is the focused recording's level, in dB.
func (s *State) currentLevel() float64 {
	recording, _ := s.focused()

	return recording.CalibrationDB
}

// currentTarget is the loudness the focused recording's measurement aims at.
func (s *State) currentTarget() float64 {
	recording, _ := s.focused()
	if recording.TargetDB != 0 {
		return recording.TargetDB
	}
	if s.trim.analysis != nil && s.trim.analysis.Calibration != nil {
		return s.trim.analysis.Calibration.TargetDB
	}

	return defaultTargetDB
}

// defaultTargetDB is berad-audio's default loudness target.
const defaultTargetDB = -20.0

// syncRegionFields shows the drafted region in its fields while they are
// not being typed in, and drafts what is typed when Enter saves it.
func (s *State) syncRegionFields() {
	trim := s.trim
	start, end := trim.region()
	for field, ms := range map[*ui.TextField]int64{trim.start: start, trim.end: end} {
		if !field.Focused() {
			field.Field.Revert()
			field.Field.SetSaved(seconds(ms))
		}
	}
}

// commitRegionField drafts the region a field was typed into.
func (s *State) commitRegionField(field *ui.TextField) bool {
	trim := s.trim
	if field != trim.start && field != trim.end {
		return false
	}
	value, _ := field.Field.Commit()
	ms, parsed := parseSeconds(value)
	start, end := trim.region()
	if field == trim.start {
		start = ms
	} else {
		end = ms
	}
	if !parsed || start < 0 || end > trim.duration() || end-start < minRegionMs {
		field.Field.Failed()
		field.Field.Revert()

		return true
	}
	trim.setDraft(start, end)
	field.Field.Revert()
	s.focus.Set(nil)

	return true
}

func seconds(ms int64) string { return strconv.FormatFloat(float64(ms)/secondMs, 'f', 3, 64) }

func parseSeconds(value string) (int64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "s")), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}

	return int64(math.Round(parsed * secondMs)), true
}

// decibels writes a level with its sign, as "+4.1 dB" or "−6.0 dB".
func decibels(value float64) string {
	text := strconv.FormatFloat(math.Abs(value), 'f', 1, 64) + " dB"
	if value < 0 {
		return "−" + text
	}

	return "+" + text
}

// levelText says how the recording's level is set, and what is suggested for
// the drafted region.
func (s *State) levelText(recording protocol.AudioRecording) (string, string) {
	trim := s.trim
	target := "its family's target"
	if recording.TargetDB != 0 {
		target = decibels(recording.TargetDB)
	}
	now := "Not set: plays as recorded. Use the suggested level, or set one by hand."
	switch recording.CalibrationMode {
	case protocol.CalibrationAuto:
		now = "Measured: " + decibels(recording.CalibrationDB) + ", aiming at " + target + "."
	case protocol.CalibrationManual:
		now = "Set by hand: " + decibels(recording.CalibrationDB) + ". The suggestion would measure it."
	}
	suggestion := "Measuring…"
	if trim.analysis != nil && trim.analysis.Calibration != nil {
		found := trim.analysis.Calibration
		suggestion = "Suggested level for this region: " + decibels(found.GainDB) + " (it measures " +
			decibels(found.MeasuredDB) + ", target " + decibels(found.TargetDB) + ")"
		if found.Limited {
			suggestion += ", limited so it does not clip"
		}
	}

	return now, suggestion
}

// drawTrim draws the waveform and its controls.
func (s *State) drawTrim(screen *ebiten.Image) {
	trim := s.trim
	if !trim.shows() {
		return
	}
	left := trim.wave.Rect.Min.X
	from, until := trim.view()
	view := "WAVEFORM · " + seconds(from) + "–" + seconds(until) + " s"
	if trim.analysis != nil && trim.analysis.Problem != "" {
		view = "WAVEFORM · " + trim.analysis.Problem
	}
	s.theme.DrawLabel(screen, view, left, trim.top+(ui.ButtonH-ui.LabelSize)/ui.Half, ui.ColorMuted())
	if at, playing := s.sounds.AuditionAt(); playing {
		trim.wave.Playhead, trim.wave.Playing = at, true
		s.theme.DrawMono(screen, "▶ "+seconds(at)+" s", trim.loop.Rect.Max.X+ui.Inset, trim.loop.Rect.Min.Y+ui.Gap*ui.Half,
			ui.ColorLive())
	} else {
		trim.wave.Playing = false
	}
	trim.wave.Draw(screen)
	trim.loop.Draw(screen, s.theme)
	s.theme.DrawLabel(screen, regionLabel, left, trim.start.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
	trim.start.Draw(screen)
	trim.end.Draw(screen)
	hint := "Drag the blue ends, or type the start and end in seconds."
	s.theme.DrawText(screen, hint, left, trim.start.Rect.Max.Y+ui.Gap, ui.ColorSubdued())
	recording, _ := s.focused()
	now, suggestion := s.levelText(recording)
	top := trim.useSuggestion.Rect.Min.Y - ui.LineHeight*levelLines
	s.theme.DrawLabel(screen, "LEVEL · CHANGES EVERY CUE THAT PLAYS IT", left, top+ui.Gap, ui.ColorMuted())
	s.theme.DrawText(screen, now, left, top+ui.LineHeight, ui.ColorInk())
	s.theme.DrawText(screen, suggestion, left, top+ui.LineHeight*ui.Half, ui.ColorMuted())
	s.drawButtons(screen, trim.buttons()...)
	if trim.useSuggestion.Disabled && trim.dirty() {
		s.theme.DrawText(screen, "Apply the region to use its suggestion.", trim.setTarget.Rect.Max.X+ui.Inset,
			trim.setTarget.Rect.Min.Y+ui.Gap, ui.ColorSubdued())
	}
}
