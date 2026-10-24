package mapview

import (
	"fmt"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Resize tool sizes, in screen pixels.
const (
	// edgeGrab is how close to an edge of the floor a press grabs it.
	edgeGrab = 6
	// sideLabelWidth and sideButtonWidth lay out the inspector's rows of
	// Remove and Add buttons, one row per side.
	sideLabelWidth  = 64
	sideButtonWidth = 88
	sidesTop        = inspectorFieldTop + 4*ui.LineHeight
	// removedWash and addedWash shade the cells a resize removes and adds.
	removedWash = 0x55
	addedWash   = 0x88
)

// Sides of a floor.
const (
	sideLeft   = "left"
	sideRight  = "right"
	sideTop    = "top"
	sideBottom = "bottom"
)

// edges says which sides of the floor a drag moves.
type edges struct {
	left, top, right, bottom bool
}

func (e edges) any() bool { return e.left || e.top || e.right || e.bottom }

// resizeDrag is a drag of one edge or a corner, from the floor's bounds when
// it began to the bounds it would give.
type resizeDrag struct {
	edges edges
	from  image.Rectangle
	to    image.Rectangle
}

// pendingResize is a resize waiting for the author to confirm removing
// anchors.
type pendingResize struct {
	floor  ids.ULID
	bounds image.Rectangle
	ground string
	// npcs counts the NPCs the resize takes off the map.
	npcs int
}

// sideButtons removes or adds one row or column on one side of the floor.
type sideButtons struct {
	side   string
	label  string
	shrink *ui.Button
	grow   *ui.Button
}

func newSideButtons() []sideButtons {
	named := []struct{ id, label string }{
		{sideLeft, "Left"}, {sideRight, "Right"}, {sideTop, "Top"}, {sideBottom, "Bottom"},
	}
	sides := make([]sideButtons, 0, len(named))
	for _, side := range named {
		sides = append(sides, sideButtons{
			side: side.id, label: side.label,
			shrink: ui.NewButton("shrink-"+side.id, "Remove"),
			grow:   ui.NewButton("grow-"+side.id, "Add"),
		})
	}

	return sides
}

// layoutSides places the side buttons in the inspector, or hides them.
func (s *State) layoutSides(left int, shown bool) {
	for index, side := range s.sides {
		side.shrink.Rect, side.grow.Rect = ui.Zero(), ui.Zero()
		if !shown {
			continue
		}
		top := sidesTop + index*(ui.ButtonH+ui.Gap)
		shrinkLeft := left + sideLabelWidth
		side.shrink.Rect = image.Rect(shrinkLeft, top, shrinkLeft+sideButtonWidth, top+ui.ButtonH)
		growLeft := side.shrink.Rect.Max.X + ui.Gap
		side.grow.Rect = image.Rect(growLeft, top, growLeft+sideButtonWidth, top+ui.ButtonH)
	}
}

// updateSides handles the side buttons: each adds or removes one row or
// column, as one undo step.
func (s *State) updateSides(input *ui.Input, ctx *frame.Context, floorData *floor.Floor) {
	bounds := floorData.Bounds()
	for _, side := range s.sides {
		side.shrink.Disabled = sideLength(bounds, side.side) <= 1
		side.grow.Disabled = sideLength(bounds, side.side) >= floor.MaxSide
		switch {
		case side.grow.Update(input):
			s.requestResize(ctx, floorData, moveSide(bounds, side.side, 1))
		case side.shrink.Update(input):
			s.requestResize(ctx, floorData, moveSide(bounds, side.side, -1))
		}
	}
}

// sideLength returns how many cells a row or column added on a side would
// hold against: the width for the left and right sides, else the height.
func sideLength(bounds image.Rectangle, side string) int {
	if side == sideLeft || side == sideRight {
		return bounds.Dx()
	}

	return bounds.Dy()
}

// moveSide moves one side of bounds outwards by delta cells; a negative delta
// moves it inwards.
func moveSide(bounds image.Rectangle, side string, delta int) image.Rectangle {
	switch side {
	case sideLeft:
		bounds.Min.X -= delta
	case sideRight:
		bounds.Max.X += delta
	case sideTop:
		bounds.Min.Y -= delta
	case sideBottom:
		bounds.Max.Y += delta
	}

	return bounds
}

// updateResize handles the Resize tool: press on an edge or corner of the
// floor, drag it to a cell boundary, and release to resize.
func (s *State) updateResize(input *ui.Input, ctx *frame.Context, floorData *floor.Floor) {
	bounds := floorData.Bounds()
	if input.Pressed && input.Cursor.In(s.Grid()) {
		grabbed := s.edgesAt(input.Cursor, bounds)
		if !grabbed.any() {
			ctx.Frame.Status("Drag an edge or corner of the floor to resize it")

			return
		}
		s.resizing = &resizeDrag{edges: grabbed, from: bounds, to: bounds}
	}
	if s.resizing == nil {
		return
	}
	if input.Down {
		s.resizing.to = s.dragBounds(input.Cursor)

		return
	}
	drag := s.resizing
	s.resizing = nil
	if drag.to != drag.from && drag.from == bounds {
		s.requestResize(ctx, floorData, drag.to)
	}
}

// edgesAt returns the edges of the floor within reach of a screen point;
// near a corner, both of its edges.
func (s *State) edgesAt(point image.Point, bounds image.Rectangle) edges {
	rect := s.camera.rectOf(bounds, s.Grid())
	alongX := point.X >= rect.Min.X-edgeGrab && point.X <= rect.Max.X+edgeGrab
	alongY := point.Y >= rect.Min.Y-edgeGrab && point.Y <= rect.Max.Y+edgeGrab
	toLeft, toRight := abs(point.X-rect.Min.X), abs(point.X-rect.Max.X)
	toTop, toBottom := abs(point.Y-rect.Min.Y), abs(point.Y-rect.Max.Y)

	return edges{
		left: alongY && toLeft <= edgeGrab && toLeft <= toRight, right: alongY && toRight <= edgeGrab && toRight < toLeft,
		top: alongX && toTop <= edgeGrab && toTop <= toBottom, bottom: alongX && toBottom <= edgeGrab && toBottom < toTop,
	}
}

// dragBounds returns the bounds the current drag gives: the dragged edges at
// the cell boundary nearest the cursor, keeping at least one cell and at most
// floor.MaxSide on each side.
func (s *State) dragBounds(cursor image.Point) image.Rectangle {
	drag := s.resizing
	nearest := s.camera.boundary(cursor, s.Grid())
	next := drag.from
	if drag.edges.left {
		next.Min.X = min(max(nearest.X, next.Max.X-floor.MaxSide), next.Max.X-1)
	}
	if drag.edges.right {
		next.Max.X = max(min(nearest.X, next.Min.X+floor.MaxSide), next.Min.X+1)
	}
	if drag.edges.top {
		next.Min.Y = min(max(nearest.Y, next.Max.Y-floor.MaxSide), next.Max.Y-1)
	}
	if drag.edges.bottom {
		next.Max.Y = max(min(nearest.Y, next.Min.Y+floor.MaxSide), next.Min.Y+1)
	}

	return next
}

// requestResize resizes the current floor. When anchors would be removed it
// first shows where they are used and asks (D4).
func (s *State) requestResize(ctx *frame.Context, floorData *floor.Floor, next image.Rectangle) {
	resize := pendingResize{floor: s.floor, bounds: next, ground: s.newCellGround(floorData), npcs: 0}
	for _, placement := range floorData.Placements() {
		if !image.Pt(placement.Col, placement.Row).In(next) {
			resize.npcs++
		}
	}
	var lost []floor.Anchor
	for _, anchor := range floorData.Anchors() {
		if !image.Pt(anchor.Col, anchor.Row).In(next) {
			lost = append(lost, anchor)
		}
	}
	if len(lost) == 0 {
		s.sendResize(ctx, resize)

		return
	}
	var uses []protocol.Use
	remaining := len(lost)
	for _, anchor := range lost {
		ctx.Requests.WhereUsed(resize.floor, anchor.ID.String(), func(found []protocol.Use, err error) {
			remaining--
			if err == nil {
				uses = append(uses, found...)
			}
			if remaining == 0 {
				s.confirmResize(resize, lost, uses)
			}
		})
	}
}

func (s *State) confirmResize(resize pendingResize, lost []floor.Anchor, uses []protocol.Use) {
	names := make([]string, 0, len(lost))
	for _, anchor := range lost {
		names = append(names, anchor.Name)
	}
	lines := []string{
		fmt.Sprintf("This removes %d %s: %s.", len(lost), plural(len(lost), "anchor", "anchors"), strings.Join(names, ", ")),
	}
	if len(uses) > 0 {
		lines = append(lines, fmt.Sprintf("%s used in %d %s:",
			plural(len(lost), "It is", "They are"), len(uses), plural(len(uses), "place", "places")))
		for _, use := range uses {
			lines = append(lines, "  "+use.Name+" ("+use.Label+")")
		}
		lines = append(lines, "Those references will be broken.")
	}
	lines = append(lines, "You can undo this.")
	s.dialog = ui.NewDialog("Resize the floor?", lines,
		ui.DialogButton("resize", "Resize"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialogKind = dialogResize
	s.pendingResize = &resize
}

func (s *State) sendResize(ctx *frame.Context, resize pendingResize) {
	args := tilefloor.ResizeArgs{
		Bounds: tilefloor.BoundsOf(resize.bounds), Ground: resize.ground, Cells: nil, Anchors: nil, Placements: nil,
	}
	s.command(ctx, resize.floor, tilefloor.ResizeEdit, "", args, func(protocol.Result) {
		removed := ""
		if resize.npcs > 0 {
			removed = fmt.Sprintf(" It took %d %s off the map.", resize.npcs, plural(resize.npcs, "NPC", "NPCs"))
		}
		ctx.Frame.Status(fmt.Sprintf("The floor is now %d × %d.%s Undo puts it back.",
			resize.bounds.Dx(), resize.bounds.Dy(), removed))
	})
}

// newCellGround returns the ground for cells a resize adds: the floor's most
// common ground on the bottom floor, and unassigned ground on the floors
// above it, as new floors there start.
func (s *State) newCellGround(floorData *floor.Floor) string {
	if indexOf(s.floors, s.floor) > 0 {
		return ""
	}
	counts := map[string]int{}
	bounds := floorData.Bounds()
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for col := bounds.Min.X; col < bounds.Max.X; col++ {
			ground, _ := floorData.Ground(col, row) //nolint:errcheck // Inside the floor.
			counts[ground]++
		}
	}
	best, most := "", 0
	for ground, count := range counts {
		if count > most || (count == most && ground < best) {
			best, most = ground, count
		}
	}

	return best
}

// locateEdge returns the middle of one edge of the current floor on screen.
func (s *State) locateEdge(side string, docs frame.Docs) (image.Point, bool) {
	var rect image.Rectangle
	loaded := docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			rect = s.camera.rectOf(floorDoc.Floor.Bounds(), s.Grid())
		}
	})
	if !loaded {
		return image.Pt(0, 0), false
	}
	middle := ui.Center(rect)
	switch side {
	case sideLeft:
		return image.Pt(rect.Min.X, middle.Y), true
	case sideRight:
		return image.Pt(rect.Max.X, middle.Y), true
	case sideTop:
		return image.Pt(middle.X, rect.Min.Y), true
	case sideBottom:
		return image.Pt(middle.X, rect.Max.Y), true
	}

	return image.Pt(0, 0), false
}

