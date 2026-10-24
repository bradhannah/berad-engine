package workflowview

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// unitChoices and clockChoices are a wait's units and clocks, as its
// inspector offers them.
func unitChoices() []flow.Choice {
	return []flow.Choice{
		{Value: flow.UnitSeconds, Label: "Seconds"}, {Value: flow.UnitMinutes, Label: "Minutes"},
		{Value: flow.UnitHours, Label: "Hours"},
	}
}

func clockChoices() []flow.Choice {
	return []flow.Choice{{Value: flow.ClockGame, Label: "Game time"}, {Value: flow.ClockReal, Label: "Real time"}}
}

// durationButtons makes a wait's unit and clock buttons.
func durationButtons(name string) ([]*ui.Button, []*ui.Button) {
	units := make([]*ui.Button, 0, len(unitChoices()))
	for _, choice := range unitChoices() {
		units = append(units, ui.NewButton(name+"-"+choice.Value, choice.Label))
	}
	clocks := make([]*ui.Button, 0, len(clockChoices()))
	for _, choice := range clockChoices() {
		clocks = append(clocks, ui.NewButton(name+"-"+choice.Value, choice.Label))
	}

	return units, clocks
}

// setSavedChoices shows a choice, a wait or a random choice's saved value:
// the chosen buttons, the wait's amount and the options' names and shares.
func setSavedChoices(made *control, node flow.Node) {
	switch made.param.Type {
	case flow.TypeChoice:
		chosen, err := node.Choice(made.param.Name)
		if err == nil {
			markChosen(made.choices, made.param.Choices, chosen)
		}
	case flow.TypeDuration:
		wait, err := node.Duration(made.param.Name)
		if err != nil {
			return
		}
		made.field.Field.SetSaved(strconv.Itoa(wait.Amount))
		markChosen(made.units, unitChoices(), wait.Unit)
		markChosen(made.clocks, clockChoices(), wait.Clock)
	case flow.TypeShares:
		options, err := node.Shares(made.param.Name)
		if err != nil || len(options) != len(made.options) {
			return
		}
		for index, option := range options {
			made.options[index].port = option.Port
			made.options[index].label.Field.SetSaved(option.Label)
			made.options[index].share.Field.SetSaved(strconv.Itoa(option.Share))
			made.options[index].remove.Disabled = len(options) == 1
		}
		made.addOption.Disabled = len(options) >= flow.MaxOptions
	case flow.TypeAnchor, flow.TypeTiles, flow.TypeTimeRange, flow.TypeTimeRanges, flow.TypeText:
	}
}

func markChosen(buttons []*ui.Button, choices []flow.Choice, chosen string) {
	for index, button := range buttons {
		button.On = index < len(choices) && choices[index].Value == chosen
	}
}

// layoutChoices places a choice's buttons in a row; a wait's amount, then
// its units and its clocks; or a random choice's options, one to a row,
// with Add option under them. It returns where the next control goes.
func layoutChoices(made *control, left, width, top int) int {
	row := func(buttons []*ui.Button) {
		gaps := (len(buttons) - 1) * rowSpace
		for index, button := range buttons {
			from := left + index*(width-gaps)/len(buttons) + index*rowSpace
			to := left + (index+1)*(width-gaps)/len(buttons) + index*rowSpace
			button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
		}
		top += ui.ButtonH + rowSpace
	}
	switch made.param.Type {
	case flow.TypeChoice:
		row(made.choices)
	case flow.TypeDuration:
		made.field.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + rowSpace
		row(made.units)
		row(made.clocks)
	case flow.TypeShares:
		top += ui.LineHeight
		labelWidth := width - shareWidth - removeWidth - ui.Half*rowSpace
		for _, option := range made.options {
			option.label.Rect = image.Rect(left, top, left+labelWidth, top+ui.ButtonH)
			shareLeft := left + labelWidth + rowSpace
			option.share.Rect = image.Rect(shareLeft, top, shareLeft+shareWidth, top+ui.ButtonH)
			option.remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
			top += ui.ButtonH + rowSpace
		}
		made.addOption.Rect = image.Rect(left, top, left+width/ui.Half, top+ui.ButtonH)
		top += ui.ButtonH + rowSpace
	case flow.TypeAnchor, flow.TypeTiles, flow.TypeTimeRange, flow.TypeTimeRanges, flow.TypeText:
	}

	return top + fieldSpace - rowSpace
}

