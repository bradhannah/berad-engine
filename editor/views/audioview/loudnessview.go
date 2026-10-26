package audioview

import (
	"image"
	"path"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/loudness"
)

// The Loudness view: the reference loudness every recording is evened out
// to and each category's adjustment, Measure all, and a check of every
// recording and every cue. It shows in place of the panel and inspector, from
// the toolbar.

// Loudness view layout and steps.
const (
	loudStepDB      = 1.0
	minReferenceDB  = -60.0
	maxReferenceDB  = -6.0
	loudValueWidth  = 76
	loudNameWidth   = 150
	loudFileWidth   = 170
	loudStatusWidth = 300
	loudRowGap      = 4
	// loudSettingButtons counts the reference's buttons and Measure all.
	loudSettingButtons = 3
)

type loudnessView struct {
	toggle          *ui.Button
	shown           bool
	quieter, louder *ui.Button
	// adjust holds each category's buttons, quieter then louder.
	adjust     map[string][2]*ui.Button
	measureAll *ui.Button
	// scroll is how far it is scrolled down, at most furthest; the tops are
	// where its parts start, as laid out.
	scroll, furthest                      int
	referenceTop, adjustTop               int
	recordingsTop, cuesTop, summaryHeight int
}

func newLoudnessView() *loudnessView {
	view := &loudnessView{
		toggle: ui.NewButton("loudness", "Loudness…"), shown: false,
		quieter: ui.NewButton("reference-quieter", "−"), louder: ui.NewButton("reference-louder", "+"),
		adjust: map[string][2]*ui.Button{}, measureAll: ui.NewButton("measure-all", "Measure all"),
		scroll: 0, furthest: 0, referenceTop: 0, adjustTop: 0, recordingsTop: 0, cuesTop: 0, summaryHeight: 0,
	}
	for _, category := range categories() {
		view.adjust[category] = [2]*ui.Button{
			ui.NewButton("adjust-quieter-"+category, "−"), ui.NewButton("adjust-louder-"+category, "+"),
		}
	}

	return view
}

func (view *loudnessView) buttons() []*ui.Button {
	buttons := make([]*ui.Button, 0, loudSettingButtons+len(categories())*ui.Half)
	buttons = append(buttons, view.quieter, view.louder, view.measureAll)
	for _, category := range categories() {
		buttons = append(buttons, view.adjust[category][0], view.adjust[category][1])
	}

	return buttons
}

// loudnessArea is where the view shows: the panel's and inspector's parts.
func (s *State) loudnessArea() image.Rectangle {
	content := frame.Content(s.size)

	return image.Rect(listWidth, toolbarHeight, s.size.X, content.Max.Y)
}

// everyRecording lists the library's recordings with a file, each once,
// ordered by ID.
func (s *State) everyRecording() []protocol.AudioRecording {
	var all []protocol.AudioRecording
	seen := map[string]bool{}
	if s.library == nil {
		return all
	}
	for _, cue := range s.library.Cues {
		for _, recording := range cue.Recordings {
			if !seen[recording.ID] && recording.Status != protocol.RecordingNoFile {
				seen[recording.ID] = true
				all = append(all, recording)
			}
		}
	}
	for _, recording := range s.library.Unused {
		if !seen[recording.ID] && recording.Status != protocol.RecordingNoFile {
			seen[recording.ID] = true
			all = append(all, recording)
		}
	}
	slices.SortFunc(all, func(a, b protocol.AudioRecording) int {
		if a.ID < b.ID {
			return -1
		}

		return 1
	})

	return all
}

// layoutLoudness places the view's buttons, and keeps its scroll within it.
func (s *State) layoutLoudness() {
	view := s.loud
	for _, button := range view.buttons() {
		button.Rect = ui.Zero()
	}
	if !view.shown {
		return
	}
	area := s.loudnessArea()
	bottom := s.layoutLoudnessContent()
	view.furthest = max(bottom+view.scroll-area.Max.Y, 0)
	if view.scroll > view.furthest {
		view.scroll = view.furthest
		s.layoutLoudnessContent()
	}
}

