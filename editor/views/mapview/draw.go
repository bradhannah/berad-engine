package mapview

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

const (
	belowAlpha  = 0x40
	gridAlpha   = 0x22
	minGridSize = 12
	minLabel    = 18
	wallShare   = 2 // a wall's core is half a cell wide
	minCore     = 2
	dotSize     = 2
	fullAlpha   = 0xff
	hexWhite    = 0xffffff
	// Insets, in eighths of a cell: pickups and anchors are drawn smaller
	// than a cell so the ground shows around them.
	eighths      = 8
	smallPickup  = 3
	mediumPickup = 2
	otherObject  = 1
	markerInset  = 2
	markerCore   = 3
	// labelBacking is the opacity behind an anchor name, so it reads over
	// whatever lies under it.
	labelBacking = 0xd0
)

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layoutWidgets(docs)
	screen.Fill(ui.ColorBackground())
	grid := s.Grid()
	area, ok := screen.SubImage(grid).(*ebiten.Image)
	if ok {
		s.drawFloors(area, docs, grid)
	}
	s.drawPanel(screen, docs)
	s.drawInspector(screen, docs)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
}

func (s *State) drawFloors(area *ebiten.Image, docs frame.Docs, grid image.Rectangle) {
	if s.floor.IsZero() {
		s.theme.DrawText(area, "This zone has no floors yet. Add one in the project browser.",
			grid.Min.X+ui.Inset, grid.Min.Y+ui.Inset, ui.ColorMuted())

		return
	}
	if below, ok := s.floorBelow(); ok && s.below.On {
		docs.View(below, func(document client.Document) {
			if floorDoc, isFloor := document.Model.(*tilefloor.Document); isFloor {
				s.drawCells(area, floorDoc.Floor, grid, true)
			}
		})
	}
	loaded := docs.View(s.floor, func(document client.Document) {
		floorDoc, isFloor := document.Model.(*tilefloor.Document)
		if !isFloor {
			return
		}
		s.drawCells(area, floorDoc.Floor, grid, false)
		s.drawAnchors(area, floorDoc.Floor, grid)
		s.drawSelection(area, grid)
	})
	if !loaded {
		s.theme.DrawText(area, "Loading the floor…", grid.Min.X+ui.Inset, grid.Min.Y+ui.Inset, ui.ColorMuted())
	}
}

// drawCells draws every visible cell. The floor below is drawn faintly, with
// nothing for unassigned ground, so the floor above can be traced over it.
func (s *State) drawCells(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle, faint bool) {
	for row := range floorData.Height() {
		for col := range floorData.Width() {
			cell := Cell{X: col, Y: row}
			rect := s.camera.cellRect(cell, grid)
			if !rect.Overlaps(grid) {
				continue
			}
			if faint {
				s.drawFaintCell(area, floorData, cell, rect)
			} else {
				s.drawCell(area, floorData, cell, rect)
			}
		}
	}
	if s.hovering && !faint {
		ui.StrokeRect(area, s.camera.cellRect(s.hover, grid), ui.Outline, ui.ColorSelect())
	}
}

func (s *State) drawFaintCell(area *ebiten.Image, floorData *floor.Floor, cell Cell, rect image.Rectangle) {
	ground, _ := floorData.Ground(cell.X, cell.Y) //nolint:errcheck // Inside the floor.
	top, _ := floorData.Top(cell.X, cell.Y)       //nolint:errcheck // Inside the floor.
	if ground != "" {
		ui.FillRect(area, rect, tint(ground, belowAlpha))
	}
	if top != "" {
		s.drawTop(area, floorData, cell, rect, top, belowAlpha)
	}
}