// updateChoices saves a choice, or a wait's unit or clock, when its button
// is clicked.
func (s *State) updateChoices(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	for index, button := range made.choices {
		if button.Update(input) && !button.On {
			s.saveParam(ctx, node, made.param.Name, made.param.Choices[index].Value)
		}
	}
	if made.param.Type != flow.TypeDuration {
		return
	}
	wait, err := node.Duration(made.param.Name)
	if err != nil {
		return
	}
	changed := wait
	for index, button := range made.units {
		if button.Update(input) {
			changed.Unit = unitChoices()[index].Value
		}
	}
	for index, button := range made.clocks {
		if button.Update(input) {
			changed.Clock = clockChoices()[index].Value
		}
	}
	if changed == wait {
		return
	}
	if changed.Seconds() > flow.MaxWait {
		ctx.Frame.Status("A wait lasts at most a week")

		return
	}
	s.saveParam(ctx, node, made.param.Name, changed)
}

// updateOptions adds an option to a random choice, or removes one with the
// output it led to.
func (s *State) updateOptions(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	if made.addOption.Update(input) {
		s.addOption(ctx, node)

		return
	}
	options, err := node.Shares(made.param.Name)
	if err != nil {
		return
	}
	for index, option := range made.options {
		if !option.remove.Update(input) || index >= len(options) || len(options) == 1 {
			continue
		}
		changed := node.WithLink(options[index].Port, ids.ULID{})
		options = slices.Delete(options, index, index+1)
		s.saveParam(ctx, changed, made.param.Name, options)

		return
	}
}

// addOption adds an option to a random choice, with the same share as its
// last option so equal shares stay equal.
func (s *State) addOption(ctx *frame.Context, node flow.Node) {
	options, err := node.Shares("options")
	if err != nil {
		return
	}
	if len(options) >= flow.MaxOptions {
		ctx.Frame.Status(fmt.Sprintf("A random choice has at most %d options", flow.MaxOptions))

		return
	}
	share := 1
	if len(options) > 0 {
		share = max(options[len(options)-1].Share, 1)
	}
	options = append(options, flow.Share{Port: newOptionPort(options), Label: newOptionLabel(options), Share: share})
	s.saveParam(ctx, node, "options", options)
}

// newOptionPort returns an output name no option uses yet.
func newOptionPort(options []flow.Share) string {
	for number := 1; ; number++ {
		name := "o" + strconv.Itoa(number)
		if !slices.ContainsFunc(options, func(option flow.Share) bool { return option.Port == name }) {
			return name
		}
	}
}

// newOptionLabel returns "option N", unused among the options' names.
func newOptionLabel(options []flow.Share) string {
	for number := len(options) + 1; ; number++ {
		name := "option " + strconv.Itoa(number)
		if !slices.ContainsFunc(options, func(option flow.Share) bool { return option.Label == name }) {
			return name
		}
	}
}

// parseAmount reads a wait's amount in its unit.
func parseAmount(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	if field != made.field {
		return nil, false, ""
	}
	wait, err := node.Duration(made.param.Name)
	if err != nil {
		return nil, true, err.Error()
	}
	changed := wait
	changed.Amount = 0
	limit := flow.MaxWait / max(flow.Duration{Amount: 1, Unit: wait.Unit, Clock: wait.Clock}.Seconds(), 1)
	amount, ok, problem := parseWhole(value, 0, limit)
	if problem != "" {
		return amount, ok, problem
	}
	changed.Amount, _ = amount.(int) //nolint:errcheck // parseWhole returns an int when it has no problem.

	return changed, true, ""
}

// parseOptionField reads an option's name or share.
func parseOptionField(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	options, err := node.Shares(made.param.Name)
	if err != nil {
		return nil, false, ""
	}
	for index, row := range made.options {
		if (field != row.label && field != row.share) || index >= len(options) {
			continue
		}
		if field == row.label {
			name := strings.TrimSpace(value)
			if name == "" {
				return nil, true, "Give the option a name"
			}
			options[index].Label = name

			return options, true, ""
		}
		share, _, problem := parseWhole(value, 0, flow.MaxShare)
		if problem != "" {
			return nil, true, problem
		}
		options[index].Share, _ = share.(int) //nolint:errcheck // parseWhole returns an int when it has no problem.
		if total(options) == 0 {
			return nil, true, "At least one option needs a share"
		}

		return options, true, ""
	}

	return nil, false, ""
}

func total(options []flow.Share) int {
	sum := 0
	for _, option := range options {
		sum += option.Share
	}

	return sum
}
