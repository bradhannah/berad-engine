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
	labelPlay   = "Play"
)

// How loud the waveform's Play plays: as recorded, evened out to the
// reference, or as the selected cue plays it.
const (
	hearRecorded = "recorded"
	hearEvened   = "evened"
	hearInCue    = "cue"
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
	loop                                   *ui.Toggle
	suggest, all, apply, cancel, duplicate *ui.Button
	measure, useReference, setLevel        *ui.Button
	// play is the one Play of the recording in focus; the part and hear
	// buttons choose what it plays: the region or the whole file, as
	// recorded, evened out or as the selected cue plays it. wholeFile and
	// the hear choices last while the window is open, a cue's apart from a
	// recording's own.
	play                     *ui.Button
	partRegion, partWhole    *ui.Button
	hearRecorded, hearEvened *ui.Button
	hearCue                  *ui.Button
	wholeFile                bool
	hearOnCue, hearAlone     string
	// meterTop is where the recording's loudness meter is, and choiceTop
	// where the hear buttons are.
	meterTop, choiceTop int
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
		loop:    &ui.Toggle{ID: "loop", Label: "Loop", Rect: ui.Zero(), On: false},
		suggest: button("suggest-region", "Suggest"), all: button("whole-region", "Use whole file"),
		apply: button("apply-region", "Apply"), cancel: button("cancel-region", "Cancel"),
		duplicate: button("duplicate-region", "Duplicate region"), measure: button("measure", "Measure"),
		useReference: button("use-reference", "Use the reference"), setLevel: button("set-level", "Set by hand…"),
		play: button("play-part", labelPlay), partRegion: button("part-region", "Region"),
		partWhole: button("part-whole", "Whole file"), hearRecorded: button("hear-recorded", "As recorded"),
		hearEvened: button("hear-evened", "Evened out"), hearCue: button("hear-cue", "In this cue"), wholeFile: false,
		hearOnCue: hearInCue, hearAlone: hearEvened, meterTop: 0, choiceTop: 0,
	}
}