// drawCell draws one cell of the current floor. Unassigned ground shows a dot
// and lets the floor below show through.
func (s *State) drawCell(area *ebiten.Image, floorData *floor.Floor, cell Cell, rect image.Rectangle) {
	size := rect.Dx()
	ground, _ := floorData.Ground(cell.X, cell.Y) //nolint:errcheck // Inside the floor.
	top, _ := floorData.Top(cell.X, cell.Y)       //nolint:errcheck // Inside the floor.
	switch {
	case ground != "":
		ui.FillRect(area, rect, tint(ground, fullAlpha))
	case !s.below.On:
		ui.FillRect(area, rect, ui.ColorBackground())
	}
	if ground == "" && size >= minGridSize {
		center := ui.Center(rect)
		ui.FillRect(area, image.Rect(center.X, center.Y, center.X+dotSize, center.Y+dotSize), ui.ColorSubdued())
	}
	if top != "" {
		s.drawTop(area, floorData, cell, rect, top, fullAlpha)
	}
	if size >= minGridSize {
		ui.StrokeRect(area, rect, ui.Hairline, ui.HexAlpha(0, gridAlpha))
	}
}

func tint(id string, alpha uint8) color.RGBA {
	return ui.HexAlpha(tiles.ColorOf(id), alpha)
}

// drawTop draws what sits on a cell. Walls are a core with an arm towards
// each neighbour they join, the stand-in for the variant art a wall family
// supplies per neighbour mask.
func (s *State) drawTop(
	area *ebiten.Image, floorData *floor.Floor, cell Cell, rect image.Rectangle, top string, alpha uint8,
) {
	definition, known := tiles.Lookup(top)
	fill := ui.HexAlpha(tiles.ColorOf(top), alpha)
	size := rect.Dx()
	core := max(size/wallShare, minCore)
	inset := (size - core) / ui.Half
	switch {
	case known && definition.Joins():
		mask := floorData.Joins(cell.X, cell.Y)
		center := image.Rect(rect.Min.X+inset, rect.Min.Y+inset, rect.Max.X-inset, rect.Max.Y-inset)
		ui.FillRect(area, center, fill)
		if mask.Has(tiles.North) {
			ui.FillRect(area, image.Rect(center.Min.X, rect.Min.Y, center.Max.X, center.Min.Y), fill)
		}
		if mask.Has(tiles.South) {
			ui.FillRect(area, image.Rect(center.Min.X, center.Max.Y, center.Max.X, rect.Max.Y), fill)
		}
		if mask.Has(tiles.West) {
			ui.FillRect(area, image.Rect(rect.Min.X, center.Min.Y, center.Min.X, center.Max.Y), fill)
		}
		if mask.Has(tiles.East) {
			ui.FillRect(area, image.Rect(center.Max.X, center.Min.Y, rect.Max.X, center.Max.Y), fill)
		}
		if definition.Category == tiles.CategoryDoor && size >= minGridSize {
			ui.StrokeRect(area, center, ui.Hairline, ui.HexAlpha(0, alpha))
		}
	case known && definition.Category == tiles.CategoryPickupSmall:
		ui.FillRect(area, rect.Inset(size*smallPickup/eighths), fill)
	case known && definition.Category == tiles.CategoryPickupMedium:
		ui.FillRect(area, rect.Inset(size*mediumPickup/eighths), fill)
	default:
		ui.FillRect(area, rect.Inset(size*otherObject/eighths), fill)
	}
}

// drawAnchors marks each anchor; while picking, every anchor is highlighted.
func (s *State) drawAnchors(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	size := s.camera.cellSize()
	for _, anchor := range floorData.Anchors() {
		rect := s.camera.cellRect(Cell{X: anchor.Col, Y: anchor.Row}, grid)
		if !rect.Overlaps(grid) {
			continue
		}
		marker := rect.Inset(max(size*markerInset/eighths, 1))
		ink := ui.ColorInk()
		if anchor.ID == s.anchor || s.pick != nil {
			ink = ui.ColorSelect()
		}
		ui.StrokeRect(area, marker, ui.Outline, ink)
		ui.FillRect(area, marker.Inset(max(marker.Dx()*markerCore/eighths, 1)), ink)
		if s.pick != nil {
			ui.StrokeRect(area, rect, ui.Outline, ui.ColorSelect())
		}
		if size >= minLabel {
			labelX, labelY := rect.Max.X+ui.Half, rect.Min.Y+(size-ui.TextSize)/ui.Half-1
			width := s.theme.TextWidth(anchor.Name)
			backing := image.Rect(labelX-1, labelY, labelX+width+ui.Half, labelY+ui.TextSize+ui.Gap)
			ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
			s.theme.DrawText(area, anchor.Name, labelX, labelY, ink)
		}
	}
}

