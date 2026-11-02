package workflowview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The value picker is a dialog over the window that chooses a value for a
// node's parameter, or another value for an operand to compare with. It
// has the room a list inside the inspector lacks: a search box, focused on
// open, that matches any part of a path, and the values under their
// headings with their type. While it is open it takes all input.
const (
	chooseValueLabel = "Choose a value"

	pickerWidth  = 760
	pickerHeight = 540
	pickerRadius = 8
	pickerShadow = 6
	pickerShade  = 0x40
	pickerButton = 110
	pickerThird  = 3
)

// valuePicker is the open picker: what it chooses for and its widgets.
type valuePicker struct {
	node  ids.ULID
	param string
	role  valueRole
	// only is the one type offered, for an operand that must match its
	// value's type; "" offers every type. operand is set when the pick is
	// saved as a reference in an operand rather than as a value parameter.
	only    worldstate.Type
	operand bool
	title   string
	search  *ui.TextField
	list    *ui.List
	focus   ui.Focus
	choose  *ui.Button
	cancel  *ui.Button
	box     image.Rectangle
}

// openValuePicker opens the picker for a parameter of a node, with the
// current choice selected and the search focused.
func (s *State) openValuePicker(node flow.Node, param, title string, only worldstate.Type, operand bool) {
	search := ui.NewTextField("value-search", textedit.NewField("", false, maxValueSearch), false, s.theme)
	search.Placeholder, search.Unmarked = "Find a value, or type a new name", true
	if operand {
		search.Placeholder = "Find " + only.Label() + " values"
	}
	picker := &valuePicker{
		node: node.ID, param: param, role: roleOf(node.Kind, param), only: only, operand: operand, title: title,
		search: search, list: &ui.List{ID: "values", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil},
		focus: ui.Focus{}, choose: ui.NewButton("picker-choose", "Choose"), cancel: ui.NewButton(ui.DialogCancel, "Cancel"),
		box: ui.Zero(),
	}
	picker.choose.Primary = true
	picker.focus.Register(search)
	picker.focus.Set(search)
	if operand {
		if ref, refers := flow.OperandRef(savedOperand(node, param)); refers {
			picker.list.Selected = encodeRef(ref)
		}
	} else if ref, err := node.Ref(param); err == nil && !ref.IsZero() {
		picker.list.Selected = encodeRef(ref)
	}
	s.values = picker
}

// pickerRows lists what the picker offers for the search typed: an operand's
// values of its one type, or a parameter's values and the rows that would
// declare a new one.
func (s *State) pickerRows(docs frame.Docs) []ui.Row {
	picker := s.values
	query := picker.search.Field.Value()
	if picker.operand {
		return s.operandRows(docs, picker.only, query)
	}

	return s.valueRows(docs, picker.role, query)
}

// layoutValuePicker centres the picker in the window and places its parts:
// the title, the search, the list, and Cancel and Choose at the foot.
func (s *State) layoutValuePicker(docs frame.Docs) {
	picker := s.values
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	width, height := min(pickerWidth, window.Dx()-ui.Half*ui.Inset), min(pickerHeight, window.Dy()-ui.Half*ui.Inset)
	left, top := (window.Dx()-width)/ui.Half, max((window.Dy()-height)/pickerThird, ui.Inset)
	picker.box = image.Rect(left, top, left+width, top+height)
	y := top + ui.Inset + ui.LineHeight + ui.Gap
	picker.search.Rect = image.Rect(left+ui.Inset, y, left+width-ui.Inset, y+ui.ButtonH)
	y += ui.ButtonH + ui.Half*ui.Gap
	foot := top + height - ui.Inset - ui.ButtonH
	picker.list.Rect = image.Rect(left+ui.Inset, y, left+width-ui.Inset, foot-ui.Half*ui.Gap)
	right := left + width - ui.Inset
	picker.choose.Rect = image.Rect(right-pickerButton, foot, right, foot+ui.ButtonH)
	right -= pickerButton + ui.Gap
	picker.cancel.Rect = image.Rect(right-pickerButton, foot, right, foot+ui.ButtonH)
	picker.list.Rows = s.pickerRows(docs)
}

// updateValuePicker runs the open picker for a frame, taking all input:
// typing searches, arrows move, a click on a row or Enter picks, Escape, a
// click outside or Cancel closes.
func (s *State) updateValuePicker(input *ui.Input, ctx *frame.Context) {
	picker := s.values
	s.layoutValuePicker(ctx.Docs)
	defer input.ClearAll()
	// The keys are read before the search field sees them, since it takes
	// the ones it uses.
	enter, escape := input.Has(ui.KeyEnter, ui.Plain()), input.Has(ui.KeyEscape, ui.Plain())
	upward, downward := input.Has(ui.KeyUp, ui.Plain()), input.Has(ui.KeyDown, ui.Plain())
	if picker.cancel.Update(input) || escape || (input.Pressed && !input.Cursor.In(picker.box)) {
		s.values = nil

		return
	}
	picker.focus.Update(input, ctx.Clipboard)
	if picker.focus.Current() == nil {
		picker.focus.Set(picker.search)
	}
	picker.list.Rows = s.pickerRows(ctx.Docs)
	switch {
	case downward:
		picker.move(1)
	case upward:
		picker.move(-1)
	}
	row, _ := rowAt(picker.list, input.Cursor)
	pressed := input.Pressed
	result := picker.list.Update(input)
	switch {
	case pressed && row != "":
		s.pickRow(ctx, row)
	case picker.choose.Update(input) || result.Activated || enter:
		if chosen := picker.chosen(); chosen != "" {
			s.pickRow(ctx, chosen)
		}
	}
}

