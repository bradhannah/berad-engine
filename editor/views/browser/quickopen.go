package browser

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Quick open (increment 38): Cmd+P in any tab opens this window's search in
// a sheet over the editor window, which the editor window places and draws
// over the tab in front. It finds what the Project's search finds, apart
// from what opens no tab, such as Settings. Enter opens what is chosen as a
// double-click does here, Cmd+Enter opens it beside the tab in front, and
// Esc or a click outside closes the sheet. It never changes this window's
// own search or selection.

// Quick open's IDs, for input scripts, and its limits.
const (
	QuickSearchID = "quick-search"
	quickListID   = "quick"
	quickLimit    = 200
	quickHint     = "↑ ↓ move · Enter opens · Cmd+Enter opens beside · Esc closes"
	quickTitle    = "Open"
	// quickAway is how far off the window the pointer is moved for this
	// window's own parts while the sheet takes the input.
	quickAway = 1 << 20
)

// Beside is what quick open asked to open beside the tab in front: a window
// of a view showing a target.
type Beside struct {
	View   string
	Target ids.ULID
}

// quickOpen is the sheet's state. rect is where the editor window places it,
// in this window's own coordinates, and room whether the tab in front has
// room for a window beside it. beside is what Cmd+Enter last opened, until
// the editor window takes it.
type quickOpen struct {
	open   bool
	search *ui.TextField
	focus  ui.Focus
	list   *ui.List
	rect   image.Rectangle
	room   bool
	note   string
	beside *Beside
}

func newQuickOpen(theme *ui.Theme) *quickOpen {
	search := ui.NewTextField(QuickSearchID, textedit.NewField("", false, searchBytes), false, theme)
	search.Placeholder, search.Unmarked = "Find a document, a sound, a constant or a line", true
	quick := &quickOpen{
		open: false, search: search, focus: ui.Focus{}, list: ui.NewList(quickListID, ""), rect: ui.Zero(),
		room: false, note: "", beside: nil,
	}
	quick.focus.Register(search)

	return quick
}

// OpenQuick opens the sheet with an empty search that has the keyboard.
func (s *State) OpenQuick() {
	s.quick.open, s.quick.note, s.quick.list.Selected = true, "", ""
	s.quick.search.Field.Revert()
	s.quick.search.Field.SetSaved("")
	s.quick.focus.Set(s.quick.search)
}

// QuickOpen reports whether the sheet shows.
func (s *State) QuickOpen() bool { return s.quick.open }

// SetQuick places the sheet, in this window's coordinates, and says whether
// the tab in front has room for a window beside it.
func (s *State) SetQuick(rect image.Rectangle, room bool) { s.quick.rect, s.quick.room = rect, room }

// TakeBeside returns what Cmd+Enter last opened, once.
func (s *State) TakeBeside() (Beside, bool) {
	beside := s.quick.beside
	s.quick.beside = nil
	if beside == nil {
		return Beside{View: "", Target: ids.ULID{}}, false
	}

	return *beside, true
}

// quickLayout places the sheet's search and list inside its rect.
func (s *State) quickLayout(docs frame.Docs) {
	rect := s.quick.rect
	left, right := rect.Min.X+ui.Inset, rect.Max.X-ui.Inset
	line := rect.Min.Y + ui.Inset + ui.LineHeight + ui.Gap
	s.quick.search.Rect = image.Rect(left, line, right, line+ui.ButtonH)
	foot := rect.Max.Y - ui.Inset - ui.LineHeight
	s.quick.list.Rect = image.Rect(left, line+ui.ButtonH+ui.Gap, right, foot-ui.Gap)
	s.quick.list.Rows = s.quickRows(docs)
}

