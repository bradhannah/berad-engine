package mapview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The Map's left panel folds when told to (increment 43): by its fold
// button or Cmd+Shift+I, for every Map, as the editor window keeps it per
// kind of view. Folded, it is a rail of its tools, each by its key, so the
// tool in hand stays one click away; the status bar names it, its tile and
// what the pointer is over.

// RailWidth is how wide the folded panel's rail is, and foldButton the
// fold button's size.
const (
	RailWidth  = 40
	foldButton = 24
	railGap    = 4
	foldID     = "fold-left"
	railPrefix = "rail-"
)

// foldState is the fold button and the rail's buttons, and whether the
// panel is folded as the frame last said.
type foldState struct {
	folded bool
	button *ui.Button
	rail   []*ui.Button
}

func newFoldState() foldState {
	rail := make([]*ui.Button, 0, len(tools()))
	for _, each := range tools() {
		rail = append(rail, ui.NewButton(railPrefix+string(each.tool), string(each.key)))
	}

	return foldState{folded: false, button: ui.NewButton(foldID, "‹"), rail: rail}
}

// panelWidth is how wide the left panel is: its rail while folded.
func (s *State) panelWidth() int {
	if s.fold.folded {
		return RailWidth
	}

	return PanelWidth
}

// layoutFold places the fold button at the panel's top right, or at the
// rail's top, and the rail's tools under it, clearing the panel's own
// widgets while it is folded.
func (s *State) layoutFold() {
	top := railGap
	if !s.fold.folded {
		s.fold.button.Label = "‹"
		s.fold.button.Rect = image.Rect(PanelWidth-foldButton-railGap*ui.Half, top, PanelWidth-railGap*ui.Half,
			top+foldButton)
		for _, button := range s.fold.rail {
			button.Rect = ui.Zero()
		}

		return
	}
	s.fold.button.Label = "›"
	left := (RailWidth - foldButton) / ui.Half
	s.fold.button.Rect = image.Rect(left, top, left+foldButton, top+foldButton)
	top += foldButton + ui.Inset
	for index, button := range s.fold.rail {
		button.Rect = image.Rect(left, top, left+foldButton, top+foldButton)
		button.On = tools()[index].tool == s.tool
		top += foldButton + railGap
	}
	s.toolList.Rect, s.tileList.Rect, s.floorList.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	s.below.Rect, s.previewBn.Rect = ui.Zero(), ui.Zero()
}

// updateFold folds or unfolds the panel from its button, and picks a tool
// from the rail. It reports whether the press was the fold's.
func (s *State) updateFold(input *ui.Input, ctx *frame.Context) bool {
	if s.fold.button.Update(input) {
		ctx.Frame.ShowLeft.On = !ctx.Frame.ShowLeft.On

		return true
	}
	for index, button := range s.fold.rail {
		if !button.Rect.Empty() && button.Update(input) {
			s.setTool(tools()[index].tool)

			return true
		}
	}

	return false
}

// drawRail draws the folded panel: the fold button and a button for each
// tool, the tool in hand filled.
func (s *State) drawRail(screen *ebiten.Image) {
	height := s.size.Y - frame.StatusHeight
	ui.FillRect(screen, image.Rect(0, 0, RailWidth, height), ui.ColorPage())
	ui.Divider(screen, image.Pt(RailWidth-ui.Outline, 0), image.Pt(RailWidth-ui.Outline, height))
	s.fold.button.Draw(screen, s.theme)
	for _, button := range s.fold.rail {
		button.Draw(screen, s.theme)
	}
}

// locateSides finds the zone's palettes and sounds in the inspector, and
// the fold button and the rail on the left.
func (s *State) locateSides(name string) (image.Point, bool) {
	if at, found := s.zonePalettes.Locate(name); found {
		return at, true
	}

	return s.locateFold(name)
}

// locateFold finds the fold button, "ui:fold-left", and a rail's tool,
// "ui:rail-<tool>", for input scripts.
func (s *State) locateFold(name string) (image.Point, bool) {
	if name == "ui:"+foldID {
		return ui.Center(s.fold.button.Rect), !s.fold.button.Rect.Empty()
	}
	for _, button := range s.fold.rail {
		if name == "ui:"+button.ID {
			return ui.Center(button.Rect), !button.Rect.Empty()
		}
	}

	return image.Pt(0, 0), false
}
