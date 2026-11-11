package mapview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Picking for another tab is a mode of its own: a banner says what the
// place is for, everything but the anchors fades and takes no click, and
// the inspector lists the floor's places.
const (
	pickWash        = 0xd0
	pickBannerH     = 40
	pickCancelWidth = 96
	pickListRows    = 8
	pickTagPad      = 5
	pickTipPad      = 6
	nowTag          = "now"
	pickCancelID    = "pick-cancel"
	pickBannerID    = "pick-banner-cancel"
)

// pickParts are the map's controls while it picks: the banner and its
// Cancel, the inspector's list of the floor's places and its Cancel pick,
// and the anchor the pointer is on, on the map or in the list.
type pickParts struct {
	banner       image.Rectangle
	bannerCancel *ui.Button
	list         *ui.List
	cancel       *ui.Button
	hovered      ids.ULID
}

func newPickParts() pickParts {
	return pickParts{
		banner: ui.Zero(), bannerCancel: ui.NewButton(pickBannerID, "Cancel  Esc"), list: ui.NewList("pick-places", ""),
		cancel: ui.NewButton(pickCancelID, "Cancel pick"), hovered: ids.ULID{},
	}
}

// isNow reports whether an anchor of this floor is the place the pick
// would replace.
func (s *State) isNow(anchor ids.ULID) bool {
	return s.pick != nil && s.pick.Now.Floor == s.floor && s.pick.Now.Item == anchor.String()
}

// layoutPick places the banner over the map and the inspector's list and
// Cancel pick, filling the list with the floor's places.
func (s *State) layoutPick(floorData *floor.Floor) {
	parts := &s.picking
	grid := s.Grid()
	parts.banner = image.Rect(grid.Min.X+ui.Inset, grid.Min.Y+ui.Inset, grid.Max.X-ui.Inset,
		grid.Min.Y+ui.Inset+pickBannerH)
	top := parts.banner.Min.Y + (pickBannerH-ui.ButtonH)/ui.Half
	parts.bannerCancel.Rect = image.Rect(parts.banner.Max.X-ui.Gap*ui.Half-pickCancelWidth, top,
		parts.banner.Max.X-ui.Gap*ui.Half, top+ui.ButtonH)
	panel := frame.Inspector(s.size)
	left, right := panel.Min.X+ui.Inset, panel.Max.X-ui.Inset
	parts.cancel.Rect = image.Rect(left, panel.Max.Y-ui.Inset-ui.ButtonH, right, panel.Max.Y-ui.Inset)
	listTop := s.pickListTop()
	parts.list.Rect = image.Rect(left-ui.Gap, listTop, right+ui.Gap,
		min(listTop+pickListRows*ui.RowHeight, parts.cancel.Rect.Min.Y-ui.Inset))
	parts.list.Rows = nil
	for _, anchor := range floorData.Anchors() {
		row := ui.NewRow(anchor.ID.String(), anchor.Name)
		row.Icon = ui.IconPlace
		if s.isNow(anchor.ID) {
			row.Detail = nowTag
		}
		parts.list.Rows = append(parts.list.Rows, row)
	}
	if len(parts.list.Rows) == 0 {
		row := ui.NewRow("", "No places on this floor.")
		row.Heading = true
		parts.list.Rows = append(parts.list.Rows, row)
	}
}

// pickLines are the inspector's lines above the list: what the place is
// for, wrapped, and what it is now.
func (s *State) pickLines() []string {
	panel := frame.Inspector(s.size)
	lines := s.theme.Wrap(labelOr(s.pick.Label, "the other tab"), panel.Dx()-ui.Half*ui.Inset)
	now := s.pick.Now.Name
	if now == "" {
		now = "no place yet"
	}

	return append(lines, "", "Now: "+now)
}

// pickListTop is where the list of places starts, under the lines.
func (s *State) pickListTop() int {
	panel := frame.Inspector(s.size)

	return panel.Min.Y + ui.Inset + ui.Half*ui.LineHeight + (len(s.pickLines())+ui.Half)*ui.LineHeight
}

