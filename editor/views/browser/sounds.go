package browser

import (
	"fmt"
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Sounds (increment 32): a category's sounds as rows, each with its
// waveform, its length and how loud it measures against its target, so one
// too loud or too quiet stands out without playing it; a sound plays in
// place. Loudness is measured, as the Audio window measures it, not judged
// by ear.

// Sizes and measures of the sounds' columns.
const (
	waveWidth       = 140
	lengthWidth     = 56
	loudWidth       = 210
	playWidth       = 40
	waveColumns     = 80
	waveRise        = 3
	soundsAskedMost = 2
	soundMeterWidth = 90
	soundMeterH     = 8
	playRadius      = 9
	peakTop         = 255
	millisPerSecond = 1000
	secondsPerMin   = 60
	// soundQuietest and soundLoudest span a sound's meter, in dB from the
	// reference loudness.
	soundQuietest = -24.0
	soundLoudest  = 12.0
)

// Column titles of the sounds' rows.
const (
	columnWave     = "Waveform"
	columnLength   = "Length"
	columnLoudness = "Loudness"
	columnPlay     = "Play"
)

// soundState is what the window knows of the sounds shown: each recording's
// peaks, by its ID and version, how loud each measures, and which cues are
// playing.
type soundState struct {
	peaks     map[string][]byte
	asking    map[string]bool
	loud      map[string]protocol.RecordingLoudness
	askedLoud bool
	playing   map[string]bool
}

func newSoundState() *soundState {
	return &soundState{
		peaks: map[string][]byte{}, asking: map[string]bool{}, loud: map[string]protocol.RecordingLoudness{},
		askedLoud: false, playing: map[string]bool{},
	}
}

// soundColumns are the sounds' columns, the play button first, so it stays
// when the pane is too narrow for them all.
func soundColumns() []rowColumn {
	blank := func(string) rowColumn { return rowColumn{title: "", width: 0, cell: nil, key: nil} }
	columns := []rowColumn{blank(columnPlay), blank(columnWave), blank(columnLength), blank(columnLoudness)}
	for position, each := range []struct {
		title string
		width int
	}{{columnPlay, playWidth}, {columnWave, waveWidth}, {columnLength, lengthWidth}, {columnLoudness, loudWidth}} {
		columns[position].title, columns[position].width = each.title, each.width
	}

	return columns
}

// cueOf returns the cue a row shows.
func (s *State) cueOf(rowID string) (protocol.AudioCue, bool) {
	var none protocol.AudioCue
	if s.audio.library == nil || len(rowID) <= len(audioCuePrefix) || rowID[:len(audioCuePrefix)] != audioCuePrefix {
		return none, false
	}
	for _, cue := range s.audio.library.Cues {
		if audioCuePrefix+cue.ID == rowID {
			return cue, true
		}
	}

	return none, false
}

// heardRecording returns the recording of a cue the sounds' rows show: its
// first with a file.
func heardRecording(cue protocol.AudioCue) (protocol.AudioRecording, bool) {
	for _, recording := range cue.Recordings {
		if recording.Status != protocol.RecordingNoFile {
			return recording, true
		}
	}

	var none protocol.AudioRecording

	return none, false
}

// fillSoundCells gives each sound's row its length; its waveform, loudness
// and play button are drawn over it.
func (s *State) fillSoundCells(rows []ui.Row) {
	for position := range rows {
		cue, isCue := s.cueOf(rows[position].ID)
		if !isCue {
			continue
		}
		length := ""
		if recording, heard := heardRecording(cue); heard {
			length = lengthText(recording.LengthMs)
		}
		cells := make([]string, len(s.columns))
		for column, each := range s.columns {
			if each.title == columnLength {
				cells[column] = length
			}
		}
		rows[position].Cells = cells
	}
}

// lengthText says how long a sound is, as "0:12", or "0.8 s" under a
// second.
func lengthText(millis int64) string {
	if millis <= 0 {
		return ""
	}
	if millis < millisPerSecond {
		return strconv.FormatFloat(float64(millis)/millisPerSecond, 'f', 1, 64) + " s"
	}
	seconds := (millis + millisPerSecond/2) / millisPerSecond

	return fmt.Sprintf("%d:%02d", seconds/secondsPerMin, seconds%secondsPerMin)
}

// syncSounds asks, while sounds are shown, how loud every recording
// measures, once, and for the waveforms of those shown, a few at a time;
// and notes which are playing.
func (s *State) syncSounds(ctx *frame.Context) {
	if kindOfPlace(s.place) != sectionAudio {
		return
	}
	if !s.sounds.askedLoud {
		s.sounds.askedLoud = true
		ctx.Requests.AudioLoudness(func(answer protocol.AudioLoudness, err error) {
			if err != nil {
				return
			}
			for _, measured := range answer.Recordings {
				s.sounds.loud[measured.ID] = measured
			}
		})
	}
	clear(s.sounds.playing)
	for _, row := range s.list.Rows {
		cue, isCue := s.cueOf(row.ID)
		if !isCue {
			continue
		}
		s.sounds.playing[cue.ID] = ctx.Sounds != nil && ctx.Sounds.Playing(cue.ID)
		recording, heard := heardRecording(cue)
		key := recording.ID + "@" + recording.Version
		if !heard || s.sounds.peaks[key] != nil || s.sounds.asking[key] || len(s.sounds.asking) >= soundsAskedMost {
			continue
		}
		s.sounds.asking[key] = true
		asked := protocol.AudioAnalysis{
			Recording: recording.ID, From: 0, To: 0, Columns: waveColumns, Region: nil, Version: "", DurationMs: 0,
			Peaks: nil, Suggested: nil, Sounds: nil, Calibration: nil, Problem: "",
		}
		ctx.Requests.AudioAnalysis(asked, func(answer protocol.AudioAnalysis, err error) {
			delete(s.sounds.asking, key)
			if err == nil {
				s.sounds.peaks[key] = answer.Peaks
			}
		})
	}
}

// soundsHeard forgets what was measured once the audio library changes, so
// it is measured again.
func (s *State) soundsHeard() {
	s.sounds.askedLoud = false
	clear(s.sounds.loud)
}

// loudnessOf says how a recording stands against the reference loudness,
// as the Audio window says it, whether that is a problem, and how far from
// the reference it plays, when known.
func (s *State) loudnessOf(recording protocol.AudioRecording) (string, bool, float64, bool) {
	measured, known := s.sounds.loud[recording.ID]
	reference := 0.0
	if s.audio.library != nil {
		reference = s.audio.library.ReferenceDB
	}

	return audioview.Stands(recording, measured, known, reference)
}

// soundColumnRect returns where a column of a row shows.
func (s *State) soundColumnRect(row image.Rectangle, title string) (image.Rectangle, bool) {
	for _, each := range s.columns {
		if each.title == title {
			left := s.list.Rect.Min.X + each.at

			return image.Rect(left, row.Min.Y, left+each.width-ui.Inset, row.Max.Y), true
		}
	}

	return ui.Zero(), false
}

// updateSounds plays or stops a sound whose play button is clicked.
func (s *State) updateSounds(input *ui.Input, ctx *frame.Context) {
	if !input.Pressed || kindOfPlace(s.place) != sectionAudio || ctx.Sounds == nil {
		return
	}
	for index, row := range s.list.Rows {
		cue, isCue := s.cueOf(row.ID)
		button, shown := s.soundColumnRect(s.list.RowRect(index), columnPlay)
		if !isCue || !shown || !input.Cursor.In(button) || !input.Cursor.In(s.list.Rect) {
			continue
		}
		input.ClearPointer()
		if ctx.Sounds.Playing(cue.ID) {
			ctx.Sounds.Stop()
		} else {
			ctx.Sounds.Play(cue.ID)
		}
	}
}

// locatePlay finds a sound's play button, for input scripts.
func (s *State) locatePlay(name string) (image.Point, bool) {
	for index, row := range s.list.Rows {
		if row.Label != name {
			continue
		}
		s.list.Reveal(row.ID)
		if button, shown := s.soundColumnRect(s.list.RowRect(index), columnPlay); shown {
			return ui.Center(button), true
		}
	}

	return image.Pt(0, 0), false
}

// hasSoundState answers "sound:<name>=<word>", what a sound's loudness says.
func (s *State) hasSoundState(name, word string) bool {
	for _, row := range s.list.Rows {
		cue, isCue := s.cueOf(row.ID)
		if !isCue || row.Label != name {
			continue
		}
		recording, heard := heardRecording(cue)
		said, _, _, _ := s.loudnessOf(recording)

		return heard && said == word
	}

	return false
}

// drawSounds draws each sound's waveform, loudness and play button over
// its row.
func (s *State) drawSounds(screen *ebiten.Image) {
	if kindOfPlace(s.place) != sectionAudio || s.viewShown() != viewRows {
		return
	}
	area, ok := screen.SubImage(s.list.Rect).(*ebiten.Image)
	if !ok {
		return
	}
	for index, row := range s.list.Rows {
		rect := s.list.RowRect(index)
		cue, isCue := s.cueOf(row.ID)
		if !isCue || !rect.Overlaps(s.list.Rect) {
			continue
		}
		recording, heard := heardRecording(cue)
		if wave, shown := s.soundColumnRect(rect, columnWave); shown {
			s.drawWave(area, wave, recording, heard)
		}
		if loud, shown := s.soundColumnRect(rect, columnLoudness); shown && heard {
			s.drawSoundLoudness(area, loud, recording)
		}
		if button, shown := s.soundColumnRect(rect, columnPlay); shown && heard {
			drawPlay(area, button, s.sounds.playing[cue.ID])
		}
	}
}

// drawWave draws a recording's waveform, or says it has no sound yet.
func (s *State) drawWave(screen *ebiten.Image, rect image.Rectangle, recording protocol.AudioRecording, heard bool) {
	if !heard {
		s.theme.DrawText(screen, "no sound yet", rect.Min.X, rect.Min.Y+ui.Gap, ui.ColorSubdued())

		return
	}
	peaks := s.sounds.peaks[recording.ID+"@"+recording.Version]
	if len(peaks) == 0 {
		return
	}
	middle, reach := float32(rect.Min.Y+rect.Dy()/ui.Half), float32(rect.Dy()/ui.Half-waveRise)
	for column := 0; column < rect.Dx(); column += ui.Half {
		peak := float32(peaks[column*len(peaks)/max(rect.Dx(), 1)]) / peakTop
		height := max(peak*reach, 1)
		x := float32(rect.Min.X + column)
		vector.StrokeLine(screen, x, middle-height, x, middle+height, 1, ui.ColorSound(), true)
	}
}

// drawSoundLoudness draws where a recording plays against its target, a
// line, with what that says, in the warning colour when it is off.
func (s *State) drawSoundLoudness(screen *ebiten.Image, rect image.Rectangle, recording protocol.AudioRecording) {
	said, warn, evened, known := s.loudnessOf(recording)
	meter := image.Rect(rect.Min.X, rect.Min.Y+(rect.Dy()-soundMeterH)/ui.Half, rect.Min.X+soundMeterWidth,
		rect.Min.Y+(rect.Dy()+soundMeterH)/ui.Half)
	ui.FillRound(screen, meter, soundMeterH/ui.Half, ui.ColorDesk())
	across := func(db float64) float32 {
		return float32(meter.Min.X) + float32((db-soundQuietest)/(soundLoudest-soundQuietest))*float32(meter.Dx())
	}
	vector.StrokeLine(screen, across(0), float32(meter.Min.Y-ui.Gap), across(0), float32(meter.Max.Y+ui.Gap), 1,
		ui.ColorInk(), true)
	ink, textInk := ui.ColorSound(), ui.ColorSubdued()
	if warn {
		ink, textInk = ui.ColorWarn(), ui.ColorWarn()
	}
	if known {
		dot := across(min(max(evened, soundQuietest), soundLoudest))
		vector.FillCircle(screen, dot, float32(meter.Min.Y+meter.Dy()/ui.Half), soundMeterH/ui.Half+1, ink, true)
	}
	s.theme.DrawText(screen, s.theme.FitText(said, rect.Dx()-soundMeterWidth-ui.Inset), meter.Max.X+ui.Inset,
		rect.Min.Y+ui.Gap, textInk)
}

// drawPlay draws a play button, or a stop button while its sound plays.
func drawPlay(screen *ebiten.Image, rect image.Rectangle, playing bool) {
	centre := ui.Center(rect)
	centreX, centreY := float32(centre.X), float32(centre.Y)
	vector.StrokeCircle(screen, centreX, centreY, playRadius, 1, ui.ColorRule(), true)
	if playing {
		vector.FillRect(screen, centreX-playRadius/waveRise, centreY-playRadius/waveRise, playRadius*ui.Half/waveRise,
			playRadius*ui.Half/waveRise, ui.ColorInk(), true)

		return
	}
	var triangle vector.Path
	triangle.MoveTo(centreX-playRadius/waveRise, centreY-playRadius/ui.Half)
	triangle.LineTo(centreX+playRadius/ui.Half, centreY)
	triangle.LineTo(centreX-playRadius/waveRise, centreY+playRadius/ui.Half)
	triangle.Close()
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(ui.ColorInk())
	vector.FillPath(screen, &triangle, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, options)
}
