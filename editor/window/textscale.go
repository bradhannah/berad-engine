package window

import (
	"fmt"
	"image"
	"math"
	"os"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// textScaleEnv sets the scale of text and controls an input script's run
// starts at, so the real-window walks can be seen at another scale. It is
// read only while input scripts drive the editor.
const textScaleEnv = "BERAD_TEXT_SCALE"

// scriptedTextScale is the scale an input script's run starts at: 100%
// unless BERAD_TEXT_SCALE gives a percentage from 75 to 200.
func scriptedTextScale() float64 {
	if !scripted() {
		return 1
	}
	given, err := strconv.Atoi(os.Getenv(textScaleEnv))
	scales := frame.TextScales()
	if err != nil || given < frame.TextPercent(scales[0]) || given > frame.TextPercent(scales[len(scales)-1]) {
		return 1
	}

	return float64(given) / percentage
}

// windowSizeEnv sets the size an input script's run opens its window at,
// in the screen's points, as WIDTHxHEIGHT, so the real-window walks can be
// seen in a small window at any scale. It is read only while input scripts
// drive the editor.
const windowSizeEnv = "BERAD_WINDOW_SIZE"

// openingSize is the size the window opens at, in the screen's points:
// 1200 by 832 at 100%, and as much larger as the scale makes text and
// controls, so that a scripted run at another scale lays out as at 100%;
// or the size BERAD_WINDOW_SIZE gives a scripted run.
func openingSize(scale float64) image.Point {
	if scripted() {
		var width, height int
		if _, err := fmt.Sscanf(os.Getenv(windowSizeEnv), "%dx%d", &width, &height); err == nil && width > 0 && height > 0 {
			return image.Pt(width, height)
		}
	}

	return image.Pt(int(math.Ceil(windowWidth*scale)), int(math.Ceil((windowHeight+rowHeight)*scale)))
}

// percentage is a whole scale as a percentage.
const percentage = 100

// followTextScale takes a scale for text and controls any pane asked for,
// with its Text button or keys, as the window's, says so, and keeps it for
// this size of screen (increment 56). The window lays itself out again at
// the new scale from the next frame. Every pane's Text button names it.
func (s *shell) followTextScale() {
	asked, asking := 0.0, s.focused()
	for _, shown := range s.panes() {
		if scale, ok := shown.host.frame.TakeTextScale(); ok {
			asked, asking = scale, shown
		}
	}
	if asked > 0 {
		s.setTextScale(asked, asking)
	}
	for _, shown := range s.panes() {
		shown.host.frame.SetTextScale(s.textScale)
	}
}

// setTextScale changes the scale of text and controls, says so in the
// focused pane's status bar and keeps it. A larger scale at which a tab
// would not fit the window is refused in the status bar of the pane that
// asked, saying which and what makes room (increment 61).
func (s *shell) setTextScale(scale float64, asking *pane) {
	if fits, refusal := s.scaleFits(scale); scale > s.textScale && !fits {
		if asking != nil && asking.host != nil {
			asking.host.frame.Refuse(refusal)
		}

		return
	}
	said := "Text and controls at " + strconv.Itoa(frame.TextPercent(scale)) + "%"
	if scale == s.textScale {
		said = "Text and controls are already at " + strconv.Itoa(frame.TextPercent(scale)) + "%"
	}
	if scale != 1 {
		said += ": Cmd+Shift+0 puts them back at 100%"
	}
	if focused := s.focused(); focused != nil {
		focused.host.frame.Status(said)
	}
	if scale == s.textScale {
		return
	}
	s.textScale = scale
	// The window's narrowest is in the screen's points, which the new scale
	// changes.
	s.minWidth = 0
	s.publishArrangement()
}
