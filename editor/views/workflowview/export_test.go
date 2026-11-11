package workflowview

import (
	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// LayoutCount is how many times the window has laid the workflow out.
func (s *State) LayoutCount() int { return s.layouts }

// ZoomPercent is the canvas's zoom, as a percentage.
func (s *State) ZoomPercent() int { return zoomLevels()[s.zoom] }

// MissingHelp lists the kinds a kind of workflow offers that have no one
// line, no help page, or a help page without what it does or an example.
func MissingHelp() []string {
	var missing []string
	for _, purpose := range flow.Purposes() {
		for _, entry := range paletteFor(purpose) {
			help, ok := helpFor(entry.kind.Name)
			if !ok || hintFor(entry.kind.Name) == "" || help.about == "" || help.example == "" || help.time == "" {
				missing = append(missing, string(purpose)+"/"+entry.kind.Name)
			}
		}
	}

	return missing
}

// NumberButton returns the button beside a number field of the selected
// node that offers a constant in its place, by the field's parameter.
func (s *State) NumberButton(param string) (*ui.Button, bool) {
	if s.form == nil {
		return nil, false
	}
	slot, found := s.form.slots[slotKey(param, -1)]
	if !found {
		return nil, false
	}

	return slot.use, true
}
