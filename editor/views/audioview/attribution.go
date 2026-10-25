package audioview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The panel's attribution of the recording in focus: who made it, where it
// came from and its licence, what was done to it before it came in, and
// whether it is cleared for a commercial game, which is ticked here.

// Attribution layout: its lines of facts, each two to a line but the last,
// and the width of a fact's label, in screen pixels.
const (
	attributionLines = 3
	factLabelWidth   = 92
	clearedLabel     = "Allowed in a commercial game"
)

type attributionSection struct {
	cleared *ui.Toggle
	edit    *ui.Button
	top     int
	// pending is the clearing asked for and not yet in the library, shown
	// meanwhile; pendingFor is its recording.
	pending    *bool
	pendingFor string
}

func newAttributionSection() *attributionSection {
	return &attributionSection{
		cleared: &ui.Toggle{ID: "cleared", Label: clearedLabel, Rect: ui.Zero(), On: false},
		edit:    ui.NewButton("edit-attribution", "Edit attribution…"), top: 0, pending: nil, pendingFor: "",
	}
}

// attributed returns the recording whose attribution the panel shows: the
// one in focus, unless it is an empty slot.
func (s *State) attributed() (protocol.AudioRecording, bool) {
	recording, ok := s.focused()

	return recording, ok && recording.Status != protocol.RecordingNoFile
}

// layoutAttribution places the section from a top edge, and returns where
// it ends.
func (s *State) layoutAttribution(left, right, top int) int {
	section := s.attribution
	section.cleared.Rect, section.edit.Rect = ui.Zero(), ui.Zero()
	recording, ok := s.attributed()
	if !ok {
		return top
	}
	section.top = top
	width := s.theme.TextWidth(section.edit.Label) + ui.Half*ui.Inset
	section.edit.Rect = image.Rect(right-width, top, right, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap + attributionLines*ui.LineHeight + ui.Gap
	width = ui.Inset + ui.Gap*ui.Half + s.theme.TextWidth(clearedLabel)
	section.cleared.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	section.cleared.On = recording.Attribution.Commercial
	if section.pending != nil && section.pendingFor == recording.ID {
		if *section.pending == recording.Attribution.Commercial {
			section.pending = nil
		} else {
			section.cleared.On = *section.pending
		}
	}

	return top + ui.ButtonH + ui.Inset
}

// updateAttribution clears the recording for a commercial game, or not, as
// its checkbox is ticked, and edits the rest in the attribution dialog.
func (s *State) updateAttribution(input *ui.Input, ctx *frame.Context) {
	section := s.attribution
	recording, ok := s.attributed()
	if !ok {
		return
	}
	switch {
	case section.cleared.Update(input):
		cleared := section.cleared.On
		section.pending, section.pendingFor = &cleared, recording.ID
		attribution := recording.Attribution
		attribution.Commercial = cleared
		said := "Cleared for a commercial game. Undo takes it back."
		if !cleared {
			said = "No longer cleared for a commercial game. Undo takes it back."
		}
		s.editIn(ctx, protocol.AudioSetAttribution,
			protocol.AttributionChange{Recording: recording.ID, Attribution: attribution},
			func(protocol.Result) { ctx.Frame.Status(said) },
			func() { section.pending = nil })
	case section.edit.Update(input):
		s.openAttribution(recording)
	}
}

// drawAttribution draws the section's facts, two to a line, and its
// checkbox, with a reminder until it is ticked.
func (s *State) drawAttribution(screen *ebiten.Image) {
	section := s.attribution
	recording, ok := s.attributed()
	if !ok {
		return
	}
	attribution := recording.Attribution
	left, right := s.panelArea().Min.X+ui.Inset, section.edit.Rect.Max.X
	s.theme.DrawLabel(screen, "ATTRIBUTION · WHO MADE IT, WHERE IT CAME FROM AND ITS LICENCE", left,
		section.top+(ui.ButtonH-ui.LabelSize)/ui.Half, ui.ColorMuted())
	middle := left + (right-left)/ui.Half
	top := section.top + ui.ButtonH + ui.Gap
	s.drawFact(screen, "CREATOR", attribution.Creator, left, middle, top)
	s.drawFact(screen, "LICENCE", attribution.License, middle, right, top)
	top += ui.LineHeight
	s.drawFact(screen, "SOURCE", attribution.Source, left, middle, top)
	s.drawFact(screen, "SOURCE TYPE", attribution.SourceType, middle, right, top)
	top += ui.LineHeight
	s.drawFact(screen, "COPIED FROM", orNone(recording.OriginalFile), left, middle, top)
	s.drawFact(screen, "PROCESSING", processingText(attribution.Processing), middle, right, top)
	s.drawButtons(screen, section.edit)
	section.cleared.Draw(screen, s.theme)
	if !section.cleared.On {
		s.theme.DrawText(screen, s.theme.FitText("Some free licences forbid it, such as CC BY-NC.",
			right-section.cleared.Rect.Max.X-ui.Inset), section.cleared.Rect.Max.X+ui.Inset,
			section.cleared.Rect.Min.Y+ui.Gap, ui.ColorMuted())
	}
}

// drawFact draws a fact's label and value between two edges, the value
// subdued while unknown.
func (s *State) drawFact(screen *ebiten.Image, label, value string, left, right, top int) {
	s.theme.DrawLabel(screen, label, left, top+ui.Gap, ui.ColorMuted())
	ink := ui.ColorInk()
	if unknown(value) || value == noneRecorded {
		ink = ui.ColorSubdued()
	}
	s.theme.DrawText(screen, s.theme.FitText(value, right-left-factLabelWidth-ui.Inset), left+factLabelWidth, top, ink)
}

// noneRecorded stands for no processing steps.
const noneRecorded = "none recorded"

// processingText lists what was done to a recording: each step, its tool
// and who did it.
func processingText(steps []protocol.ProcessingStep) string {
	if len(steps) == 0 {
		return noneRecorded
	}
	described := make([]string, 0, len(steps))
	for _, step := range steps {
		described = append(described, step.Description+" · "+step.Tool+" · "+step.PerformedBy)
	}

	return strings.Join(described, "; ")
}
