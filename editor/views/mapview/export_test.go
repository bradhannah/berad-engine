package mapview

// Initials exposes the NPC token's initials to tests.
var Initials = initials

// Outside reports the cell under the cursor when it is beyond the floor
// bounds, and the inspector's line for it.
func (s *State) Outside() (string, bool) { return outsideLine(s.hover), s.outside }