// updatePicking takes the window's input while it picks: Cancel or Escape
// cancels, a name in the list or a ring on the map answers, and the map
// still pans and zooms. The place may be on another floor, so the floor
// list and PageUp and PageDown still change the floor shown.
func (s *State) updatePicking(input *ui.Input, ctx *frame.Context) {
	s.updateZoom(input, ctx)
	frame.CommonKeys(input, ctx)
	s.pickFloor(input, ctx)
	ctx.Docs.View(s.floor, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		s.layoutPick(floorDoc.Floor)
		parts := &s.picking
		parts.hovered = ids.ULID{}
		if anchor, err := ids.Parse(pickRowAt(parts.list, input.Cursor)); err == nil {
			parts.hovered = anchor
		}
		switch {
		case parts.bannerCancel.Update(input), parts.cancel.Update(input), input.Consume(ui.KeyEscape, ui.Plain()):
			s.cancelPick(ctx)
		case input.Pressed && !parts.hovered.IsZero():
			if anchor, _, found := floorDoc.Floor.Anchor(parts.hovered); found {
				s.answerPick(ctx, anchor)
			}
		case input.Cursor.In(parts.list.Rect), input.Cursor.In(parts.banner):
			parts.list.Update(input)
		default:
			s.updatePickGrid(input, ctx, floorDoc.Floor)
		}
	})
}

// pickFloor changes the floor shown while picking, from the floor list or
// with PageUp and PageDown.
func (s *State) pickFloor(input *ui.Input, ctx *frame.Context) {
	switch {
	case input.Consume(ui.KeyPageUp, ui.Plain()):
		s.stepFloor(1, ctx)
	case input.Consume(ui.KeyPageDown, ui.Plain()):
		s.stepFloor(-1, ctx)
	case s.floorList.Update(input).Changed:
		if id, err := ids.Parse(s.floorList.Selected); err == nil {
			s.switchFloor(id, ctx)
		}
	}
}

// updatePickGrid pans and zooms the map and answers a click on an anchor.
func (s *State) updatePickGrid(input *ui.Input, ctx *frame.Context, floorData *floor.Floor) {
	grid := s.Grid()
	s.panAndZoom(input, grid)
	cell, inGrid := s.camera.cellAt(input.Cursor, grid)
	onFloor := inGrid && floorData.In(cell.X, cell.Y)
	s.hover, s.hovering, s.outside = cell, onFloor, false
	if anchor, found := floorData.AnchorAt(cell.X, cell.Y); onFloor && found {
		s.picking.hovered = anchor.ID
	}
	if s.panning || input.Space || s.zoom.contains(input.Cursor) {
		return
	}
	s.updatePick(input, ctx, floorData, cell, onFloor)
}

// pickRowAt returns the anchor row under a point in the list, or "".
func pickRowAt(list *ui.List, point image.Point) string {
	if !point.In(list.Rect) {
		return ""
	}
	for index, row := range list.Rows {
		if !row.Heading && point.In(list.RowRect(index)) {
			return row.ID
		}
	}

	return ""
}

// cancelPick ends the pick from the map.
func (s *State) cancelPick(ctx *frame.Context) {
	ctx.Requests.PickCancel(protocol.PickCancel{Pick: s.pick.Pick, Reason: "cancelled in the map"})
	s.pick = nil
	ctx.Frame.Status("Pick cancelled")
}

// answerPick answers the pick with an anchor.
func (s *State) answerPick(ctx *frame.Context, anchor floor.Anchor) {
	ctx.Requests.PickAnswer(protocol.PickAnswer{
		Pick: s.pick.Pick, Floor: s.floor, Item: anchor.ID.String(), Name: anchor.Name,
	})
	s.pick = nil
	ctx.Frame.Status("Picked “" + anchor.Name + "”")
}

// drawPickWash fades everything on the map but its anchors, drawn again
// over the wash, and names the anchor under the pointer.
func (s *State) drawPickWash(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	ui.FillRect(area, grid, ui.HexAlpha(hexWhite, pickWash))
	s.drawAnchors(area, floorData, grid)
	anchor, _, found := floorData.Anchor(s.picking.hovered)
	if !found {
		return
	}
	rect := s.camera.cellRect(Cell{X: anchor.Col, Y: anchor.Row}, grid)
	tip := anchor.Name + " · click to choose"
	box := image.Rect(rect.Min.X, rect.Max.Y+ui.Gap, rect.Min.X+s.theme.TextWidth(tip)+ui.Half*pickTipPad,
		rect.Max.Y+ui.Gap+ui.LineHeight+pickTipPad)
	ui.FillRound(area, box, ui.Radius, ui.ColorInk())
	s.theme.DrawText(area, tip, box.Min.X+pickTipPad, box.Min.Y+pickTipPad/ui.Half+ui.Half, ui.ColorBackground())
}

