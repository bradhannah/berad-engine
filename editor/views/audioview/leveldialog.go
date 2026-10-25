package audioview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// What a level dialog sets: a recording's gain by hand, or the loudness its
// measurement aims at.
type levelKind int

const (
	levelGain levelKind = iota
	levelTarget
)

// The levels berad-audio accepts, in dB.
const (
	minGainDB   = -48.0
	maxGainDB   = 12.0
	minTargetDB = -60.0
	maxTargetDB = -6.0
	levelWidth  = 380
)

// levelDialog asks for one level, in dB.
type levelDialog struct {
	kind                levelKind
	recording           string
	field               *ui.TextField
	set, family, cancel *ui.Button
	focus               ui.Focus
	problem             string
	rect                image.Rectangle
}

func newLevelDialog(theme *ui.Theme, kind levelKind, recording string, value float64) *levelDialog {
	field := ui.NewTextField("level", textedit.NewField("", false, fieldBytes), false, theme)
	field.Unmarked = true
	dialog := &levelDialog{
		kind: kind, recording: recording, field: field, set: ui.DialogButton("set-level", "Set"),
		family: ui.DialogButton("family-level", "Family's"), cancel: ui.DialogButton(ui.DialogCancel, "Cancel"),
		focus: ui.Focus{}, problem: "", rect: ui.Zero(),
	}
	dialog.set.Primary = true
	dialog.field.Field.Restore(strconv.FormatFloat(value, 'f', 1, 64))
	dialog.focus.Register(dialog.field)
	dialog.focus.Set(dialog.field)

	return dialog
}

// limits are the levels the dialog accepts.
func (d *levelDialog) limits() (float64, float64) {
	if d.kind == levelTarget {
		return minTargetDB, maxTargetDB
	}

	return minGainDB, maxGainDB
}

// text is the dialog's title and what it explains.
func (d *levelDialog) text() (string, []string) {
	low, high := d.limits()
	span := "From " + decibels(low) + " to " + decibels(high) + "."
	if d.kind == levelTarget {
		return "Target loudness of “" + d.recording + "”", []string{
			"How loud measuring makes it, in dB.", span, "Family's uses the target of its acoustic family.",
		}
	}

	return "Gain of “" + d.recording + "”", []string{
		"Set by hand, in dB, it stays when the region or file changes.", span,
	}
}

func (d *levelDialog) layout(window image.Rectangle) {
	_, lines := d.text()
	width := min(levelWidth, window.Dx()-ui.Half*ui.Inset)
	height := ui.Inset*ui.Half + (len(lines)+ui.Half)*ui.LineHeight + ui.ButtonH*ui.Half + ui.Inset
	left := window.Min.X + (window.Dx()-width)/ui.Half
	top := window.Min.Y + max((window.Dy()-height)/ui.Half, ui.Inset)
	d.rect = image.Rect(left, top, left+width, top+height)
	fieldTop := top + ui.Inset + (len(lines)+1)*ui.LineHeight
	d.field.Rect = image.Rect(left+ui.Inset, fieldTop, left+width-ui.Inset, fieldTop+ui.ButtonH)
	right, bottom := d.rect.Max.X-ui.Inset, d.rect.Max.Y-ui.Inset
	buttons := []*ui.Button{d.set, d.cancel}
	if d.kind == levelTarget {
		buttons = []*ui.Button{d.set, d.family, d.cancel}
	}
	d.family.Rect = ui.Zero()
	for index := len(buttons) - 1; index >= 0; index-- {
		buttons[index].Rect = image.Rect(right-actionWidth, bottom-ui.ButtonH, right, bottom)
		right -= actionWidth + ui.Gap*ui.Half
	}
}

// updateLevelDialog handles the level dialog, which takes all input.
func (s *State) updateLevelDialog(input *ui.Input, ctx *frame.Context) {
	dialog := s.trim.dialog
	dialog.layout(image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	defer input.ClearAll()
	field, outcome := dialog.focus.Update(input, ctx.Clipboard)
	switch {
	case dialog.cancel.Update(input) || input.Has(ui.KeyEscape, ui.Plain()) && field == nil:
		s.trim.dialog = nil
	case dialog.family.Update(input):
		s.setLevel(ctx, protocol.CalibrationChange{
			Recording: dialog.recording, GainDB: nil, TargetDB: nil, DefaultTarget: true,
		})
	case dialog.set.Update(input) || outcome == textedit.OutcomeCommit:
		s.submitLevel(ctx)
	}
}

// submitLevel sets the typed level when it is one berad-audio accepts.
func (s *State) submitLevel(ctx *frame.Context) {
	dialog := s.trim.dialog
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(dialog.field.Field.Value()), "dB"))
	text = strings.ReplaceAll(text, "−", "-")
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	low, high := dialog.limits()
	if err != nil || value < low || value > high {
		dialog.problem = "Type a number of dB from " + decibels(low) + " to " + decibels(high) + "."

		return
	}
	change := protocol.CalibrationChange{Recording: dialog.recording, GainDB: &value, TargetDB: nil, DefaultTarget: false}
	if dialog.kind == levelTarget {
		change.GainDB, change.TargetDB = nil, &value
	}
	s.setLevel(ctx, change)
}

// setLevel sends a level and closes the dialog.
func (s *State) setLevel(ctx *frame.Context, change protocol.CalibrationChange) {
	s.trim.dialog = nil
	s.edit(ctx, protocol.AudioSetCalibration, change, nil)
}

// drawLevelDialog draws the level dialog over a shaded window.
func (s *State) drawLevelDialog(screen *ebiten.Image) {
	dialog := s.trim.dialog
	dialog.layout(screen.Bounds())
	ui.FillRect(screen, screen.Bounds(), ui.ColorShade())
	ui.FillRound(screen, dialog.rect, dialogRadius, ui.ColorBackground())
	ui.StrokeRound(screen, dialog.rect, dialogRadius, ui.Border, ui.ColorInk())
	left, top := dialog.rect.Min.X+ui.Inset, dialog.rect.Min.Y+ui.Inset
	title, lines := dialog.text()
	s.theme.DrawStrong(screen, title, left, top, ui.ColorInk())
	for index, line := range lines {
		s.theme.DrawText(screen, line, left, top+(index+1)*ui.LineHeight, ui.ColorMuted())
	}
	dialog.field.Draw(screen)
	if dialog.problem != "" {
		s.theme.DrawText(screen, dialog.problem, left, dialog.field.Rect.Max.Y+ui.Gap, ui.ColorWarn())
	}
	for _, button := range []*ui.Button{dialog.set, dialog.family, dialog.cancel} {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
}

// locate finds the dialog's field or a button, for input scripts.
func (d *levelDialog) locate(name string) (image.Point, bool) {
	if name == d.field.ID {
		return d.field.Center(), true
	}
	for _, button := range []*ui.Button{d.set, d.family, d.cancel} {
		if name == button.ID && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}
