package mapview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// Initials exposes the NPC token's initials to tests.
var Initials = initials

// Outside reports the cell under the cursor when it is beyond the floor
// bounds, and the inspector's line for it.
func (s *State) Outside() (string, bool) { return outsideLine(s.hover), s.outside }

// MenuItems lists the open right-click menu's items, or nil.
func (s *State) MenuItems() []string {
	if s.menu == nil {
		return nil
	}
	items := make([]string, 0, len(s.menu.Items))
	for _, item := range s.menu.Items {
		label := item.ID
		if item.Disabled {
			label += " (disabled)"
		}
		items = append(items, label)
	}

	return items
}

// NPCHint returns where the hint for a project with no NPCs is drawn, and
// where the heading of the floor list below it starts, for tests.
func (s *State) NPCHint() (image.Rectangle, int) { return s.npcHint(), s.floorsHeadingTop() }

// DialogLines returns the open dialog's lines, for tests.
func (s *State) DialogLines() []string {
	if s.dialog == nil {
		return nil
	}

	return s.dialog.Lines
}

// InspectorParts returns where the inspector's parts scroll, how far and the
// furthest, and what stays at its foot: the preview's lines, Clock, Watch
// and Keys.
func (s *State) InspectorParts(docs frame.Docs) (image.Rectangle, int, int, []frametest.Part) {
	panel := frame.Inspector(s.size)
	preview := image.Rect(panel.Min.X, footTop(panel)-ui.ButtonH-ui.Inset-s.previewHeight(docs), panel.Max.X,
		footTop(panel)-ui.ButtonH-ui.Inset)

	return s.body.Body, s.body.Offset, s.body.Most, []frametest.Part{
		{Name: "preview", Rect: preview}, {Name: "Clock", Rect: s.clockBn.Rect}, {Name: "Watch", Rect: s.watchBn.Rect},
		{Name: "Keys", Rect: s.keysBn.Rect},
	}
}
