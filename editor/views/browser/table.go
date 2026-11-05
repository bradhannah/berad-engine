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

// The table (increment 30): once anything is typed or ticked, the list
// shows what is left as a table, under a bar of the ticked values, how many
// things are shown of all, and a way to save the ticks as a view.

// Sizes of the table's bar and header, in pixels.
const (
	barHeight    = 34
	headerHeight = 22
	chipHeight   = 22
	facetChipPad = 8
	chipClose    = 14
	nameWidth    = 200
	namingWidth  = 200
	saveWidth    = 128
	keepWidth    = 64
	deleteWidth  = 104
)

// column is one of the table's columns after the name: which cell it shows,
// its title, and how far from the list's left it starts.
type column struct {
	cell  int
	title string
	at    int
}

// chip is a ticked value in the table's bar; a click takes its tick away.
type chip struct {
	rect         image.Rectangle
	facet, value string
	label        string
}

// narrowed reports whether the list is a table: something is typed in the
// search or ticked in a facet.
func (s *State) narrowed() bool { return newMatcher(s.search.Field.Value()).active() || s.ticked() }

// layoutLists places the search box, the facets column when the window has
// room for it, and the list: the tree, or the table under its bar.
func (s *State) layoutLists(docs frame.Docs, content image.Rectangle) {
	things, labels := s.things(docs)
	s.withFacets = content.Dx() >= facetWidth+facetRoom
	searchTop := toolbarHeight + ui.Inset/ui.Half
	left, top := 0, toolbarHeight+searchHeight
	s.search.Rect = image.Rect(ui.Inset, searchTop, content.Max.X-ui.Inset, searchTop+ui.ButtonH)
	s.facets.Rect, s.facets.Rows = ui.Zero(), nil
	if s.withFacets {
		s.search.Rect.Max.X = facetWidth - ui.Inset
		s.facets.Rect = image.Rect(0, toolbarHeight+searchHeight, facetWidth, content.Max.Y)
		s.facets.Rows = s.facetRows(docs, things, labels)
		left, top = facetWidth, toolbarHeight
	}
	s.layoutBar(docs, image.Rect(left, top, content.Max.X, top+barHeight), labels)
	if !s.narrowed() {
		s.chips, s.columns, s.list.Columns = nil, nil, nil
		s.list.Rect = image.Rect(left, top, content.Max.X, content.Max.Y)
		s.list.Rows = s.rows(docs)

		return
	}
	s.list.Rect = image.Rect(left, top+barHeight+headerHeight, content.Max.X, content.Max.Y)
	s.columns = tableColumns(s.list.Rect.Dx())
	s.list.Columns = make([]int, 0, len(s.columns))
	for _, each := range s.columns {
		s.list.Columns = append(s.list.Columns, each.at)
	}
	s.list.Rows = s.tableRows(things)
}

// tableColumns chooses the columns that fit beside a name of readable
// width, Kind, Zone and Changed first, and gives the name what is left.
func tableColumns(width int) []column {
	wanted := []struct {
		cell  int
		title string
		width int
	}{
		{columnKind, "Kind", 110}, {columnZone, zoneWord, 110}, {columnChanged, "Changed", 90},
		{columnFolder, "Folder", 130}, {columnUsedBy, "Used by", 130},
	}
	room, used := width-nameWidth, 0
	var chosen []column
	widths := map[int]int{}
	for _, each := range wanted {
		if each.width <= room-used {
			chosen = append(chosen, column{cell: each.cell, title: each.title, at: 0})
			widths[each.cell] = each.width
			used += each.width
		}
	}
	slices.SortFunc(chosen, func(a, b column) int { return a.cell - b.cell })
	at := width - used
	for index := range chosen {
		chosen[index].at = at
		at += widths[chosen[index].cell]
	}

	return chosen
}

// tableRows lists the things left, by name, with their cells, and counts
// them and every thing there is.
func (s *State) tableRows(things []thing) []ui.Row {
	var rows []ui.Row
	s.shown, s.total = 0, 0
	for _, found := range things {
		if !found.extra {
			s.total++
		}
		if !s.kept(found, "") {
			continue
		}
		row := found.row
		row.Cells = make([]string, 0, len(s.columns))
		for _, each := range s.columns {
			row.Cells = append(row.Cells, found.cells[each.cell])
		}
		rows = append(rows, row)
	}
	s.shown = len(rows)

	return rows
}

