package browser

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
)

// Rows (increment 32): a place's things with their details in columns that
// suit their kind, sorted by a column's heading, and its folders opening in
// place.

// Sizes and keys of the rows' columns.
const (
	headerHeight = 24
	// nameRoom is the least width a name keeps beside the columns.
	nameRoom = 200
	// openPrefix keys a folder opened in place in the main pane.
	openPrefix = "open:"
	// sortName sorts by name, as rows are until a heading is clicked.
	sortName = "Name"
	// countWidth pads a count, so counts sort as numbers.
	countWidth = 6
)

// Widths of the columns, in pixels.
const (
	narrowColumn  = 70
	changedWidth  = 90
	kindWidth     = 110
	runsWidth     = 120
	usedWidth     = 130
	loopWidth     = 140
	followsWidth  = 150
	problemsWidth = 150
)

// floorsLabel names floors, in a column and a group.
const floorsLabel = "Floors"

// rowColumn is a column of a place's rows: its title and width, the text
// it shows for a thing, and how it sorts, when not as that text.
type rowColumn struct {
	title string
	width int
	cell  func(entry project.Entry, found thing) string
	key   func(entry project.Entry, found thing) string
}

// placedColumn is a column shown, and where it starts.
type placedColumn struct {
	rowColumn

	at int
}

// changedColumn says when a thing changed, and sorts by when.
func changedColumn() rowColumn {
	return rowColumn{
		title: "Changed", width: changedWidth,
		cell: func(_ project.Entry, found thing) string { return found.cells[columnChanged] },
		key:  func(entry project.Entry, _ thing) string { return entry.Changed.UTC().Format(time.RFC3339) },
	}
}

// cellColumn shows one of a thing's cells.
func cellColumn(title string, width, cell int) rowColumn {
	return rowColumn{
		title: title, width: width, key: nil,
		cell: func(_ project.Entry, found thing) string { return found.cells[cell] },
	}
}

// columnsOf returns the columns a place's rows show, by its kind.
func (s *State) columnsOf(docs frame.Docs, index *project.Index, place string) []rowColumn {
	switch kindOfPlace(place) {
	case sectionWorkflows:
		return []rowColumn{
			cellColumn("Runs as", runsWidth, columnKind), cellColumn("Runs in", kindWidth, columnZone),
			cellColumn("Used by", usedWidth, columnUsedBy), changedColumn(),
			{title: "Problems", width: problemsWidth, key: nil, cell: func(_ project.Entry, found thing) string {
				return strings.Join(found.values[facetProblems], ", ")
			}},
		}
	case sectionNPCs:
		return []rowColumn{
			cellColumn("Lives in", kindWidth, columnZone),
			{title: "Follows", width: followsWidth, key: nil, cell: func(entry project.Entry, _ thing) string {
				return s.follows(docs, index, entry.ID)
			}},
			changedColumn(),
		}
	case sectionZones:
		return s.zoneColumns(docs, index)
	case sectionSprites:
		return []rowColumn{s.spriteUsers(docs, index), changedColumn()}
	case sectionPalettes, sectionTemplates:
		return []rowColumn{changedColumn()}
	}
	if place == placeMatches {
		return []rowColumn{cellColumn("Kind", kindWidth, columnKind), changedColumn()}
	}

	return nil
}

// zoneColumns are a zone's floors, the people living in it, and its loop.
func (s *State) zoneColumns(docs frame.Docs, index *project.Index) []rowColumn {
	people := map[ids.ULID]int{}
	for _, entry := range index.OfFormat(npc.FormatName) {
		if person, loaded := s.npcOf(docs, entry.ID); loaded && !person.Zone.IsZero() {
			people[person.Zone]++
		}
	}
	count := func(value int) string { return fmt.Sprintf("%0*d", countWidth, value) }

	return []rowColumn{
		{title: floorsLabel, width: narrowColumn, cell: func(entry project.Entry, _ thing) string {
			return strconv.Itoa(len(s.floorsOf(docs, entry.ID)))
		}, key: func(entry project.Entry, _ thing) string { return count(len(s.floorsOf(docs, entry.ID))) }},
		{title: "People", width: narrowColumn, cell: func(entry project.Entry, _ thing) string {
			return strconv.Itoa(people[entry.ID])
		}, key: func(entry project.Entry, _ thing) string { return count(people[entry.ID]) }},
		{title: "Loop", width: loopWidth, key: nil, cell: func(entry project.Entry, _ thing) string {
			if loop := s.loopOf(docs, entry.ID); !loop.IsZero() {
				return index.Name(loop, "a deleted workflow")
			}

			return ""
		}},
	}
}