// move selects the next or previous row that can be picked.
func (p *valuePicker) move(step int) {
	var pickable []string
	for _, row := range p.list.Rows {
		if !row.Heading {
			pickable = append(pickable, row.ID)
		}
	}
	if len(pickable) == 0 {
		return
	}
	current := -1
	for index, id := range pickable {
		if id == p.list.Selected {
			current = index
		}
	}
	p.list.Selected = pickable[min(max(current+step, 0), len(pickable)-1)]
}

// chosen is the row Enter or Choose picks: the selected one, else the top
// value that matches, else the top row that would declare one.
func (p *valuePicker) chosen() string {
	first, declare := "", ""
	for _, row := range p.list.Rows {
		switch {
		case row.Heading:
		case row.ID == p.list.Selected:
			return row.ID
		case strings.HasPrefix(row.ID, declarePrefix):
			if declare == "" {
				declare = row.ID
			}
		case first == "":
			first = row.ID
		}
	}
	if first != "" {
		return first
	}

	return declare
}

// pickRow closes the picker and chooses, or declares and chooses, the row's
// value: as a reference for an operand, or as the parameter's value.
func (s *State) pickRow(ctx *frame.Context, rowID string) {
	picker := s.values
	s.values = nil
	node, found := s.currentNode(ctx.Docs, picker.node)
	if !found {
		return
	}
	if target, declaring := decodeTarget(rowID); declaring {
		s.declareValue(ctx, node.ID, picker.param, target)

		return
	}
	ref, refers := decodeRef(rowID)
	if !refers {
		return
	}
	if picker.operand {
		s.saveParam(ctx, node, picker.param, flow.RefOperand(ref))

		return
	}
	s.chooseValue(ctx, ctx.Docs, node, picker.param, ref)
}

// drawValuePicker draws the picker over the shaded window: the title and
// what it chooses for, the search, the list or why it is empty, the
// keyboard's ways and the buttons.
func (s *State) drawValuePicker(screen *ebiten.Image, docs frame.Docs) {
	picker := s.values
	s.layoutValuePicker(docs)
	ui.FillRect(screen, screen.Bounds(), ui.ColorShade())
	ui.FillRound(screen, picker.box.Add(image.Pt(0, pickerShadow)), pickerRadius, ui.HexAlpha(0, pickerShade))
	ui.FillRound(screen, picker.box, pickerRadius, ui.ColorBackground())
	ui.StrokeRound(screen, picker.box, pickerRadius, ui.Border, ui.ColorInk())
	left, top := picker.box.Min.X+ui.Inset, picker.box.Min.Y+ui.Inset
	s.theme.DrawStrong(screen, chooseValueLabel, left, top, ui.ColorInk())
	s.theme.DrawText(screen, picker.title, left+s.theme.StrongWidth(chooseValueLabel)+ui.Half*ui.Gap, top,
		ui.ColorMuted())
	picker.search.Draw(screen)
	ui.StrokeRound(screen, picker.list.Rect.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	switch {
	case len(picker.list.Rows) > 0:
		picker.list.Draw(screen, s.theme)
	case picker.operand:
		s.theme.DrawText(screen, "No other "+picker.only.Label()+" to compare with.", picker.list.Rect.Min.X+textPad,
			picker.list.Rect.Min.Y+ui.Gap, ui.ColorMuted())
	default:
		s.theme.DrawText(screen, "No value matches. Type a name to declare one.", picker.list.Rect.Min.X+textPad,
			picker.list.Rect.Min.Y+ui.Gap, ui.ColorMuted())
	}
	s.theme.DrawText(screen, "↑ ↓ move · Enter picks · Esc closes", left,
		picker.cancel.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
	picker.cancel.Draw(screen, s.theme)
	picker.choose.Draw(screen, s.theme)
}

// locateInPicker finds a part of the open picker for input scripts: a row,
// "valuerow:<path>" or "declare:<path>", its search, "ui:value-search", or
// a button, "ui:picker-choose" and "ui:cancel".
func (s *State) locateInPicker(docs frame.Docs, kind, value string) (image.Point, bool) {
	picker := s.values
	s.layoutValuePicker(docs)
	switch kind {
	case locateValue, locateDeclare:
		return locateRowIn(picker.list, kind, value)
	case "ui":
		switch value {
		case picker.search.ID:
			return picker.search.Center(), true
		case picker.choose.ID:
			return ui.Center(picker.choose.Rect), true
		case picker.cancel.ID:
			return ui.Center(picker.cancel.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// locateRowIn finds a list's value row by its path, or its declaring row by
// what it would declare.
func locateRowIn(list *ui.List, kind, value string) (image.Point, bool) {
	for index, row := range list.Rows {
		rect := list.RowRect(index)
		if row.Heading || !rect.In(list.Rect) {
			continue
		}
		target, declaring := decodeTarget(row.ID)
		if (kind == locateValue && row.Label == value) || (kind == locateDeclare && declaring && target.located() == value) {
			return image.Pt(rect.Min.X+textPad, ui.Center(rect).Y), true
		}
	}

	return image.Pt(0, 0), false
}

// pickerTitle says what a picker chooses for: a parameter by its label, or
// an operand by the type it must match.
func pickerTitle(param flow.Param, only worldstate.Type) string {
	if only != "" {
		return "for " + param.Label + " · " + strings.ToLower(only.Label()) + "s only, to match its value"
	}

	return "for " + param.Label
}
