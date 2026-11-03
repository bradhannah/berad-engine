package zonepicker

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// The list's columns, right-aligned in from its right edge, and how far a
// folder is indented on the rail for each level.
const (
	columnWidth = 56
	railIndent  = 12
	folderSep   = "/"
	// railFixed is how many entries the rail has before its folders.
	railFixed = 2
)

// columns are the list's column headings, from the left.
func columns() []string { return []string{"Floors", "NPCs", "Places"} }

// query is the search typed, as zones are matched against it.
func (p *Picker) query() string {
	return strings.ToLower(strings.TrimSpace(p.search.Field.Value()))
}

// matches reports whether a zone has the search typed in its name, folder
// or a tag, and every tag whose chip is on.
func (p *Picker) matches(each Zone) bool {
	for _, chip := range p.tags {
		if chip.On && !slices.Contains(each.Tags, chip.Label) {
			return false
		}
	}
	query := p.query()
	if query == "" || strings.Contains(strings.ToLower(each.Name), query) ||
		strings.Contains(strings.ToLower(each.Folder), query) {
		return true
	}

	return slices.ContainsFunc(each.Tags, func(tag string) bool { return strings.Contains(strings.ToLower(tag), query) })
}

// inScope reports whether a zone is in a rail entry: every zone in All
// zones, those chosen lately in Recent, and those in a folder or the
// folders inside it.
func (p *Picker) inScope(each Zone, scope string) bool {
	switch scope {
	case railAll:
		return true
	case railRecent:
		return slices.Contains(p.config.Recent, each.ID)
	default:
		return each.Folder == scope || strings.HasPrefix(each.Folder, scope+folderSep)
	}
}

// rows lists the zones that match the search and the chips in the rail
// entry chosen, Recent most recent first, and counts how many match in
// each entry.
func (p *Picker) rows(zones []Zone) []ui.Row {
	p.counts = map[string]int{}
	p.shown = p.shown[:0]
	for _, each := range zones {
		if !p.matches(each) {
			continue
		}
		for _, entry := range p.rail {
			if p.inScope(each, entry.id) {
				p.counts[entry.id]++
			}
		}
		if p.inScope(each, p.scope) {
			p.shown = append(p.shown, each)
		}
	}
	if p.scope == railRecent {
		slices.SortStableFunc(p.shown, func(a, b Zone) int {
			return slices.Index(p.config.Recent, a.ID) - slices.Index(p.config.Recent, b.ID)
		})
	}
	rows := make([]ui.Row, 0, len(p.shown))
	for _, each := range p.shown {
		row := ui.NewRow(each.ID.String(), each.Name)
		row.Icon = ui.IconZone
		switch {
		case each.ID == p.config.Now:
			row.Aside = "now"
		case each.Refused != "":
			row.Aside = each.Refused
		case p.config.Mine[each.ID] > 0:
			row.Aside = mine(p.config.Mine[each.ID])
		case each.Folder != "":
			row.Aside = each.Folder
		}
		rows = append(rows, row)
	}

	return rows
}

// mine says how many of the asker's places are in a zone.
func mine(count int) string {
	if count == 1 {
		return "1 of its places"
	}

	return strconv.Itoa(count) + " of its places"
}

// layoutRail places the rail: All zones, Recent, then each folder, indented
// by how deep it is.
func (p *Picker) layoutRail(folders []string, left, top int) {
	p.rail = []railEntry{
		{id: railAll, label: "All zones", depth: 0, rect: ui.Zero()},
		{id: railRecent, label: "Recent", depth: 0, rect: ui.Zero()},
	}
	for _, folder := range folders {
		depth := strings.Count(folder, folderSep)
		p.rail = append(p.rail, railEntry{
			id: folder, label: folder[strings.LastIndex(folder, folderSep)+1:], depth: depth, rect: ui.Zero(),
		})
	}
	row := top
	for index := range p.rail {
		if index == railFixed {
			row += ui.RowHeight
		}
		p.rail[index].rect = image.Rect(left, row, left+railWidth, row+ui.RowHeight)
		row += ui.RowHeight
	}
}

// drawRail draws the rail, each entry with how many zones match in it, the
// one chosen washed, and Folders over the folders.
func (p *Picker) drawRail(screen *ebiten.Image) {
	for index, entry := range p.rail {
		rect := entry.rect
		if index == railFixed {
			p.theme.DrawLabel(screen, "Folders", rect.Min.X+textPad, rect.Min.Y-ui.RowHeight+(ui.RowHeight-ui.TextSize)/ui.Half,
				ui.ColorMuted())
		}
		if entry.id == p.scope {
			ui.FillRound(screen, rect, ui.Radius, ui.ColorSelectWash())
			ui.StrokeRound(screen, rect, ui.Radius, ui.Hairline, ui.ColorSelect())
		}
		top := rect.Min.Y + (ui.RowHeight-ui.TextSize)/ui.Half - 1
		label := p.theme.FitText(entry.label, railWidth-ui.Half*textPad-entry.depth*railIndent-columnWidth/ui.Half)
		p.theme.DrawText(screen, label, rect.Min.X+textPad+entry.depth*railIndent, top, ui.ColorInk())
		count := strconv.Itoa(p.counts[entry.id])
		p.theme.DrawText(screen, count, rect.Max.X-textPad-p.theme.TextWidth(count), top, ui.ColorMuted())
	}
}

// drawList draws the column headings and the list in its frame, each zone
// with its floors, NPCs and places counted in the columns, or says why it
// is empty.
func (p *Picker) drawList(screen *ebiten.Image) {
	heading := p.list.Rect.Min.Y - ui.RowHeight + (ui.RowHeight-ui.TextSize)/ui.Half
	p.theme.DrawLabel(screen, "Zone", p.list.Rect.Min.X+textPad, heading, ui.ColorMuted())
	for index, label := range columns() {
		right := p.columnRight(index)
		p.theme.DrawLabel(screen, label, right-p.theme.LabelWidth(label), heading, ui.ColorMuted())
	}
	ui.StrokeRound(screen, p.list.Rect.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	if len(p.list.Rows) == 0 {
		empty := "No zone matches."
		if p.query() == "" && p.scope == railRecent {
			empty = "No zone chosen lately."
		}
		p.theme.DrawText(screen, empty, p.list.Rect.Min.X+textPad, p.list.Rect.Min.Y+ui.Gap, ui.ColorMuted())

		return
	}
	p.list.Draw(screen, p.theme)
	area, ok := screen.SubImage(p.list.Rect).(*ebiten.Image)
	if !ok {
		return
	}
	for index, each := range p.shown {
		rect := p.list.RowRect(index)
		if !rect.Overlaps(p.list.Rect) {
			continue
		}
		top := rect.Min.Y + (ui.RowHeight-ui.TextSize)/ui.Half - 1
		for column, count := range []int{len(each.Floors), len(each.Living), len(each.Places)} {
			text := strconv.Itoa(count)
			p.theme.DrawText(area, text, p.columnRight(column)-p.theme.TextWidth(text), top, ui.ColorMuted())
		}
	}
}

// columnRight is where a column's numbers end, from the left.
func (p *Picker) columnRight(column int) int {
	return p.list.Rect.Max.X - textPad - (len(columns())-1-column)*columnWidth
}