// spriteUsers says how many NPCs show a sprite.
func (s *State) spriteUsers(docs frame.Docs, index *project.Index) rowColumn {
	users := map[ids.ULID]int{}
	for _, entry := range index.OfFormat(npc.FormatName) {
		if person, loaded := s.npcOf(docs, entry.ID); loaded && !person.Sprite.IsZero() {
			users[person.Sprite]++
		}
	}

	return rowColumn{title: "Used by", width: kindWidth, cell: func(entry project.Entry, _ thing) string {
		if used := users[entry.ID]; used > 0 {
			return strconv.Itoa(used) + " " + plural(used, npcLabel, "NPCs")
		}

		return ""
	}, key: func(entry project.Entry, _ thing) string { return fmt.Sprintf("%0*d", countWidth, users[entry.ID]) }}
}

// placeColumns chooses the columns that fit beside a name of readable width,
// in their order, and where each starts.
func placeColumns(columns []rowColumn, width int) []placedColumn {
	room, used := width-nameRoom, 0
	var chosen []rowColumn
	for _, each := range columns {
		if each.width <= room-used {
			chosen = append(chosen, each)
			used += each.width
		}
	}
	placed := make([]placedColumn, 0, len(chosen))
	at := width - used
	for _, each := range chosen {
		placed = append(placed, placedColumn{rowColumn: each, at: at})
		at += each.width
	}

	return placed
}

// fillCells gives each document row the cells of the columns shown.
func fillCells(rows []ui.Row, columns []placedColumn, index *project.Index, things map[string]thing) {
	for position := range rows {
		documentID, err := ids.Parse(rows[position].ID)
		if err != nil {
			continue
		}
		entry, found := index.Entry(documentID)
		if !found {
			continue
		}
		cells := make([]string, 0, len(columns))
		for _, each := range columns {
			cells = append(cells, each.cell(entry, things[rows[position].ID]))
		}
		rows[position].Cells = cells
	}
}

// sortRows sorts a place's things by the column chosen, folders first and
// by name, each thing keeping the rows under it, such as a zone's floors.
func sortRows(rows []ui.Row, column *placedColumn, descending bool, index *project.Index,
	things map[string]thing,
) []ui.Row {
	type unit struct {
		rows   []ui.Row
		folder bool
		key    string
	}
	var units []unit
	for _, row := range rows {
		if row.Indent > 0 && len(units) > 0 {
			units[len(units)-1].rows = append(units[len(units)-1].rows, row)

			continue
		}
		_, folder := parseFolderRow(row.ID)
		key := strings.ToLower(row.Label)
		if documentID, err := ids.Parse(row.ID); err == nil && column != nil {
			if entry, found := index.Entry(documentID); found {
				key = sortKey(column, entry, things[row.ID]) + "\x00" + key
			}
		}
		units = append(units, unit{rows: []ui.Row{row}, folder: folder || row.Heading, key: key})
	}
	slices.SortStableFunc(units, func(first, second unit) int {
		if first.folder != second.folder {
			if first.folder {
				return -1
			}

			return 1
		}
		if descending && !first.folder {
			return strings.Compare(second.key, first.key)
		}

		return strings.Compare(first.key, second.key)
	})
	sorted := make([]ui.Row, 0, len(rows))
	for _, each := range units {
		sorted = append(sorted, each.rows...)
	}

	return sorted
}

// sortKey is how a thing sorts by a column.
func sortKey(column *placedColumn, entry project.Entry, found thing) string {
	if column.key != nil {
		return column.key(entry, found)
	}

	return strings.ToLower(column.cell(entry, found))
}

// choice returns how the place shown is shown: its view and its sort.
func (s *State) choice() protocol.FolderChoice {
	return s.home.kept.Folders[s.place]
}

