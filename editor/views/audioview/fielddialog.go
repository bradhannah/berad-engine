package audioview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// fieldDialogWidth is a field dialog's width, in screen pixels.
const fieldDialogWidth = 380

// fieldDialog asks for one value, such as a level in dB or a new family's
// name: a title, lines that explain it, the field, then Set and Cancel.
// submit sends the typed value, or returns why it cannot be.
type fieldDialog struct {
	title       string
	lines       []string
	field       *ui.TextField
	set, cancel *ui.Button
	focus       ui.Focus
	problem     string
	rect        image.Rectangle
	submit      func(ctx *frame.Context, value string) string
}

// newFieldDialog returns a dialog whose field, fieldID, starts with value;
// setID names its Set button.
func newFieldDialog(
	theme *ui.Theme, fieldID, setID, title string, lines []string, value string,
	submit func(ctx *frame.Context, value string) string,
) *fieldDialog {
	field := ui.NewTextField(fieldID, textedit.NewField("", false, nameBytes), false, theme)
	field.Unmarked = true
	dialog := &fieldDialog{
		title: title, lines: lines, field: field, set: ui.DialogButton(setID, "Set"),
		cancel: ui.DialogButton(ui.DialogCancel, "Cancel"), focus: ui.Focus{}, problem: "", rect: ui.Zero(),
		submit: submit,
	}
	dialog.set.Primary = true
	dialog.field.Field.Restore(value)
	dialog.focus.Register(dialog.field)
	dialog.focus.Set(dialog.field)

	return dialog
}

func (d *fieldDialog) buttons() []*ui.Button { return []*ui.Button{d.set, d.cancel} }

func (d *fieldDialog) layout(window image.Rectangle) {
	width := min(fieldDialogWidth, window.Dx()-ui.Half*ui.Inset)
	height := ui.Inset*ui.Half + (len(d.lines)+ui.Half)*ui.LineHeight + ui.ButtonH*ui.Half + ui.Inset
	left := window.Min.X + (window.Dx()-width)/ui.Half
	top := window.Min.Y + max((window.Dy()-height)/ui.Half, ui.Inset)
	d.rect = image.Rect(left, top, left+width, top+height)
	fieldTop := top + ui.Inset + (len(d.lines)+1)*ui.LineHeight
	d.field.Rect = image.Rect(left+ui.Inset, fieldTop, left+width-ui.Inset, fieldTop+ui.ButtonH)
	right, bottom := d.rect.Max.X-ui.Inset, d.rect.Max.Y-ui.Inset
	buttons := d.buttons()
	for index := len(buttons) - 1; index >= 0; index-- {
		buttons[index].Rect = image.Rect(right-actionWidth, bottom-ui.ButtonH, right, bottom)
		right -= actionWidth + ui.Gap*ui.Half
	}
}

// updatePrompt handles the field dialog, which takes all input, and closes it
// once its value is sent or it is cancelled.
func (s *State) updatePrompt(input *ui.Input, ctx *frame.Context) {
	dialog := s.prompt
	dialog.layout(image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	defer input.ClearAll()
	field, outcome := dialog.focus.Update(input, ctx.Clipboard)
	switch {
	case dialog.cancel.Update(input) || input.Has(ui.KeyEscape, ui.Plain()) && field == nil:
		s.prompt = nil
	case dialog.set.Update(input) || outcome == textedit.OutcomeCommit:
		if dialog.problem = dialog.submit(ctx, dialog.field.Field.Value()); dialog.problem == "" {
			s.prompt = nil
		}
	}
}

// draw draws the dialog over a shaded window.
func (d *fieldDialog) draw(screen *ebiten.Image, theme *ui.Theme) {
	d.layout(screen.Bounds())
	ui.FillRect(screen, screen.Bounds(), ui.ColorShade())
	ui.FillRound(screen, d.rect, dialogRadius, ui.ColorBackground())
	ui.StrokeRound(screen, d.rect, dialogRadius, ui.Border, ui.ColorInk())
	left, top := d.rect.Min.X+ui.Inset, d.rect.Min.Y+ui.Inset
	theme.DrawStrong(screen, d.title, left, top, ui.ColorInk())
	for index, line := range d.lines {
		theme.DrawText(screen, line, left, top+(index+1)*ui.LineHeight, ui.ColorMuted())
	}
	d.field.Draw(screen)
	if d.problem != "" {
		theme.DrawText(screen, d.problem, left, d.field.Rect.Max.Y+ui.Gap, ui.ColorWarn())
	}
	for _, button := range d.buttons() {
		button.Draw(screen, theme)
	}
}

// locate finds the dialog's field or a button, for input scripts.
func (d *fieldDialog) locate(name string) (image.Point, bool) {
	if name == d.field.ID {
		return d.field.Center(), true
	}
	for _, button := range d.buttons() {
		if name == button.ID {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}
