package mapview

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

const (
	belowAlpha = 0x40
	gridAlpha  = 0x22
	// outsideGridAlpha keeps the grid beyond the floor bounds quieter than
	// the floor's own.
	outsideGridAlpha = 0x12
	minGridSize      = 12
	minLabel         = 18
	wallShare        = 2 // a wall's core is half a cell wide
	minCore          = 2
	dotSize          = 2
	fullAlpha        = 0xff
	hexWhite         = 0xffffff
	// Insets, in eighths of a cell: pickups and anchors are drawn smaller
	// than a cell so the ground shows around them.
	eighths      = 8
	smallPickup  = 3
	mediumPickup = 2
	otherObject  = 1
	// An anchor is a ring a third of a cell across, at least a few pixels.
	anchorRadius    = 0.3
	minAnchorRadius = 3
	anchorRing      = 2.5
	// labelBacking is the opacity behind an anchor name, so it reads over
	// whatever lies under it.
	labelBacking = 0xd0
	// hexOrigin marks the zone origin and its axes; axisAlpha keeps the axes
	// quieter than the origin cell's outline.
	hexOrigin = 0xc2410c
	// hexBlue is the selection blue, for marks drawn with partial alpha.
	hexBlue   = 0x1f5fbf
	axisAlpha = 0x99
	// A stairs chevron spans a third of the cell's width and a sixth of its
	// height either side of the centre.
	chevronWidth  = 3
	chevronHeight = 6
)

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layoutWidgets(docs)
	s.sprites.refresh(docs, s.camera.tile)
	screen.Fill(ui.ColorBackground())
	grid := s.Grid()
	area, ok := screen.SubImage(grid).(*ebiten.Image)
	if ok {
		area.Fill(ui.ColorCanvas())
		s.drawFloors(area, docs, grid)
		if !s.floor.IsZero() {
			s.zoom.layout(grid, s.camera)
			s.zoom.draw(area, s.theme)
		}
	}
	s.drawPanel(screen, docs)
	s.drawInspector(screen, docs)
	if s.menu != nil {
		s.menu.Draw(screen, s.theme)
	}
	s.keySheet.Draw(screen, s.theme)
	s.zonePalettes.DrawMenu(screen)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
	s.guard.Draw(screen, s.theme)
}

func (s *State) drawFloors(area *ebiten.Image, docs frame.Docs, grid image.Rectangle) {
	if s.floor.IsZero() {
		s.theme.DrawText(area, "This zone has no floors yet. Add one in the project browser.",
			grid.Min.X+ui.Inset, grid.Min.Y+ui.Inset, ui.ColorMuted())

		return
	}
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, isFloor := document.Model.(*tilefloor.Document); isFloor {
			s.drawOutside(area, floorDoc.Floor.Bounds(), grid)
		}
	})
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
		s.drawFloorEdges(area, floorDoc.Floor, grid)
		s.drawOrigin(area, grid)
		s.drawAnchors(area, floorDoc.Floor, grid)
		if s.preview != nil || s.remote != nil {
			s.drawLiveNPCs(area, grid)
		} else {
			s.drawPlacements(area, floorDoc.Floor, grid)
		}
		s.drawSelection(area, grid)
		s.drawPlayer(area, grid)
		s.drawStartCell(area, floorDoc.Floor, grid)
	})
	if !loaded {
		s.theme.DrawText(area, "Loading the floor…", grid.Min.X+ui.Inset, grid.Min.Y+ui.Inset, ui.ColorMuted())
	}
}

// drawOutside shades the zone beyond the floor bounds and draws a fainter
// grid over it, so the space around the floor can be pictured, then clears
// the floor's own rectangle for the floors drawn over it.
func (s *State) drawOutside(area *ebiten.Image, bounds, grid image.Rectangle) {
	ui.FillRect(area, grid, ui.ColorOutside())
	if s.camera.cellSize() >= minGridSize {
		ink := ui.HexAlpha(0, outsideGridAlpha)
		cells := s.camera.inView(grid)
		for col := cells.Min.X; col <= cells.Max.X; col++ {
			left := s.camera.cellRect(Cell{X: col, Y: 0}, grid).Min.X
			ui.FillRect(area, image.Rect(left, grid.Min.Y, left+ui.Hairline, grid.Max.Y), ink)
		}
		for row := cells.Min.Y; row <= cells.Max.Y; row++ {
			top := s.camera.cellRect(Cell{X: 0, Y: row}, grid).Min.Y
			ui.FillRect(area, image.Rect(grid.Min.X, top, grid.Max.X, top+ui.Hairline), ink)
		}
	}
	ui.FillRect(area, s.camera.rectOf(bounds, grid), ui.ColorBackground())
	if s.outside {
		ui.StrokeRect(area, s.camera.cellRect(s.hover, grid), ui.Hairline, ui.ColorSubdued())
	}
}