// drawPickBanner draws the banner over the map: what the place is for,
// what it is now, and Cancel.
func (s *State) drawPickBanner(screen *ebiten.Image) {
	parts := &s.picking
	ui.FillRound(screen, parts.banner, ui.Radius, ui.ColorSelect())
	left := parts.banner.Min.X + ui.Inset
	middle := parts.banner.Min.Y + pickBannerH/ui.Half
	ui.DrawRing(screen, image.Pt(left+ui.IconSize/ui.Half, middle), ui.ColorBackground(), false)
	text := "Choose a place for " + labelOr(s.pick.Label, "the other tab")
	if s.pick.Now.Name != "" {
		text += "  ·  Now: " + s.pick.Now.Name
	}
	textLeft := left + ui.IconSize + ui.Gap*ui.Half
	room := parts.bannerCancel.Rect.Min.X - ui.Inset - textLeft
	for len(text) > 0 && s.theme.TextWidth(text) > room {
		text = text[:len(text)-1]
	}
	s.theme.DrawText(screen, text, textLeft, middle-ui.TextSize/ui.Half-1, ui.ColorBackground())
	ui.StrokeRound(screen, parts.bannerCancel.Rect, ui.Radius, ui.Border, ui.ColorBackground())
	label := parts.bannerCancel.Label
	s.theme.DrawText(screen, label, parts.bannerCancel.Rect.Min.X+(pickCancelWidth-s.theme.TextWidth(label))/ui.Half,
		parts.bannerCancel.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorBackground())
}

// drawPickPanel fades the tools, which rest while picking, and draws the
// floor list over the fade: it still changes the floor shown.
func (s *State) drawPickPanel(screen *ebiten.Image) {
	height := s.size.Y - frame.StatusHeight
	ui.FillRect(screen, image.Rect(0, 0, PanelWidth, height), ui.HexAlpha(hexWhite, pickWash))
	s.theme.Heading(screen, floorsHeading, ui.Inset, s.floorsHeadingTop())
	s.floorList.Draw(screen, s.theme)
	s.theme.DrawText(screen, "Tools rest while you pick.", ui.Inset, height-ui.Inset-ui.LineHeight, ui.ColorMuted())
}

// drawPickInspector draws the inspector while picking: what the place is
// for, what it is now, the floor's places and Cancel pick.
func (s *State) drawPickInspector(screen *ebiten.Image) {
	panel := frame.Inspector(s.size)
	textX, textY := panel.Min.X+ui.Inset, panel.Min.Y+ui.Inset
	s.theme.DrawLabel(screen, "PICKING A PLACE", textX, textY, ui.ColorSelect())
	textY += ui.LineHeight + ui.LineHeight/ui.Half
	s.theme.Heading(screen, "FOR", textX, textY)
	textY += ui.LineHeight
	for _, line := range s.pickLines() {
		s.theme.DrawText(screen, line, textX, textY, ui.ColorInk())
		textY += ui.LineHeight
	}
	s.theme.Heading(screen, "PLACES ON THIS FLOOR", textX, s.picking.list.Rect.Min.Y-ui.LineHeight)
	for index, row := range s.picking.list.Rows {
		if row.ID == s.picking.hovered.String() {
			ui.FillRect(screen, s.picking.list.RowRect(index), ui.ColorSelectWash())
		}
	}
	s.picking.list.Draw(screen, s.theme)
	s.picking.cancel.Draw(screen, s.theme)
}

// locatePick finds the banner's Cancel, Cancel pick or a place in the list
// by its name as "place:<name>", while picking.
func (s *State) locatePick(kind, value string) (image.Point, bool) {
	switch {
	case kind == "ui" && value == pickCancelID:
		return ui.Center(s.picking.cancel.Rect), !s.picking.cancel.Rect.Empty()
	case kind == "ui" && value == pickBannerID:
		return ui.Center(s.picking.bannerCancel.Rect), !s.picking.bannerCancel.Rect.Empty()
	case kind == "place":
		for index, row := range s.picking.list.Rows {
			if row.Label == value && !row.Heading {
				return ui.Center(s.picking.list.RowRect(index)), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// locatePicking lays out the picking controls and finds one, while picking.
func (s *State) locatePicking(docs frame.Docs, kind, value string) (image.Point, bool) {
	if s.pick == nil {
		return image.Pt(0, 0), false
	}
	found := false
	point := image.Pt(0, 0)
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			s.layoutPick(floorDoc.Floor)
			point, found = s.locatePick(kind, value)
		}
	})

	return point, found
}
