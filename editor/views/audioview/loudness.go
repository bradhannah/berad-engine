package audioview

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/loudness"
)

// Loudness, as the window shows it: every recording is evened out to the
// project's reference loudness, measured so its author need not judge it by
// ear; a cue plays at a step around the reference, plus its category's
// adjustment. Meters show where a recording sits as recorded, hollow, and as
// it plays, filled, against the reference, a line.

// Meter scale and marks, in dB from the reference and screen pixels.
const (
	meterQuietest = -24.0
	meterLoudest  = 12.0
	meterHeight   = 16
	meterDot      = 11
	meterRing     = 19
	meterOverhang = 3
	// peakCeilingDB is berad-audio's loudest sample a level lets a recording
	// reach, which a louder step must stay under so it does not distort.
	peakCeilingDB = -3.0
	// evenTolerance is how far from the reference, in dB, still counts as
	// evened out.
	evenTolerance = 0.5
	referenceDB   = -20.0
)

// soundFacts is what the window knows of how loud a recording is, all in
// dB: as recorded and as evened out, both from the reference, and how much
// louder it can play before it distorts. known says it was measured; level
// that a level is set, by measuring or by hand; ownTargetDB a target its
// author set for it alone, 0 for the reference.
type soundFacts struct {
	known, level, manual bool
	problem              string
	recordedDB, evenedDB float64
	headroomDB           float64
	ownTargetDB          float64
}

// reference is the loudness recordings are evened out to.
func (s *State) reference() float64 {
	if s.library == nil || s.library.ReferenceDB == 0 {
		return referenceDB
	}

	return s.library.ReferenceDB
}

// adjustment is a category's adjustment, in dB.
func (s *State) adjustment(category string) float64 {
	if s.library == nil {
		return 0
	}

	return s.library.CategoryGainsDB[category]
}

// facts works out how loud a recording is from its last measurement.
func (s *State) facts(recording protocol.AudioRecording) soundFacts {
	found := soundFacts{
		known: false, level: false, manual: recording.CalibrationMode == protocol.CalibrationManual, problem: "",
		recordedDB: 0, evenedDB: 0, headroomDB: 0, ownTargetDB: recording.TargetDB,
	}
	found.level = recording.CalibrationMode == protocol.CalibrationAuto || found.manual
	measured, ok := s.measured[recording.ID]
	switch {
	case recording.Status == protocol.RecordingNoFile:
		found.problem = stateNoSound
	case !ok:
		found.problem = "Measuring…"
	case measured.Problem != "":
		found.problem = measured.Problem
	default:
		level := 0.0
		if found.level {
			level = recording.CalibrationDB
		}
		found.known = true
		found.recordedDB = measured.MeasuredDB - s.reference()
		found.evenedDB = found.recordedDB + level
		found.headroomDB = peakCeilingDB - (measured.PeakDB + level)
	}

	return found
}

// evenStatus says how a recording stands against the reference, and whether
// that is a problem.
func evenStatus(found soundFacts) (string, bool) {
	switch {
	case !found.known:
		return found.problem, false
	case !found.level:
		return "Not measured yet", true
	case found.ownTargetDB != 0 && !found.manual:
		return "Evened out to its own target, " + decibels(found.ownTargetDB), false
	case found.evenedDB < -evenTolerance:
		return "Short by " + dbAmount(-found.evenedDB) + ": it cannot get that loud", true
	case found.manual && found.evenedDB > evenTolerance:
		return "Set by hand, " + dbAmount(found.evenedDB) + " above the reference", false
	case found.manual:
		return "Set by hand", false
	}

	return "Evened out", false
}

// cueBoost is how many dB a cue plays its recordings above their evened-out
// loudness: its step and its category's adjustment.
func cueBoost(cue protocol.AudioCue) float64 {
	if cue.Gain <= 0 {
		return math.Inf(-1)
	}

	return loudness.DB(cue.Gain)
}

