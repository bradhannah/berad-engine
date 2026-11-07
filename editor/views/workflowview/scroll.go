package workflowview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// inspectorScroll is how far the node inspector is scrolled: what is below
// the node's name moves up under it, and what went past its notes comes
// into view. most is the furthest it scrolls, as last laid out; of is the
// node it is scrolled for, as another node starts at the top.
type inspectorScroll struct {
	offset, most int
	of           ids.ULID
}

// formBody is the part of the node inspector that scrolls: below the node's
// name, above its notes, Delete node and the save bar.
func (s *State) formBody() image.Rectangle {
	panel := frame.Inspector(s.size)
	bottom := panel.Max.Y - ui.Inset
	switch {
	case !s.save.Rect.Empty():
		bottom = s.save.Rect.Min.Y - ui.Gap
	case s.form != nil && s.form.remove != nil:
		bottom = s.form.remove.Rect.Min.Y - ui.Gap
	}
	if node, ok := s.selectedNode(); ok {
		bottom -= s.noteLines(node, panel.Dx()-ui.Half*ui.Inset)*ui.LineHeight + ui.Gap
	}

	return image.Rect(panel.Min.X, panel.Min.Y+formTop-ui.Gap, panel.Max.X, bottom-ui.Gap)
}

// fitScroll keeps the node inspector scrolled no further than what is laid
// out down to bottom needs, reporting whether that moved it, so it is laid
// out again.
func (s *State) fitScroll(bottom int) bool {
	s.scrolled.most = max(bottom+s.scrolled.offset-s.formBody().Max.Y, 0)
	if s.scrolled.offset <= s.scrolled.most {
		return false
	}
	s.scrolled.offset = s.scrolled.most

	return true
}

// scrollForm scrolls the node inspector with the wheel over it. A press
// outside its body, on the notes, Delete node or the save bar, goes to
// those alone, never to what is scrolled behind them.
func (s *State) scrollForm(input *ui.Input, ctx *frame.Context) {
	if s.form == nil || s.scrolled.most == 0 || s.pal.help != "" || !input.Cursor.In(frame.Inspector(s.size)) {
		return
	}
	if input.WheelY != 0 {
		s.scrolled.offset = min(max(s.scrolled.offset-int(input.WheelY*ui.RowHeight), 0), s.scrolled.most)
		input.WheelY = 0
		s.layoutWidgets(ctx.Docs)
	}
	if !input.Pressed || input.Cursor.In(s.formBody()) {
		return
	}
	s.updateFooter(input, ctx)
	input.ClearPointer()
}

// locateScrolled finds a widget for an input script once the node inspector
// has scrolled it into view, as an author scrolls to it: one hidden above
// or below the inspector's body is scrolled to its middle and found again.
// The save bar's buttons and Delete node stay where they are.
func (s *State) locateScrolled(widget string, docs frame.Docs) (image.Point, bool) {
	point, found := s.locateWidget(widget)
	if !found || s.form == nil || s.scrolled.most == 0 || s.footerWidget(widget) ||
		!point.In(frame.Inspector(s.size)) || point.In(s.formBody()) {
		return point, found
	}
	body := s.formBody()
	s.scrolled.offset = min(max(s.scrolled.offset+point.Y-(body.Min.Y+body.Max.Y)/ui.Half, 0), s.scrolled.most)
	s.layoutWidgets(docs)

	return s.locateWidget(widget)
}

// footerWidget reports whether a widget stays put under the inspector's
// body: Save, Revert or Delete node.
func (s *State) footerWidget(widget string) bool {
	return widget == ui.SaveID || widget == ui.RevertID || (s.form.remove != nil && widget == s.form.remove.ID)
}

// drawFormBody draws what scrolls in the node inspector, clipped to its
// body, and beside it the thumb while it scrolls.
func (s *State) drawFormBody(screen *ebiten.Image, docs frame.Docs, node flow.Node, left int) {
	body := s.formImage(screen)
	s.drawControls(body, docs, node, left)
	s.drawColours(body)
	ui.DrawScrollThumb(screen, s.formBody(), s.scrolled.offset, s.scrolled.most)
}

// formImage is the screen clipped to the node inspector's body, for drawing
// what scrolls.
func (s *State) formImage(screen *ebiten.Image) *ebiten.Image {
	if body, ok := screen.SubImage(s.formBody()).(*ebiten.Image); ok {
		return body
	}

	return screen
}

// hasScrollState answers "inspector-scrolls", whether the node inspector
// holds more than shows, so it scrolls.
func (s *State) hasScrollState(state string, _ frame.Docs) (bool, bool) {
	if state != "inspector-scrolls" {
		return false, false
	}

	return s.form != nil && s.scrolled.most > 0, true
}