func (s *State) drawSelection(area *ebiten.Image, grid image.Rectangle) {
	for _, cell := range s.selected {
		ui.StrokeRect(area, s.camera.cellRect(cell, grid), ui.Outline, ui.ColorSelect())
	}
	if s.rect != nil {
		from := s.camera.cellRect(s.rect.from, grid)
		to := s.camera.cellRect(s.rect.to, grid)
		ui.StrokeRect(area, from.Union(to), ui.Outline, ui.ColorSelect())
	}
}

// drawPanel draws the tools, tiles and floors on the left.
func (s *State) drawPanel(screen *ebiten.Image, docs frame.Docs) {
	height := s.size.Y - frame.StatusHeight
	ui.FillRect(screen, image.Rect(0, 0, PanelWidth, height), ui.ColorPanel())
	ui.FillRect(screen, image.Rect(PanelWidth-ui.Hairline, 0, PanelWidth, height), ui.ColorRule())
	heading := func(label string, list *ui.List) {
		s.theme.Heading(screen, label, ui.Inset, list.Rect.Min.Y-headingSpace+ui.Gap)
	}
	heading("TOOLS", s.toolList)
	s.toolList.Draw(screen, s.theme)
	tileHeading := "TILES"
	switch s.tool {
	case ToolWalls:
		tileHeading = "WALL FAMILY"
	case ToolSelect:
		tileHeading = "CHANGE SELECTED WALLS TO"
	case ToolErase, ToolAnchor:
		tileHeading = ""
	case ToolGround, ToolObjects:
	}
	heading(tileHeading, s.tileList)
	s.tileList.Draw(screen, s.theme)
	heading("FLOORS (TOP FIRST)", s.floorList)
	s.floorList.Draw(screen, s.theme)
	s.below.Draw(screen, s.theme)
	_ = docs
}

// drawInspector draws the inspector: the zone and floor, then whatever is
// selected or under the cursor.
func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	textX := panel.Min.X + ui.Inset
	textY := panel.Min.Y + ui.Inset
	line := func(text string, ink color.Color) {
		s.theme.DrawText(screen, text, textX, textY, ink)
		textY += ui.LineHeight
	}
	zoneName, floorName := "", ""
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			zoneName, floorName = index.Name(s.zone, ""), index.Name(s.floor, "")
		}
	})
	line("INSPECTOR", ui.ColorMuted())
	line("Zone: "+zoneName, ui.ColorInk())
	loaded := docs.View(s.floor, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		floorData := floorDoc.Floor
		line(fmt.Sprintf("Floor: %s  %d \u00d7 %d", floorName, floorData.Width(), floorData.Height()), ui.ColorInk())
		zoom := strconv.Itoa(s.camera.cellSize())
		line("revision "+strconv.FormatUint(document.Revision, 10)+"  zoom "+zoom+" px", ui.ColorSubdued())
		textY += ui.LineHeight / ui.Half
		switch {
		case s.pick != nil:
			line("PICKING", ui.ColorSelect())
			line("Click an anchor for", ui.ColorInk())
			line(labelOr(s.pick.Label, "the other window")+".", ui.ColorInk())
			line("Escape cancels.", ui.ColorMuted())
		case !s.anchor.IsZero():
			s.drawAnchorInspector(screen, floorData, textX, &textY)
		case len(s.selected) > 0:
			line(fmt.Sprintf("%d walls selected", len(s.selected)), ui.ColorInk())
			s.replace.Draw(screen, s.theme)
			textY = familyListTop
			line("Pick a family on the left to change them.", ui.ColorMuted())
		default:
			s.drawCellInspector(screen, floorData, textX, &textY)
		}
	})
	if !loaded {
		line("Floor: "+floorName, ui.ColorInk())
	}
	s.drawHelp(screen, textX, panel.Max.Y-7*ui.LineHeight)
}

