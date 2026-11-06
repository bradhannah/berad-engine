package browser

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/editor/views/writesheet"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// The Constants section of an NPC's or a zone's inspector, in screen pixels
// and for input scripts: how many constants it lists before Open Constants,
// a row's parts, and their names, by the constant's path, such as
// "ui:constant-write-npc.shop.greeting".
const (
	constantsShown   = 4
	constantRowGap   = 6
	constantButtonW  = 66
	constantNumberW  = 90
	constantWritePre = "constant-write-"
	constantClearPre = "constant-default-"
	constantFieldPre = "constant-number-"
	openConstantsID  = "open-constants"
	maxConstantDigit = 20
	constantStatePre = "constant:"
	previewLength    = 40
)

// constantsPanel is the Constants section of an NPC's or a zone's
// inspector: the constants of the templates on it and its own, each with
// its own value or the default. A text is written in the writing sheet, a
// number typed in place, and Default gives one back the default. top is
// where its heading goes, and more how many it does not list.
type constantsPanel struct {
	theme *ui.Theme
	owner ids.ULID
	scope worldstate.Scope
	name  string
	rows  []*constantRow
	byID  map[ids.ULID]*constantRow
	top   int
	more  int
	open  *ui.Button
	focus ui.Focus
	sheet *writesheet.Sheet
	// sheetOf is the constant the sheet writes, declared in sheetIn.
	sheetOf ids.ULID
	sheetIn valuetree.List
}

// constantRow is one constant as the NPC or zone has it.
type constantRow struct {
	held   worldstate.Held
	value  worldstate.Value
	own    bool
	mine   bool
	write  *ui.Button
	clear  *ui.Button
	number *ui.TextField
	rect   image.Rectangle
}

func newConstantsPanel(theme *ui.Theme) *constantsPanel {
	return &constantsPanel{
		theme: theme, owner: ids.ULID{}, scope: "", name: "", rows: nil, byID: map[ids.ULID]*constantRow{}, top: 0, more: 0,
		open: ui.NewButton(openConstantsID, "Open Constants"), focus: ui.Focus{}, sheet: nil, sheetOf: ids.ULID{},
		sheetIn: valuetree.List{Document: ids.ULID{}, Edit: "", Values: nil},
	}
}

// constantsOwner returns the selected NPC or zone, whose constants the
// inspector lists.
func (s *State) constantsOwner(docs frame.Docs) (ids.ULID, worldstate.Scope, bool) {
	entry, ok := s.selected(docs)
	switch {
	case !ok:
		return ids.ULID{}, "", false
	case entry.Format == npc.FormatName:
		return entry.ID, worldstate.ScopeNPC, true
	case entry.Format == zone.FormatName:
		return entry.ID, worldstate.ScopeZone, true
	}

	return ids.ULID{}, "", false
}

// syncConstants lists the selected NPC's or zone's constants, keeping what
// is typed in a number while it stays selected.
func (s *State) syncConstants(ctx *frame.Context) {
	panel := s.constants
	owner, scope, ok := s.constantsOwner(ctx.Docs)
	if !ok || owner != panel.owner {
		panel.byID, panel.focus = map[ids.ULID]*constantRow{}, ui.Focus{}
	}
	panel.owner, panel.scope, panel.rows = owner, scope, nil
	if !ok {
		return
	}
	s.subscribe(worldstate.ID(), ctx)
	catalog, ready := valuetree.Catalog(ctx.Docs)
	if !ready {
		return
	}
	held, known := catalog.Owner(owner)
	if !known {
		return
	}
	panel.name = held.Name
	for _, constant := range catalog.HeldConstants(scope, owner) {
		row := panel.row(constant)
		_, inTemplate := catalog.World().Template(constant.Declaration.ID)
		row.mine = !inTemplate
		row.value, row.own = held.Owned.ConstantValue(constant.Declaration)
		if !row.own {
			row.value = constant.Declaration.Zero()
		}
		if constant.Declaration.Type == worldstate.TypeInteger {
			row.number.Field.SetSaved(strconv.Itoa(row.value.Integer))
		}
		panel.rows = append(panel.rows, row)
	}
}

// row returns a constant's row, made once while the owner stays selected.
func (p *constantsPanel) row(constant worldstate.Held) *constantRow {
	if made, found := p.byID[constant.Declaration.ID]; found {
		made.held = constant

		return made
	}
	made := &constantRow{
		held: constant, value: worldstate.NoValue(), own: false, mine: false,
		write: ui.NewButton(constantWritePre+constant.Path, "Write…"),
		clear: ui.NewButton(constantClearPre+constant.Path, "Default"),
		number: ui.NewTextField(constantFieldPre+constant.Path, textedit.NewField("", false, maxConstantDigit), false,
			p.theme),
		rect: ui.Zero(),
	}
	p.byID[constant.Declaration.ID] = made
	p.focus.Register(made.number)

	return made
}