// arrangeRows gives a place's rows their columns and sorts them, and keeps
// the columns for the heading row.
func (s *State) arrangeRows(docs frame.Docs, index *project.Index, rows []ui.Row, things map[string]thing,
	width int,
) []ui.Row {
	s.columns = placeColumns(s.columnsOf(docs, index, s.place), width)
	s.list.Columns = s.list.Columns[:0]
	for _, each := range s.columns {
		s.list.Columns = append(s.list.Columns, each.at)
	}
	if len(s.columns) == 0 {
		s.list.Columns = nil

		return rows
	}
	fillCells(rows, s.columns, index, things)
	if s.place == placeMatches || s.place == placeHome {
		return rows
	}
	choice := s.choice()
	var sorting *placedColumn
	for position := range s.columns {
		if s.columns[position].title == choice.Sort {
			sorting = &s.columns[position]
		}
	}

	return sortRows(rows, sorting, choice.Descending, index, things)
}

// headerRect is where the column headings are, over the main pane's rows.
func (s *State) headerRect() image.Rectangle {
	if len(s.columns) == 0 {
		return ui.Zero()
	}

	return image.Rect(s.list.Rect.Min.X, s.list.Rect.Min.Y-headerHeight, s.list.Rect.Max.X, s.list.Rect.Min.Y)
}

// headingAt returns the column heading at a point: a column's title, or
// Name for the names.
func (s *State) headingAt(point image.Point) (string, bool) {
	header := s.headerRect()
	if !point.In(header) {
		return "", false
	}
	for _, each := range s.columns {
		if point.X >= header.Min.X+each.at && point.X < header.Min.X+each.at+each.width {
			return each.title, true
		}
	}
	if len(s.columns) > 0 && point.X < header.Min.X+s.columns[0].at {
		return sortName, true
	}

	return "", false
}

// updateHeader sorts by a heading clicked, and the other way when it is
// the one sorted by already; the place keeps it.
func (s *State) updateHeader(input *ui.Input, ctx *frame.Context) {
	if !input.Pressed {
		return
	}
	title, found := s.headingAt(input.Cursor)
	if !found {
		return
	}
	input.ClearPointer()
	choice := s.choice()
	switch {
	case title == sortName && choice.Sort == "":
		choice.Descending = !choice.Descending
	case title == sortName:
		choice.Sort, choice.Descending = "", false
	case choice.Sort == title:
		choice.Descending = !choice.Descending
	default:
		choice.Sort, choice.Descending = title, false
	}
	s.home.kept.Folders[s.place] = choice
	s.keepHome(ctx)
}

// locateHeading finds a column's heading, for input scripts.
func (s *State) locateHeading(title string) (image.Point, bool) {
	header := s.headerRect()
	if header.Empty() {
		return image.Pt(0, 0), false
	}
	if title == sortName && len(s.columns) > 0 {
		return image.Pt(header.Min.X+ui.Inset+ui.Gap, header.Min.Y+headerHeight/ui.Half), true
	}
	for _, each := range s.columns {
		if each.title == title {
			return image.Pt(header.Min.X+each.at+ui.Gap, header.Min.Y+headerHeight/ui.Half), true
		}
	}

	return image.Pt(0, 0), false
}

// drawHeadings draws the column headings, the one sorted by marked with
// its direction.
func (s *State) drawHeadings(screen *ebiten.Image) {
	header := s.headerRect()
	if header.Empty() {
		return
	}
	ui.Divider(screen, image.Pt(header.Min.X, header.Max.Y-ui.Outline), image.Pt(header.Max.X, header.Max.Y-ui.Outline))
	choice := s.choice()
	titleTop := header.Min.Y + (headerHeight-ui.LabelSize)/ui.Half
	mark := func(title string) string {
		label := strings.ToUpper(title)
		sorted := choice.Sort == title || title == sortName && choice.Sort == ""
		switch {
		case sorted && choice.Descending:
			return label + " ▼"
		case sorted:
			return label + " ▲"
		}

		return label
	}
	s.theme.DrawLabel(screen, mark(sortName), header.Min.X+ui.Inset, titleTop, ui.ColorMuted())
	for _, each := range s.columns {
		s.theme.DrawLabel(screen, mark(each.title), header.Min.X+each.at, titleTop, ui.ColorMuted())
	}
}

// folderOpen reports whether a folder in the main pane is open in place.
func (s *State) folderOpen(rowID string) bool { return s.closed[openPrefix+rowID] }
