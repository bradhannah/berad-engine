package workflowview

import (
	"image"
	"strconv"
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
// open, that matches any part of a path, type chips, and the values under
// their headings with their type. While it is open it takes all input.
const (
	chooseValueLabel = "Choose a value"

	pickerWidth  = 760
	pickerHeight = 540
	pickerRadius = 8
	pickerShadow = 6
	pickerShade  = 0x40
	pickerButton = 110
	pickerThird  = 3
	// pickerLocate names the picker's own parts for input scripts:
	// "picker:type:<type>" for a type chip, "picker:type:all" for All.
	pickerLocate = "picker"
	chipPrefix   = "picker-type-"
	allTypes     = "all"
	typeLabel    = "Type"
)

// valuePicker is the open picker: what it chooses for and its widgets.
type valuePicker struct {
	node  ids.ULID
	param string
	role  valueRole
	// only is the one type offered, locked: an operand's value's type, or
	// an NPC for a value to keep one in; "" offers every type, narrowed by
	// the chip chosen, kind. operand is set when the pick is saved as a
	// reference in an operand rather than as a value parameter.
	only    worldstate.Type
	kind    worldstate.Type
	operand bool
	title   string
	search  *ui.TextField
	list    *ui.List
	focus   ui.Focus
	chips   []*ui.Button
	kinds   []worldstate.Type
	// hidden is the values the search matches that the type hides; others
	// lists them, for a locked type, as rows that cannot be picked. note
	// says why the last one clicked was not picked.
	hidden []valueOption
	others bool
	show   *ui.Button
	note   string
	choose *ui.Button
	cancel *ui.Button
	box    image.Rectangle
}

// openValuePicker opens the picker for a parameter of a node, with the
// current choice selected and the search focused.
func (s *State) openValuePicker(node flow.Node, param, title string, only worldstate.Type, operand bool) {
	search := ui.NewTextField("value-search", textedit.NewField("", false, maxValueSearch), false, s.theme)
	search.Placeholder, search.Unmarked = "Find a value, or type a new name", true
	if operand {
		search.Placeholder = "Find " + only.Label() + " values"
	}
	chips, kinds := pickerChips(only)
	picker := &valuePicker{
		node: node.ID, param: param, role: roleOf(node.Kind, param), only: only, kind: "", operand: operand,
		title: title, search: search, list: &ui.List{ID: "values", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil},
		focus: ui.Focus{}, chips: chips, kinds: kinds, hidden: nil, others: false,
		show: ui.NewButton("picker-show", "Show them"), note: "", choose: ui.NewButton("picker-choose", "Choose"),
		cancel: ui.NewButton(ui.DialogCancel, "Cancel"), box: ui.Zero(),
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

// pickerChips makes the type chips: the one type offered, locked on, or
// All and each type a saved value can have.
func pickerChips(only worldstate.Type) ([]*ui.Button, []worldstate.Type) {
	if only != "" {
		chip := ui.NewButton(chipPrefix+string(only), only.Label()+" only")
		chip.On = true

		return []*ui.Button{chip}, []worldstate.Type{only}
	}
	kinds := append([]worldstate.Type{""}, worldstate.Types(worldstate.ScopeWorld)...)
	chips := make([]*ui.Button, 0, len(kinds))
	for _, kind := range kinds {
		id, label := string(kind), kind.Label()
		if kind == "" {
			id, label = allTypes, "All"
		}
		chips = append(chips, ui.NewButton(chipPrefix+id, label))
	}
	chips[0].On = true

	return chips, kinds
}

// offers reports whether the picker lists a value: of its one type, or of
// the chip chosen.
func (p *valuePicker) offers(option valueOption) bool {
	switch {
	case p.only != "":
		return option.declared.Type == p.only
	case p.kind != "":
		return option.declared.Type == p.kind
	default:
		return true
	}
}

// layoutValuePicker centres the picker in the window and places its parts:
// the title, the search, the type chips, the list, the line under it, and
// Cancel and Choose at the foot.
func (s *State) layoutValuePicker(docs frame.Docs) {
	picker := s.values
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	width, height := min(pickerWidth, window.Dx()-ui.Half*ui.Inset), min(pickerHeight, window.Dy()-ui.Half*ui.Inset)
	left, top := (window.Dx()-width)/ui.Half, max((window.Dy()-height)/pickerThird, ui.Inset)
	picker.box = image.Rect(left, top, left+width, top+height)
	right := left + width - ui.Inset
	line := top + ui.Inset + ui.LineHeight + ui.Gap
	picker.search.Rect = image.Rect(left+ui.Inset, line, right, line+ui.ButtonH)
	line += ui.ButtonH + ui.Half*ui.Gap
	chip := left + ui.Inset + s.theme.StrongWidth(typeLabel) + ui.Half*ui.Gap
	for _, button := range picker.chips {
		button.Rect = image.Rect(chip, line, chip+s.theme.TextWidth(button.Label)+ui.Half*ui.Inset, line+ui.ButtonH)
		chip = button.Rect.Max.X + ui.Gap
	}
	line += ui.ButtonH + ui.Half*ui.Gap
	foot := top + height - ui.Inset - ui.ButtonH
	below := foot - ui.Half*ui.Gap - ui.ButtonH
	picker.list.Rect = image.Rect(left+ui.Inset, line, right, below-ui.Half*ui.Gap)
	picker.choose.Rect = image.Rect(right-pickerButton, foot, right, foot+ui.ButtonH)
	picker.cancel.Rect = image.Rect(right-ui.Half*pickerButton-ui.Gap, foot, right-pickerButton-ui.Gap, foot+ui.ButtonH)
	picker.list.Rows = s.pickerRows(docs)
	picker.show.Rect, picker.show.Label = ui.Zero(), "Show them"
	if len(picker.hidden) > 0 && picker.note == "" {
		picker.show.Rect = image.Rect(right-pickerButton, below, right, below+ui.ButtonH)
	}
	if picker.others && picker.only != "" {
		picker.show.Label = "Hide them"
	}
}

// pickerKeys are the keys the picker reads before its search field takes
// the ones it uses.
type pickerKeys struct {
	enter, escape, up, down bool
}

func readPickerKeys(input *ui.Input) pickerKeys {
	return pickerKeys{
		enter: input.Has(ui.KeyEnter, ui.Plain()), escape: input.Has(ui.KeyEscape, ui.Plain()),
		up: input.Has(ui.KeyUp, ui.Plain()), down: input.Has(ui.KeyDown, ui.Plain()),
	}
}

// updateValuePicker runs the open picker for a frame, taking all input:
// typing searches, a chip narrows by type, arrows move, a click on a row or
// Enter picks, Escape, a click outside or Cancel closes.
func (s *State) updateValuePicker(input *ui.Input, ctx *frame.Context) {
	picker := s.values
	s.layoutValuePicker(ctx.Docs)
	defer input.ClearAll()
	keys := readPickerKeys(input)
	if picker.cancel.Update(input) || keys.escape || (input.Pressed && !input.Cursor.In(picker.box)) {
		s.values = nil

		return
	}
	if input.Pressed || input.Text != "" {
		picker.note = ""
	}
	picker.focus.Update(input, ctx.Clipboard)
	if picker.focus.Current() == nil {
		picker.focus.Set(picker.search)
	}
	if picker.updateControls(input) {
		s.layoutValuePicker(ctx.Docs)
	}
	picker.list.Rows = s.pickerRows(ctx.Docs)
	switch {
	case keys.down:
		picker.move(1)
	case keys.up:
		picker.move(-1)
	}
	s.updatePickerList(input, ctx, keys.enter)
}

// updatePickerList picks the row clicked, or the one Enter, Choose or a
// double-click picks.
func (s *State) updatePickerList(input *ui.Input, ctx *frame.Context, enter bool) {
	picker := s.values
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

// updateControls handles the type chips and Show them, and reports whether
// one was clicked. Show them chooses All, or, for a locked type, lists the
// values of another type, or hides them again.
func (p *valuePicker) updateControls(input *ui.Input) bool {
	for index, chip := range p.chips {
		if chip.Update(input) && p.only == "" {
			p.setKind(index)

			return true
		}
	}
	if !p.show.Update(input) {
		return false
	}
	if p.only == "" {
		p.setKind(0)
	} else {
		p.others = !p.others
	}

	return true
}

// setKind chooses the type chip at an index.
func (p *valuePicker) setKind(index int) {
	p.kind = p.kinds[index]
	for which, chip := range p.chips {
		chip.On = which == index
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
		case strings.HasPrefix(row.ID, otherPrefix):
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
// value: as a reference for an operand, or as the parameter's value. A
// value of another type stays unpicked, with a note why.
func (s *State) pickRow(ctx *frame.Context, rowID string) {
	picker := s.values
	if strings.HasPrefix(rowID, otherPrefix) {
		picker.note = "Only " + picker.only.Label() + " values can be picked here, to match its value."

		return
	}
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
// what it chooses for, the search, the type chips, the list or why it is
// empty, the line under it, the keyboard's ways and the buttons.
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
	s.theme.DrawStrong(screen, typeLabel, left, centredText(picker.chips[0].Rect), ui.ColorInk())
	for _, chip := range picker.chips {
		chip.Draw(screen, s.theme)
	}
	s.drawPickerList(screen)
	room := picker.list.Rect.Dx()
	if !picker.show.Rect.Empty() {
		room = picker.show.Rect.Min.X - left - ui.Half*ui.Gap
	}
	below := picker.choose.Rect.Min.Y - ui.Half*ui.Gap - ui.ButtonH
	ink := ui.ColorMuted()
	if picker.note != "" {
		ink = ui.ColorWarn()
	}
	s.theme.DrawText(screen, s.clip(picker.hiddenLine(), room), left, below+(ui.ButtonH-ui.TextSize)/ui.Half-1, ink)
	picker.show.Draw(screen, s.theme)
	s.theme.DrawText(screen, "↑ ↓ move · Enter picks · Esc closes", left, centredText(picker.cancel.Rect),
		ui.ColorMuted())
	picker.cancel.Draw(screen, s.theme)
	picker.choose.Draw(screen, s.theme)
}

// centredText is the top of a line of text centred on a row as high as a
// button.
func centredText(rect image.Rectangle) int { return rect.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1 }

// drawPickerList draws the list in its frame, or says why it is empty.
func (s *State) drawPickerList(screen *ebiten.Image) {
	picker := s.values
	ui.StrokeRound(screen, picker.list.Rect.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	empty := picker.list.Rect.Min.Add(image.Pt(textPad, ui.Gap))
	switch {
	case len(picker.list.Rows) > 0:
		picker.list.Draw(screen, s.theme)
	case picker.operand:
		s.theme.DrawText(screen, "No other "+picker.only.Label()+" to compare with.", empty.X, empty.Y, ui.ColorMuted())
	default:
		s.theme.DrawText(screen, "No value matches. Type a name to declare one.", empty.X, empty.Y, ui.ColorMuted())
	}
}

// locateInPicker finds a part of the open picker for input scripts: a row,
// "valuerow:<path>" or "declare:<path>", a type chip, "picker:type:<type>",
// its search, "ui:value-search", or a button, "ui:picker-choose",
// "ui:picker-show" and "ui:cancel".
func (s *State) locateInPicker(docs frame.Docs, kind, value string) (image.Point, bool) {
	picker := s.values
	s.layoutValuePicker(docs)
	switch kind {
	case locateValue, locateDeclare:
		return locateRowIn(picker.list, kind, value)
	case pickerLocate:
		return picker.locatePart(value)
	case "ui":
		for _, button := range []*ui.Button{picker.choose, picker.cancel, picker.show} {
			if value == button.ID && !button.Rect.Empty() {
				return ui.Center(button.Rect), true
			}
		}
		if value == picker.search.ID {
			return picker.search.Center(), true
		}
	}

	return image.Pt(0, 0), false
}

// locatePart finds a type chip, "type:<type>".
func (p *valuePicker) locatePart(value string) (image.Point, bool) {
	if named, isType := strings.CutPrefix(value, "type:"); isType {
		for _, chip := range p.chips {
			if chip.ID == chipPrefix+named {
				return ui.Center(chip.Rect), true
			}
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
		if (kind == locateValue && !declaring && row.Label == value) ||
			(kind == locateDeclare && declaring && target.located() == value) {
			return image.Pt(rect.Min.X+textPad, ui.Center(rect).Y), true
		}
	}

	return image.Pt(0, 0), false
}

// hasPickerState answers "picker", whether the picker is open, and
// "picker-hidden=<n>", how many values the search matches that its type
// hides.
func (s *State) hasPickerState(state string, docs frame.Docs) (bool, bool) {
	switch {
	case state == pickerLocate:
		return s.values != nil, true
	case strings.HasPrefix(state, "picker-hidden="):
		if s.values == nil {
			return false, true
		}
		s.layoutValuePicker(docs)

		return strconv.Itoa(len(s.values.hidden)) == strings.TrimPrefix(state, "picker-hidden="), true
	default:
		return false, false
	}
}

// pickerTitle says what a picker chooses for: a parameter by its label, an
// operand by the type it must match, or a value to keep an NPC in.
func pickerTitle(param flow.Param, only worldstate.Type, operand bool) string {
	switch {
	case operand:
		return "for " + param.Label + " · " + strings.ToLower(only.Label()) + "s only, to match its value"
	case only != "":
		return "for " + param.Label + " · " + only.Label() + " values only"
	default:
		return "for " + param.Label
	}
}