// cueStep is the step a cue plays at, its category's adjustment taken out,
// and false when its gain is between steps.
func (s *State) cueStep(cue protocol.AudioCue) (loudness.Step, float64, bool) {
	stepDB := loudness.CueStepDB(cue.Gain, s.adjustment(cue.Category))
	step, found := loudness.Match(stepDB)

	return step, stepDB, found
}

// stepLabel names the step a cue plays at, or its dB between steps.
func (s *State) stepLabel(cue protocol.AudioCue) string {
	step, stepDB, found := s.cueStep(cue)
	switch {
	case found:
		return step.Label
	case cue.Gain <= 0:
		return "Silent"
	}

	return "Custom, " + decibels(stepDB)
}

// playStatus says how a cue plays a recording: as meant, or so loud it
// would distort, by how much.
func playStatus(found soundFacts, cue protocol.AudioCue) (string, bool) {
	if status, problem := evenStatus(found); !found.known || problem {
		return status, problem
	}
	if over := cueBoost(cue) - found.headroomDB; over > evenTolerance/ui.Half {
		return "Too loud for this recording: it would distort by " + dbAmount(over), true
	}

	return "Plays as meant", false
}

// dbAmount writes a positive number of dB, as "3.5 dB".
func dbAmount(value float64) string { return strconv.FormatFloat(math.Abs(value), 'f', 1, 64) + " dB" }

// askLoudness asks how loud every recording measures, again once the
// question out now is answered; the orchestrator keeps its measurements, so
// asking after every change is quick.
func (s *State) askLoudness(ctx *frame.Context) {
	if s.measuring {
		s.remeasure = true

		return
	}
	s.measuring = true
	ctx.Requests.AudioLoudness(func(answer protocol.AudioLoudness, err error) {
		s.measuring = false
		if err != nil {
			ctx.Frame.Status("The recordings could not be measured: " + err.Error())
		} else {
			s.measured = map[string]protocol.RecordingLoudness{}
			for _, recording := range answer.Recordings {
				s.measured[recording.ID] = recording
			}
		}
		if s.remeasure {
			s.remeasure = false
			s.askLoudness(ctx)
		}
	})
}

// meterMarks is what a meter shows, in dB from the reference: where the
// recording sits as recorded and as it plays, whether each is known, and
// whether how it plays is a problem.
type meterMarks struct {
	recordedDB, playsDB float64
	recorded, plays     bool
	problem             bool
	// cueDB is where the selected cue plays the recording, darker than
	// evened out, when cue is set; ringDB is the mark Play plays at, ringed
	// when ring is set.
	cueDB, ringDB float64
	cue, ring     bool
}

// meterX is where a loudness falls across a meter.
func meterX(rect image.Rectangle, db float64) int {
	db = min(max(db, meterQuietest), meterLoudest)

	return rect.Min.X + int(math.Round((db-meterQuietest)/(meterLoudest-meterQuietest)*float64(rect.Dx())))
}

// drawMeter draws a meter: its track, the reference, and the marks.
func drawMeter(screen *ebiten.Image, rect image.Rectangle, marks meterMarks) {
	ui.FillRound(screen, rect, ui.Radius, ui.ColorPanel())
	ui.StrokeRound(screen, rect, ui.Radius, ui.Hairline, ui.ColorRuleSoft())
	middle := rect.Min.Y + rect.Dy()/ui.Half
	reference := meterX(rect, 0)
	ui.Line(screen, image.Pt(reference, rect.Min.Y-meterOverhang), image.Pt(reference, rect.Max.Y+meterOverhang),
		ui.Outline, ui.ColorInk())
	if marks.recorded && marks.plays {
		ui.Line(screen, image.Pt(meterX(rect, marks.recordedDB), middle), image.Pt(meterX(rect, marks.playsDB), middle),
			ui.Border, ui.ColorSubdued())
	}
	if marks.recorded {
		dot := meterDotAt(rect, marks.recordedDB, meterDot)
		ui.FillRound(screen, dot, meterDot/ui.Half, ui.ColorBackground())
		ui.StrokeRound(screen, dot, meterDot/ui.Half, ui.Border, ui.ColorMuted())
	}
	if marks.plays {
		ink := ui.ColorSelect()
		if marks.problem {
			ink = ui.ColorWarn()
		}
		dot := meterDotAt(rect, marks.playsDB, meterDot+ui.Half)
		ui.FillRound(screen, dot, float32(dot.Dx())/ui.Half, ink)
	}
	if marks.cue {
		dot := meterDotAt(rect, marks.cueDB, meterDot+ui.Half)
		ui.FillRound(screen, dot, float32(dot.Dx())/ui.Half, ui.ColorInk())
	}
	if marks.ring {
		ring := meterDotAt(rect, marks.ringDB, meterRing)
		ui.StrokeRound(screen, ring, meterRing/ui.Half, ui.Outline, ui.ColorSelect())
	}
}

