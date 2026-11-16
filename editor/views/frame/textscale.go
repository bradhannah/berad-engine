package frame

import (
	"image"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// The scale for text and controls (increment 56): the status bar's Text
// button opens its steps, Cmd+Shift+= and Cmd+Shift+- step it and
// Cmd+Shift+0 puts it back at 100%. The editor window applies it to every
// pane and keeps it for each size of screen.

const (
	// textScaleWidth is the width of the Text button in the status bar.
	textScaleWidth = 100
	// textScaleID names the Text button, and textItemPrefix each step in
	// its menu, for input scripts.
	textScaleID    = "text-scale"
	textItemPrefix = "text-"
	percent        = 100
)

// TextScales are the steps of the scale for text and controls.
func TextScales() []float64 { return []float64{0.75, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2} }

// TextPercent is a scale as a whole percentage, as the status bar names it.
func TextPercent(scale float64) int { return int(math.Round(scale * percent)) }

// textScaleState is the status bar's Text button and its menu: the scale
// the window draws at, and a step asked for, zero until one is.
type textScaleState struct {
	shown  float64
	asked  float64
	button *ui.Button
	menu   ui.Menu
}

func newTextScaleState() textScaleState {
	button := ui.NewButton(textScaleID, "Text 100%")
	button.Caret = true

	return textScaleState{shown: 1, asked: 0, button: button, menu: ui.Menu{Items: nil, Width: 0}}
}

// SetTextScale is the scale the window draws text and controls at, as the
// editor window gives it.
func (f *Frame) SetTextScale(scale float64) {
	f.text.shown = scale
	f.text.button.Label = "Text " + strconv.Itoa(TextPercent(scale)) + "%"
}

// TextScale is the scale the window draws text and controls at.
func (f *Frame) TextScale() float64 { return f.text.shown }

// AskTextScale asks the editor window for a scale for text and controls.
func (f *Frame) AskTextScale(scale float64) { f.text.asked = scale }

// StepTextScale asks for the next step larger, or with a negative count
// smaller, staying at the first or last; from a step already asked for,
// when the window has not yet taken it.
func (f *Frame) StepTextScale(steps int) {
	from := f.text.shown
	if f.text.asked > 0 {
		from = f.text.asked
	}
	scales := TextScales()
	step := slices.IndexFunc(scales, func(scale float64) bool { return scale >= from })
	if step < 0 {
		step = len(scales) - 1
	}
	if steps > 0 && scales[step] > from {
		steps--
	}
	f.AskTextScale(scales[min(max(step+steps, 0), len(scales)-1)])
}

// TakeTextScale returns the scale asked for since it was last taken.
func (f *Frame) TakeTextScale() (float64, bool) {
	asked := f.text.asked
	f.text.asked = 0

	return asked, asked > 0
}

// layoutTextScale places the Text button ending at right, and returns
// where the next part of the status bar ends.
func (f *Frame) layoutTextScale(size image.Point, right int) int {
	top := size.Y - StatusHeight + (StatusHeight-ui.ButtonH)/ui.Half
	f.text.button.Rect = image.Rect(right-textScaleWidth, top, right, top+ui.ButtonH)

	return f.text.button.Rect.Min.X - statusGap/ui.Half
}

// updateTextScale runs the Text button, and its menu while it is open,
// which takes every press. It reports whether the menu took the input.
func (f *Frame) updateTextScale(input *ui.Input, size image.Point) bool {
	if f.text.menu.IsOpen() {
		if chosen := f.text.menu.Update(input); chosen != "" {
			if scale, err := strconv.Atoi(strings.TrimPrefix(chosen, textItemPrefix)); err == nil {
				f.AskTextScale(float64(scale) / percent)
			}
		}

		return true
	}
	if !f.text.button.Update(input) {
		return false
	}
	items := make([]ui.MenuItem, 0, len(TextScales()))
	for _, scale := range TextScales() {
		label := strconv.Itoa(TextPercent(scale)) + "%"
		if TextPercent(scale) == TextPercent(f.text.shown) {
			label += " · now"
		}
		items = append(items, ui.MenuItem{
			ID: textItemPrefix + strconv.Itoa(TextPercent(scale)), Label: label, Icon: ui.IconNone,
			Disabled: false, Danger: false, Rule: false,
		})
	}
	f.text.menu = ui.Menu{Items: items, Width: textScaleWidth}
	f.text.menu.Open(f.text.button.Rect, image.Rectangle{Min: image.Pt(0, 0), Max: size})

	return true
}

// DrawTextMenu draws the Text button's menu while it is open, over the
// view.
func (f *Frame) DrawTextMenu(screen *ebiten.Image, theme *ui.Theme) { f.text.menu.Draw(screen, theme) }

// locateTextScale finds the Text button, "ui:text-scale", a step in its
// open menu, "ui:text-125", and answers "state:text-scale=125" while text
// and controls are drawn at that percentage, for input scripts.
func (f *Frame) locateTextScale(name string) (image.Point, bool, bool) {
	switch {
	case name == "ui:"+textScaleID:
		return ui.Center(f.text.button.Rect), !f.text.button.Rect.Empty(), true
	case strings.HasPrefix(name, "ui:"+textItemPrefix):
		point, found := f.text.menu.Locate(strings.TrimPrefix(name, "ui:"))

		return point, found, true
	case strings.HasPrefix(name, "state:"+textScaleID+"="):
		wanted := strings.TrimPrefix(name, "state:"+textScaleID+"=")

		return image.Pt(0, 0), wanted == strconv.Itoa(TextPercent(f.text.shown)), true
	}

	return image.Pt(0, 0), false, false
}
