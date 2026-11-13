package mapview

import "image"

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
