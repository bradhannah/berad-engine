package audioview

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// Processing steps in the sound dialog: what was done to a recording before
// it came in, each with its tool and who did it.

// stepWhatShare is the part of a step's row its description takes, in
// tenths; the tool and who did it share the rest.
const (
	stepWhatShare = 4
	stepShares    = 10
)

// stepRow is one processing step in the dialog, with its button to take it
// out.
type stepRow struct {
	what, tool, by *ui.TextField
	remove         *ui.Button
}

// addStepRow adds a step to the dialog, filled with what is known of it.
func (dlg *soundDialog) addStepRow(step protocol.ProcessingStep) {
	index := strconv.Itoa(len(dlg.steps))
	field := func(id, placeholder, value string) *ui.TextField {
		made := ui.NewTextField(id+index, textedit.NewField("", false, textBytes), false, dlg.theme)
		made.Placeholder, made.Unmarked = placeholder, true
		if !unknown(value) {
			made.Field.Restore(value)
		}

		return made
	}
	dlg.steps = append(dlg.steps, &stepRow{
		what: field("step-what-", "Trimmed the silence", step.Description),
		tool: field("step-tool-", "Audacity", step.Tool), by: field("step-by-", "Who did it", step.PerformedBy),
		remove: ui.NewButton("step-remove-"+index, "×"),
	})
	dlg.registerFields()
}

// removeStep takes a step out, and numbers the rest again.
func (dlg *soundDialog) removeStep(removed int) {
	kept := dlg.stepValues()
	dlg.steps = nil
	for index, step := range kept {
		if index != removed {
			dlg.addStepRow(step)
		}
	}
	dlg.registerFields()
}

// registerFields gives the dialog's fields keyboard focus in order, keeping
// the focused one while it is still there.
func (dlg *soundDialog) registerFields() {
	current := dlg.focus.Current()
	dlg.focus = ui.Focus{}
	fields := []*ui.TextField{}
	if dlg.newCue() {
		fields = append(fields, dlg.name, dlg.id)
	}
	if dlg.attributes() {
		fields = append(fields, dlg.creator, dlg.source, dlg.sourceType, dlg.license)
		for _, step := range dlg.steps {
			fields = append(fields, step.what, step.tool, step.by)
		}
	}
	dlg.focus.Register(fields...)
	for _, field := range fields {
		if field == current {
			dlg.focus.Set(field)
		}
	}
}

// stepValues are the steps as typed, blank where nothing is.
func (dlg *soundDialog) stepValues() []protocol.ProcessingStep {
	steps := make([]protocol.ProcessingStep, 0, len(dlg.steps))
	for _, step := range dlg.steps {
		steps = append(steps, protocol.ProcessingStep{
			Description: step.what.Field.Value(), Tool: step.tool.Field.Value(), PerformedBy: step.by.Field.Value(),
		})
	}

	return steps
}

// placeSteps lays the steps out from a top edge, under one row of labels,
// with Add step under them, and returns where they end.
func (dlg *soundDialog) placeSteps(left, right, top int) int {
	row := top + ui.LineHeight
	if len(dlg.steps) > 0 {
		row += ui.LineHeight
	}
	fields := right - left - ui.ButtonH - ui.Gap
	whatWidth := fields * stepWhatShare / stepShares
	otherWidth := (fields - whatWidth - ui.Gap*ui.Half) / ui.Half
	for _, step := range dlg.steps {
		step.what.Rect = image.Rect(left, row, left+whatWidth, row+ui.ButtonH)
		toolLeft := step.what.Rect.Max.X + ui.Gap
		step.tool.Rect = image.Rect(toolLeft, row, toolLeft+otherWidth, row+ui.ButtonH)
		step.by.Rect = image.Rect(step.tool.Rect.Max.X+ui.Gap, row, right-ui.ButtonH-ui.Gap, row+ui.ButtonH)
		step.remove.Rect = image.Rect(right-ui.ButtonH, row, right, row+ui.ButtonH)
		row += ui.ButtonH + ui.Gap
	}
	width := dlg.theme.TextWidth(dlg.addStep.Label) + ui.Half*ui.Inset
	dlg.addStep.Rect = image.Rect(left, row, left+width, row+ui.ButtonH)

	return row + ui.ButtonH + ui.Gap
}

// updateSteps adds a step for Add step, and takes one out for its ×.
func (dlg *soundDialog) updateSteps(input *ui.Input) {
	if dlg.addStep.Update(input) {
		dlg.addStepRow(protocol.ProcessingStep{Description: "", Tool: "", PerformedBy: ""})
		dlg.focus.Set(dlg.steps[len(dlg.steps)-1].what)

		return
	}
	for index, step := range dlg.steps {
		if step.remove.Update(input) {
			dlg.removeStep(index)

			return
		}
	}
}

// drawSteps draws the steps under their heading and labels.
func (dlg *soundDialog) drawSteps(screen *ebiten.Image, column int) {
	heading := dlg.addStep.Rect.Min.Y - ui.LineHeight + ui.Gap
	if len(dlg.steps) > 0 {
		first := dlg.steps[0]
		heading = first.what.Rect.Min.Y - ui.LineHeight*ui.Half + ui.Gap
		for field, label := range map[*ui.TextField]string{first.what: "WHAT WAS DONE", first.tool: "TOOL", first.by: "BY"} {
			dlg.theme.DrawLabel(screen, label, field.Rect.Min.X, field.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
		}
	}
	dlg.theme.DrawLabel(screen, "PROCESSING · WHAT WAS DONE TO IT BEFORE IT CAME IN", column, heading, ui.ColorInk())
	for _, step := range dlg.steps {
		step.what.Draw(screen)
		step.tool.Draw(screen)
		step.by.Draw(screen)
		step.remove.Draw(screen, dlg.theme)
	}
	dlg.addStep.Draw(screen, dlg.theme)
}

// locateStep finds a step's field or button, or Add step, for input scripts.
func (dlg *soundDialog) locateStep(name string) (image.Point, bool) {
	if name == dlg.addStep.ID && !dlg.addStep.Rect.Empty() {
		return ui.Center(dlg.addStep.Rect), true
	}
	for _, step := range dlg.steps {
		for _, field := range []*ui.TextField{step.what, step.tool, step.by} {
			if name == field.ID {
				return field.Center(), true
			}
		}
		if name == step.remove.ID {
			return ui.Center(step.remove.Rect), true
		}
	}

	return image.Pt(0, 0), false
}
