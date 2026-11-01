package workflowview

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// The legend is a card over the canvas's bottom-right corner, above the
// zoom control, saying what each look of a line means.
const (
	legendCard   = 236
	legendSample = 36
	legendPad    = 10
	legendGap    = 10
)

// legendRow is one look of a line and what it means.
type legendRow struct {
	text   string
	dashed bool
	width  float32
	ink    func() color.RGBA
}

// legendRows lists the looks in the order the legend shows them.
func legendRows() []legendRow {
	return []legendRow{
		{text: "Goes forward", dashed: false, width: lineWidth, ink: ui.ColorInk},
		{text: "Loops back", dashed: true, width: lineWidth, ink: ui.ColorInk},
		{text: "A time switch's otherwise", dashed: false, width: lineWidth, ink: ui.ColorMuted},
		{text: "Coloured by its author", dashed: false, width: lineWidth, ink: func() color.RGBA { return tints()[0].ink }},
		{text: "Selected, or of the selected node", dashed: false, width: selectedLine, ink: ui.ColorSelect},
		{text: "Live: an NPC just took it", dashed: false, width: selectedLine, ink: ui.ColorLive},
	}
}

// legendLayout is where the legend card sits while the toolbar's Legend is
// on and a workflow is open: nowhere otherwise.
func (s *State) legendLayout() image.Rectangle {
	if !s.legend.On || s.workflow == nil {
		return ui.Zero()
	}
	height := ui.Half*legendPad + len(legendRows())*ui.LineHeight
	right, bottom := s.Canvas().Max.X-zoomInset, s.zoomCard.card.Rect.Min.Y-legendGap

	return image.Rect(right-legendCard, bottom-height, right, bottom)
}

// drawLegend draws the legend card: a sample of each look beside what it
// means.
func (s *State) drawLegend(screen *ebiten.Image) {
	box := s.legendBox
	if box.Empty() {
		return
	}
	ui.FillRound(screen, box.Add(image.Pt(0, zoomShadow)), ui.Radius, ui.HexAlpha(0, zoomShade))
	ui.Box(screen, box, ui.ColorBackground())
	top := box.Min.Y + legendPad
	for _, row := range legendRows() {
		middle := top + ui.LineHeight/ui.Half
		start, end := image.Pt(box.Min.X+legendPad, middle), image.Pt(box.Min.X+legendPad+legendSample, middle)
		if row.dashed {
			dashedLine(screen, start, end, row.width, row.ink())
		} else {
			ui.Line(screen, start, end, row.width, row.ink())
		}
		s.theme.DrawText(screen, row.text, end.X+legendGap, top+(ui.LineHeight-ui.TextSize)/ui.Half, ui.ColorInk())
		top += ui.LineHeight
	}
}