// drawCells draws every visible cell. The floor below is drawn faintly, with
// nothing for unassigned ground, so the floor above can be traced over it.
func (s *State) drawCells(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle, faint bool) {
	shown := s.camera.visible(floorData.Bounds(), grid)
	for row := shown.Min.Y; row < shown.Max.Y; row++ {
		for col := shown.Min.X; col < shown.Max.X; col++ {
			cell := Cell{X: col, Y: row}
			rect := s.camera.cellRect(cell, grid)
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
		s.drawGround(area, rect, ground, belowAlpha)
	}
	if top != "" {
		s.drawTop(area, floorData, cell, rect, top, belowAlpha)
	}
}

// drawGround draws a cell's ground: the sprite that shows it, over the
// window's background, or its flat colour.
func (s *State) drawGround(area *ebiten.Image, rect image.Rectangle, ground string, alpha uint8) {
	picture, shown := s.sprites.image(ground)
	if !shown {
		ui.FillRect(area, rect, tint(ground, alpha))

		return
	}
	ui.FillRect(area, rect, ui.HexAlpha(hexWhite, alpha))
	drawSprite(area, picture, rect, alpha)
}

// drawCell draws one cell of the current floor. Unassigned ground shows a dot
// and lets the floor below show through.
func (s *State) drawCell(area *ebiten.Image, floorData *floor.Floor, cell Cell, rect image.Rectangle) {
	size := rect.Dx()
	ground, _ := floorData.Ground(cell.X, cell.Y) //nolint:errcheck // Inside the floor.
	top, _ := floorData.Top(cell.X, cell.Y)       //nolint:errcheck // Inside the floor.
	switch {
	case ground != "":
		s.drawGround(area, rect, ground, fullAlpha)
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

// drawOrigin marks the zone origin, cell 0,0, that every floor of the zone
// is positioned from: an axis along the top and the left edge of the cell
// across the whole view, an outline round the cell and a label.
func (s *State) drawOrigin(area *ebiten.Image, grid image.Rectangle) {
	cell := s.camera.cellRect(image.Pt(0, 0), grid)
	axis := ui.HexAlpha(hexOrigin, axisAlpha)
	ui.FillRect(area, image.Rect(grid.Min.X, cell.Min.Y, grid.Max.X, cell.Min.Y+ui.Hairline), axis)
	ui.FillRect(area, image.Rect(cell.Min.X, grid.Min.Y, cell.Min.X+ui.Hairline, grid.Max.Y), axis)
	ui.StrokeRect(area, cell, ui.Outline, ui.Hex(hexOrigin))
	if cell.Dx() < minLabel {
		return
	}
	label := "0,0"
	labelX, labelY := cell.Min.X+ui.Half, cell.Min.Y-ui.TextSize-ui.Gap-ui.Half
	backing := image.Rect(labelX-1, labelY, labelX+s.theme.TextWidth(label)+ui.Half, labelY+ui.TextSize+ui.Gap)
	ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
	s.theme.DrawText(area, label, labelX, labelY, ui.Hex(hexOrigin))
}

func tint(id string, alpha uint8) color.RGBA {
	return ui.HexAlpha(tiles.ColorOf(id), alpha)
}

// drawTop draws what sits on a cell: the sprite that shows it, filling the
// cell, or its stand-in. Walls are a core with an arm towards each neighbour
// they join, the stand-in for the variant art a wall family supplies per
// neighbour mask.
func (s *State) drawTop(
	area *ebiten.Image, floorData *floor.Floor, cell Cell, rect image.Rectangle, top string, alpha uint8,
) {
	if picture, shown := s.sprites.image(top); shown {
		drawSprite(area, picture, rect, alpha)

		return
	}
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
	case known && tiles.Climb(top) != 0:
		s.drawStairs(area, rect.Inset(size*otherObject/eighths), fill, tiles.Climb(top) > 0, alpha)
	default:
		ui.FillRect(area, rect.Inset(size*objectInset(definition.Category)/eighths), fill)
	}
}

// objectInset is how far, in eighths of a cell, a tile on top is drawn inside
// its cell: pickups smaller, so their size is recognisable.
func objectInset(category tiles.Category) int {
	switch category {
	case tiles.CategoryPickupSmall:
		return smallPickup
	case tiles.CategoryPickupMedium:
		return mediumPickup
	case tiles.CategoryGround, tiles.CategoryWall, tiles.CategoryDoor, tiles.CategoryStairsUp,
		tiles.CategoryStairsDown:
		return otherObject
	default:
		return otherObject
	}
}

// drawStairs draws stairs as a block with a chevron pointing up or down.
func (s *State) drawStairs(area *ebiten.Image, rect image.Rectangle, fill color.Color, upwards bool, alpha uint8) {
	ui.FillRect(area, rect, fill)
	if rect.Dx() < minGridSize {
		return
	}
	centre := ui.Center(rect)
	width, height := rect.Dx()/chevronWidth, rect.Dy()/chevronHeight
	tip, foot := centre.Y-height, centre.Y+height
	if !upwards {
		tip, foot = foot, tip
	}
	ink := ui.HexAlpha(hexWhite, alpha)
	ui.Line(area, image.Pt(centre.X-width, foot), image.Pt(centre.X, tip), ui.Outline, ink)
	ui.Line(area, image.Pt(centre.X, tip), image.Pt(centre.X+width, foot), ui.Outline, ink)
}

// drawAnchors marks each anchor with a blue ring and its name; the selected
// anchor's ring is filled, and while picking every anchor's cell is
// outlined.
func (s *State) drawAnchors(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	size := s.camera.cellSize()
	blue := ui.ColorSelect()
	for _, anchor := range floorData.Anchors() {
		rect := s.camera.cellRect(Cell{X: anchor.Col, Y: anchor.Row}, grid)
		if !rect.Overlaps(grid) {
			continue
		}
		centreX, centreY := float32(rect.Min.X)+float32(size)/ui.Half, float32(rect.Min.Y)+float32(size)/ui.Half
		radius := max(float32(size)*anchorRadius, minAnchorRadius)
		fill := ui.ColorBackground()
		if anchor.ID == s.anchor {
			fill = blue
		}
		vector.FillCircle(area, centreX, centreY, radius, fill, true)
		vector.StrokeCircle(area, centreX, centreY, radius, anchorRing, blue, true)
		if s.pick != nil {
			ui.StrokeRect(area, rect, ui.Outline, blue)
		}
		if size >= minLabel {
			labelX, labelY := rect.Max.X+ui.Half, rect.Min.Y+(size-ui.LabelSize)/ui.Half-ui.Half
			width := s.theme.SmallWidth(anchor.Name)
			backing := image.Rect(labelX-1, labelY, labelX+width+ui.Half, labelY+ui.LabelSize+ui.Gap+ui.Half)
			ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
			s.theme.DrawSmall(area, anchor.Name, labelX, labelY, blue)
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
	ui.FillRect(screen, image.Rect(0, 0, PanelWidth, height), ui.ColorBackground())
	ui.Divider(screen, image.Pt(PanelWidth-ui.Outline, 0), image.Pt(PanelWidth-ui.Outline, height))
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
	case ToolNPC:
		tileHeading = "NPC TO PLACE"
		if len(s.tileList.Rows) == 0 {
			s.theme.DrawText(screen, "No NPCs yet. Use New NPC", ui.Inset, s.tileList.Rect.Min.Y+ui.Gap, ui.ColorMuted())
			s.theme.DrawText(screen, "in the project browser.", ui.Inset, s.tileList.Rect.Min.Y+ui.Gap+ui.LineHeight,
				ui.ColorMuted())
		}
	case ToolErase, ToolAnchor, ToolResize:
		tileHeading = ""
	case ToolGround, ToolObjects:
	}
	heading(tileHeading, s.tileList)
	s.tileList.Draw(screen, s.theme)
	heading("FLOORS (TOP FIRST)", s.floorList)
	s.floorList.Draw(screen, s.theme)
	s.below.Draw(screen, s.theme)
	s.previewBn.Draw(screen, s.theme)
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
	s.theme.Heading(screen, "INSPECTOR", textX, textY)
	textY += ui.LineHeight
	line("Zone: "+zoneName, ui.ColorInk())
	if s.zonePalettesShown() {
		s.zonePalettes.Draw(screen)
		textY = s.zoneListFoot + ui.Inset
	}
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
			s.theme.DrawLabel(screen, "PICKING", textX, textY, ui.ColorSelect())
			textY += ui.LineHeight
			line("Click an anchor for", ui.ColorInk())
			line(labelOr(s.pick.Label, "the other window")+".", ui.ColorInk())
			line("Escape cancels.", ui.ColorMuted())
		case !s.anchor.IsZero():
			s.drawAnchorInspector(screen, floorData, textX, &textY)
		case !s.placement.IsZero():
			s.drawPlacementInspector(screen, floorData, textX, &textY)
		case len(s.selected) > 0:
			line(fmt.Sprintf("%d walls selected", len(s.selected)), ui.ColorInk())
			s.replace.Draw(screen, s.theme)
			textY = familyListTop
			line("Pick a family on the left to change them.", ui.ColorMuted())
		case s.tool == ToolResize:
			s.drawResizeInspector(screen, floorData, textX, &textY)
			s.drawHover(screen, floorData, textX, &textY)
		default:
			s.drawCellInspector(screen, floorData, textX, &textY)
		}
	})
	if !loaded {
		line("Floor: "+floorName, ui.ColorInk())
	}
	s.drawPreview(screen, docs, textX, footTop(panel))
	s.keysBn.Draw(screen, s.theme)
}