func (s *State) drawAnchorInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	anchor, _, ok := floorData.Anchor(s.anchor)
	if !ok {
		return
	}
	s.theme.DrawText(screen, "ANCHOR NAME", textX, inspectorFieldTop-ui.LineHeight, ui.ColorMuted())
	s.nameField.Draw(screen)
	hint := "Enter saves, Escape reverts"
	if s.nameField.Field.Dirty() {
		hint = "Unsaved. " + hint
	}
	top := inspectorFieldTop + ui.ButtonH + ui.Gap
	s.theme.DrawText(screen, hint, textX, top, ui.ColorMuted())
	s.theme.DrawText(screen, fmt.Sprintf("at %d, %d", anchor.Col, anchor.Row), textX, top+ui.LineHeight, ui.ColorInk())
	s.theme.DrawText(screen, "ID "+anchor.ID.String(), textX, top+2*ui.LineHeight, ui.ColorSubdued())
	s.deleteBtn.Draw(screen, s.theme)
	*textY = s.deleteBtn.Rect.Max.Y + ui.LineHeight
}

func (s *State) drawCellInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	line := func(text string, ink color.Color) {
		s.theme.DrawText(screen, text, textX, *textY, ink)
		*textY += ui.LineHeight
	}
	for _, info := range tools() {
		if info.tool == s.tool {
			line(info.label, ui.ColorInk())
			line(info.help, ui.ColorMuted())
		}
	}
	if floorData == nil || !s.hovering {
		return
	}
	*textY += ui.LineHeight / ui.Half
	ground, _ := floorData.Ground(s.hover.X, s.hover.Y) //nolint:errcheck // The hover cell is on the floor.
	top, _ := floorData.Top(s.hover.X, s.hover.Y)       //nolint:errcheck // The hover cell is on the floor.
	line(fmt.Sprintf("Cell %d, %d", s.hover.X, s.hover.Y), ui.ColorInk())
	line("Ground: "+tileName(ground), ui.ColorInk())
	line("On top: "+tileName(top), ui.ColorInk())
	shown := ground
	if top != "" {
		shown = top
	}
	if definition, ok := tiles.Lookup(shown); ok {
		properties := fmt.Sprintf("passable %s, blocks sight %s", yesNo(definition.Passable), yesNo(definition.BlocksSight))
		line(properties, ui.ColorMuted())
		line("can be picked up "+yesNo(definition.Gettable), ui.ColorMuted())
	}
}

func tileName(tileID string) string {
	if tileID == "" {
		return "nothing"
	}
	if definition, ok := tiles.Lookup(tileID); ok {
		return definition.Name
	}

	return tileID + " (unknown)"
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

func (s *State) drawHelp(screen *ebiten.Image, left, top int) {
	lines := []string{
		"KEYS",
		"G W O E A S: tools   1-9: tile",
		"PageUp/PageDown: floor   F: floor below",
		"Scroll: pan   Cmd+scroll or Cmd+=/-: zoom",
		"Space+drag: pan   Cmd+0: reset view",
		"Cmd+Z / Cmd+Shift+Z: undo / redo",
		"Cmd+N: new view   Cmd+W: close",
	}
	for index, text := range lines {
		ink := ui.ColorInk()
		if index == 0 {
			ink = ui.ColorMuted()
		}
		s.theme.DrawText(screen, text, left, top+index*ui.LineHeight, ink)
	}
}