// layoutBar places the table's bar: a chip for each ticked value, then how
// many are shown, and at the right Save as a view…, the name of a view
// being saved, or Delete view when the ticks are a saved view's.
func (s *State) layoutBar(docs frame.Docs, bar image.Rectangle, labels facetLabels) {
	s.chips = nil
	s.saveViewBn.Rect, s.keepViewBn.Rect, s.deleteViewBn.Rect, s.viewName.Rect = ui.Zero(), ui.Zero(), ui.Zero(), ui.Zero()
	if !s.narrowed() {
		s.naming = false

		return
	}
	left, top := bar.Min.X+ui.Inset, bar.Min.Y+(barHeight-chipHeight)/ui.Half
	for _, def := range facetDefs() {
		values := slices.Sorted(maps.Keys(s.ticks[def.id]))
		for _, value := range values {
			label := labels[def.id][value].label
			if label == "" {
				label = value
			}
			label = def.label + ": " + label
			width := s.theme.TextWidth(label) + ui.Half*facetChipPad + chipClose
			s.chips = append(s.chips, chip{
				rect: image.Rect(left, top, left+width, top+chipHeight), facet: def.id, value: value, label: label,
			})
			left += width + ui.Gap
		}
	}
	right, buttonTop := bar.Max.X-ui.Inset, bar.Min.Y+(barHeight-ui.ButtonH)/ui.Half
	place := func(width int) image.Rectangle {
		rect := image.Rect(right-width, buttonTop, right, buttonTop+ui.ButtonH)
		right = rect.Min.X - ui.Gap

		return rect
	}
	switch _, saved := s.currentView(docs); {
	case saved:
		s.deleteViewBn.Rect = place(deleteWidth)
	case s.naming:
		s.keepViewBn.Rect = place(keepWidth)
		s.viewName.Rect = place(namingWidth)
	default:
		s.saveViewBn.Rect = place(saveWidth)
	}
}

// showing says how many things the table shows of all there are.
func (s *State) showing() string {
	return strconv.Itoa(s.shown) + " of " + strconv.Itoa(s.total)
}

// updateBar handles the bar: a chip's click takes its tick away, and the
// buttons start naming a view, save it, or delete the view shown.
func (s *State) updateBar(input *ui.Input, ctx *frame.Context) {
	for _, each := range s.chips {
		if input.Pressed && input.Cursor.In(each.rect) {
			input.ClearPointer()
			s.tick(each.facet, each.value, false)
			s.focus.Set(nil)

			return
		}
	}
	switch {
	case s.saveViewBn.Update(input):
		s.naming = true
		s.viewName.Field.SetSaved(s.suggestedName())
		s.viewName.Field.Revert()
		s.focus.Set(s.viewName)
	case s.keepViewBn.Update(input):
		s.saveView(ctx)
	case s.deleteViewBn.Update(input):
		s.deleteView(ctx)
	}
}

// suggestedName names a view after its ticks and search, as "Workflows,
// Harbour, This week".
func (s *State) suggestedName() string {
	parts := make([]string, 0, len(s.chips)+1)
	for _, each := range s.chips {
		_, value, _ := strings.Cut(each.label, ": ")
		parts = append(parts, value)
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

// viewRows lists the saved views at the top of the facets column, the one
// now ticked marked.
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
		row.Tree, row.Ticked = true, view.Name == current.Name
		rows = append(rows, row)
	}

	return rows
}

// drawTable draws the facets column's edge and, for a table, its bar and
// the titles of its columns.
func (s *State) drawTable(screen *ebiten.Image) {
	if s.withFacets {
		bottom := frame.Content(s.size).Max.Y
		ui.Divider(screen, image.Pt(facetWidth-ui.Outline, toolbarHeight), image.Pt(facetWidth-ui.Outline, bottom))
		s.facets.Draw(screen, s.theme)
	}
	if !s.narrowed() {
		return
	}
	textTop := func(rect image.Rectangle) int { return rect.Min.Y + (rect.Dy()-ui.TextSize)/ui.Half - 1 }
	left := s.list.Rect.Min.X + ui.Inset
	for _, each := range s.chips {
		ui.FillRound(screen, each.rect, float32(chipHeight)/ui.Half, ui.ColorSelectWash())
		ui.StrokeRound(screen, each.rect, float32(chipHeight)/ui.Half, ui.Outline, ui.ColorSelect())
		s.theme.DrawText(screen, each.label, each.rect.Min.X+facetChipPad, textTop(each.rect), ui.ColorSelect())
		s.theme.DrawText(screen, "×", each.rect.Max.X-chipClose, textTop(each.rect), ui.ColorSelect())
		left = each.rect.Max.X + ui.Gap
	}
	barTop := s.list.Rect.Min.Y - headerHeight - barHeight
	s.theme.DrawText(screen, s.showing(), left+ui.Gap, textTop(image.Rect(0, barTop, 0, barTop+barHeight)),
		ui.ColorSubdued())
	for _, button := range []*ui.Button{s.saveViewBn, s.keepViewBn, s.deleteViewBn} {
		button.Draw(screen, s.theme)
	}
	if !s.viewName.Rect.Empty() {
		s.viewName.Draw(screen)
	}
	header := image.Rect(s.list.Rect.Min.X, s.list.Rect.Min.Y-headerHeight, s.list.Rect.Max.X, s.list.Rect.Min.Y)
	ui.Divider(screen, image.Pt(header.Min.X, header.Max.Y-ui.Outline), image.Pt(header.Max.X, header.Max.Y-ui.Outline))
	titleTop := header.Min.Y + (headerHeight-ui.LabelSize)/ui.Half
	s.theme.DrawLabel(screen, "NAME", header.Min.X+ui.Inset, titleTop, ui.ColorMuted())
	for _, each := range s.columns {
		s.theme.DrawLabel(screen, strings.ToUpper(each.title), header.Min.X+each.at, titleTop, ui.ColorMuted())
	}
}