// hasBounds reports whether the current floor has bounds "C,R,W,H".
func (s *State) hasBounds(value string, docs frame.Docs) bool {
	var col, row, width, height int
	if _, err := fmt.Sscanf(value, "%d,%d,%d,%d", &col, &row, &width, &height); err != nil {
		return false
	}
	matches := false
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			matches = floorDoc.Floor.Bounds() == image.Rect(col, row, col+width, row+height)
		}
	})

	return matches
}

// drawFloorEdges outlines the floor. With the Resize tool, the edges within
// reach of the cursor are drawn thick, and a drag shows the new outline with
// the cells it removes and adds shaded.
func (s *State) drawFloorEdges(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	bounds := floorData.Bounds()
	rect := s.camera.rectOf(bounds, grid)
	ui.StrokeRect(area, rect, ui.Hairline, ui.ColorMuted())
	if s.tool != ToolResize || s.pick != nil {
		return
	}
	if s.resizing == nil {
		s.drawGrabbedEdges(area, rect, s.edgesAt(s.cursor, bounds))

		return
	}
	next := s.resizing.to
	for _, cells := range []struct {
		rects []image.Rectangle
		wash  uint8
		hex   uint32
	}{{subtract(bounds, next), removedWash, hexRemoved}, {subtract(next, bounds), addedWash, hexAdded}} {
		for _, removed := range cells.rects {
			ui.FillRect(area, s.camera.rectOf(removed, grid), ui.HexAlpha(cells.hex, cells.wash))
		}
	}
	ui.StrokeRect(area, s.camera.rectOf(next, grid), ui.Outline, ui.ColorSelect())
}