// quickRows are what the search finds that opens a tab, at most quickLimit
// of them; none until something is typed.
func (s *State) quickRows(docs frame.Docs) []ui.Row {
	query := strings.TrimSpace(s.quick.search.Field.Value())
	if query == "" {
		return nil
	}
	found, _ := s.thingsFor(docs, query)
	rows := make([]ui.Row, 0, min(len(found), quickLimit))
	for _, each := range found {
		if !each.matches || !s.opensATab(docs, each.row.ID) {
			continue
		}
		row := each.row
		row.Indent, row.Tree = 0, false
		rows = append(rows, row)
		if len(rows) == quickLimit {
			break
		}
	}

	return rows
}

// updateQuick runs the sheet for a frame, taking all of the input: typing
// searches, arrows move, Enter or a double-click opens, Cmd+Enter opens
// beside, and Esc or a press outside closes.
func (s *State) updateQuick(input *ui.Input, ctx *frame.Context) {
	defer input.ClearAll()
	s.quickLayout(ctx.Docs)
	besideKey := input.ConsumeExact(ui.KeyEnter, ui.Cmd())
	enter := input.Has(ui.KeyEnter, ui.Plain())
	if input.Has(ui.KeyEscape, ui.Plain()) || (input.Pressed && !input.Cursor.In(s.quick.rect)) {
		s.quick.open = false

		return
	}
	before := s.quick.search.Field.Value()
	s.quick.focus.Update(input, ctx.Clipboard)
	s.quick.focus.Set(s.quick.search)
	if s.quick.search.Field.Value() != before {
		s.quick.note, s.quick.list.Selected = "", ""
		s.quickLayout(ctx.Docs)
	}
	switch {
	case input.Has(ui.KeyUp, ui.Plain()):
		s.moveQuick(-1)
	case input.Has(ui.KeyDown, ui.Plain()):
		s.moveQuick(1)
	}
	result := s.quick.list.Update(input)
	if result.Activated || enter || besideKey {
		s.openQuickChoice(ctx, besideKey)
	}
}

// moveQuick selects the next or previous row.
func (s *State) moveQuick(step int) {
	rows := s.quick.list.Rows
	if len(rows) == 0 {
		return
	}
	current := -1
	for index, row := range rows {
		if row.ID == s.quick.list.Selected {
			current = index
		}
	}
	s.quick.list.Selected = rows[min(max(current+step, 0), len(rows)-1)].ID
	s.quick.list.Reveal(s.quick.list.Selected)
}

// openQuickChoice opens the row selected, or the first when none is, and
// closes the sheet; or says in the sheet why it cannot.
func (s *State) openQuickChoice(ctx *frame.Context, beside bool) {
	rows := s.quick.list.Rows
	chosen := s.quick.list.Selected
	if chosen == "" && len(rows) > 0 {
		chosen = rows[0].ID
	}
	switch {
	case chosen == "":
		s.quick.note = "Nothing found to open"

		return
	case beside && !s.quick.room:
		s.quick.note = "The tab in front already shows two windows: close one to open another beside it"

		return
	}
	view, target, refusal := s.thingWindow(ctx.Docs, chosen)
	if refusal != "" {
		s.quick.note = refusal

		return
	}
	s.openThing(ctx, chosen, view, target)
	if beside {
		s.quick.beside = &Beside{View: view, Target: target}
	}
	s.quick.open = false
}

// opensATab reports whether a row found by a search opens a tab.
func (s *State) opensATab(docs frame.Docs, id string) bool {
	view, _, refusal := s.thingWindow(docs, id)

	return view != "" || refusal != ""
}

