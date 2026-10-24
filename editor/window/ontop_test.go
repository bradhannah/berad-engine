package window

import (
	"slices"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// TestOnTopAsksTheWindowToFloat asks the window to float once when the
// toggle turns on, and to stop when it turns off.
func TestOnTopAsksTheWindowToFloat(t *testing.T) {
	t.Parallel()
	var asked []bool
	h := &host{frame: frame.NewFrame(time.Now), float: func(on bool) { asked = append(asked, on) }}
	h.applyOnTop()
	h.frame.OnTop.On = true
	h.applyOnTop()
	h.applyOnTop()
	h.frame.OnTop.On = false
	h.applyOnTop()
	if !slices.Equal(asked, []bool{true, false}) {
		t.Fatalf("asked %v", asked)
	}
}
