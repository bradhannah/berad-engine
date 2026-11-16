package stateview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valueform"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Inspector layout, in screen pixels.
const (
	templatesPerRow = 2
	noteLines       = 3
)

// inspect shows the selected row in the inspector: a declaration in the
// form, or an NPC's or zone's templates to apply.
func (s *State) inspect(selected item) {
	if selected.kind != itemDeclared && s.named != "" {
		s.focus = s.newFocus()
	}
	s.named = ""
	s.inspectPath(selected)
	s.inspectHeld(selected)
	if selected.kind == itemDeclared {
		if s.form.Show(selected.declared, valuetree.Dotted(selected.list.Values, selected.declared.ID)) {
			s.focus = s.newFocus()
			s.focus.Register(s.form.Fields()...)
		}
		s.named = s.list.Selected
		s.form.Locked = s.lockReason()
	}
	if selected.kind != itemOwner {
		s.templates = nil

		return
	}
	var names []string
	for _, template := range s.templatesOf(selected.scope) {
		names = append(names, template.Name)
	}
	if len(names) != len(s.templates) || !slices.EqualFunc(names, s.templates, func(name string, button *ui.Button) bool {
		return templateID+name == button.ID
	}) {
		s.templates = nil
		for _, name := range names {
			s.templates = append(s.templates, ui.NewButton(templateID+name, name))
		}
	}
	for index, template := range s.templatesOf(selected.scope) {
		s.templates[index].On = slices.Contains(selected.owner.Owned.Templates, template.ID)
	}
}

// inspectPath shows the selected NPC's or zone's path name in its field,
// keeping what is typed while it stays selected.
func (s *State) inspectPath(selected item) {
	if selected.kind != itemOwner {
		s.pathOf = ""

		return
	}
	if s.pathOf != s.list.Selected {
		s.path.Field.Revert()
		s.focus = s.newFocus()
		s.focus.Register(s.path)
		s.pathOf = s.list.Selected
	}
	s.path.Field.SetSaved(selected.owner.Path)
}

// templatesOf lists a scope's templates, in name order.
func (s *State) templatesOf(scope worldstate.Scope) []worldstate.Declaration {
	var templates []worldstate.Declaration
	for _, declared := range s.shown.catalog.World().Children(scope, ids.ULID{}) {
		if declared.Type == worldstate.TypeTemplate {
			templates = append(templates, declared)
		}
	}

	return templates
}

// inspectorTop is where the inspector's parts start, under its heading and
// the selected row's path.
func inspectorTop(panel image.Rectangle) int {
	return panel.Min.Y + ui.Inset + ui.Half*ui.LineHeight + ui.Gap
}

// layoutInspector places the inspector's parts for the selected row: the
// save bar and Delete at its foot, then its fields from the top down,
// scrolled between its heading and the foot (increment 61).
func (s *State) layoutInspector(panel image.Rectangle) {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	s.form.Hide()
	s.save.Rect, s.takeOff.Rect, s.path.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	s.signal.name.Rect, s.signal.description.Rect, s.signal.remove.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	s.held.text.Rect, s.held.number.Rect, s.held.useDefault.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	for _, button := range s.templates {
		button.Rect = ui.Zero()
	}
	s.scroll.For(s.list.Selected)
	s.scroll.Body = ui.Zero()
	selected, found := s.selected()
	if !found {
		return
	}
	s.scroll.Body = image.Rect(panel.Min.X, inspectorTop(panel), panel.Max.X,
		max(inspectorTop(panel), s.layoutFoot(selected, left, width, panel.Max.Y-ui.Inset)))
	top := inspectorTop(panel) - s.scroll.Offset
	switch {
	case s.isHeldConstant(selected):
		s.layoutHeld(left, width, top)
	case selected.kind == itemDeclared:
		s.formBottom = s.form.Layout(left, top, width)
	case selected.kind == itemOwner:
		pathTop := top + ui.LineHeight
		s.path.Rect = image.Rect(left, pathTop, left+width, pathTop+ui.ButtonH)
		top := pathTop + ui.ButtonH + ui.Gap + noteLines*ui.LineHeight + ui.LineHeight
		gap := ui.Gap
		for index, button := range s.templates {
			column, row := index%templatesPerRow, index/templatesPerRow
			from := left + column*(width+gap)/templatesPerRow
			at := top + row*(ui.ButtonH+gap)
			button.Rect = image.Rect(from, at, from+(width-gap)/templatesPerRow, at+ui.ButtonH)
		}
	case selected.kind == itemApplied:
		top += noteLines * ui.LineHeight
		s.takeOff.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	case isSignal(selected.kind):
		s.layoutSignal(left, width, top)
	}
}

