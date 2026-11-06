package browser

import (
	"image"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The bar (increment 32): over the main pane, the search and a dropdown for
// each facet, each value with how many things ticking it would list. A
// search or a tick narrows the places column to the places holding what is
// kept, and Matches lists it all; nothing swaps what a list is.

// Sizes of the bar and its dropdowns, in pixels.
const (
	barHeight     = 40
	searchWidth   = 220
	facetPad      = 26
	dropdownWidth = 240
	dropdownRows  = 12
	namingWidth   = 200
	saveWidth     = 128
	keepWidth     = 64
	deleteWidth   = 104
	clearWidth    = 64
	// facetBnPrefix starts a facet's button, as "facet-zone".
	facetBnPrefix = "facet-"
)

// barFacets are the facets the bar offers. Folder is left out, as the
// places column is the folders.
func barFacets() []facetDef {
	return slices.DeleteFunc(facetDefs(), func(def facetDef) bool { return def.id == facetFolder })
}

// newFacetButtons returns a button for each facet in the bar.
func newFacetButtons() map[string]*ui.Button {
	buttons := map[string]*ui.Button{}
	for _, def := range barFacets() {
		button := ui.NewButton(facetBnPrefix+def.id, def.label)
		button.Caret = true
		buttons[def.id] = button
	}

	return buttons
}

// facetButtons lists the bar's facet buttons in order.
func (s *State) facetButtons() []*ui.Button {
	buttons := make([]*ui.Button, 0, len(s.facetBns))
	for _, def := range barFacets() {
		buttons = append(buttons, s.facetBns[def.id])
	}

	return buttons
}

// narrowed reports whether something is typed in the search or ticked in a
// facet.
func (s *State) narrowed() bool { return newMatcher(s.search.Field.Value()).active() || s.ticked() }

// layoutPanes places the places column, the bar and the main pane beside
// it, and an open facet's dropdown.
func (s *State) layoutPanes(docs frame.Docs, content image.Rectangle) {
	things, labels := s.things(docs)
	index := s.index(docs)
	counted := s.countPlaces(docs, things)
	s.shown, s.total = counted.kept[placeMatches], 0
	for _, found := range things {
		if !found.extra {
			s.total++
		}
	}
	left := s.layoutPlaces(docs, index, counted, content)
	bar := image.Rect(left, toolbarHeight, content.Max.X, toolbarHeight+barHeight)
	s.barBottom = s.layoutBar(docs, bar, labels)
	headBottom := s.layoutHead(s.barBottom, content.Max.X)
	s.list.Rect = image.Rect(left, headBottom, content.Max.X, content.Max.Y)
	byID := make(map[string]thing, len(things))
	for _, found := range things {
		byID[found.row.ID] = found
	}
	s.list.Rows = s.arrangeRows(docs, index, s.contentRows(docs, index, things, counted), byID, s.list.Rect.Dx())
	if len(s.columns) > 0 {
		s.list.Rect.Min.Y += headerHeight
	}
	switch s.viewShown() {
	case viewCards:
		s.grid.layout(s.list.Rows, s.list.Rect, cardTile())
	case viewPictures:
		s.grid.layout(s.list.Rows, s.list.Rect, s.pictureTile())
	case viewPreview:
		s.layoutPreview(docs, s.list.Rect)
	}
	s.layoutDropdown(things, labels)
}

// layoutBar places the search, a button for each facet, and at the right,
// while narrowed, Clear and Save as a view…, the name of a view being
// saved, or Delete view when the ticks are a saved view's. The facets go on
// a second line when they do not fit beside the search; it returns where
// the bar ends.
func (s *State) layoutBar(docs frame.Docs, bar image.Rectangle, labels facetLabels) int {
	top := bar.Min.Y + (barHeight-ui.ButtonH)/ui.Half
	right := bar.Max.X - ui.Inset
	place := func(width int) image.Rectangle {
		rect := image.Rect(right-width, top, right, top+ui.ButtonH)
		right = rect.Min.X - ui.Gap

		return rect
	}
	s.saveViewBn.Rect, s.keepViewBn.Rect, s.deleteViewBn.Rect, s.viewName.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	s.clearBn.Rect = ui.Zero()
	if s.narrowed() {
		switch _, saved := s.currentView(docs); {
		case saved:
			s.deleteViewBn.Rect = place(deleteWidth)
		case s.naming:
			s.keepViewBn.Rect = place(keepWidth)
			s.viewName.Rect = place(namingWidth)
		default:
			s.saveViewBn.Rect = place(saveWidth)
		}
		s.clearBn.Rect = place(clearWidth)
		right -= s.theme.TextWidth(s.showing()) + ui.Gap
	} else {
		s.naming = false
	}
	left := bar.Min.X + ui.Inset
	s.search.Rect = image.Rect(left, top, left+min(searchWidth, max(0, right-left)), top+ui.ButtonH)
	left = s.search.Rect.Max.X + ui.Gap
	widths, total := map[string]int{}, 0
	for _, def := range barFacets() {
		button := s.facetBns[def.id]
		button.Label, button.On = s.facetLabel(def, labels), len(s.ticks[def.id]) > 0
		widths[def.id] = s.theme.TextWidth(button.Label) + facetPad
		total += widths[def.id] + ui.Gap
	}
	bottom := bar.Max.Y
	if left+total > right {
		left, top, right = bar.Min.X+ui.Inset, top+barHeight, bar.Max.X-ui.Inset
		bottom += barHeight
	}
	for _, def := range barFacets() {
		button := s.facetBns[def.id]
		button.Rect = ui.Zero()
		if left+widths[def.id] <= right {
			button.Rect = image.Rect(left, top, left+widths[def.id], top+ui.ButtonH)
			left += widths[def.id] + ui.Gap
		}
	}

	return bottom
}

// facetLabel names a facet's button: the facet, then its ticked value, as
// "Zone: Harbour", and how many more are ticked.
func (s *State) facetLabel(def facetDef, labels facetLabels) string {
	ticked := slices.Sorted(maps.Keys(s.ticks[def.id]))
	if len(ticked) == 0 {
		return def.label
	}
	first := labels[def.id][ticked[0]].label
	if first == "" {
		first = ticked[0]
	}
	if len(ticked) > 1 {
		first += " +" + strconv.Itoa(len(ticked)-1)
	}

	return def.label + ": " + first
}

// layoutDropdown places an open facet's values under its button, each with
// how many things ticking it would list.
func (s *State) layoutDropdown(things []thing, labels facetLabels) {
	s.facets.Rect, s.facets.Rows = ui.Zero(), nil
	button, open := s.facetBns[s.facetOpen]
	if !open || button.Rect.Empty() {
		return
	}
	for _, def := range barFacets() {
		if def.id != s.facetOpen {
			continue
		}
		rows := s.facetGroup(def, s.counts(things, def.id), labels[def.id])
		if len(rows) > 0 {
			rows = rows[1:]
		}
		if len(rows) == 0 {
			row := ui.NewRow("", "Nothing to narrow by")
			row.Muted = true
			rows = []ui.Row{row}
		}
		s.facets.Rows = rows
	}
	height := min(len(s.facets.Rows), dropdownRows)*ui.RowHeight + ui.Inset
	left := min(button.Rect.Min.X, frame.Content(s.size).Max.X-dropdownWidth)
	s.facets.Rect = image.Rect(left, button.Rect.Max.Y+ui.Gap, left+dropdownWidth, button.Rect.Max.Y+ui.Gap+height)
}

// showing says how many things are kept of all there are.
func (s *State) showing() string {
	return strconv.Itoa(s.shown) + " of " + strconv.Itoa(s.total)
}

// updateBar handles the bar: a facet's button opens or closes its values,
// Clear takes every tick and the search away, and the view buttons start
// naming a view, save it, or delete the view shown.
func (s *State) updateBar(input *ui.Input, ctx *frame.Context) {
	for _, def := range barFacets() {
		if s.facetBns[def.id].Update(input) {
			s.focus.Set(nil)
			if s.facetOpen == def.id {
				s.facetOpen = ""
			} else {
				s.facetOpen = def.id
			}
		}
	}
	switch {
	case s.clearBn.Update(input):
		s.clearNarrowing()
	case s.saveViewBn.Update(input):
		s.naming = true
		s.viewName.Field.SetSaved(s.suggestedName(ctx.Docs))
		s.viewName.Field.Revert()
		s.focus.Set(s.viewName)
	case s.keepViewBn.Update(input):
		s.saveView(ctx)
	case s.deleteViewBn.Update(input):
		s.deleteView(ctx)
	}
}

// updateDropdown handles an open facet's values: a click ticks a value or
// takes its tick away, or shows the rest; Escape or a click elsewhere
// closes it. It reports whether it took the input.
func (s *State) updateDropdown(input *ui.Input) bool {
	if s.facetOpen == "" || s.facets.Rect.Empty() {
		return false
	}
	if input.Consume(ui.KeyEscape, ui.Plain()) {
		s.facetOpen = ""

		return true
	}
	button := s.facetBns[s.facetOpen]
	if input.Pressed && !input.Cursor.In(s.facets.Rect) && !input.Cursor.In(button.Rect) {
		s.facetOpen = ""

		return false
	}
	if !input.Cursor.In(s.facets.Rect) {
		return false
	}
	result := s.facets.Update(input)
	chosen := s.facets.Selected
	s.facets.Selected = ""
	if result.Changed {
		switch {
		case strings.HasPrefix(chosen, tickPrefix):
			facet, value, _ := strings.Cut(strings.TrimPrefix(chosen, tickPrefix), ":")
			s.tick(facet, value, !s.ticks[facet][value])
		case strings.HasPrefix(chosen, morePrefix):
			facet := strings.TrimPrefix(chosen, morePrefix)
			s.more[facet] = !s.more[facet]
		}
	}
	input.ClearPointer()

	return true
}

// followNarrowing shows Matches as narrowing starts, and the place shown
// before it once narrowing ends, leaving the selection and the search's
// focus as they are.
func (s *State) followNarrowing() {
	narrowed := s.narrowed()
	switch {
	case narrowed && !s.wasNarrowed:
		s.placeBefore = s.place
		s.show(placeMatches)
	case !narrowed && s.wasNarrowed:
		if s.place == placeMatches {
			s.show(orPlace(s.placeBefore, placeHome))
		}
		s.facetOpen = ""
	}
	s.wasNarrowed = narrowed
}

// orPlace returns the first of two places that is set.
func orPlace(first, second string) string {
	if first != "" {
		return first
	}

	return second
}

// clearNarrowing takes every tick and the search away.
func (s *State) clearNarrowing() {
	s.ticks = map[string]map[string]bool{}
	s.search.Field.SetSaved("")
	s.search.Field.Revert()
	s.facetOpen, s.naming = "", false
	s.focus.Set(nil)
}

// suggestedName names a view after its ticks and search, as "Workflows,
// Harbour, This week".
func (s *State) suggestedName(docs frame.Docs) string {
	_, labels := s.things(docs)
	var parts []string
	for _, def := range barFacets() {
		for _, value := range slices.Sorted(maps.Keys(s.ticks[def.id])) {
			label := labels[def.id][value].label
			if label == "" {
				label = value
			}
			parts = append(parts, label)
		}
	}
	if search := strings.TrimSpace(s.search.Field.Value()); search != "" {
		parts = append(parts, "“"+search+"”")
	}
	name := strings.Join(parts, ", ")
	if len(name) > viewsplugin.MaxNameLength {
		name = strings.TrimSpace(name[:viewsplugin.MaxNameLength])
	}

	return name
}

// savedViews returns the project's saved views.
func savedViews(docs frame.Docs) []viewsplugin.View {
	var found []viewsplugin.View
	docs.View(viewsplugin.ID(), func(document client.Document) {
		if viewsDoc, ok := document.Model.(*viewsplugin.Document); ok {
			found = viewsDoc.Views
		}
	})

	return found
}

// view is what is ticked and typed now, as a view named name.
func (s *State) view(name string) viewsplugin.View {
	ticks := map[string][]string{}
	for facet, values := range s.ticks {
		if len(values) > 0 {
			ticks[facet] = slices.Sorted(maps.Keys(values))
		}
	}

	return viewsplugin.View{Name: name, Search: strings.TrimSpace(s.search.Field.Value()), Ticks: ticks}
}

// currentView returns the saved view whose ticks and search are now's.
func (s *State) currentView(docs frame.Docs) (viewsplugin.View, bool) {
	now := s.view("")
	for _, saved := range savedViews(docs) {
		if saved.Same(now) {
			return saved, true
		}
	}

	return viewsplugin.View{Name: "", Search: "", Ticks: nil}, false
}

// saveView saves what is ticked and typed under the name typed, replacing
// a view of that name.
func (s *State) saveView(ctx *frame.Context) {
	name := strings.TrimSpace(s.viewName.Field.Value())
	if name == "" {
		ctx.Frame.Status("Name the view to save it.")

		return
	}
	views := slices.DeleteFunc(slices.Clone(savedViews(ctx.Docs)), func(view viewsplugin.View) bool {
		return strings.EqualFold(view.Name, name)
	})
	views = append(views, s.view(name))
	s.command(ctx, viewsplugin.ID(), viewsplugin.SetEdit, "", viewsplugin.SetArgs{Views: views},
		func(_ protocol.Result, _ error) { ctx.Frame.Status("Saved the view “" + name + "”.") }, nil)
	s.naming = false
	s.focus.Set(nil)
}

// deleteView deletes the saved view whose ticks are now's.
func (s *State) deleteView(ctx *frame.Context) {
	current, saved := s.currentView(ctx.Docs)
	if !saved {
		return
	}
	views := slices.DeleteFunc(slices.Clone(savedViews(ctx.Docs)), func(view viewsplugin.View) bool {
		return view.Name == current.Name
	})
	s.command(ctx, viewsplugin.ID(), viewsplugin.SetEdit, "", viewsplugin.SetArgs{Views: views},
		func(_ protocol.Result, _ error) {
			ctx.Frame.Status("Deleted the view “" + current.Name + "”. Undo brings it back.")
		}, nil)
}

// applyView ticks a saved view's values and types its search.
func (s *State) applyView(docs frame.Docs, name string) {
	for _, saved := range savedViews(docs) {
		if saved.Name != name {
			continue
		}
		s.ticks = map[string]map[string]bool{}
		for facet, values := range saved.Ticks {
			for _, value := range values {
				s.tick(facet, value, true)
			}
		}
		s.search.Field.SetSaved(saved.Search)
		s.search.Field.Revert()
		s.naming = false
	}
}

// viewRows lists the saved views under the places, the one now ticked
// marked.
func (s *State) viewRows(docs frame.Docs) []ui.Row {
	views := savedViews(docs)
	if len(views) == 0 {
		return nil
	}
	heading := s.section(sectionViews, "Views", 0, false, false)
	heading.Detail = ""
	rows := []ui.Row{heading}
	if s.closed[sectionViews] {
		return rows
	}
	current, _ := s.currentView(docs)
	for _, view := range views {
		row := ui.NewRow(viewPrefix+view.Name, view.Name)
		row.Icon, row.Ticked = ui.IconFolder, view.Name == current.Name
		rows = append(rows, row)
	}

	return rows
}

// drawBar draws the places column's edge, the bar and an open dropdown.
func (s *State) drawBar(screen *ebiten.Image) {
	bottom := frame.Content(s.size).Max.Y
	edge := s.places.Rect.Max.X - ui.Outline
	ui.Divider(screen, image.Pt(edge, toolbarHeight), image.Pt(edge, bottom))
	s.places.Draw(screen, s.theme)
	barBottom := s.barBottom - ui.Outline
	ui.Divider(screen, image.Pt(s.places.Rect.Max.X, barBottom), image.Pt(frame.Content(s.size).Max.X, barBottom))
	s.search.Draw(screen)
	for _, def := range barFacets() {
		s.facetBns[def.id].Draw(screen, s.theme)
	}
	if s.narrowed() {
		at := s.clearBn.Rect.Min.X - ui.Gap - s.theme.TextWidth(s.showing())
		top := toolbarHeight + (barHeight-ui.TextSize)/ui.Half - 1
		s.theme.DrawText(screen, s.showing(), at, top, ui.ColorSubdued())
	}
	for _, button := range []*ui.Button{s.clearBn, s.saveViewBn, s.keepViewBn, s.deleteViewBn} {
		button.Draw(screen, s.theme)
	}
	if !s.viewName.Rect.Empty() {
		s.viewName.Draw(screen)
	}
}

// drawDropdown draws an open facet's values over the main pane.
func (s *State) drawDropdown(screen *ebiten.Image) {
	if s.facets.Rect.Empty() {
		return
	}
	ui.Box(screen, s.facets.Rect, ui.ColorBackground())
	s.facets.Draw(screen, s.theme)
}
