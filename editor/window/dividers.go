package window

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Dividers drag (increment 53): the line between the canvas and the
// inspector, and between a Map's or a Workflow's left panel and its canvas.
// Dragging one sets the panel's width for every view of its kind, and a
// double-click puts it back. The canvas keeps room of its own: a panel
// dragged past it stops, and says what makes room.

// Divider sizes, in screen pixels: the grip either side of the line, the
// narrowest each panel goes, and the room the canvas keeps.
const (
	dividerGrip  = 8
	minInspector = 220
	minLeft      = 140
	minCanvas    = 240
)

// dragging is which divider is being dragged.
type dragging int

const (
	dragNone dragging = iota
	dragInspector
	dragLeft
)

// leftPanel is a view whose left panel's width the author drags.
type leftPanel interface {
	LeftEdge() (int, bool)
	SetLeftWidth(width int)
}

// noRoom is the refusal a panel dragged into the canvas's room gives.
const noRoom = "The canvas keeps the rest of the window: Cmd+I folds the inspector and " +
	"Cmd+Shift+I the left panel, to make room"

// useWidths makes the host's inspector width the one the view is laid out
// with, before it is run, drawn or asked where its parts are.
func (h *host) useWidths() { frame.SetInspectorWidth(h.inspectorWidth) }

// inspectorDivider is the grip on the inspector's edge, while it shows.
func (h *host) inspectorDivider() image.Rectangle {
	if h.folded() {
		return ui.Zero()
	}
	x := h.visible.X - frame.InspectorWide()

	return image.Rect(x-dividerGrip/ui.Half, 0, x+dividerGrip/ui.Half, h.visible.Y-frame.StatusHeight)
}

// leftDivider is the grip on the left panel's edge, while it shows.
func (h *host) leftDivider() image.Rectangle {
	panel, ok := h.view.(leftPanel)
	if !ok {
		return ui.Zero()
	}
	x, shows := panel.LeftEdge()
	if !shows {
		return ui.Zero()
	}

	return image.Rect(x-dividerGrip/ui.Half, 0, x+dividerGrip/ui.Half, h.visible.Y-frame.StatusHeight)
}

// leftEdge is where the left panel ends, or 0 with none showing.
func (h *host) leftEdge() int {
	if panel, ok := h.view.(leftPanel); ok {
		if x, shows := panel.LeftEdge(); shows {
			return x
		}
	}

	return 0
}

// setLeft sets the left panel's width; zero is the view's own.
func (h *host) setLeft(width int) {
	h.leftWidth = width
	if panel, ok := h.view.(leftPanel); ok {
		panel.SetLeftWidth(width)
	}
}

// updateDividers drags a divider, or puts it back on a double-click, taking
// the pointer from the view while it does.
func (h *host) updateDividers(input *ui.Input) {
	inspector, left := h.inspectorDivider(), h.leftDivider()
	h.showResize(input.Cursor.In(inspector) || input.Cursor.In(left) || h.dragging != dragNone)
	switch {
	case h.dragging == dragInspector && input.Down:
		h.dragInspectorTo(input.Cursor.X)
	case h.dragging == dragLeft && input.Down:
		h.dragLeftTo(input.Cursor.X)
	case h.dragging != dragNone:
		h.endDrag()
	case input.Pressed && input.Cursor.In(inspector):
		h.pressDivider(input, dragInspector, func() { h.inspectorWidth = 0 })
	case input.Pressed && input.Cursor.In(left):
		h.pressDivider(input, dragLeft, func() { h.setLeft(0) })
	default:
		return
	}
	input.ClearPointer()
}

// pressDivider starts a drag, or on a double-click puts the panel back.
func (h *host) pressDivider(input *ui.Input, which dragging, back func()) {
	if input.DoubleClick {
		back()
		h.arranged = true

		return
	}
	h.dragging, h.squeezed = which, false
}

// dragInspectorTo sets the inspector's width from where its edge is
// dragged, keeping the canvas's room.
func (h *host) dragInspectorTo(x int) {
	most := h.visible.X - h.leftEdge() - minCanvas
	wanted := h.visible.X - x
	h.squeezed = h.squeezed || wanted > most
	h.inspectorWidth = max(minInspector, min(wanted, most))
}

// dragLeftTo sets the left panel's width from where its edge is dragged,
// keeping the canvas's room.
func (h *host) dragLeftTo(x int) {
	most := h.visible.X - frame.InspectorWide() - minCanvas
	h.squeezed = h.squeezed || x > most
	h.setLeft(max(minLeft, min(x, most)))
}

// endDrag ends a drag, saying what makes room when the panel was dragged
// into the canvas's.
func (h *host) endDrag() {
	h.dragging, h.arranged = dragNone, true
	if h.squeezed {
		h.frame.Refuse(noRoom)
	}
}

// showResize shows the pointer that drags a divider while it is over one.
func (h *host) showResize(over bool) {
	switch {
	case over:
		ebiten.SetCursorShape(ebiten.CursorShapeEWResize)
		h.resizing = true
	case h.resizing:
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		h.resizing = false
	}
}

// locateDivider finds the dividers for input scripts, "ui:inspector-divider"
// and "ui:left-divider", and answers "state:inspector-width=<n>" and
// "state:left-width=<n>", each panel's width as it shows.
func (h *host) locateDivider(name string) (image.Point, bool, bool) {
	switch name {
	case "ui:inspector-divider":
		divider := h.inspectorDivider()

		return ui.Center(divider), !divider.Empty(), true
	case "ui:left-divider":
		divider := h.leftDivider()

		return ui.Center(divider), !divider.Empty(), true
	}
	if width, ok := strings.CutPrefix(name, "state:inspector-width="); ok {
		return image.Pt(0, 0), strconv.Itoa(frame.InspectorWide()) == width, true
	}
	if width, ok := strings.CutPrefix(name, "state:left-width="); ok {
		return image.Pt(0, 0), strconv.Itoa(h.leftEdge()) == width, true
	}

	return image.Pt(0, 0), false, false
}