// layoutFoot places what stays at the inspector's foot for the selected
// row, from its bottom up: the save bar, and a value's or a signal's Delete.
// It returns where the fields above it must end.
func (s *State) layoutFoot(selected item, left, width, bottom int) int {
	switch {
	case s.isHeldConstant(selected), selected.kind == itemOwner:
		s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)

		return s.save.Rect.Min.Y - footerGap
	case selected.kind == itemDeclared:
		s.form.Delete.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
		saveTop := s.form.Delete.Rect.Min.Y - footerGap - ui.SaveBarHeight(width)
		s.save.Rect = image.Rect(left, saveTop, left+width, saveTop+ui.SaveBarHeight(width))

		return saveTop - footerGap
	case isSignal(selected.kind):
		s.signal.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)

		return s.signal.remove.Rect.Min.Y - footerGap
	}

	return bottom
}

// updateInspector handles the inspector's buttons for the selected row.
func (s *State) updateInspector(input *ui.Input, ctx *frame.Context) {
	selected, found := s.selected()
	if !found {
		return
	}
	switch selected.kind {
	case itemDeclared:
		s.updateHolders(input)
		s.updateDeclared(input, ctx, selected)
	case itemOwner:
		for index, template := range s.templatesOf(selected.scope) {
			if index < len(s.templates) && s.templates[index].Update(input) {
				s.toggleTemplate(ctx, selected, template)
			}
		}
	case itemApplied:
		if s.takeOff.Update(input) {
			s.toggleTemplate(ctx, selected, selected.declared)
		}
	case itemSignal, itemSignalGroup:
		if s.signal.remove.Update(input) {
			s.startSignalDelete(ctx, selected)
		}
	case itemHeld:
		if s.isHeldConstant(selected) {
			s.updateHeld(input, ctx, selected)
		}
	case itemTemplates, itemName:
	}
}

// askUses asks where the selected declaration is used, once per
// updateDeclared handles the selected declaration's form: a change to it,
// Delete, or declaring a new value beside it.
func (s *State) updateDeclared(input *ui.Input, ctx *frame.Context, selected item) {
	switch changed, change := s.form.Update(input); change {
	case valueform.ChangeValue:
		s.change(ctx, selected.list, setOnly(changed), "")
	case valueform.ChangeDelete:
		s.startDelete(ctx, selected)
	case valueform.ChangeDeclareNew:
		s.openFind(ctx, strings.TrimSuffix(selected.path, selected.declared.Name))
	case valueform.ChangeWrite:
		s.openSheet(ctx, selected)
	case valueform.ChangeNone, valueform.ChangePickPlace, valueform.ChangeChoosePlace:
	}
}

// declaration and change.
func (s *State) askUses(ctx *frame.Context) {
	selected, found := s.selected()
	if !found || selected.kind != itemDeclared || s.usesAsking || s.usesOf == s.list.Selected {
		return
	}
	asked := s.list.Selected
	s.usesAsking = true
	ctx.Requests.WhereUsed(worldstate.ID(), selected.declared.ID.String(), func(uses []protocol.Use, err error) {
		s.usesAsking = false
		if err == nil {
			s.uses, s.usesOf, s.usesFor = uses, asked, asked
		}
	})
}

// lockReason says why the selected value's type cannot change: something
// uses it, or where it is used is not known yet; "" leaves it free. The
// last answer for the row holds while the question is asked again after
// each change.
func (s *State) lockReason() string {
	switch {
	case s.usesFor != s.list.Selected:
		return "Its type waits while its uses are checked."
	case len(s.uses) == 1:
		return "Fixed while a node uses it. For another type, declare a new " + s.valueWord() + "."
	case len(s.uses) > 1:
		return "Fixed while " + strconv.Itoa(len(s.uses)) + " nodes use it. For another type, declare a new " +
			s.valueWord() + "."
	}

	return ""
}