func (s *State) drawAnchorInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	anchor, _, ok := floorData.Anchor(s.anchor)
	if !ok {
		return
	}
	s.theme.Heading(screen, "ANCHOR NAME", textX, inspectorFieldTop-ui.LineHeight)
	s.nameField.Draw(screen)
	top := inspectorFieldTop + ui.ButtonH + ui.Gap
	s.theme.DrawText(screen, fmt.Sprintf("at %d, %d", anchor.Col, anchor.Row), textX, top, ui.ColorInk())
	s.theme.DrawText(screen, "ID "+anchor.ID.String(), textX, top+ui.LineHeight, ui.ColorSubdued())
	s.save.Draw(screen, s.theme)
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
	if floorData == nil || (!s.hovering && !s.outside) {
		return
	}
	*textY += ui.LineHeight / ui.Half
	s.drawHover(screen, floorData, textX, textY)
}

// drawHover describes the cell under the cursor.
func (s *State) drawHover(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	line := func(text string, ink color.Color) {
		s.theme.DrawText(screen, text, textX, *textY, ink)
		*textY += ui.LineHeight
	}
	if s.outside {
		line(outsideLine(s.hover), ui.ColorMuted())

		return
	}
	if !s.hovering {
		return
	}
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

// outsideLine describes a cell beyond the floor bounds.
func outsideLine(cell Cell) string {
	return fmt.Sprintf("Cell %d, %d · outside the floor", cell.X, cell.Y)
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

// keyRows lists the keys of the Map window for its key sheet.
func keyRows() []ui.KeyRow {
	return []ui.KeyRow{
		{Label: "Tools", Keys: []string{"G", "W", "O", "E", "A", "N", "S", "R"}},
		{Label: "Tile", Keys: []string{"1\u20139"}},
		{Label: "Floor above, below", Keys: []string{"PageUp", "PageDown"}},
		{Label: "Show floor below", Keys: []string{"F"}},
		{Label: "Pan", Keys: []string{"Scroll", "Space+drag", "Middle-drag"}},
		{Label: "Zoom", Keys: []string{"Cmd+scroll", "Cmd+=", "Cmd+-"}},
		{Label: "Reset view", Keys: []string{"Cmd+0"}},
		{Label: "Menu for a cell", Keys: []string{"Right-click", "Ctrl+click"}},
		{Label: "Preview, walk", Keys: []string{"P", "Arrows"}},
		{Label: "Delete selected", Keys: []string{"Backspace"}},
		{Label: "Undo, redo", Keys: []string{"Cmd+Z", "Cmd+Shift+Z"}},
		{Label: "New view of the zone", Keys: []string{"Cmd+N"}},
		{Label: "Save", Keys: []string{"Cmd+S"}},
		{Label: "Close window", Keys: []string{"Cmd+W"}},
		{Label: "Quit the editor", Keys: []string{"Cmd+Q"}},
		{Label: "This list", Keys: []string{"?"}},
	}
}