// layoutLoudnessContent places the buttons and the tables' tops, and returns
// where the view ends.
func (s *State) layoutLoudnessContent() int {
	view, area := s.loud, s.loudnessArea()
	left, right := area.Min.X+ui.Inset, area.Max.X-ui.Inset
	top := area.Min.Y + ui.Inset - view.scroll + ui.LineHeight*ui.Half + ui.Inset
	view.referenceTop = top
	s.stepper(view.quieter, view.louder, left+loudNameWidth, top)
	width := s.theme.TextWidth(view.measureAll.Label) + ui.Half*ui.Inset
	view.measureAll.Rect = image.Rect(right-width, top, right, top+ui.ButtonH)
	reference := s.reference()
	view.quieter.Disabled, view.louder.Disabled = reference-loudStepDB < minReferenceDB,
		reference+loudStepDB > maxReferenceDB
	top += ui.ButtonH + ui.Inset + ui.LineHeight
	view.adjustTop = top
	for _, category := range categories() {
		buttons := view.adjust[category]
		s.stepper(buttons[0], buttons[1], left+loudNameWidth, top)
		adjustment := s.adjustment(category)
		buttons[0].Disabled = adjustment-loudStepDB < loudness.MinCategoryDB
		buttons[1].Disabled = adjustment+loudStepDB > loudness.MaxCategoryDB
		top += ui.ButtonH + ui.Gap
	}
	top += ui.Inset + ui.LineHeight*ui.Half
	view.recordingsTop = top
	top += ui.LineHeight + len(s.everyRecording())*(ui.RowHeight+loudRowGap) + ui.Inset + ui.LineHeight
	view.cuesTop = top
	cues := 0
	if s.library != nil {
		cues = len(s.library.Cues)
	}

	return top + ui.LineHeight + cues*(ui.RowHeight+loudRowGap) + ui.Inset
}

// stepper places a quieter and a louder button either side of a value.
func (s *State) stepper(quieter, louder *ui.Button, left, top int) {
	quieter.Rect = image.Rect(left, top, left+gainWidth, top+ui.ButtonH)
	right := quieter.Rect.Max.X + loudValueWidth
	louder.Rect = image.Rect(right, top, right+gainWidth, top+ui.ButtonH)
}

// updateLoudness handles the view's buttons and its scrolling.
func (s *State) updateLoudness(input *ui.Input, ctx *frame.Context) {
	view := s.loud
	if !view.shown {
		return
	}
	if input.WheelY != 0 && input.Cursor.In(s.loudnessArea()) {
		view.scroll = min(max(view.scroll-int(input.WheelY*ui.RowHeight), 0), view.furthest)
		input.WheelY = 0
	}
	switch {
	case view.quieter.Update(input):
		s.setReference(ctx, s.reference()-loudStepDB)
	case view.louder.Update(input):
		s.setReference(ctx, s.reference()+loudStepDB)
	case view.measureAll.Update(input):
		s.edit(ctx, protocol.AudioMeasureAll, protocol.AudioItem{ID: ""},
			func(protocol.Result) { ctx.Frame.Status("Every recording is measured. Undo puts the levels back.") })
	}
	for _, category := range categories() {
		buttons := view.adjust[category]
		for index, direction := range []float64{-loudStepDB, loudStepDB} {
			if buttons[index].Update(input) {
				s.adjustCategory(ctx, category, s.adjustment(category)+direction)
			}
		}
	}
}

// setReference sets the reference loudness, measuring every measured
// recording again.
func (s *State) setReference(ctx *frame.Context, target float64) {
	s.edit(ctx, protocol.AudioSetReference, protocol.ReferenceChange{TargetDB: target},
		func(protocol.Result) {
			ctx.Frame.Status("Recordings are evened out to " + decibels(target) + ". Undo puts it back.")
		})
}

