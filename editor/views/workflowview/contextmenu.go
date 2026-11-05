package workflowview

import (
	"strings"

	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The canvas's right-click menu's choices, located as "menu:<id>".
const (
	menuDeleteNode = "delete-node"
	menuDeleteLine = "delete-line"
	locateMenu     = "menu"
)

// contextClick is a right-click waiting for the selection to settle: where
// the pointer was, and whether one is waiting.
type contextClick struct {
	at      image.Point
	pending bool
}

func noContextClick() contextClick { return contextClick{at: image.Pt(0, 0), pending: false} }

// rightClick selects the line or node a right-click lands on, as a click
// would, and asks for the menu once the selection has settled. On empty
// canvas it selects nothing and opens no menu.
func (s *State) rightClick(point, pointer image.Point) {
	s.context = noContextClick()
	if key, onLine := s.linkAt(point); onLine {
		s.selected, s.selectedLink = ids.ULID{}, key
	} else if hit, onNode := s.nodeAt(point); onNode {
		s.selected, s.selectedLink = hit, noRoute()
	} else {
		s.selected, s.selectedLink = ids.ULID{}, noRoute()

		return
	}
	s.context = contextClick{at: pointer, pending: true}
}

// openContextMenu opens the right-click menu on the selection once it has
// settled: Delete line on a line, Delete node on a node, which the start
// node cannot be. When the inspector's unsaved changes held the selection
// back, it does not open.
func (s *State) openContextMenu() {
	click := s.context
	s.context = noContextClick()
	if !click.pending || s.guard.Active() || s.workflow == nil {
		return
	}
	item := ui.MenuItem{
		ID: menuDeleteNode, Label: "Delete node", Icon: ui.IconNone, Disabled: false, Danger: true, Rule: false,
	}
	switch {
	case s.lineSelected():
		item.ID, item.Label = menuDeleteLine, "Delete line"
	case s.selected.IsZero():
		return
	case s.selected == s.workflow.Start():
		item.Disabled = true
	}
	items := []ui.MenuItem{item}
	s.contextMenu.Items, s.contextMenu.Width = items, ui.MenuWidth(items, s.theme)
	s.contextMenu.OpenAt(click.at, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
}

// updateOverlay gives a frame's input to the picker's dialog or the
// right-click menu while one is open, and reports whether one was.
func (s *State) updateOverlay(input *ui.Input, ctx *frame.Context) bool {
	switch {
	case s.picker.Dialog() != nil:
		s.picker.UpdateDialog(input, ctx, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	case s.values != nil:
		s.updateValuePicker(input, ctx)
	case s.signalPick != nil:
		s.updateSignalPicker(input, ctx)
	case s.places != nil:
		s.updatePlaces(input, ctx)
	case s.dialog != nil:
		s.updateDialog(input, ctx)
	case s.zones != nil:
		s.updateZones(input, ctx)
	case s.contextMenu.IsOpen():
		s.chooseFromContext(ctx, s.contextMenu.Update(input))
	default:
		return false
	}

	return true
}

// chooseFromContext carries out a right-click menu choice on the selection.
func (s *State) chooseFromContext(ctx *frame.Context, chosen string) {
	if strings.HasPrefix(chosen, menuWorkflow) {
		s.chooseModule(ctx, chosen)

		return
	}
	switch chosen {
	case menuDeleteNode:
		s.removeSelected(ctx)
	case menuDeleteLine:
		if s.lineSelected() {
			s.removeLine(ctx)
		}
	}
}
