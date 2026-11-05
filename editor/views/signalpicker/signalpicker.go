// Package signalpicker is the signal picker: a dialog over a window that
// chooses one of a project's signals, which may be far too many for a row
// of buttons. A search box, focused on open, matches any part of a
// signal's path. With nothing typed, the signals the window's workflow
// already sends or answers come first, then every signal under its folding
// groups; each says how often it is sent and answered. While it is open it
// takes all input.
package signalpicker

import (
	"image"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Layout, in screen pixels.
const (
	width     = 480
	height    = 520
	radius    = 8
	shadow    = 6
	shade     = 0x40
	buttonW   = 100
	third     = 3
	textPad   = 9
	maxSearch = 200
)

// Row ID prefixes: a signal under IN THIS WORKFLOW, a signal under ALL
// SIGNALS or MATCHES, and a group; headings.
const (
	herePrefix  = "here:"
	rowPrefix   = "signal:"
	groupPrefix = "group:"
	headHere    = "heading:here"
	headAll     = "heading:all"
)

// Locate names for input scripts: "signalrow:<path>" for a signal's row,
// as listed under ALL SIGNALS or MATCHES, and "signalgroup:<path>" for a
// group's.
const (
	LocateSignal = "signalrow"
	LocateGroup  = "signalgroup"
)

// Option is a signal the picker can list: its ID, its path, how often it
// is sent and answered, and, when the window's workflow sends or answers it
// already, how.
type Option struct {
	ID     ids.ULID
	Path   string
	Detail string
	Here   string
}

// Config is what a picker is for: its title, the signal chosen at open,
// and what a pick does.
type Config struct {
	Title    string
	Selected ids.ULID
	Pick     func(ctx *frame.Context, option Option)
}

// Picker is an open signal picker.
type Picker struct {
	config  Config
	options []Option
	theme   *ui.Theme
	search  *ui.TextField
	focus   ui.Focus
	list    *ui.List
	choose  *ui.Button
	cancel  *ui.Button
	// open is the groups opened, by path; box is the dialog's area.
	open   map[string]bool
	box    image.Rectangle
	closed bool
}

// New opens a picker with the search focused and the chosen signal's
// groups open.
func New(config Config, options []Option, theme *ui.Theme) *Picker {
	search := ui.NewTextField("signal-search", textedit.NewField("", false, maxSearch), false, theme)
	search.Placeholder, search.Unmarked = "Type any part of a name or group", true
	picker := &Picker{
		config: config, options: options, theme: theme, search: search, focus: ui.Focus{},
		list: ui.NewList("signals", ""), choose: ui.NewButton("signal-choose", "Choose"),
		cancel: ui.NewButton(ui.DialogCancel, "Cancel"), open: map[string]bool{}, box: ui.Zero(), closed: false,
	}
	picker.choose.Primary = true
	picker.focus.Register(search)
	picker.focus.Set(search)
	for _, option := range options {
		if option.ID == config.Selected {
			picker.list.Selected = rowPrefix + option.Path
			for _, group := range groupsOf(option.Path) {
				picker.open[group] = true
			}
		}
	}

	return picker
}

// groupsOf lists the groups a path is in, outermost first: shop.stock.low
// is in shop and shop.stock.
func groupsOf(path string) []string {
	parts := strings.Split(path, ".")
	groups := make([]string, 0, len(parts)-1)
	for index := 1; index < len(parts); index++ {
		groups = append(groups, strings.Join(parts[:index], "."))
	}

	return groups
}

// Focused returns the picker's search field.
func (p *Picker) Focused() *ui.TextField { return p.search }

// query is the search typed, in lower case.
func (p *Picker) query() string { return strings.ToLower(strings.TrimSpace(p.search.Field.Value())) }

// rows lists the picker's rows: the matches while something is typed;
// otherwise the workflow's own signals, then every signal in its groups.
func (p *Picker) rows() []ui.Row {
	if query := p.query(); query != "" {
		var rows []ui.Row
		for _, option := range p.options {
			if strings.Contains(strings.ToLower(option.Path), query) {
				row := ui.NewRow(rowPrefix+option.Path, option.Path)
				row.Detail = option.Detail
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			return nil
		}

		return append([]ui.Row{ui.NewHeading(headAll, "MATCHES "+strconv.Itoa(len(rows)))}, rows...)
	}
	var rows []ui.Row
	for _, option := range p.options {
		if option.Here != "" {
			row := ui.NewRow(herePrefix+option.Path, option.Path)
			row.Detail = option.Here
			rows = append(rows, row)
		}
	}
	if len(rows) > 0 {
		rows = append([]ui.Row{ui.NewHeading(headHere, "IN THIS WORKFLOW "+strconv.Itoa(len(rows)))}, rows...)
	}
	rows = append(rows, ui.NewHeading(headAll, "ALL SIGNALS "+strconv.Itoa(len(p.options))))

	return append(rows, p.treeRows("", 0)...)
}

// treeRows lists what a group holds, groups first, each group's contents
// under it while it is open.
func (p *Picker) treeRows(group string, depth int) []ui.Row {
	children := map[string]bool{}
	var signals []Option
	for _, option := range p.options {
		inside, ok := strings.CutPrefix(option.Path, group)
		if group != "" && (!ok || !strings.HasPrefix(inside, ".")) {
			continue
		}
		inside = strings.TrimPrefix(inside, ".")
		if name, _, nested := strings.Cut(inside, "."); nested {
			children[name] = true
		} else {
			signals = append(signals, option)
		}
	}
	names := slices.Sorted(maps.Keys(children))
	var rows []ui.Row
	for _, name := range names {
		path := strings.TrimPrefix(group+"."+name, ".")
		row := ui.NewRow(groupPrefix+path, name)
		row.Icon, row.Strong, row.Indent, row.Toggle = ui.IconFolder, true, depth, ui.DisclosureClosed
		row.Detail = strconv.Itoa(p.within(path))
		if p.open[path] {
			row.Toggle = ui.DisclosureOpen
		}
		rows = append(rows, row)
		if p.open[path] {
			rows = append(rows, p.treeRows(path, depth+1)...)
		}
	}
	for _, option := range signals {
		row := ui.NewRow(rowPrefix+option.Path, option.Path[strings.LastIndex(option.Path, ".")+1:])
		row.Indent, row.Detail = depth, option.Detail
		rows = append(rows, row)
	}

	return rows
}

// within counts the signals in a group, however deep.
func (p *Picker) within(group string) int {
	count := 0
	for _, option := range p.options {
		if strings.HasPrefix(option.Path, group+".") {
			count++
		}
	}

	return count
}

// layout places the dialog in the middle of a window, a third of the way
// down: the title, the search, the list, and the keys and buttons at its
// foot.
func (p *Picker) layout(window image.Rectangle) {
	boxWidth, boxHeight := min(width, window.Dx()-ui.Half*ui.Inset), min(height, window.Dy()-ui.Half*ui.Inset)
	left := window.Min.X + (window.Dx()-boxWidth)/ui.Half
	top := window.Min.Y + max((window.Dy()-boxHeight)/third, ui.Inset)
	p.box = image.Rect(left, top, left+boxWidth, top+boxHeight)
	right := left + boxWidth - ui.Inset
	line := top + ui.Inset + ui.LineHeight + ui.Gap
	p.search.Rect = image.Rect(left+ui.Inset, line, right, line+ui.ButtonH)
	line += ui.ButtonH + ui.Gap
	foot := top + boxHeight - ui.Inset - ui.ButtonH
	p.list.Rect = image.Rect(left+ui.Inset, line, right, foot-ui.Gap)
	p.choose.Rect = image.Rect(right-buttonW, foot, right, foot+ui.ButtonH)
	p.cancel.Rect = image.Rect(right-buttonW*ui.Half-ui.Gap, foot, right-buttonW-ui.Gap, foot+ui.ButtonH)
	p.list.Rows = p.rows()
}

// Update runs the picker for a frame in a window, taking all input: typing
// searches, arrows move, a click on a signal or Enter picks it, a click on
// a group opens or closes it; Escape, a click outside or Cancel closes. It
// reports whether the picker is still open.
func (p *Picker) Update(input *ui.Input, ctx *frame.Context, window image.Rectangle) bool {
	p.layout(window)
	defer input.ClearAll()
	enter, escape := input.Has(ui.KeyEnter, ui.Plain()), input.Has(ui.KeyEscape, ui.Plain())
	upward, downward := input.Has(ui.KeyUp, ui.Plain()), input.Has(ui.KeyDown, ui.Plain())
	if p.cancel.Update(input) || escape || (input.Pressed && !input.Cursor.In(p.box)) {
		return false
	}
	p.focus.Update(input, ctx.Clipboard)
	p.list.Rows = p.rows()
	switch {
	case upward:
		p.move(-1)
	case downward:
		p.move(1)
	}
	row := p.rowAt(input.Cursor)
	pressed := input.Pressed
	result := p.list.Update(input)
	switch {
	case result.Toggled != "":
		p.fold(result.Toggled)
	case pressed && row != "":
		p.pickRow(ctx, row)
	case p.choose.Update(input) || result.Activated || enter:
		if chosen := p.chosen(); chosen != "" {
			p.pickRow(ctx, chosen)
		}
	}

	return !p.closed
}

// rowAt returns the row under a point that can be clicked, or "".
func (p *Picker) rowAt(point image.Point) string {
	if !point.In(p.list.Rect) {
		return ""
	}
	for index, row := range p.list.Rows {
		if !row.Heading && point.In(p.list.RowRect(index)) {
			return row.ID
		}
	}

	return ""
}

// move moves the selection to the next or previous row that is not a
// heading.
func (p *Picker) move(step int) {
	var rows []string
	for _, row := range p.list.Rows {
		if !row.Heading {
			rows = append(rows, row.ID)
		}
	}
	if len(rows) == 0 {
		return
	}
	at := slices.Index(rows, p.list.Selected)
	if at < 0 && step < 0 {
		at = len(rows)
	}
	p.list.Selected = rows[min(max(at+step, 0), len(rows)-1)]
	p.list.Reveal(p.list.Selected)
}

// chosen is the row Enter or Choose picks: the selected one, else the
// first signal listed.
func (p *Picker) chosen() string {
	first := ""
	for _, row := range p.list.Rows {
		switch {
		case row.Heading:
		case row.ID == p.list.Selected:
			return row.ID
		case first == "" && !strings.HasPrefix(row.ID, groupPrefix):
			first = row.ID
		}
	}

	return first
}

// fold opens or closes a group by its row.
func (p *Picker) fold(rowID string) {
	if path, isGroup := strings.CutPrefix(rowID, groupPrefix); isGroup {
		p.open[path] = !p.open[path]
		p.list.Rows = p.rows()
	}
}

// pickRow closes the picker and hands the row's signal to the window; a
// group's row opens or closes the group instead.
func (p *Picker) pickRow(ctx *frame.Context, rowID string) {
	if strings.HasPrefix(rowID, groupPrefix) {
		p.list.Selected = rowID
		p.fold(rowID)

		return
	}
	path := strings.TrimPrefix(strings.TrimPrefix(rowID, herePrefix), rowPrefix)
	for _, option := range p.options {
		if option.Path == path {
			p.closed = true
			p.config.Pick(ctx, option)

			return
		}
	}
}

// Draw draws the picker over the shaded window: the title, the search, the
// list or why it is empty, the keys and the buttons.
func (p *Picker) Draw(screen *ebiten.Image, window image.Rectangle) {
	p.layout(window)
	ui.FillRect(screen, window, ui.ColorShade())
	ui.FillRound(screen, p.box.Add(image.Pt(0, shadow)), radius, ui.HexAlpha(0, shade))
	ui.FillRound(screen, p.box, radius, ui.ColorBackground())
	ui.StrokeRound(screen, p.box, radius, ui.Border, ui.ColorInk())
	left, top := p.box.Min.X+ui.Inset, p.box.Min.Y+ui.Inset
	p.theme.DrawStrong(screen, p.config.Title, left, top, ui.ColorInk())
	p.search.Draw(screen)
	ui.StrokeRound(screen, p.list.Rect.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	if len(p.list.Rows) > 0 {
		p.list.Draw(screen, p.theme)
	} else {
		empty := "No signal matches. Declare one in World state's Signals window."
		if len(p.options) == 0 {
			empty = "No signals yet. Declare them in World state's Signals window."
		}
		p.theme.DrawText(screen, p.theme.FitText(empty, p.list.Rect.Dx()-ui.Half*textPad),
			p.list.Rect.Min.X+textPad, p.list.Rect.Min.Y+ui.Gap, ui.ColorMuted())
	}
	hint := "↑ ↓ move · Enter chooses · Esc closes"
	baseline := p.choose.Rect.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	p.theme.DrawText(screen, p.theme.FitText(hint, p.cancel.Rect.Min.X-left-ui.Gap), left, baseline, ui.ColorMuted())
	p.cancel.Draw(screen, p.theme)
	p.choose.Draw(screen, p.theme)
}

// Locate finds a part of the picker for input scripts: a signal's row, as
// "signalrow:<path>"; a group's, as "signalgroup:<path>"; or, as
// "ui:<id>", the search or a button.
func (p *Picker) Locate(window image.Rectangle, kind, value string) (image.Point, bool) {
	p.layout(window)
	switch kind {
	case LocateSignal:
		return p.list.Locate(rowPrefix + value)
	case LocateGroup:
		return p.list.Locate(groupPrefix + value)
	}
	for _, button := range []*ui.Button{p.choose, p.cancel} {
		if button.ID == value {
			return ui.Center(button.Rect), true
		}
	}
	if value == p.search.ID {
		return p.search.Center(), true
	}

	return image.Pt(0, 0), false
}