func (t *trimmer) buttons() []*ui.Button {
	return []*ui.Button{
		t.zoomIn, t.zoomOut, t.fit, t.play, t.partRegion, t.partWhole, t.suggest, t.all, t.apply, t.cancel,
		t.duplicate, t.measure, t.useReference, t.setLevel, t.hearRecorded, t.hearEvened, t.hearCue,
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
		Region: copyWindow(trim.draft), Version: "", DurationMs: 0, Peaks: nil, Suggested: nil, Sounds: nil,
		Calibration: nil, Problem: "",
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
	trim.play.Label, trim.play.Primary = labelPlay, true
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
	s.row(left, top, trim.play)
	s.segments(trim.play.Rect.Max.X+ui.Inset, top, trim.partRegion, trim.partWhole)
	trim.partRegion.On, trim.partWhole.On = !trim.wholeFile, trim.wholeFile
	trim.loop.Rect = image.Rect(trim.partWhole.Rect.Max.X+ui.Inset, top, trim.partWhole.Rect.Max.X+ui.Inset+loopWidth,
		top+ui.ButtonH)
	s.row(trim.loop.Rect.Max.X+ui.Inset, top, trim.duplicate)
	top += ui.ButtonH + ui.Inset + ui.LineHeight
	trim.start.Rect = image.Rect(left, top, left+fieldWidth, top+ui.ButtonH)
	trim.end.Rect = image.Rect(trim.start.Rect.Max.X+ui.Gap, top, trim.start.Rect.Max.X+ui.Gap+fieldWidth, top+ui.ButtonH)
	s.row(trim.end.Rect.Max.X+ui.Inset, top, trim.suggest, trim.all, trim.apply, trim.cancel)
	top = s.layoutLoudnessSection(left, top+ui.ButtonH+ui.LineHeight+ui.Inset+ui.LineHeight)
	trim.layoutWave()
	recording, _ := s.focused()
	trim.wave.Marks = marksOf(s.pieces(recording.File), recording.ID)
	trim.apply.Disabled, trim.cancel.Disabled = !trim.dirty(), !trim.dirty()
	trim.suggest.Disabled = trim.analysis == nil || trim.analysis.Suggested == nil

	return top
}

// layoutLoudnessSection places the recording's meter, its listening buttons
// and its level's buttons from a top edge, and returns where they end.
func (s *State) layoutLoudnessSection(left, top int) int {
	trim := s.trim
	trim.meterTop = top
	top += ui.RowHeight + ui.LineHeight*levelLines
	trim.choiceTop = top
	hear := []*ui.Button{trim.hearRecorded, trim.hearEvened}
	trim.hearCue.Rect = ui.Zero()
	if cue, ok := s.selectedCue(); ok {
		hear = append(hear, trim.hearCue)
		trim.hearCue.Disabled = cue.Gain <= 0
	}
	s.segments(left+s.theme.TextWidth(playItLabel)+ui.Inset, top, hear...)
	chosen := s.hearing()
	trim.hearRecorded.On, trim.hearEvened.On, trim.hearCue.On = chosen == hearRecorded, chosen == hearEvened,
		chosen == hearInCue
	top += ui.ButtonH + ui.Gap + ui.LineHeight
	recording, _ := s.focused()
	buttons := []*ui.Button{trim.measure, trim.setLevel}
	if recording.CalibrationMode == protocol.CalibrationManual || recording.TargetDB != 0 {
		buttons = []*ui.Button{trim.useReference, trim.setLevel}
	}
	s.row(left, top, buttons...)
	trim.measure.Disabled = trim.dirty() || recording.CalibrationMode == protocol.CalibrationAuto
	trim.measure.Label = "Measure"
	if recording.CalibrationMode == protocol.CalibrationAuto {
		trim.measure.Label = "Measured"
	}

	return top + ui.ButtonH + ui.Inset
}

// levelLines counts the lines about a recording's loudness under its meter:
// as recorded, and as evened out.
const levelLines = 2

// playItLabel names the hear buttons.
const playItLabel = "PLAY IT"

// segments lays buttons out side by side with no gap, as one choice.
func (s *State) segments(left, top int, buttons ...*ui.Button) {
	for _, button := range buttons {
		width := s.theme.TextWidth(button.Label) + ui.Half*ui.Inset
		button.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		left += width
	}
}

// hearing is how loud the waveform's Play plays: on a cue's screen, its
// choice there, as the cue plays it at first; on a recording's own, evened
// out at first.
func (s *State) hearing() string {
	if cue, ok := s.selectedCue(); ok {
		if s.trim.hearOnCue == hearInCue && cue.Gain <= 0 {
			return hearEvened
		}

		return s.trim.hearOnCue
	}
	if s.trim.hearAlone == hearInCue {
		return hearEvened
	}

	return s.trim.hearAlone
}

// chooseHearing sets how loud the waveform's Play plays, on this screen.
func (s *State) chooseHearing(hear string) {
	if _, ok := s.selectedCue(); ok {
		s.trim.hearOnCue = hear

		return
	}
	s.trim.hearAlone = hear
}

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
	case trim.play.Update(input):
		s.playPart(ctx)
	case trim.partRegion.Update(input):
		trim.wholeFile = false
		s.replay(ctx)
	case trim.partWhole.Update(input):
		trim.wholeFile = true
		s.replay(ctx)
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
	case trim.measure.Update(input):
		s.edit(ctx, protocol.AudioAcceptCalibration, protocol.AudioItem{ID: trim.recording},
			func(protocol.Result) { ctx.Frame.Status("Evened out to the reference. Undo puts it back.") })
	case trim.useReference.Update(input):
		s.edit(ctx, protocol.AudioUseReference, protocol.AudioItem{ID: trim.recording},
			func(protocol.Result) { ctx.Frame.Status("Measured to the reference again. Undo puts it back.") })
	case trim.setLevel.Update(input):
		s.prompt = s.levelDialog(trim.recording, s.currentLevel())
	default:
		s.updateHearing(input, ctx)
	}
}

// updateHearing handles the choice of how loud Play plays.
func (s *State) updateHearing(input *ui.Input, ctx *frame.Context) {
	trim := s.trim
	for hear, button := range map[string]*ui.Button{
		hearRecorded: trim.hearRecorded, hearEvened: trim.hearEvened, hearInCue: trim.hearCue,
	} {
		if button.Update(input) {
			s.chooseHearing(hear)
			s.replay(ctx)
		}
	}
}

