package audioview

import (
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	beradaudio "github.com/bradhannah/berad-sdk/modules/berad-audio"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// How far away an ambient cue is heard: berad-audio's distance curve, which
// the game uses, drawn for its distance, and its fades. The inspector shows
// them for a cue in the ambient category.

// Ambient layout, in screen pixels, and the values the fields accept.
const (
	numberWidth  = 56
	numberBytes  = 12
	curveHeight  = 40
	curvePad     = 3
	maxRadius    = 1000.0
	maxFadeS     = 60.0
	msPerSecond  = 1000.0
	ambientLines = 2
)

// Why typed ambient settings are refused.
const (
	problemRadius = "Type how many tiles away it is heard, from 0 to 1000."
	problemFade   = "Type a fade in seconds, from 0 to 60."
)

// ambientSection is the inspector's part for an ambient cue: how many tiles
// away it is heard and its fades, or a button that gives it a distance.
type ambientSection struct {
	radius, fadeIn, fadeOut *ui.TextField
	setDistance             *ui.Button
	// top is where its label is, and curve where the curve is drawn; both
	// are empty when it does not show.
	top   int
	curve image.Rectangle
}

func newAmbientSection(theme *ui.Theme) *ambientSection {
	field := func(id string) *ui.TextField {
		return ui.NewTextField(id, textedit.NewField("", false, numberBytes), false, theme)
	}

	return &ambientSection{
		radius: field("radius"), fadeIn: field("fade-in"), fadeOut: field("fade-out"),
		setDistance: ui.NewButton("set-distance", "Hear it from a distance"), top: 0, curve: ui.Zero(),
	}
}

func (a *ambientSection) fields() []*ui.TextField {
	return []*ui.TextField{a.radius, a.fadeIn, a.fadeOut}
}

// showsAmbient reports whether a cue has an ambient section.
func showsAmbient(cue protocol.AudioCue) bool { return cue.Category == protocol.CategoryAmbient }

// listedAmbient shows a cue's saved settings in the fields.
func (a *ambientSection) listed(cue protocol.AudioCue) {
	if cue.Ambient == nil {
		return
	}
	a.radius.Field.SetSaved(number(cue.Ambient.RadiusTiles))
	a.fadeIn.Field.SetSaved(number(float64(cue.Ambient.FadeInMs) / msPerSecond))
	a.fadeOut.Field.SetSaved(number(float64(cue.Ambient.FadeOutMs) / msPerSecond))
}

// number writes a value without trailing zeros, such as 8 or 1.2.
func number(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

// layoutAmbient places the section from a top edge and returns its bottom.
func (s *State) layoutAmbient(cue protocol.AudioCue, left, right, top int) int {
	section := s.inspector.ambient
	section.top, section.curve = top, ui.Zero()
	top += ui.LineHeight
	switch {
	case len(cue.Recordings) > 1:
		return top + ui.LineHeight*ambientLines + ui.Inset
	case cue.Ambient == nil:
		section.setDistance.Rect = image.Rect(left, top+ui.LineHeight, right, top+ui.LineHeight+ui.ButtonH)

		return section.setDistance.Rect.Max.Y + ui.Inset
	}
	section.radius.Rect = image.Rect(left, top, left+numberWidth, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap
	fadeLeft := left + s.theme.TextWidth("Fade in") + ui.Gap*ui.Half
	section.fadeIn.Rect = image.Rect(fadeLeft, top, fadeLeft+numberWidth, top+ui.ButtonH)
	outLeft := section.fadeIn.Rect.Max.X + s.theme.TextWidth(" s   out") + ui.Gap*ui.Half
	section.fadeOut.Rect = image.Rect(outLeft, top, outLeft+numberWidth, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap*ui.Half
	section.curve = image.Rect(left, top, right, top+curveHeight)

	return section.curve.Max.Y + ui.LineHeight + ui.Inset
}

// commitAmbient sends the ambient settings after Enter in one of their
// fields, with the others as saved. It reports whether field was one.
func (s *State) commitAmbient(ctx *frame.Context, field *ui.TextField) bool {
	section := s.inspector.ambient
	cue, ok := s.selectedCue()
	if !ok || field != section.radius && field != section.fadeIn && field != section.fadeOut {
		return false
	}
	value, changed := field.Field.Commit()
	if !changed {
		return true
	}
	read := func(from *ui.TextField, limit float64) (float64, bool) {
		text := from.Field.Saved()
		if from == field {
			text = value
		}
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(text), "tiles"), "s"))
		parsed, err := strconv.ParseFloat(text, 64)

		return parsed, err == nil && parsed >= 0 && parsed <= limit && !math.IsNaN(parsed)
	}
	radius, radiusOK := read(section.radius, maxRadius)
	fadeIn, fadeInOK := read(section.fadeIn, maxFadeS)
	fadeOut, fadeOutOK := read(section.fadeOut, maxFadeS)
	if !radiusOK || !fadeInOK || !fadeOutOK {
		field.Field.Failed()
		ctx.Frame.Status(map[bool]string{true: problemRadius, false: problemFade}[field == section.radius])

		return true
	}
	s.editIn(ctx, protocol.AudioSetAmbient, protocol.AmbientChange{Cue: cue.ID, Ambient: protocol.AmbientSettings{
		RadiusTiles: radius, FadeInMs: int64(math.Round(fadeIn * msPerSecond)),
		FadeOutMs: int64(math.Round(fadeOut * msPerSecond)),
	}}, nil, field.Field.Failed)

	return true
}

// giveDistance gives an ambient cue the distance a new one starts with.
func (s *State) giveDistance(ctx *frame.Context, cue protocol.AudioCue) {
	s.edit(ctx, protocol.AudioSetAmbient, protocol.AmbientChange{Cue: cue.ID, Ambient: protocol.AmbientSettings{
		RadiusTiles: newRadiusTiles, FadeInMs: newFadeMs, FadeOutMs: newFadeMs,
	}}, nil)
}

// What a cue given a distance starts with, as a new ambient cue does.
const (
	newRadiusTiles = 8
	newFadeMs      = 1000
)

// drawAmbient draws the section: its label, its fields with their units,
// the curve, and what the curve means; or why it has no fields.
func (s *State) drawAmbient(screen *ebiten.Image, cue protocol.AudioCue, left int) {
	section := s.inspector.ambient
	s.theme.DrawLabel(screen, "HEARD FROM", left, section.top+ui.Gap, ui.ColorMuted())
	top := section.top + ui.LineHeight
	switch {
	case len(cue.Recordings) > 1:
		s.theme.DrawText(screen, "It plays recordings in turn, so it", left, top, ui.ColorMuted())
		s.theme.DrawText(screen, "has no distance; one recording can.", left, top+ui.LineHeight, ui.ColorMuted())

		return
	case cue.Ambient == nil:
		s.theme.DrawText(screen, "Everywhere, until it has a distance.", left, top, ui.ColorMuted())
		section.setDistance.Draw(screen, s.theme)

		return
	}
	textTop := func(field *ui.TextField) int { return field.Rect.Min.Y + (ui.ButtonH-ui.LineHeight)/ui.Half + ui.Gap }
	s.theme.DrawText(screen, "tiles away · 0 is everywhere", section.radius.Rect.Max.X+ui.Gap*ui.Half,
		textTop(section.radius), ui.ColorMuted())
	s.theme.DrawText(screen, "Fade in", left, textTop(section.fadeIn), ui.ColorMuted())
	s.theme.DrawText(screen, " s   out", section.fadeIn.Rect.Max.X, textTop(section.fadeIn), ui.ColorMuted())
	s.theme.DrawText(screen, " s", section.fadeOut.Rect.Max.X, textTop(section.fadeOut), ui.ColorMuted())
	s.drawFields(screen, section.fields()...)
	drawDistanceCurve(screen, section.curve, cue.Ambient.RadiusTiles)
	s.theme.DrawText(screen, distanceCaption(cue.Ambient.RadiusTiles), left, section.curve.Max.Y+ui.Gap,
		ui.ColorMuted())
}

// drawDistanceCurve draws how loud the cue is, from full at the top to
// silent at the bottom, from the sound's place on the left to its distance
// on the right, as berad-audio plays it.
func drawDistanceCurve(screen *ebiten.Image, area image.Rectangle, radius float64) {
	ui.FillRound(screen, area, ui.Radius, ui.ColorCanvas())
	inner := area.Inset(curvePad)
	heightOf := func(column int) int {
		distance := radius * float64(column-inner.Min.X) / float64(max(inner.Dx(), 1))
		gain := beradaudio.QuadraticDistanceGain(distance, radius)

		return inner.Max.Y - int(math.Round(gain*float64(inner.Dy())))
	}
	previous := image.Pt(inner.Min.X, heightOf(inner.Min.X))
	for column := inner.Min.X + 1; column <= inner.Max.X; column++ {
		point := image.Pt(column, heightOf(column))
		ui.Line(screen, previous, point, ui.Outline, ui.ColorInk())
		previous = point
	}
	ui.StrokeRound(screen, area, ui.Radius, ui.Border, ui.ColorRule())
}

// distanceCaption says what the curve shows.
func distanceCaption(radius float64) string {
	half := radius / math.Sqrt2
	switch {
	case radius == 0:
		return "Full volume everywhere, as a background."
	case half <= 1:
		return "Full volume within 1 tile; silent at " + number(radius) + "."
	}

	return "Half volume at " + strconv.FormatFloat(half, 'f', 1, 64) + " tiles; silent at " + number(radius) + "."
}
