package window

import (
	"fmt"
	"image"
	"math"
	"slices"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// What will not fit is refused, with the reason and the keys that make room
// (increment 61): a pane asked for beside a tab whose panes would then need
// more than the window's width opens in a tab of its own, a larger scale of
// text and controls at which a tab would not fit is refused, and a split's
// divider stops where either pane would lose its panels or its canvas.

// paneNeed is the narrowest a pane goes: its left panel, its inspector and
// the canvas's room; a pane not connected yet needs only the canvas's room.
func paneNeed(shown *pane) int {
	if shown.host == nil {
		return minCanvas
	}

	return shown.host.leftEdge() + shown.host.inspectorRoom() + minCanvas
}

// makesRoom names the keys that make room, ending what a refusal says.
const makesRoom = "Cmd+I folds the inspector, Cmd+Shift+I the left panel, and Cmd+Shift+- makes text and " +
	"controls smaller, to make room"

// noRoomBeside is the refusal a split's divider dragged into a pane's room
// gives.
const noRoomBeside = "Each side keeps its panels and the canvas's room: " + makesRoom

// roomBeside reports whether a pane fits beside a tab's panes in the window,
// and if not, what the pane says as it opens in a tab of its own.
func (s *shell) roomBeside(beside *tab, shown *pane) (bool, string) {
	need := neededWidth(&tab{panes: append(slices.Clone(beside.panes), shown), focus: 0, split: defaultSplit, tint: 0})
	if need <= s.size.X {
		return true, ""
	}
	title := beside.panes[beside.focus].place.title

	return false, fmt.Sprintf("Opened in a tab of its own: beside %s it needs a window %d wide, and this one is %d. %s",
		quoted(title), s.screenPoints(need), s.screenPoints(s.size.X), makesRoom)
}

// quoted puts a name in curly quotes, or names a tab when it has none yet.
func quoted(name string) string {
	if name == "" {
		return "the tab"
	}

	return "“" + name + "”"
}

// scaleFits reports whether every tab fits the window at a scale of text
// and controls, and if not, why: the window keeps its size in the screen's
// points, so a larger scale leaves it fewer of its own.
func (s *shell) scaleFits(scale float64) (bool, string) {
	across := int(math.Floor(float64(s.screenPoints(s.size.X)) / scale))
	if need := int(math.Ceil(windowMinHeight * scale)); need > s.screenPoints(s.size.Y) {
		return false, fmt.Sprintf("Text and controls stay at %d%%: at %d%%, the window needs to be %d high, "+
			"and this one is %d. Cmd+Shift+- makes text and controls smaller", int(math.Round(s.textScale*percentage)),
			int(math.Round(scale*percentage)), need, s.screenPoints(s.size.Y))
	}
	for index, each := range s.tabs {
		if need := neededWidth(each); need > across {
			return false, fmt.Sprintf("Text and controls stay at %d%%: at %d%%, %s needs a window %d wide, "+
				"and this one is %d. %s", int(math.Round(s.textScale*percentage)), int(math.Round(scale*percentage)),
				quoted(s.tabTitle(index)), int(math.Ceil(float64(need)*scale)), s.screenPoints(s.size.X), makesRoom)
		}
	}

	return true, ""
}

// dividerRange is where a split's divider may be in an area: far enough
// from each edge that the pane on that side keeps its panels and the
// canvas's room. When the two need more than the area, it is the middle of
// what each needs, which only a window made narrower than the tab can give.
func dividerRange(each *tab, area image.Rectangle) (int, int) {
	least := area.Min.X + paneNeed(each.panes[0]) + dividerWidth/ui.Half
	most := area.Max.X - paneNeed(each.panes[1]) - dividerWidth/ui.Half
	if least > most {
		middle := (least + most) / ui.Half

		return middle, middle
	}

	return least, most
}