// meterDotAt is a dot of a size centred on a loudness.
func meterDotAt(rect image.Rectangle, db float64, size int) image.Rectangle {
	x, y := meterX(rect, db), rect.Min.Y+rect.Dy()/ui.Half

	return image.Rect(x-size/ui.Half, y-size/ui.Half, x-size/ui.Half+size, y-size/ui.Half+size)
}

// drawStepPips draws five bars rising like the steps, those up to a step's
// filled, before a label.
func drawStepPips(screen *ebiten.Image, left, top int, step loudness.Step, ink color.Color) {
	const pipWidth, pipGap, pipRise, pipBase = 3, 2, 2, 3
	for index, each := range loudness.Steps() {
		height := pipBase + index*pipRise
		var fill color.Color = ui.ColorRuleSoft()
		if each.DB <= step.DB {
			fill = ink
		}
		x := left + index*(pipWidth+pipGap)
		ui.FillRect(screen, image.Rect(x, top+ui.ButtonH-ui.Gap*ui.Half-height, x+pipWidth,
			top+ui.ButtonH-ui.Gap*ui.Half), fill)
	}
}

// chainHeight is the height of how a cue plays: its label and its boxes, on
// two rows when they do not fit on one.
const chainHeight = ui.LineHeight + ui.ButtonH*ui.Half + ui.Gap

// cueMarks is where a recording sits as recorded and as a cue plays it.
func (s *State) cueMarks(recording protocol.AudioRecording, cue protocol.AudioCue) (meterMarks, string, bool) {
	found := s.facts(recording)
	status, problem := playStatus(found, cue)
	plays := found.recordedDB + cueBoost(cue)
	if found.level {
		plays = found.evenedDB + cueBoost(cue)
	}

	return meterMarks{
		recordedDB: found.recordedDB, playsDB: plays, recorded: found.known, plays: found.known && cue.Gain > 0,
		problem: problem, cueDB: 0, ringDB: 0, cue: false, ring: false,
	}, status, problem
}

// drawCueLoudness draws, under a cue's recordings, where each sits as this
// cue plays it, then how the cue plays.
func (s *State) drawCueLoudness(screen *ebiten.Image, cue protocol.AudioCue, left int) {
	pane := s.panel
	right := s.panelArea().Max.X - ui.Inset
	s.theme.DrawLabel(screen, "LOUDNESS · HOLLOW AS RECORDED, FILLED AS THIS CUE PLAYS IT, THE LINE THE REFERENCE",
		left, pane.loudTop-ui.LineHeight+ui.Gap, ui.ColorMuted())
	statusWidth := min(loudStatusWidth, (right-left)/ui.Half)
	meterLeft, meterRight := left+loudNameWidth, right-statusWidth-ui.Inset
	for index, recording := range cue.Recordings {
		top := pane.loudTop + index*(ui.RowHeight+loudRowGap)
		s.theme.DrawMono(screen, s.theme.FitText(itoa(index+1)+"  "+recording.ID, loudNameWidth-ui.Gap), left,
			top+ui.Gap, ui.ColorInk())
		if recording.Status == protocol.RecordingNoFile {
			s.theme.DrawText(screen, stateNoSound, meterLeft, top+ui.Gap, ui.ColorSubdued())

			continue
		}
		marks, status, problem := s.cueMarks(recording, cue)
		if meterRight > meterLeft {
			drawMeter(screen, image.Rect(meterLeft, top+(ui.RowHeight-meterHeight)/ui.Half, meterRight,
				top+(ui.RowHeight-meterHeight)/ui.Half+meterHeight), marks)
		}
		ink := ui.ColorMuted()
		if problem {
			ink = ui.ColorWarn()
		}
		s.theme.DrawText(screen, s.theme.FitText(status, statusWidth), meterRight+ui.Inset, top+ui.Gap, ink)
	}
	s.drawChain(screen, cue, left, pane.chainTop)
}