// layoutConstants places the Constants section from top, each listed
// constant a row, and returns where the next part goes. With none, it
// takes no room.
func (s *State) layoutConstants(left, width, top int) int {
	panel := s.constants
	panel.open.Rect, panel.more = ui.Zero(), 0
	for _, row := range panel.byID {
		row.rect, row.write.Rect, row.clear.Rect, row.number.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	}
	if len(panel.rows) == 0 {
		return top
	}
	panel.top = top
	rowY := top + ui.LineHeight
	for index, row := range panel.rows {
		if index == constantsShown {
			panel.more = len(panel.rows) - constantsShown

			break
		}
		row.rect = image.Rect(left, rowY, left+width, rowY+ui.LineHeight+ui.ButtonH)
		buttonsTop := rowY + ui.LineHeight
		right := left + width
		if row.own && !row.mine {
			row.clear.Rect = image.Rect(right-constantButtonW, buttonsTop, right, buttonsTop+ui.ButtonH)
			right -= constantButtonW + ui.Gap
		}
		if row.held.Declaration.Type == worldstate.TypeInteger {
			row.number.Rect = image.Rect(left, buttonsTop, left+constantNumberW, buttonsTop+ui.ButtonH)
		} else {
			row.write.Rect = image.Rect(right-constantButtonW, buttonsTop, right, buttonsTop+ui.ButtonH)
		}
		rowY += ui.LineHeight + ui.ButtonH + constantRowGap
	}
	panel.open.Rect = image.Rect(left, rowY, left+min(width, s.theme.TextWidth(panel.open.Label)+ui.Half*ui.Inset),
		rowY+ui.ButtonH)

	return rowY + ui.ButtonH + ui.Gap
}

// updateConstants handles the Constants section: Write…, Default, a number
// saved with Enter, and Open Constants.
func (s *State) updateConstants(input *ui.Input, ctx *frame.Context) {
	panel := s.constants
	if panel.open.Update(input) {
		ctx.Requests.Open(worldConstantsView, worldstate.ID(), false)

		return
	}
	field, outcome := panel.focus.Update(input, ctx.Clipboard)
	for _, row := range panel.rows {
		switch {
		case row.write.Update(input):
			s.openConstantSheet(ctx, row)

			return
		case row.clear.Update(input):
			s.setOwnConstant(ctx, row, nil, row.held.Declaration.Name+" is the default again. Undo brings back "+
				panel.name+"'s own.")

			return
		case field == row.number && outcome == textedit.OutcomeCommit:
			s.saveNumber(ctx, row)
		}
	}
}

// saveNumber saves a number typed as the NPC's or zone's own, or as its own
// constant's value.
func (s *State) saveNumber(ctx *frame.Context, row *constantRow) {
	typed, changed := row.number.Field.Commit()
	if !changed {
		return
	}
	number, err := strconv.Atoi(strings.TrimSpace(typed))
	if err != nil || number > worldstate.MaxInteger || number < -worldstate.MaxInteger {
		row.number.Field.Failed()
		ctx.Frame.Status("Enter a whole number")

		return
	}
	value := worldstate.NoValue()
	value.Type, value.Integer = worldstate.TypeInteger, number
	if row.mine {
		s.saveDefault(ctx, row.held.Declaration.ID, valueList(ctx.Docs, s.constants.owner, true), value)

		return
	}
	s.setOwnConstant(ctx, row, value.Encode(), "Set "+s.constants.name+"'s "+row.held.Declaration.Name+".")
}

// valueList is the list a constant is declared in: the NPC's or zone's own,
// or the world state's, for a template's.
func valueList(docs frame.Docs, owner ids.ULID, mine bool) valuetree.List {
	if mine {
		list, _ := valuetree.Owned(docs, owner)

		return list
	}
	list, _ := valuetree.World(docs)

	return list
}

// saveDefault saves a constant's default in the list it is declared in.
func (s *State) saveDefault(ctx *frame.Context, constant ids.ULID, list valuetree.List, value worldstate.Value) {
	if list.Values == nil {
		return
	}
	declared, found := list.Values.Value(constant)
	if !found {
		return
	}
	declared.Default = value.Encode()
	s.command(ctx, list.Document, list.Edit, "", stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{declared}, Remove: nil,
	}, ctx.Frame.Report("Saved "+declared.Name+"."), func() {})
}

// setOwnConstant sets the NPC's or zone's own value of a template's
// constant, or with none clears it.
func (s *State) setOwnConstant(ctx *frame.Context, row *constantRow, raw []byte, report string) {
	edit := npcplugin.SetConstantEdit
	if s.constants.scope == worldstate.ScopeZone {
		edit = zoneplugin.SetConstantEdit
	}
	s.command(ctx, s.constants.owner, edit, "", stateplugin.ConstantArgs{Constant: row.held.Declaration.ID, Value: raw},
		ctx.Frame.Report(report), func() {})
}

