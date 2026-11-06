package stateview

import (
	"encoding/json"
	"image"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/editor/views/writesheet"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/show"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The Constants window's own parts, in screen pixels and for input
// scripts: an NPC's or zone's own value's field, as tall as a few lines of
// text, and Use the default; and how much of a text a row shows.
const (
	heldTextHeight = 160
	heldTextID     = "held-value"
	useDefaultID   = "use-default"
	writeID        = "write"
	holdersShown   = 8
	previewRunes   = 32
	quoteOpen      = "“"
	quoteClose     = "”"
)

// heldConstant is an NPC's or zone's own value of one of its templates'
// constants, as the Constants window edits it: typed in its field, a text
// or a number, and cleared by Use the default. of is the row it shows.
type heldConstant struct {
	of         string
	text       *ui.TextField
	number     *ui.TextField
	useDefault *ui.Button
	write      *ui.Button
	kind       worldstate.Type
}

func newHeldConstant(theme *ui.Theme) heldConstant {
	return heldConstant{
		of:         "",
		text:       ui.NewTextField(heldTextID, textedit.NewField("", true, worldstate.MaxConstantText), true, theme),
		number:     ui.NewTextField(heldTextID, textedit.NewField("", false, maxNumberText), false, theme),
		useDefault: ui.NewButton(useDefaultID, "Use the default"),
		write:      ui.NewButton(writeID, "Write…"),
		kind:       worldstate.TypeText,
	}
}

// maxNumberText bounds a number typed, in characters.
const maxNumberText = 20

// field is the field the value is typed in.
func (h *heldConstant) field() *ui.TextField {
	if h.kind == worldstate.TypeInteger {
		return h.number
	}

	return h.text
}

// revert drops what was typed.
func (h *heldConstant) revert() {
	h.text.Field.Revert()
	h.number.Field.Revert()
}

// shows reports whether the window's part lists a declaration: groups and
// templates in both, values in Values and constants in Constants.
func (s *State) shows(declared worldstate.Declaration) bool {
	switch {
	case !declared.Type.IsValue():
		return true
	case s.part == PartConstants:
		return declared.IsConstant()
	}

	return declared.IsValue()
}

// shownChildren counts what a group or template holds that the part lists.
func (s *State) shownChildren(list *worldstate.Declarations, scope worldstate.Scope, parent ids.ULID) int {
	count := 0
	for _, child := range list.Children(scope, parent) {
		if s.shows(child) {
			count++
		}
	}

	return count
}

// valueTypes are the types Add value, or Add constant, offers in a scope.
func (s *State) valueTypes(scope worldstate.Scope) []worldstate.Type {
	if s.part == PartConstants {
		return worldstate.ConstantTypes()
	}

	return worldstate.Types(scope)
}

// The words for what the parts' rows hold.
const (
	wordValue    = "value"
	wordConstant = "constant"
)

// valueWord names what the part's rows hold: a value or a constant.
func (s *State) valueWord() string {
	if s.part == PartConstants {
		return wordConstant
	}

	return wordValue
}

// declaredName names a declaration for a menu: a value, a constant, a group
// or a template.
func (s *State) declaredName(declared worldstate.Declaration) string {
	if declared.IsConstant() {
		return wordConstant
	}

	return kindName(declared.Type)
}

// isHeldConstant reports whether a row is a template's constant as an NPC or
// zone holds it, which the Constants window edits.
func (s *State) isHeldConstant(selected item) bool {
	return s.part == PartConstants && selected.kind == itemHeld && selected.declared.IsConstant()
}

// ownValue returns an NPC's or zone's value of a constant, and whether it
// is its own rather than the default.
func ownValue(declared worldstate.Declaration, owner worldstate.Owner) (worldstate.Value, bool) {
	if value, own := owner.Owned.ConstantValue(declared); own {
		return value, true
	}

	return declared.Zero(), false
}

// preview shows a constant's value in a row: a number as it is, a text's
// first line as it reads, without its markup, cut short, in quotes.
func preview(declared worldstate.Declaration, value worldstate.Value) string {
	if declared.Type != worldstate.TypeText {
		return declared.Label(value)
	}
	first, _, more := strings.Cut(show.Plain(value.Text), "\n")
	if utf8.RuneCountInString(first) > previewRunes {
		first, more = string([]rune(first)[:previewRunes]), true
	}
	if more {
		first = strings.TrimSpace(first) + "…"
	}

	return quoteOpen + first + quoteClose
}

// constantDetail says what a constant is: its type and the start of its
// default, and for a template's, how many hold their own value.
func constantDetail(declared worldstate.Declaration, catalog *worldstate.Catalog) string {
	written := declared.Type.Label() + " · " + preview(declared, declared.Zero())
	if own := ownCount(declared, catalog); own > 0 {
		written += " · " + strconv.Itoa(own) + " own"
	}

	return written
}

// ownCount counts the NPCs or zones holding their own value of a template's
// constant.
func ownCount(declared worldstate.Declaration, catalog *worldstate.Catalog) int {
	if catalog == nil {
		return 0
	}
	template, inTemplate := catalog.World().Template(declared.ID)
	if !inTemplate {
		return 0
	}
	count := 0
	for _, holder := range catalog.Holders(template) {
		owner, _ := catalog.Owner(holder)
		if _, own := owner.Owned.ConstantValue(declared); own {
			count++
		}
	}

	return count
}

// heldDetail says what an NPC's or zone's value of a constant is, and
// whether it is its own or the default.
func heldDetail(declared worldstate.Declaration, owner worldstate.Owner) string {
	value, own := ownValue(declared, owner)
	whose := "default"
	if own {
		whose = "own"
	}

	return declared.Type.Label() + " · " + preview(declared, value) + " · " + whose
}

// inspectHeld shows an NPC's or zone's value of a constant in its field,
// keeping what is typed while the row stays selected.
func (s *State) inspectHeld(selected item) {
	if !s.isHeldConstant(selected) {
		s.held.of = ""

		return
	}
	if s.held.of != s.list.Selected {
		s.held.revert()
		s.held.kind = selected.declared.Type
		s.focus = ui.Focus{}
		s.focus.Register(s.held.field())
		s.held.of = s.list.Selected
	}
	value, own := ownValue(selected.declared, selected.owner)
	if selected.declared.Type == worldstate.TypeInteger {
		s.held.number.Field.SetSaved(strconv.Itoa(value.Integer))
	} else {
		s.held.text.Field.SetSaved(value.Text)
	}
	s.held.useDefault.Disabled = !own
}

// layoutHeld places the value's field, Use the default and Save.
func (s *State) layoutHeld(panel image.Rectangle) {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	top := inspectorTop(panel) + ui.LineHeight
	height := ui.ButtonH
	if s.held.kind == worldstate.TypeText {
		height = heldTextHeight
	}
	s.held.text.Rect, s.held.number.Rect = ui.Zero(), ui.Zero()
	s.held.field().Rect = image.Rect(left, top, left+width, top+height)
	top += height + ui.Gap
	s.held.write.Rect = ui.Zero()
	from := left
	if s.held.kind == worldstate.TypeText {
		writeWidth := s.theme.TextWidth(s.held.write.Label) + ui.Half*ui.Inset
		s.held.write.Rect = image.Rect(left, top, left+writeWidth, top+ui.ButtonH)
		from += writeWidth + ui.Gap
	}
	s.held.useDefault.Rect = image.Rect(from, top, from+min(width, s.theme.TextWidth(s.held.useDefault.Label)+
		ui.Half*ui.Inset), top+ui.ButtonH)
	bottom := panel.Max.Y - ui.Inset
	s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
}

// updateHeld handles Use the default, which clears the NPC's or zone's own
// value.
func (s *State) updateHeld(input *ui.Input, ctx *frame.Context, selected item) {
	if s.held.write.Update(input) {
		s.openSheet(ctx, selected)

		return
	}
	if !s.held.useDefault.Update(input) {
		return
	}
	s.held.revert()
	s.setOwnConstant(ctx, selected, nil, selected.owner.Name+"'s "+selected.declared.Name+" is the default again. "+
		"Undo brings back its own.")
}

// saveHeld saves the value typed as the NPC's or zone's own.
func (s *State) saveHeld(ctx *frame.Context) bool {
	selected, found := s.shown.items[s.held.of]
	if !found {
		return true
	}
	field := s.held.field()
	value := worldstate.NoValue()
	value.Type = selected.declared.Type
	if value.Type == worldstate.TypeInteger {
		number, err := strconv.Atoi(strings.TrimSpace(field.Field.Value()))
		if err != nil || number > worldstate.MaxInteger || number < -worldstate.MaxInteger {
			s.focus.Set(field)
			ctx.Frame.Status("Enter a whole number")

			return false
		}
		value.Integer = number
	} else {
		value.Text = field.Field.Value()
	}
	if !selected.declared.Fits(value) {
		s.focus.Set(field)
		ctx.Frame.Status("That does not fit " + selected.declared.Name)

		return false
	}
	field.Field.Commit()
	s.setOwnConstant(ctx, selected, value.Encode(), "Set "+selected.owner.Name+"'s "+selected.declared.Name+".")

	return true
}

// setOwnConstant sets an NPC's or zone's own value of a constant, or with
// none clears it.
func (s *State) setOwnConstant(ctx *frame.Context, selected item, raw json.RawMessage, report string) {
	edit := npcplugin.SetConstantEdit
	if selected.scope == worldstate.ScopeZone {
		edit = zoneplugin.SetConstantEdit
	}
	s.send(ctx, selected.ownerID, edit, stateplugin.ConstantArgs{Constant: selected.declared.ID, Value: raw},
		func(result protocol.Result, err error) { ctx.Frame.Report(report)(result, err) })
}

// drawHeld draws the NPC's or zone's value, Use the default, and what the
// default is.
func (s *State) drawHeld(screen *ebiten.Image, selected item, left, top, width int) {
	s.theme.Heading(screen, "VALUE", left, top)
	s.held.field().Draw(screen)
	s.held.useDefault.Draw(screen, s.theme)
	s.held.write.Draw(screen, s.theme)
	template := "its template"
	if id, inTemplate := s.shown.catalog.World().Template(selected.declared.ID); inTemplate {
		if declared, found := s.shown.catalog.World().Value(id); found {
			template = "the " + declared.Name + " template"
		}
	}
	about := "Its own value. The default, from " + template + ", is " +
		preview(selected.declared, selected.declared.Zero()) + "."
	if _, own := ownValue(selected.declared, selected.owner); !own {
		about = "It has the default, from " + template + ". Type its own value here."
	}
	under := s.held.useDefault.Rect.Max.Y + ui.Gap
	lines := s.theme.Wrap(about, width)
	for index, line := range lines {
		s.theme.DrawText(screen, line, left, under+index*ui.LineHeight, ui.ColorMuted())
	}
	value, _ := ownValue(selected.declared, selected.owner)
	s.drawMarkupWarning(screen, selected.declared, value, left, under+len(lines)*ui.LineHeight+ui.Gap, width)
	s.save.Draw(screen, s.theme)
}

// drawHolders lists who holds a template's constant, under its uses: each
// NPC or zone the template is on, with its own value or the default. A
// click on one selects it in the tree.
func (s *State) drawHolders(screen *ebiten.Image, selected item, left, top, width int) {
	s.holders = map[string]image.Rectangle{}
	catalog := s.shown.catalog
	template, inTemplate := catalog.World().Template(selected.declared.ID)
	if !inTemplate || !selected.declared.IsConstant() {
		return
	}
	holders := catalog.Holders(template)
	declared, _ := catalog.World().Value(template)
	shown := s.holdersRoom(top, len(holders))
	if shown < 0 {
		return
	}
	s.theme.Heading(screen, "ON "+strconv.Itoa(len(holders))+" "+strings.ToUpper(plural(selected.scope))+
		", WITH THE "+strings.ToUpper(declared.Name)+" TEMPLATE", left, top)
	top += ui.LineHeight
	for index, holder := range holders {
		if index == shown {
			s.theme.DrawText(screen, "and "+strconv.Itoa(len(holders)-shown)+" more", left, top, ui.ColorMuted())

			break
		}
		owner, _ := catalog.Owner(holder)
		value, own := ownValue(selected.declared, owner)
		line, colour := owner.Name+" · own · "+preview(selected.declared, value), ui.ColorInk()
		if !own {
			line, colour = owner.Name+" · default", ui.ColorMuted()
		}
		s.theme.DrawText(screen, s.theme.FitText(line, width), left, top, colour)
		s.holders[heldPrefix+holder.String()+":"+selected.declared.ID.String()] = image.Rect(left, top, left+width,
			top+ui.LineHeight)
		top += ui.LineHeight
	}
}

// holdersRoom is how many of a constant's holders the list shows from top,
// above the save bar, keeping a line for how many more: at most
// holdersShown, or -1 when not even its heading fits.
func (s *State) holdersRoom(top, holders int) int {
	room := holdersShown + 1
	if !s.save.Rect.Empty() {
		room = (s.save.Rect.Min.Y-ui.Gap-top)/ui.LineHeight - 1
	}
	if room < 1 && holders > 0 {
		return -1
	}
	if holders <= min(room, holdersShown) {
		return holders
	}

	return max(min(room-1, holdersShown), 0)
}

// updateHolders selects an NPC's or zone's value of a constant clicked in
// the list of who holds it, opening its row in the tree.
func (s *State) updateHolders(input *ui.Input) {
	if !input.Pressed {
		return
	}
	for row, rect := range s.holders {
		if !input.Cursor.In(rect) {
			continue
		}
		input.ClearPointer()
		owner, _, _ := strings.Cut(strings.TrimPrefix(row, heldPrefix), ":")
		s.toggled[ownerPrefix+owner] = true
		s.list.Selected = row

		return
	}
}

// words are what Find matches in a text constant besides its path: its
// default and every NPC's or zone's own value of it, so a line is found by
// what it says.
func words(declared worldstate.Declaration, catalog *worldstate.Catalog) string {
	if !declared.IsConstant() || declared.Type != worldstate.TypeText {
		return ""
	}
	said := []string{declared.Zero().Text}
	if template, inTemplate := catalog.World().Template(declared.ID); inTemplate {
		for _, holder := range catalog.Holders(template) {
			owner, _ := catalog.Owner(holder)
			if value, own := owner.Owned.ConstantValue(declared); own {
				said = append(said, value.Text)
			}
		}
	}

	return strings.Join(said, "\n")
}

// openSheet opens the writing sheet for a text constant: its default, and
// for a template's, each NPC's or zone's own value, starting with the one
// selected. What is typed in the inspector is saved first.
func (s *State) openSheet(ctx *frame.Context, selected item) {
	declared := selected.declared
	if !declared.IsConstant() || declared.Type != worldstate.TypeText || (s.Unsaved(ctx.Docs) > 0 && !s.Save(ctx)) {
		return
	}
	list := selected.list
	if selected.kind == itemHeld {
		world, _ := valuetree.World(ctx.Docs)
		list = world
	}
	catalog := s.shown.catalog
	holders := []writesheet.Holder{{Key: "", Label: "Default", Text: declared.Zero().Text}}
	chosen := 0
	if template, inTemplate := catalog.World().Template(declared.ID); inTemplate && list.Document == worldstate.ID() {
		for _, holder := range catalog.Holders(template) {
			owner, _ := catalog.Owner(holder)
			value, _ := ownValue(declared, owner)
			holders = append(holders, writesheet.Holder{Key: holder.String(), Label: owner.Name + "'s own",
				Text: value.Text})
			if selected.kind == itemHeld && selected.ownerID == holder {
				chosen = len(holders) - 1
			}
		}
	}
	said := ""
	if s.usesOf == s.list.Selected && len(s.uses) > 0 {
		said = "used in " + s.uses[0].Name
	}
	s.sheet = writesheet.New(s.theme, list.Values.Path(declared.ID), holders, chosen, said)
	s.sheetOf, s.sheetIn = declared.ID, list
}

// updateSheet gives a frame to the writing sheet, saving what it hands
// back, and closes it when it is done.
func (s *State) updateSheet(input *ui.Input, ctx *frame.Context) {
	if !s.syncSheet(ctx.Docs) {
		s.sheet = nil

		return
	}
	saving, open := s.sheet.Update(input, s.screen(), ctx.Clipboard)
	if saving != nil {
		s.saveSheet(ctx, *saving)
	}
	if !open {
		s.sheet = nil
	}
}

// saveSheet saves text written in the sheet: the constant's default, or an
// NPC's or zone's own value.
func (s *State) saveSheet(ctx *frame.Context, saving writesheet.Saving) {
	declared, found := s.sheetIn.Values.Value(s.sheetOf)
	if !found {
		return
	}
	value := worldstate.NoValue()
	value.Type, value.Text = worldstate.TypeText, saving.Text
	if saving.Key == "" {
		declared.Default = value.Encode()
		s.change(ctx, s.sheetIn, setOnly(declared), "Saved "+declared.Name+"'s default.")

		return
	}
	owner, err := ids.Parse(saving.Key)
	if err != nil {
		return
	}
	held, known := s.shown.catalog.Owner(owner)
	if !known {
		return
	}
	s.setOwnConstant(ctx, item{
		kind: itemHeld, scope: held.Scope, list: s.sheetIn, declared: declared, owner: held, ownerID: owner, path: "",
	}, value.Encode(), "Saved "+held.Name+"'s "+declared.Name+".")
}

// syncSheet gives the sheet what is saved now, as documents change, and
// reports whether the constant it writes is still declared.
func (s *State) syncSheet(docs frame.Docs) bool {
	list, loaded := valuetree.Owned(docs, s.sheetIn.Document)
	if s.sheetIn.Document == worldstate.ID() {
		list, loaded = valuetree.World(docs)
	}
	if !loaded {
		return false
	}
	declared, found := list.Values.Value(s.sheetOf)
	if !found {
		return false
	}
	s.sheetIn = list
	s.sheet.SetSaved("", declared.Zero().Text)
	catalog := s.shown.catalog
	if template, inTemplate := catalog.World().Template(declared.ID); inTemplate && list.Document == worldstate.ID() {
		for _, holder := range catalog.Holders(template) {
			owner, _ := catalog.Owner(holder)
			value, _ := ownValue(declared, owner)
			s.sheet.SetSaved(holder.String(), value.Text)
		}
	}

	return true
}

// sheetState answers "sheet", whether the writing sheet is open, and the
// sheet's own states while it is.
func (s *State) sheetState(state string) (bool, bool) {
	if state == "sheet" {
		return s.sheet != nil, true
	}
	if state == "markup-warning" {
		return s.markupWarning(), true
	}
	if s.sheet == nil {
		return false, false
	}

	return s.sheet.HasState(state)
}

// drawMarkupWarning warns of a tag in a text constant's value left open or
// not known, and returns where the next part goes.
func (s *State) drawMarkupWarning(screen *ebiten.Image, declared worldstate.Declaration, value worldstate.Value,
	left, top, width int,
) int {
	if !declared.IsConstant() || declared.Type != worldstate.TypeText {
		return top
	}
	problems := show.Problems(value.Text)
	if len(problems) == 0 {
		return top
	}
	for _, line := range s.theme.Wrap("Its markup: "+problems[0]+". It shows as typed.", width) {
		s.theme.DrawText(screen, line, left, top, ui.ColorWarn())
		top += ui.LineHeight
	}

	return top
}

// markupWarning answers "markup-warning", whether the selected text
// constant's value has a tag left open or not known.
func (s *State) markupWarning() bool {
	selected, found := s.selected()
	if !found || !selected.declared.IsConstant() || selected.declared.Type != worldstate.TypeText {
		return false
	}
	value := selected.declared.Zero()
	if selected.kind == itemHeld {
		value, _ = ownValue(selected.declared, selected.owner)
	}

	return len(show.Problems(value.Text)) > 0
}