// replay plays again, as now chosen, what Play is playing, so a choice can
// be compared with the last while it loops.
func (s *State) replay(ctx *frame.Context) {
	if s.trim.auditioned != s.trim.play || !auditioning(ctx.Sounds) {
		return
	}
	s.trim.auditioned = nil
	s.playPart(ctx)
}

// playPart plays, or stops, the recording in focus: the drafted region or
// the whole file, as loud as chosen.
func (s *State) playPart(ctx *frame.Context) {
	trim := s.trim
	window := copyWindow(trim.draft)
	if trim.wholeFile {
		window = nil
	}
	s.auditionAt(ctx, trim.play, window, s.hearingGain(window))
}

// hearingGain is the gain, in dB, Play plays part of the recording in focus
// at, as loud as chosen.
func (s *State) hearingGain(window *protocol.AudioWindow) float64 {
	switch s.hearing() {
	case hearRecorded:
		return 0
	case hearInCue:
		if cue, ok := s.selectedCue(); ok && cue.Gain > 0 {
			return s.evenedGain(window) + cueBoost(cue)
		}
	}

	return s.evenedGain(window)
}

// playSentence says what Play plays, and how loud.
func (s *State) playSentence() string {
	what := "the region"
	if s.trim.wholeFile {
		what = "the whole file"
	}
	recording, _ := s.focused()
	found := s.facts(recording)
	hear := s.hearing()
	cue, onCue := s.selectedCue()
	switch {
	case hear == hearRecorded && found.known:
		return "Play plays " + what + " as recorded, unaltered: " + than(found.recordedDB, "the reference") + "."
	case hear == hearRecorded:
		return "Play plays " + what + " as recorded, unaltered."
	case hear == hearInCue && onCue:
		return "Play plays " + what + " as " + cue.Name + " plays it at " + s.stepLabel(cue) + ": evened out, " +
			louderBy(cueBoost(cue)) + "."
	case !found.level:
		return "Play plays " + what + " as recorded: it is not evened out yet."
	}

	return "Play plays " + what + " evened out: at the reference, where every cue starts from."
}

// louderBy says how much louder or quieter a cue plays a recording than
// evened out.
func louderBy(boost float64) string {
	if math.Abs(boost) < noAdjustmentDB {
		return "at the reference"
	}
	louder := " louder"
	if boost < 0 {
		louder = " quieter"
	}

	return "then " + strconv.FormatFloat(math.Abs(boost), 'f', 1, 64) + " dB" + louder
}