// drawChain draws how a cue plays: each recording evened out, then the
// cue's step, then a Play sound's, then the player's volume.
func (s *State) drawChain(screen *ebiten.Image, cue protocol.AudioCue, left, top int) {
	s.theme.DrawLabel(screen, "HOW IT PLAYS", left, top+ui.Gap, ui.ColorMuted())
	top += ui.LineHeight
	boxes := []struct {
		text string
		ours bool
	}{
		{"Each recording · evened out", false},
		{"This cue · " + categoryLabel(cue.Category) + " · " + s.stepLabel(cue), true},
		{"A Play sound · as the cue is, unless it asks", false},
		{"The player · their " + categoryLabel(cue.Category) + " volume", false},
	}
	start, right := left, s.panelArea().Max.X-ui.Inset
	for index, box := range boxes {
		width := s.theme.TextWidth(box.text) + ui.Inset
		if left+width > right && left > start {
			left, top = start, top+ui.ButtonH+ui.Gap
		}
		rect := image.Rect(left, top, left+width, top+ui.ButtonH)
		fill, ink := ui.ColorPanel(), ui.ColorInk()
		if box.ours {
			fill, ink = ui.ColorSelectWash(), ui.ColorSelect()
		}
		ui.FillRound(screen, rect, ui.Radius, fill)
		ui.StrokeRound(screen, rect, ui.Radius, ui.Hairline, ui.ColorRuleSoft())
		s.theme.DrawTextIn(screen, box.text, rect, ink)
		left = rect.Max.X + ui.Gap
		if next := index + 1; next < len(boxes) &&
			left+s.theme.TextWidth("› "+boxes[next].text)+ui.Inset+ui.Gap <= right {
			s.theme.DrawText(screen, "›", left, top+ui.Gap, ui.ColorSubdued())
			left += s.theme.TextWidth("›") + ui.Gap
		}
	}
}

// drawUsedLoudness draws, beside each cue that plays a recording, its step
// and where it plays the recording.
func (s *State) drawUsedLoudness(screen *ebiten.Image, recording protocol.AudioRecording) {
	right := s.panelArea().Max.X - ui.Inset
	cues := s.usedBy(recording.ID)
	for index, button := range s.panel.cues {
		if index >= len(cues) || button.Rect.Empty() {
			continue
		}
		cue := cues[index]
		left := button.Rect.Max.X + ui.Inset
		step := s.stepLabel(cue)
		s.theme.DrawText(screen, step, left, button.Rect.Min.Y+ui.Gap, ui.ColorMuted())
		left += max(s.theme.TextWidth(step), loudNameWidth/ui.Half) + ui.Inset
		marks, status, problem := s.cueMarks(recording, cue)
		statusWidth := min(loudStatusWidth, (right-left)/ui.Half)
		meterRight := right - statusWidth - ui.Inset
		if meterRight > left {
			drawMeter(screen, image.Rect(left, button.Rect.Min.Y+(ui.ButtonH-meterHeight)/ui.Half, meterRight,
				button.Rect.Min.Y+(ui.ButtonH-meterHeight)/ui.Half+meterHeight), marks)
		}
		ink := ui.ColorMuted()
		if problem {
			ink = ui.ColorWarn()
		}
		s.theme.DrawText(screen, s.theme.FitText(status, statusWidth), meterRight+ui.Inset, button.Rect.Min.Y+ui.Gap,
			ink)
	}
}