// openConstantSheet opens the writing sheet on the NPC's or zone's own
// value of a text constant, with the default beside it to switch to.
func (s *State) openConstantSheet(ctx *frame.Context, row *constantRow) {
	panel := s.constants
	declared := row.held.Declaration
	holders := []writesheet.Holder{{Key: "", Label: "Default", Text: declared.Zero().Text}}
	chosen := 0
	if !row.mine {
		holders = append(holders, writesheet.Holder{Key: panel.owner.String(), Label: panel.name + "'s own",
			Text: row.value.Text})
		chosen = 1
	}
	panel.sheet = writesheet.New(s.theme, row.held.Path, holders, chosen, "")
	panel.sheetOf, panel.sheetIn = declared.ID, valueList(ctx.Docs, panel.owner, row.mine)
}

// updateConstantSheet gives a frame to the writing sheet, saving what it
// hands back, and closes it when it is done.
func (s *State) updateConstantSheet(input *ui.Input, ctx *frame.Context) {
	panel := s.constants
	saving, open := panel.sheet.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, ctx.Clipboard)
	if saving != nil {
		value := worldstate.NoValue()
		value.Type, value.Text = worldstate.TypeText, saving.Text
		if saving.Key == "" {
			s.saveDefault(ctx, panel.sheetOf, valueList(ctx.Docs, panel.sheetIn.Document, panel.sheetIn.Document !=
				worldstate.ID()), value)
		} else if row, found := panel.byID[panel.sheetOf]; found {
			s.setOwnConstant(ctx, row, value.Encode(), "Saved "+panel.name+"'s "+row.held.Declaration.Name+".")
		}
	}
	if !open {
		panel.sheet = nil
	}
}

// drawConstants draws the Constants section: each constant's name, whose
// value it is and the start of it, with its buttons.
func (s *State) drawConstants(screen *ebiten.Image, left int) {
	panel := s.constants
	if len(panel.rows) == 0 || panel.open.Rect.Empty() {
		return
	}
	s.theme.Heading(screen, "CONSTANTS", left, panel.top)
	for _, row := range panel.rows {
		if row.rect.Empty() {
			continue
		}
		whose, ink := "default", ui.ColorMuted()
		switch {
		case row.mine:
			whose, ink = "its own", ui.ColorInk()
		case row.own:
			whose, ink = "own", ui.ColorInk()
		}
		name := row.held.Declaration.Name + " · " + whose
		s.theme.DrawStrong(screen, s.theme.FitText(name, row.rect.Dx()), left, row.rect.Min.Y, ui.ColorInk())
		if row.held.Declaration.Type == worldstate.TypeInteger {
			row.number.Draw(screen)
		} else {
			room := row.write.Rect.Min.X - left - ui.Gap
			if !row.clear.Rect.Empty() {
				room = row.clear.Rect.Min.X - left - ui.Gap - constantButtonW - ui.Gap
			}
			s.theme.DrawTextIn(screen, s.theme.FitText(textPreview(row.value.Text), room),
				image.Rect(left, row.write.Rect.Min.Y, left+room, row.write.Rect.Max.Y), ink)
		}
		row.write.Draw(screen, s.theme)
		row.clear.Draw(screen, s.theme)
	}
	if panel.more > 0 {
		s.theme.DrawText(screen, "and "+strconv.Itoa(panel.more)+" more", panel.open.Rect.Max.X+ui.Gap,
			panel.open.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half, ui.ColorMuted())
	}
	panel.open.Draw(screen, s.theme)
}

// textPreview is a text's first line, cut short, in quotes.
func textPreview(text string) string {
	first, _, more := strings.Cut(text, "\n")
	if runes := []rune(first); len(runes) > previewLength {
		first, more = string(runes[:previewLength]), true
	}
	if more {
		first = strings.TrimSpace(first) + "…"
	}

	return "“" + first + "”"
}

// locateConstant finds a part of the Constants section, or of the writing
// sheet while it is open.
func (s *State) locateConstant(name string) (image.Point, bool) {
	panel := s.constants
	if panel.sheet != nil {
		return panel.sheet.Locate(name)
	}
	widget, isWidget := strings.CutPrefix(name, "ui:")
	if !isWidget {
		return image.Pt(0, 0), false
	}
	if widget == openConstantsID && !panel.open.Rect.Empty() {
		return ui.Center(panel.open.Rect), true
	}
	for _, row := range panel.rows {
		for _, button := range []*ui.Button{row.write, row.clear} {
			if button.ID == widget && !button.Rect.Empty() {
				return ui.Center(button.Rect), true
			}
		}
		if row.number.ID == widget && !row.number.Rect.Empty() {
			return row.number.Center(), true
		}
	}

	return image.Pt(0, 0), false
}

// constantState answers "constant:<path>=<value>", a constant as the
// selected NPC or zone has it, and the writing sheet's states.
func (s *State) constantState(state string) (bool, bool) {
	panel := s.constants
	if state == "sheet" {
		return panel.sheet != nil, true
	}
	if panel.sheet != nil {
		if found, handled := panel.sheet.HasState(state); handled {
			return found, true
		}
	}
	asked, isConstant := strings.CutPrefix(state, constantStatePre)
	if !isConstant {
		return false, false
	}
	path, want, _ := strings.Cut(asked, "=")
	for _, row := range panel.rows {
		if row.held.Path == path {
			return row.held.Declaration.Label(row.value) == want, true
		}
	}

	return false, true
}
