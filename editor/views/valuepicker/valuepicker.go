// Package valuepicker is the value picker: a dialog over a window that
// finds one of a project's values, shared by the windows that choose or
// find values. It has the room a list inside an inspector lacks: a search
// box, focused on open, that matches any part of a path; type chips; a rail
// of scopes with how many values match in each; the values under their
// headings with their type, as a tree of folding groups while nothing is
// typed; and a panel that declares a name nothing has. While it is open it
// takes all input.
//
// A window gives it what it lists and where a name can be declared, as a
// Source, and is told what was picked or declared.
package valuepicker

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Layout, in screen pixels.
const (
	width      = 760
	height     = 540
	radius     = 8
	shadow     = 6
	shade      = 0x40
	buttonW    = 110
	third      = 3
	textPad    = 9
	buttonPad  = 10
	rowSpace   = 6
	maxSearch  = 200
	typeLabel  = "Type"
	chipPrefix = "picker-type-"
	allTypes   = "all"
)

// Locate names for input scripts: "valuerow:<path>" for a value's row,
// "declare:<where>" for where a name can be declared, and "picker:..." for
// a chip, a group or a scope on the rail.
const (
	LocateValue   = "valuerow"
	LocateDeclare = "declare"
	LocatePart    = "picker"
)

// Option is a value the picker can list: its row's ID, handed back when it
// is picked; its path; its declaration; and where it is listed: under a
// heading, in a scope on the rail, and from a root, so what its path has
// between the root and its own name is its group. Detail, when set, is
// added to the type at the right of its row.
type Option struct {
	ID       string
	Path     string
	Declared worldstate.Declaration
	Heading  string
	Rail     string
	Root     string
	Detail   string
}

// group is the group a value is in below its root, as world.quests, or ""
// for one at the top.
func (o Option) group() string {
	cut := strings.LastIndex(o.Path, ".")
	if cut <= len(o.Root) || !strings.HasPrefix(o.Path, o.Root+".") {
		return ""
	}

	return o.Path[:cut]
}

// Target is where a name typed can be declared: its ID, as input scripts
// name it; its path, which no value may have already; its button's label;
// what the panel calls it; and where its limits are set afterwards.
type Target struct {
	ID      string
	Path    string
	Label   string
	Heading string
	SetIn   string
}

// RailEntry is one scope on the rail: its ID, as Option.Rail names it, and
// its label.
type RailEntry struct {
	ID    string
	Label string
}

// Everywhere is the rail's first entry, every scope at once.
const Everywhere = "everywhere"

// Source is a window's side of a picker: the values it lists and where a
// name could be declared, and what a pick or a declare does.
type Source interface {
	Options(docs frame.Docs) []Option
	Targets(docs frame.Docs, query string) []Target
	Pick(ctx *frame.Context, option Option)
	Declare(ctx *frame.Context, target Target, kind worldstate.Type, start json.RawMessage)
}

// Config is what a picker is for: its title and what it chooses for, the
// search's placeholder and what an empty list says; the one type it offers,
// locked, or none; the rail; the row selected at open and the search typed;
// the group last
// opened, kept by the window between pickers; and its buttons' and the
// declare panel's words.
type Config struct {
	Title       string
	For         string
	Placeholder string
	Empty       string
	Only        worldstate.Type
	Rail        []RailEntry
	Selected    string
	Query       string
	LastGroup   *string
	PickLabel   string
	PickHint    string
	DeclareAs   string
	DeclareLead string
}