// thingWindow is the window a row found by a search opens in and what it
// shows there, or why it opens in none; "" and no reason for a row that
// opens no tab at all.
func (s *State) thingWindow(docs frame.Docs, rowID string) (string, ids.ULID, string) {
	switch {
	case strings.HasPrefix(rowID, foundConstant), rowID == worldConstantsRow:
		return worldConstantsView, worldstate.ID(), ""
	case strings.HasPrefix(rowID, foundLine):
		workflowText, _, _ := strings.Cut(strings.TrimPrefix(rowID, foundLine), ":")
		if workflowID, err := ids.Parse(workflowText); err == nil {
			return workflowview.ViewName, workflowID, ""
		}
	case rowID == worldStateRow:
		return worldStateView, worldstate.ID(), ""
	case rowID == worldSignalsRow:
		return worldSignalsView, worldstate.ID(), ""
	case strings.HasPrefix(rowID, audioCuePrefix):
		return audioview.ViewName, project.IndexID(), ""
	}
	documentID, err := ids.Parse(rowID)
	if err != nil {
		return "", ids.ULID{}, ""
	}
	entry, listed := s.index(docs).Entry(documentID)
	if !listed {
		return "", ids.ULID{}, ""
	}

	return s.entryWindow(docs, entry)
}

// openThing opens a row found by a search in its window, at the constant,
// line or cue it is where it is one.
func (s *State) openThing(ctx *frame.Context, rowID, view string, target ids.ULID) {
	switch {
	case strings.HasPrefix(rowID, foundConstant):
		ctx.Requests.OpenItem(view, target, strings.TrimPrefix(rowID, foundConstant))
	case strings.HasPrefix(rowID, foundLine):
		_, node, _ := strings.Cut(strings.TrimPrefix(rowID, foundLine), ":")
		ctx.Requests.OpenItem(view, target, workflowview.NodeItem+node)
	case strings.HasPrefix(rowID, audioCuePrefix):
		ctx.Requests.OpenItem(view, target, strings.TrimPrefix(rowID, audioCuePrefix))
	default:
		ctx.Requests.Open(view, target, false)
	}
}

// DrawQuick draws the sheet where the editor window placed it, floating on a
// hard shadow: its title and keys, the search, what it found and a note
// when something could not be opened.
func (s *State) DrawQuick(screen *ebiten.Image, docs frame.Docs) {
	if !s.quick.open {
		return
	}
	s.quickLayout(docs)
	rect := s.quick.rect
	ui.Float(screen, rect, ui.Radius)
	ui.FillRound(screen, rect, ui.Radius, ui.ColorPage())
	ui.StrokeRound(screen, rect, ui.Radius, ui.Border, ui.ColorInk())
	left, top := rect.Min.X+ui.Inset, rect.Min.Y+ui.Inset
	s.theme.DrawStrong(screen, quickTitle, left, top, ui.ColorInk())
	hintLeft := left + s.theme.StrongWidth(quickTitle) + ui.Inset
	s.theme.DrawText(screen, s.theme.FitText(quickHint, rect.Max.X-ui.Inset-hintLeft), hintLeft, top, ui.ColorSubdued())
	s.quick.search.Draw(screen)
	s.quick.list.Draw(screen, s.theme)
	foot := rect.Max.Y - ui.Inset - ui.LineHeight
	switch {
	case s.quick.note != "":
		s.theme.DrawText(screen, s.theme.FitText(s.quick.note, rect.Dx()-ui.Half*ui.Inset), left, foot, ui.ColorWarn())
	case s.quick.search.Field.Value() != "" && len(s.quick.list.Rows) == 0:
		s.theme.DrawText(screen, "Nothing found", left, foot, ui.ColorSubdued())
	case len(s.quick.list.Rows) == quickLimit:
		s.theme.DrawText(screen, "The first 200 found: type more to narrow", left, foot, ui.ColorSubdued())
	}
}

// locateQuick finds the sheet's parts for input scripts: "quick:<label>"
// for a row found, and "ui:quick-search".
func (s *State) locateQuick(name string, docs frame.Docs) (image.Point, bool) {
	s.quickLayout(docs)
	if name == "ui:"+QuickSearchID {
		return ui.Center(s.quick.search.Rect), true
	}
	label, ok := strings.CutPrefix(name, quickListID+":")
	if !ok {
		return image.Pt(0, 0), false
	}
	for index, row := range s.quick.list.Rows {
		if row.Label == label {
			return ui.Center(s.quick.list.RowRect(index)), true
		}
	}

	return image.Pt(0, 0), false
}
