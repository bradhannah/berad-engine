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
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The Constants window's own parts, in screen pixels and for input
// scripts: an NPC's or zone's own value's field, as tall as a few lines of
// text, and Use the default; and how much of a text a row shows.
const (
	heldTextHeight = 160
	heldTextID     = "held-value"
	useDefaultID   = "use-default"
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
	kind       worldstate.Type
}

func newHeldConstant(theme *ui.Theme) heldConstant {
	return heldConstant{
		of:         "",
		text:       ui.NewTextField(heldTextID, textedit.NewField("", true, worldstate.MaxConstantText), true, theme),
		number:     ui.NewTextField(heldTextID, textedit.NewField("", false, maxNumberText), false, theme),
		useDefault: ui.NewButton(useDefaultID, "Use the default"),
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
// first line, cut short, in quotes.
func preview(declared worldstate.Declaration, value worldstate.Value) string {
	if declared.Type != worldstate.TypeText {
		return declared.Label(value)
	}
	first, _, more := strings.Cut(value.Text, "\n")
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
	s.held.useDefault.Rect = image.Rect(left, top, left+min(width, s.theme.TextWidth(s.held.useDefault.Label)+
		ui.Half*ui.Inset), top+ui.ButtonH)
	bottom := panel.Max.Y - ui.Inset
	s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
}

// updateHeld handles Use the default, which clears the NPC's or zone's own
// value.
func (s *State) updateHeld(input *ui.Input, ctx *frame.Context, selected item) {
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
	for index, line := range s.theme.Wrap(about, width) {
		s.theme.DrawText(screen, line, left, under+index*ui.LineHeight, ui.ColorMuted())
	}
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
	s.theme.Heading(screen, "ON "+strconv.Itoa(len(holders))+" "+strings.ToUpper(plural(selected.scope))+
		", WITH THE "+strings.ToUpper(declared.Name)+" TEMPLATE", left, top)
	top += ui.LineHeight
	for index, holder := range holders {
		if index == holdersShown {
			s.theme.DrawText(screen, "and "+strconv.Itoa(len(holders)-holdersShown)+" more", left, top, ui.ColorMuted())

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