// than says how much louder or quieter a loudness in dB is than another.
func than(level float64, other string) string {
	if math.Abs(level) < noAdjustmentDB {
		return "as loud as " + other
	}
	louder := " louder than "
	if level < 0 {
		louder = " quieter than "
	}

	return strconv.FormatFloat(math.Abs(level), 'f', 1, 64) + " dB" + louder + other
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

// evenedGain is the level, in dB, the recording in focus plays part of
// itself at: for a measured recording and a drafted region, the gain
// suggested for the draft.
func (s *State) evenedGain(window *protocol.AudioWindow) float64 {
	recording, _ := s.focused()
	analysis := s.trim.analysis
	if recording.CalibrationMode == protocol.CalibrationAuto && analysis != nil && analysis.Calibration != nil &&
		window != nil {
		return analysis.Calibration.GainDB
	}

	return recording.CalibrationDB
}

// auditionAt plays part of the recording in focus at a gain in dB. Pressed
// again while it plays, its button stops it.
func (s *State) auditionAt(ctx *frame.Context, button *ui.Button, window *protocol.AudioWindow, gainDB float64) {
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
	ctx.Sounds.Audition(recording.ID, window, gainDB, trim.loop.On)
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

// loudnessText says how loud the recording in focus is as recorded, and
// how it is evened out, in words first.
func (s *State) loudnessText(recording protocol.AudioRecording) (string, string, bool) {
	found := s.facts(recording)
	if !found.known {
		return found.problem, "", false
	}
	recorded := "As recorded: " + dbAmount(found.recordedDB) + " quieter than the reference."
	if found.recordedDB >= 0 {
		recorded = "As recorded: " + dbAmount(found.recordedDB) + " louder than the reference."
	}
	status, problem := evenStatus(found)
	switch {
	case !found.level:
		status = "Not measured yet: it plays as recorded. Measure evens it out."
	case problem:
		status += ", raised as far as it goes without distorting."
	case found.manual:
		status += ": " + decibels(recording.CalibrationDB) + ". Use the reference measures it again."
	case recording.TargetDB != 0:
		status = "Evened out to its own target, " + decibels(recording.TargetDB) + "."
	default:
		status = "Evened out to the reference automatically, the same in every cue that plays it."
	}

	return recorded, status, problem
}

// trimMarks is where the recording in focus sits on its meter: as recorded,
// evened out and, on a cue's screen, as the cue plays it, the one Play
// plays at ringed.
func (s *State) trimMarks(recording protocol.AudioRecording, problem bool) meterMarks {
	found := s.facts(recording)
	marks := meterMarks{
		recordedDB: found.recordedDB, playsDB: found.evenedDB, recorded: found.known, plays: found.known && found.level,
		problem: problem, cueDB: 0, ringDB: found.recordedDB, cue: false, ring: found.known,
	}
	evened := found.recordedDB
	if found.level {
		evened = found.evenedDB
	}
	if cue, ok := s.selectedCue(); ok && found.known && cue.Gain > 0 {
		marks.cueDB, marks.cue = evened+cueBoost(cue), true
	}
	switch s.hearing() {
	case hearEvened:
		marks.ringDB = evened
	case hearInCue:
		marks.ringDB, marks.ring = marks.cueDB, marks.cue
	}

	return marks
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
	s.drawLoudnessSection(screen, left, trim.wave.Rect.Max.X)
	s.drawButtons(screen, trim.buttons()...)
	if trim.measure.Disabled && trim.dirty() {
		s.theme.DrawText(screen, "Apply the region to measure it.", trim.setLevel.Rect.Max.X+ui.Inset,
			trim.setLevel.Rect.Min.Y+ui.Gap, ui.ColorSubdued())
	}
}

// drawLoudnessSection draws the recording's meter, what it says, and what
// Play plays, between a left and a right edge.
func (s *State) drawLoudnessSection(screen *ebiten.Image, left, right int) {
	trim := s.trim
	recording, _ := s.focused()
	legend := "LOUDNESS · HOLLOW AS RECORDED, BLUE EVENED OUT, THE LINE THE REFERENCE"
	if _, ok := s.selectedCue(); ok {
		legend = "LOUDNESS · HOLLOW AS RECORDED, BLUE EVENED OUT, BLACK IN THIS CUE"
	}
	s.theme.DrawLabel(screen, legend, left, trim.meterTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
	recorded, status, problem := s.loudnessText(recording)
	drawMeter(screen, image.Rect(left, trim.meterTop+(ui.RowHeight-meterHeight)/ui.Half, right,
		trim.meterTop+(ui.RowHeight-meterHeight)/ui.Half+meterHeight), s.trimMarks(recording, problem))
	top := trim.meterTop + ui.RowHeight
	s.theme.DrawText(screen, s.theme.FitText(recorded, right-left), left, top, ui.ColorInk())
	ink := ui.ColorInk()
	if problem {
		ink = ui.ColorWarn()
	}
	s.theme.DrawText(screen, s.theme.FitText(status, right-left), left, top+ui.LineHeight, ink)
	s.theme.DrawLabel(screen, playItLabel, left, trim.choiceTop+(ui.ButtonH-ui.LabelSize)/ui.Half, ui.ColorMuted())
	chosen := trim.hearEvened.Rect.Max.X
	if !trim.hearCue.Rect.Empty() {
		chosen = trim.hearCue.Rect.Max.X
	}
	s.theme.DrawText(screen, "Ringed on the meter.", chosen+ui.Inset, trim.choiceTop+ui.Gap, ui.ColorSubdued())
	s.theme.DrawText(screen, s.theme.FitText(s.playSentence(), right-left), left, trim.choiceTop+ui.ButtonH+ui.Gap,
		ui.ColorInk())
}
