package pixelview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestTheStatusBarNamesWhatIsInHand names the sprite, the tool, the colour
// it paints with, and the pixel the pointer is over with its colour
// (increment 49).
func TestTheStatusBarNamesWhatIsInHand(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("swatch:gloomy / grey 3")
	f.drag("pixel:1,1")
	f.frame(ui.NewInput(f.at("pixel:1,1")))
	for _, words := range []string{"Lantern · Pencil · gloomy / grey 3", "pixel 1, 1: gloomy / grey 3"} {
		if _, said := f.ctx.Frame.LocateStatus("state:hand:" + words); !said {
			t.Fatalf("the status bar says %q", words)
		}
	}
	f.keys("E")
	f.frame(ui.NewInput(f.at("pixel:2,2")))
	if _, said := f.ctx.Frame.LocateStatus("state:hand:Lantern · Eraser · pixel 2, 2"); !said {
		t.Fatal("the eraser names no colour to paint with")
	}
}
