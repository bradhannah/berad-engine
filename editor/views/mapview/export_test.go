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

// LeftParts returns where the left panel's lists scroll, the furthest they
// go, and what stays at its foot.
func (s *State) LeftParts() (image.Rectangle, int, []frametest.Part) {
	return s.lists.Body, s.lists.Most, []frametest.Part{
		{Name: "Show floor below", Rect: s.below.Rect}, {Name: "Preview", Rect: s.previewBn.Rect},
	}
}

// InspectorButton reports whether the inspector shows a button for the
// selected ambient sound or portal, and whether it is disabled, or the
// chosen one of its set.
func (s *State) InspectorButton(id string) (bool, bool, bool) {
	for _, button := range s.inspect.buttons() {
		if button.ID == id {
			return !button.Rect.Empty(), button.Disabled, button.On
		}
	}

	return false, false, false
}

// ToolRows lists the tool list's rows as "id=label [key]", a group's
// heading as "# label", and reports whether its selected row is drawn as
// the one in hand.
func (s *State) ToolRows() ([]string, bool) {
	rows := make([]string, 0, len(s.toolList.Rows))
	for _, row := range s.toolList.Rows {
		if row.Heading {
			rows = append(rows, "# "+row.Label)

			continue
		}
		rows = append(rows, row.ID+"="+row.Label+" ["+row.Key+"]")
	}

	return rows, s.toolList.InHand
}

// TileChips returns how many columns the list of what a tool paints with is
// laid out in, none for a plain list, and its rows' names.
func (s *State) TileChips() (int, []string) {
	names := make([]string, 0, len(s.tileList.Rows))
	for _, row := range s.tileList.Rows {
		names = append(names, row.Label)
	}

	return s.tileList.Grid, names
}

// PreviewButton returns the Preview button's label and key.
func (s *State) PreviewButton() (string, string) { return s.previewBn.Label, s.previewBn.Key }
