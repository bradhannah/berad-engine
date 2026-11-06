package browser

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// inspectorScroll is how far a document's inspector is scrolled: what is
// below its header moves up under it, and what went past the save bar
// comes into view. most is the furthest it scrolls, as last laid out; of
// is the row it is scrolled for, as another row starts at the top.
type inspectorScroll struct {
	offset, most int
	of           string
}

// inspectorBody is the part of a document's inspector that scrolls: below
// the header's rule, above the save bar.
func (s *State) inspectorBody() image.Rectangle {
	panel := frame.Inspector(s.size)
	bottom := panel.Max.Y
	if !s.save.Rect.Empty() {
		bottom = s.save.Rect.Min.Y - footerGap
	}

	return image.Rect(panel.Min.X, headerRule+ui.Hairline, panel.Max.X, bottom)
}

// fitScroll keeps the inspector scrolled no further than what is laid out
// down to bottom needs, and lays it out again when that moved it.
func (s *State) fitScroll(docs frame.Docs, bottom int) {
	s.scroll.most = max(bottom+s.scroll.offset-s.inspectorBody().Max.Y, 0)
	if s.scroll.offset > s.scroll.most {
		s.scroll.offset = s.scroll.most
		s.layoutInspector(docs)
	}
}

// scrollInspector scrolls a document's inspector with the wheel over it.
// A press on the save bar or Delete, or on the header, goes to them alone,
// never to what is hidden behind them.
func (s *State) scrollInspector(input *ui.Input, ctx *frame.Context) {
	if s.scroll.most == 0 || !input.Cursor.In(frame.Inspector(s.size)) {
		return
	}
	if input.WheelY != 0 {
		s.scroll.offset = min(max(s.scroll.offset-int(input.WheelY*ui.RowHeight), 0), s.scroll.most)
		input.WheelY = 0
		s.layout(ctx.Docs)
	}
	if input.Cursor.In(s.inspectorBody()) {
		return
	}
	if !s.saveBar(input, ctx) && s.remove.Update(input) {
		s.deleteSelected(ctx)
	}
	input.ClearPointer()
}

// scrolledTo is a widget's place for an input script once the inspector has
// scrolled it into view, as an author scrolls to it: one in the body hidden
// above or below it is scrolled to the body's middle and located again.
// The save bar's buttons and Delete stay where they are.
func (s *State) scrolledTo(
	widget string, point image.Point, found bool, docs frame.Docs, again func() (image.Point, bool),
) (image.Point, bool) {
	body, footer := s.inspectorBody(), widget == ui.SaveID || widget == ui.RevertID || widget == s.remove.ID
	if !found || footer || s.scroll.most == 0 || point.X < body.Min.X || point.X >= body.Max.X ||
		point.In(body) {
		return point, found
	}
	s.scroll.offset = min(max(s.scroll.offset+point.Y-(body.Min.Y+body.Max.Y)/ui.Half, 0), s.scroll.most)
	s.layout(docs)

	return again()
}

// bodyImage is the screen clipped to the inspector's body, for drawing
// what scrolls.
func (s *State) bodyImage(screen *ebiten.Image) *ebiten.Image {
	if body, ok := screen.SubImage(s.inspectorBody()).(*ebiten.Image); ok {
		return body
	}

	return screen
}

// drawThumb draws, while the inspector scrolls, a thumb at its right edge:
// how much of it shows, and where.
func (s *State) drawThumb(screen *ebiten.Image) {
	ui.DrawScrollThumb(screen, s.inspectorBody(), s.scroll.offset, s.scroll.most)
}