// Picker is an open value picker.
type Picker struct {
	config Config
	source Source
	theme  *ui.Theme
	kind   worldstate.Type
	search *ui.TextField
	list   *ui.List
	focus  ui.Focus
	chips  []*ui.Button
	kinds  []worldstate.Type
	// hidden is the values the search matches that the type hides; others
	// lists them, for a locked type, as rows that cannot be picked. note
	// says why the last one clicked was not picked.
	hidden []Option
	others bool
	show   *ui.Button
	note   string
	// railRects place the rail's scopes, scope is the one chosen, and counts
	// how many values each has; open is the groups opened. options, paths
	// and parents say, for each row as listed, its option, its path and its
	// group's row.
	railRects []image.Rectangle
	scope     string
	counts    map[string]int
	open      map[string]bool
	options   map[string]Option
	paths     map[string]string
	parents   map[string]string
	declare   *declarePanel
	choose    *ui.Button
	cancel    *ui.Button
	box       image.Rectangle
	window    image.Rectangle
	closed    bool
}

// New opens a picker with the search focused.
func New(config Config, source Source, theme *ui.Theme) *Picker {
	search := ui.NewTextField("value-search", textedit.NewField("", false, maxSearch), false, theme)
	search.Placeholder, search.Unmarked = config.Placeholder, true
	search.Field.SetSaved(config.Query)
	chips, kinds := typeChips(config.Only)
	last := ""
	if config.LastGroup != nil {
		last = *config.LastGroup
	}
	picker := &Picker{
		config: config, source: source, theme: theme, kind: "", search: search,
		list: ui.NewList("values", config.Selected), focus: ui.Focus{}, chips: chips, kinds: kinds, hidden: nil,
		others: false, show: ui.NewButton("picker-show", "Show them"), note: "", railRects: nil, scope: Everywhere,
		counts: map[string]int{}, open: map[string]bool{last: last != ""}, options: map[string]Option{},
		paths: map[string]string{}, parents: map[string]string{}, declare: newDeclarePanel(config, theme),
		choose: ui.NewButton("picker-choose", config.PickLabel),
		cancel: ui.NewButton(ui.DialogCancel, "Cancel"),
		box:    ui.Zero(), window: ui.Zero(), closed: false,
	}
	picker.choose.Primary = true
	picker.focus.Register(search, picker.declare.start)
	picker.focus.Set(search)

	return picker
}