// Shades for the cells a resize removes and adds.
const (
	hexRemoved = 0xb3261e
	hexAdded   = 0x9fbef0
)

func (s *State) drawGrabbedEdges(area *ebiten.Image, rect image.Rectangle, grabbed edges) {
	thick := func(line image.Rectangle) { ui.FillRect(area, line, ui.ColorSelect()) }
	reach := ui.Outline
	if grabbed.left {
		thick(image.Rect(rect.Min.X-reach, rect.Min.Y, rect.Min.X+reach, rect.Max.Y))
	}
	if grabbed.right {
		thick(image.Rect(rect.Max.X-reach, rect.Min.Y, rect.Max.X+reach, rect.Max.Y))
	}
	if grabbed.top {
		thick(image.Rect(rect.Min.X, rect.Min.Y-reach, rect.Max.X, rect.Min.Y+reach))
	}
	if grabbed.bottom {
		thick(image.Rect(rect.Min.X, rect.Max.Y-reach, rect.Max.X, rect.Max.Y+reach))
	}
}

// subtract returns rectangles covering the cells of from outside other.
func subtract(from, other image.Rectangle) []image.Rectangle {
	kept := from.Intersect(other)
	if kept.Empty() {
		return []image.Rectangle{from}
	}
	parts := []image.Rectangle{
		image.Rect(from.Min.X, from.Min.Y, from.Max.X, kept.Min.Y),
		image.Rect(from.Min.X, kept.Max.Y, from.Max.X, from.Max.Y),
		image.Rect(from.Min.X, kept.Min.Y, kept.Min.X, kept.Max.Y),
		image.Rect(kept.Max.X, kept.Min.Y, from.Max.X, kept.Max.Y),
	}
	result := parts[:0]
	for _, part := range parts {
		if !part.Empty() {
			result = append(result, part)
		}
	}

	return result
}

