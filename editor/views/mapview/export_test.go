package mapview

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