// adjustCategory sets a category's adjustment, moving every cue in it.
func (s *State) adjustCategory(ctx *frame.Context, category string, gain float64) {
	s.edit(ctx, protocol.AudioSetCategoryGain, protocol.CategoryGainChange{Category: category, GainDB: gain},
		func(protocol.Result) {
			ctx.Frame.Status(categoryLabel(category) + " cues are adjusted " + signedDB(gain) + ". Undo puts it back.")
		})
}

// drawLoudness draws the view: its settings, then every recording and cue.
func (s *State) drawLoudness(window *ebiten.Image) {
	view, area := s.loud, s.loudnessArea()
	screen, clipped := window.SubImage(area).(*ebiten.Image)
	if !clipped {
		return
	}
	ui.FillRect(screen, area, ui.ColorBackground())
	ui.Divider(screen, area.Min, image.Pt(area.Min.X, area.Max.Y))
	left, right := area.Min.X+ui.Inset, area.Max.X-ui.Inset
	top := area.Min.Y + ui.Inset - view.scroll
	s.theme.DrawTitle(screen, "Loudness", left, top, ui.ColorInk())
	s.theme.DrawText(screen, s.theme.FitText("Every recording is evened out to the reference, measured rather than "+
		"judged by ear; a cue then plays at a step around it.", right-left), left, top+ui.LineHeight, ui.ColorMuted())
	s.drawSetting(screen, "REFERENCE", decibels(s.reference()), view.quieter, view.louder,
		"Every recording is evened out to this.")
	s.theme.DrawLabel(screen, "CATEGORY ADJUSTMENTS · EVERY CUE OF THE CATEGORY APPLIES ITS OWN", left,
		view.adjustTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
	for _, category := range categories() {
		buttons := view.adjust[category]
		s.drawSetting(screen, categoryLabel(category), signedDB(s.adjustment(category)), buttons[0], buttons[1], "")
	}
	s.drawButtons(screen, view.measureAll)
	s.drawRecordingChecks(screen, left, right)
	s.drawCueChecks(screen, left, right)
	if view.furthest > 0 {
		height := max(area.Dy()*area.Dy()/(area.Dy()+view.furthest), ui.ButtonH)
		thumbTop := area.Min.Y + (area.Dy()-height)*view.scroll/view.furthest
		ui.FillRound(screen, image.Rect(area.Max.X-ui.Gap/ui.Half-thumbWidth, thumbTop, area.Max.X-ui.Gap/ui.Half,
			thumbTop+height), ui.Half, ui.ColorRule())
	}
}

// drawSetting draws a setting's name, its value between its buttons, and a
// note after them.
func (s *State) drawSetting(screen *ebiten.Image, name, value string, quieter, louder *ui.Button, note string) {
	top := quieter.Rect.Min.Y
	left := s.loudnessArea().Min.X + ui.Inset
	if name == "REFERENCE" {
		s.theme.DrawLabel(screen, "REFERENCE LOUDNESS", left, top+(ui.ButtonH-ui.LabelSize)/ui.Half, ui.ColorMuted())
	} else {
		s.theme.DrawText(screen, name, left, top+ui.Gap, ui.ColorInk())
	}
	s.theme.DrawTextIn(screen, value, image.Rect(quieter.Rect.Max.X, top, louder.Rect.Min.X, louder.Rect.Max.Y),
		ui.ColorInk())
	if note != "" {
		s.theme.DrawText(screen, note, louder.Rect.Max.X+ui.Inset, top+ui.Gap, ui.ColorMuted())
	}
	s.drawButtons(screen, quieter, louder)
}

// drawRecordingChecks lists every recording: its file, a meter from as
// recorded to evened out, and whether it is.
func (s *State) drawRecordingChecks(screen *ebiten.Image, left, right int) {
	top := s.loud.recordingsTop
	recordings := s.everyRecording()
	evened, short, unmeasured := 0, 0, 0
	for _, recording := range recordings {
		found := s.facts(recording)
		_, problem := evenStatus(found)
		switch {
		case found.known && !found.level:
			unmeasured++
		case problem:
			short++
		case found.known:
			evened++
		}
	}
	summary := "RECORDINGS · " + itoa(evened) + " OF " + itoa(len(recordings)) + " EVENED OUT · " + itoa(short) +
		" CANNOT REACH THE REFERENCE · " + itoa(unmeasured) + " NOT MEASURED YET"
	s.theme.DrawLabel(screen, s.theme.FitText(summary, right-left), left, top+ui.Gap, ui.ColorMuted())
	top += ui.LineHeight
	meterLeft := left + loudNameWidth + loudFileWidth
	meterRight := max(right-loudStatusWidth-ui.Inset, meterLeft+ui.Inset)
	for _, recording := range recordings {
		found := s.facts(recording)
		status, problem := evenStatus(found)
		s.theme.DrawMono(screen, s.theme.FitText(recording.ID, loudNameWidth-ui.Gap), left, top+ui.Gap, ui.ColorInk())
		s.theme.DrawText(screen, s.theme.FitText(path.Base(recording.File), loudFileWidth-ui.Gap),
			left+loudNameWidth, top+ui.Gap, ui.ColorSubdued())
		meter := image.Rect(meterLeft, top+(ui.RowHeight-meterHeight)/ui.Half, meterRight,
			top+(ui.RowHeight-meterHeight)/ui.Half+meterHeight)
		drawMeter(screen, meter, meterMarks{
			recordedDB: found.recordedDB, playsDB: found.evenedDB, recorded: found.known, plays: found.known && found.level,
			problem: problem, cueDB: 0, ringDB: 0, cue: false, ring: false,
		})
		ink := ui.ColorInk()
		if problem {
			ink = ui.ColorWarn()
		}
		s.theme.DrawText(screen, s.theme.FitText(status, loudStatusWidth), meterRight+ui.Inset, top+ui.Gap, ink)
		top += ui.RowHeight + loudRowGap
	}
}

// drawCueChecks lists every cue: its category, the step it plays at, and
// whether every recording can sound as meant.
func (s *State) drawCueChecks(screen *ebiten.Image, left, right int) {
	top := s.loud.cuesTop
	s.theme.DrawLabel(screen, "CUES · CAN EVERY RECORDING SOUND AS MEANT?", left, top+ui.Gap, ui.ColorMuted())
	top += ui.LineHeight
	if s.library == nil {
		return
	}
	for _, cue := range s.library.Cues {
		status, problem := "Every recording can", false
		for _, recording := range cue.Recordings {
			if recording.Status == protocol.RecordingNoFile {
				continue
			}
			if said, wrong := playStatus(s.facts(recording), cue); wrong {
				status, problem = recording.ID+": "+said, true

				break
			}
		}
		s.theme.DrawText(screen, s.theme.FitText(cue.Name, loudNameWidth-ui.Gap), left, top+ui.Gap, ui.ColorInk())
		s.theme.DrawText(screen, categoryLabel(cue.Category), left+loudNameWidth, top+ui.Gap, ui.ColorMuted())
		s.theme.DrawText(screen, s.stepLabel(cue), left+loudNameWidth+loudFileWidth/ui.Half, top+ui.Gap, ui.ColorInk())
		ink := ui.ColorInk()
		if problem {
			ink = ui.ColorWarn()
		}
		statusLeft := left + loudNameWidth + loudFileWidth + ui.Inset
		s.theme.DrawText(screen, s.theme.FitText(status, right-statusLeft), statusLeft, top+ui.Gap, ink)
		top += ui.RowHeight + loudRowGap
	}
}