// drawResizeInspector shows the floor's size and where it lies around the
// zone origin, with a row of buttons per side.
func (s *State) drawResizeInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	bounds := floorData.Bounds()
	if s.resizing != nil {
		bounds = s.resizing.to
	}
	line := func(text string, muted bool) {
		ink := ui.ColorInk()
		if muted {
			ink = ui.ColorMuted()
		}
		s.theme.DrawText(screen, text, textX, *textY, ink)
		*textY += ui.LineHeight
	}
	line("Resize floor", false)
	line("Drag an edge or corner, or use the buttons.", true)
	*textY = sidesTop - 2*ui.LineHeight - ui.Gap
	line(fmt.Sprintf("%d × %d: columns %d to %d, rows %d to %d",
		bounds.Dx(), bounds.Dy(), bounds.Min.X, bounds.Max.X-1, bounds.Min.Y, bounds.Max.Y-1), false)
	line("0,0 is the zone origin; every floor shares it.", true)
	for _, side := range s.sides {
		labelY := side.shrink.Rect.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
		s.theme.DrawText(screen, side.label, textX, labelY, ui.ColorInk())
		side.shrink.Draw(screen, s.theme)
		side.grow.Draw(screen, s.theme)
	}
	*textY = sidesTop + len(s.sides)*(ui.ButtonH+ui.Gap) + ui.LineHeight/ui.Half
}