// typeChips makes the type chips: the one type offered, locked on, or All
// and each type a saved value can have.
func typeChips(only worldstate.Type) ([]*ui.Button, []worldstate.Type) {
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

// Focused returns the picker's focused field, the search or the declare
// panel's start, for the editor to send it what is typed.
func (p *Picker) Focused() *ui.TextField {
	if p.focus.Current() == nil {
		p.focus.Set(p.search)
	}

	return p.focus.Current()
}

// offers reports whether the picker lists a value: of its one type, or of
// the chip chosen.
func (p *Picker) offers(option Option) bool {
	switch {
	case p.config.Only != "":
		return option.Declared.Type == p.config.Only
	case p.kind != "":
		return option.Declared.Type == p.kind
	default:
		return true
	}
}

// layout centres the picker in the window and places its parts: the title,
// the search, the type chips, the rail and the list beside it, or the
// panel that declares a value in the list's place, the line under the list,
// and the buttons at the foot.
func (p *Picker) layout(docs frame.Docs, window image.Rectangle) {
	p.window = window
	boxWidth, boxHeight := min(width, window.Dx()-ui.Half*ui.Inset), min(height, window.Dy()-ui.Half*ui.Inset)
	left := window.Min.X + (window.Dx()-boxWidth)/ui.Half
	top := window.Min.Y + max((window.Dy()-boxHeight)/third, ui.Inset)
	p.box = image.Rect(left, top, left+boxWidth, top+boxHeight)
	right := left + boxWidth - ui.Inset
	line := top + ui.Inset + ui.LineHeight + ui.Gap
	p.search.Rect = image.Rect(left+ui.Inset, line, right, line+ui.ButtonH)
	line += ui.ButtonH + ui.Half*ui.Gap
	chip := left + ui.Inset + p.theme.StrongWidth(typeLabel) + ui.Half*ui.Gap
	for _, button := range p.chips {
		button.Rect = image.Rect(chip, line, chip+p.theme.TextWidth(button.Label)+ui.Half*ui.Inset, line+ui.ButtonH)
		chip = button.Rect.Max.X + ui.Gap
	}
	line += ui.ButtonH + ui.Half*ui.Gap
	foot := top + boxHeight - ui.Inset - ui.ButtonH
	below := foot - ui.Half*ui.Gap - ui.ButtonH
	p.layoutRail(left+ui.Inset, line)
	p.list.Rect = image.Rect(left+ui.Inset+railWidth+ui.Inset, line, right, below-ui.Half*ui.Gap)
	p.list.Rows = p.rows(docs)
	p.show.Rect, p.show.Label = ui.Zero(), "Show them"
	if len(p.hidden) > 0 && p.note == "" && !p.declare.showing() {
		p.show.Rect = image.Rect(right-buttonW, below, right, below+ui.ButtonH)
	}
	if p.others && p.config.Only != "" {
		p.show.Label = "Hide them"
	}
	p.layoutFoot(right, foot)
	p.layoutDeclare(p.list.Rect)
}

// keys are the keys the picker reads before its search field takes the
// ones it uses.
type keys struct {
	enter, escape, up, down, left, right bool
}

func readKeys(input *ui.Input) keys {
	return keys{
		enter: input.Has(ui.KeyEnter, ui.Plain()), escape: input.Has(ui.KeyEscape, ui.Plain()),
		up: input.Has(ui.KeyUp, ui.Plain()), down: input.Has(ui.KeyDown, ui.Plain()),
		left: input.Has(ui.KeyLeft, ui.Plain()), right: input.Has(ui.KeyRight, ui.Plain()),
	}
}

// Update runs the picker for a frame in a window, taking all input: typing
// searches, a chip narrows by type, arrows move, a click on a row or Enter
// picks, or declares the name typed while the panel shows; Escape, a click
// outside or Cancel closes. It reports whether the picker is still open.
func (p *Picker) Update(input *ui.Input, ctx *frame.Context, window image.Rectangle) bool {
	p.layout(ctx.Docs, window)
	defer input.ClearAll()
	pressed := readKeys(input)
	if p.cancel.Update(input) || pressed.escape || (input.Pressed && !input.Cursor.In(p.box)) {
		return false
	}
	if input.Pressed || input.Text != "" {
		p.note = ""
	}
	p.focus.Update(input, ctx.Clipboard)
	if p.focus.Current() == nil {
		p.focus.Set(p.search)
	}
	if p.updateControls(input) {
		p.layout(ctx.Docs, window)
	}
	p.list.Rows = p.rows(ctx.Docs)
	if p.moveBy(pressed) {
		p.list.Rows = p.rows(ctx.Docs)
	}
	if !p.updateDeclare(input, ctx, pressed.enter) {
		p.updateList(input, ctx, pressed.enter)
	}

	return !p.closed
}

// updateList picks the row clicked, or the one Enter, the pick button or a
// double-click picks.
func (p *Picker) updateList(input *ui.Input, ctx *frame.Context, enter bool) {
	row := rowAt(p.list, input.Cursor)
	pressed := input.Pressed
	result := p.list.Update(input)
	switch {
	case pressed && row != "":
		p.pickRow(ctx, row)
	case p.choose.Update(input) || result.Activated || enter:
		if chosen := p.chosen(); chosen != "" {
			p.pickRow(ctx, chosen)
		}
	}
}

// rowAt returns the row under a point that can be picked, or "".
func rowAt(list *ui.List, point image.Point) string {
	if !point.In(list.Rect) {
		return ""
	}
	for index, row := range list.Rows {
		if !row.Heading && point.In(list.RowRect(index)) {
			return row.ID
		}
	}

	return ""
}

// updateControls handles the type chips, the rail and Show them, and
// reports whether one was clicked. Show them chooses All, or, for a locked
// type, lists the values of another type, or hides them again.
func (p *Picker) updateControls(input *ui.Input) bool {
	if p.updateRail(input) {
		return true
	}
	for index, chip := range p.chips {
		if chip.Update(input) && p.config.Only == "" {
			p.setKind(index)

			return true
		}
	}
	if !p.show.Update(input) {
		return false
	}
	if p.config.Only == "" {
		p.setKind(0)
	} else {
		p.others = !p.others
	}

	return true
}

// setKind chooses the type chip at an index.
func (p *Picker) setKind(index int) {
	p.kind = p.kinds[index]
	for which, chip := range p.chips {
		chip.On = which == index
	}
}

// move selects the next or previous row that can be picked.
func (p *Picker) move(step int) {
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

// chosen is the row Enter or the pick button picks: the selected one, else
// the top value that matches.
func (p *Picker) chosen() string {
	first := ""
	for _, row := range p.list.Rows {
		switch {
		case row.Heading:
		case row.ID == p.list.Selected:
			return row.ID
		case strings.HasPrefix(row.ID, otherPrefix), strings.HasPrefix(row.ID, groupPrefix):
		case first == "":
			first = row.ID
		}
	}

	return first
}

// pickRow closes the picker and hands the row's value to the window. A
// group's row opens or closes it instead, and a value of another type
// stays unpicked, with a note why.
func (p *Picker) pickRow(ctx *frame.Context, rowID string) {
	switch {
	case strings.HasPrefix(rowID, groupPrefix):
		p.list.Selected = rowID
		p.foldGroup(rowID, !p.open[rowID])
		p.list.Rows = p.rows(ctx.Docs)

		return
	case strings.HasPrefix(rowID, otherPrefix):
		p.note = "Only " + p.config.Only.Label() + " values can be picked here, to match its value."

		return
	}
	option, found := p.options[rowID]
	if !found {
		return
	}
	p.closed = true
	p.source.Pick(ctx, option)
}

// Draw draws the picker over the shaded window: the title and what it
// chooses for, the search, the type chips, the list or why it is empty, the
// line under it, the keyboard's ways and the buttons.
func (p *Picker) Draw(screen *ebiten.Image, docs frame.Docs, window image.Rectangle) {
	p.layout(docs, window)
	ui.FillRect(screen, window, ui.ColorShade())
	ui.FillRound(screen, p.box.Add(image.Pt(0, shadow)), radius, ui.HexAlpha(0, shade))
	ui.FillRound(screen, p.box, radius, ui.ColorBackground())
	ui.StrokeRound(screen, p.box, radius, ui.Border, ui.ColorInk())
	left, top := p.box.Min.X+ui.Inset, p.box.Min.Y+ui.Inset
	p.theme.DrawStrong(screen, p.config.Title, left, top, ui.ColorInk())
	p.theme.DrawText(screen, p.config.For, left+p.theme.StrongWidth(p.config.Title)+ui.Half*ui.Gap, top,
		ui.ColorMuted())
	p.search.Draw(screen)
	p.theme.DrawStrong(screen, typeLabel, left, centred(p.chips[0].Rect), ui.ColorInk())
	for _, chip := range p.chips {
		chip.Draw(screen, p.theme)
	}
	p.drawRail(screen)
	if p.declare.showing() {
		p.drawDeclare(screen)
	} else {
		p.drawList(screen)
	}
	room := p.list.Rect.Dx()
	if !p.show.Rect.Empty() {
		room = p.show.Rect.Min.X - p.list.Rect.Min.X - ui.Half*ui.Gap
	}
	below := p.choose.Rect.Min.Y - ui.Half*ui.Gap - ui.ButtonH
	if p.declare.showing() {
		below = p.declare.declare.Rect.Min.Y - ui.Half*ui.Gap - ui.ButtonH
	}
	ink := ui.ColorMuted()
	if p.note != "" {
		ink = ui.ColorWarn()
	}
	p.theme.DrawText(screen, p.theme.FitText(p.hiddenLine(), room), p.list.Rect.Min.X,
		below+(ui.ButtonH-ui.TextSize)/ui.Half-1, ink)
	p.show.Draw(screen, p.theme)
	p.drawFoot(screen, left)
}

// drawFoot draws the keyboard's ways and the buttons at the foot.
func (p *Picker) drawFoot(screen *ebiten.Image, left int) {
	leftmost := p.cancel.Rect.Min.X
	if !p.declare.ask.Rect.Empty() {
		leftmost = p.declare.ask.Rect.Min.X
	}
	hint := "↑ ↓ move · → opens · Enter " + p.config.PickHint + " · Esc closes"
	if p.declare.showing() {
		hint = "Enter declares · Esc closes"
	}
	p.theme.DrawText(screen, p.theme.FitText(hint, leftmost-left-ui.Inset), left, centred(p.cancel.Rect),
		ui.ColorMuted())
	for _, button := range []*ui.Button{p.declare.ask, p.cancel, p.choose, p.declare.declare} {
		button.Draw(screen, p.theme)
	}
}

// centred is the top of a line of text centred on a row as high as a
// button.
func centred(rect image.Rectangle) int { return rect.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1 }

// drawList draws the list in its frame, or says why it is empty.
func (p *Picker) drawList(screen *ebiten.Image) {
	ui.StrokeRound(screen, p.list.Rect.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	empty := p.list.Rect.Min.Add(image.Pt(textPad, ui.Gap))
	if len(p.list.Rows) > 0 {
		p.list.Draw(screen, p.theme)

		return
	}
	p.theme.DrawText(screen, p.config.Empty, empty.X, empty.Y, ui.ColorMuted())
}

// Locate finds a part of the picker for input scripts: a row,
// "valuerow:<path>"; where a value can be declared, "declare:<where>"; a
// chip, a group or a scope on the rail, "picker:..." (see locatePart); a
// field, "ui:value-search" or "ui:declare-start"; or a button, as
// "ui:picker-choose", "ui:picker-declare", "ui:declare-type-bool" or
// "ui:cancel".
func (p *Picker) Locate(docs frame.Docs, window image.Rectangle, kind, value string) (image.Point, bool) {
	p.layout(docs, window)
	switch kind {
	case LocateValue:
		return p.locateRow(value)
	case LocateDeclare:
		return p.declare.locateWhere(value)
	case LocatePart:
		return p.locatePart(value)
	case "ui":
		buttons, start := p.declare.widgets()
		for _, button := range append(buttons, p.choose, p.cancel, p.show) {
			if value == button.ID && !button.Rect.Empty() {
				return ui.Center(button.Rect), true
			}
		}
		for _, field := range []*ui.TextField{p.search, start} {
			if value == field.ID && !field.Rect.Empty() {
				return field.Center(), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// HasState answers "picker-hidden=<n>", how many values the search matches
// that its type hides; "picker-count:<scope>=<n>", how many it matches in a
// scope on the rail; and "picker-detail:<path>=<detail>", what a value's
// row says at its right. It reports whether the state was one of these.
func (p *Picker) HasState(docs frame.Docs, window image.Rectangle, state string) (bool, bool) {
	hidden, isHidden := strings.CutPrefix(state, "picker-hidden=")
	counted, isCount := strings.CutPrefix(state, "picker-count:")
	detailed, isDetail := strings.CutPrefix(state, "picker-detail:")
	if !isHidden && !isCount && !isDetail {
		return false, false
	}
	p.layout(docs, window)
	switch {
	case isHidden:
		return strconv.Itoa(len(p.hidden)) == hidden, true
	case isDetail:
		path, want, _ := strings.Cut(detailed, "=")

		return slices.ContainsFunc(p.list.Rows, func(row ui.Row) bool {
			return p.paths[row.ID] == path && row.Detail == want
		}), true
	}
	scope, want, _ := strings.Cut(counted, "=")

	return strconv.Itoa(p.counts[scope]) == want, true
}