// drawInspector draws what the selected row has to show.
func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs) {
	panel := s.panel()
	ui.Divider(screen, panel.Min, image.Pt(panel.Min.X, panel.Max.Y))
	if panel.Max.X < s.size.X {
		ui.Divider(screen, image.Pt(panel.Max.X, panel.Min.Y), image.Pt(panel.Max.X, panel.Max.Y))
	}
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	selected, found := s.selected()
	if !found {
		hint := "Select a " + s.valueWord() + ", a group, a template, an NPC or a zone."
		if s.part == PartSignals {
			hint = "Select a signal or a group."
		}
		s.theme.Heading(screen, "INSPECTOR", left, panel.Min.Y+ui.Inset)
		s.theme.DrawText(screen, s.theme.FitText(hint, width), left, panel.Min.Y+ui.Inset+ui.LineHeight,
			ui.ColorMuted())

		return
	}
	s.theme.Heading(screen, heading(selected), left, panel.Min.Y+ui.Inset)
	s.theme.DrawMonoStrong(screen, s.theme.FitText(selected.path, width), left, panel.Min.Y+ui.Inset+ui.LineHeight,
		ui.ColorInk())
	if body, ok := s.scroll.Clip(screen); ok {
		s.scroll.Fit(s.drawBody(body, docs, panel, selected))
	}
	s.save.Draw(screen, s.theme)
	if selected.kind == itemDeclared {
		s.form.Delete.Draw(screen, s.theme)
	}
	s.signal.remove.Draw(screen, s.theme)
	s.scroll.DrawThumb(screen)
}

// drawBody draws the inspector's fields, which scroll between its heading
// and its foot, and returns where they end.
func (s *State) drawBody(body *ebiten.Image, docs frame.Docs, panel image.Rectangle, selected item) int {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	top := inspectorTop(panel) - s.scroll.Offset
	switch selected.kind {
	case itemDeclared:
		s.form.Draw(body)
		under := s.drawUses(body, left, s.formBottom, width)
		under = s.drawMarkupWarning(body, selected.declared, selected.declared.Zero(), left, under, width)

		return s.drawHolders(body, selected, left, under+ui.Gap, width)
	case itemOwner:
		return s.drawOwner(body, selected, left, top, width)
	case itemHeld:
		if s.isHeldConstant(selected) {
			return s.drawHeld(body, selected, left, top, width)
		}

		return s.drawNote(body, note(selected), left, top, width)
	case itemApplied, itemTemplates, itemName:
		s.drawNote(body, note(selected), left, top, width)
		s.takeOff.Draw(body, s.theme)

		return max(s.takeOff.Rect.Max.Y, top+noteLines*ui.LineHeight)
	case itemSignal, itemSignalGroup:
		return s.drawSignal(body, docs, panel, selected.kind == itemSignalGroup)
	}

	return top
}

// heading names what the inspector shows.
func heading(selected item) string {
	switch selected.kind {
	case itemOwner:
		if selected.scope == worldstate.ScopeZone {
			return "ZONE"
		}

		return "NPC"
	case itemApplied:
		return "APPLIED TEMPLATE"
	case itemTemplates:
		return "TEMPLATES"
	case itemHeld:
		if selected.declared.IsConstant() {
			return strings.ToUpper(selected.owner.Name) + "'S OWN"
		}

		return "FROM A TEMPLATE"
	case itemName:
		return "NAME"
	case itemSignal:
		return "SIGNAL"
	case itemSignalGroup:
		return "SIGNAL GROUP"
	case itemDeclared:
	}
	if selected.declared.IsConstant() {
		return "SELECTED CONSTANT"
	}
	if selected.declared.Type.IsValue() {
		return "SELECTED VALUE"
	}

	return "SELECTED " + string(selected.declared.Type)
}

// note explains a row the inspector does not edit.
func note(selected item) string {
	switch selected.kind {
	case itemApplied:
		return "The " + selected.declared.Name + " template gives " + selected.owner.Name +
			" its values. Change them in the template; taking it off takes them away."
	case itemTemplates:
		return "A template is a set of values. Applied to many " + plural(selected.scope) +
			", it gives each its own copy. Add template makes one."
	case itemHeld:
		return "Given by a template, with " + selected.declared.Type.Label() + " values starting as " +
			selected.declared.Label(selected.declared.Zero()) + ". Change it in its template."
	case itemName:
		return "Text, starting as " + selected.owner.Name + ", its name in the project browser. " +
			"Workflows may set it, such as to a name the player learns."
	case itemDeclared, itemOwner, itemSignal, itemSignalGroup:
	}

	return ""
}

func plural(scope worldstate.Scope) string {
	if scope == worldstate.ScopeZone {
		return "zones"
	}

	return "NPCs"
}

// drawOwner draws an NPC's or zone's path name and its templates, and
// returns where they end.
func (s *State) drawOwner(screen *ebiten.Image, selected item, left, top, width int) int {
	s.theme.Heading(screen, "PATH NAME", left, top)
	s.path.Draw(screen)
	top = s.path.Rect.Max.Y + ui.Gap
	s.drawNote(screen, s.pathNote(selected), left, top, width)
	top += noteLines * ui.LineHeight
	s.theme.Heading(screen, "TEMPLATES", left, top)
	bottom := top + ui.Half*ui.LineHeight
	if len(s.templates) == 0 {
		s.theme.DrawText(screen, "No "+string(selected.scope)+" templates yet.", left, top+ui.LineHeight, ui.ColorMuted())
	}
	for _, button := range s.templates {
		button.Draw(screen, s.theme)
		bottom = max(bottom, button.Rect.Max.Y+ui.Gap)
	}

	return bottom
}

// drawNote draws a few lines of explanation, and returns where they end.
func (s *State) drawNote(screen *ebiten.Image, text string, left, top, width int) int {
	lines := s.theme.Wrap(text, width)
	for index, line := range lines {
		if index == noteLines {
			break
		}
		s.theme.DrawText(screen, line, left, top+index*ui.LineHeight, ui.ColorMuted())
	}

	return top + min(len(lines), noteLines)*ui.LineHeight
}

// drawUses lists where the selected declaration is used.
func (s *State) drawUses(screen *ebiten.Image, left, top, width int) int {
	s.theme.Heading(screen, "USED IN", left, top)
	top += ui.LineHeight
	switch {
	case s.usesOf != s.list.Selected:
		s.theme.DrawText(screen, "…", left, top, ui.ColorMuted())
	case len(s.uses) == 0:
		s.theme.DrawText(screen, "Nothing uses it yet.", left, top, ui.ColorMuted())
	default:
		for index, use := range s.uses {
			if index == usesShown {
				s.theme.DrawText(screen, "and "+strconv.Itoa(len(s.uses)-usesShown)+" more", left, top, ui.ColorMuted())

				break
			}
			s.theme.DrawText(screen, s.theme.FitText(use.Name+" · "+use.Label, width), left, top, ui.ColorInk())
			top += ui.LineHeight
		}
	}

	return top + ui.LineHeight
}

// pathNote says what is worth knowing about an NPC's or zone's path name:
// how workflows name its values, that another has it too, or a suggestion
// while it has none.
func (s *State) pathNote(selected item) string {
	catalog, owner := s.shown.catalog, selected.owner
	taken := map[string]bool{}
	for _, other := range catalog.Owners(selected.scope) {
		held, _ := catalog.Owner(other)
		if other != selected.ownerID && held.Path != "" {
			if held.Path == owner.Path {
				return "Also the path name of " + held.Name + ". Give each its own."
			}
			taken[held.Path] = true
		}
	}
	if owner.Path == "" {
		return "None yet, so workflows cannot name it. Suggested: " +
			worldstate.UniqueName(worldstate.SuggestName(owner.Name, string(selected.scope)), taken)
	}

	return "Workflows name its values as " + string(selected.scope) + "." + owner.Path + ".… from elsewhere."
}
