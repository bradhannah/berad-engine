package pixelview_test

import (
	"strings"
	"testing"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
)

// TestAStrokeIsSaidInThePixelEditorsWords says a stroke as the pixels it
// painted so far, in which colour, on which sprite, and a fill as the
// pixels it filled (increment 51).
func TestAStrokeIsSaidInThePixelEditorsWords(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("swatch:gloomy / grey 3")
	f.drag("pixel:1,1", "pixel:4,1")
	commands := f.requests.Of(artplugin.PaintEdit)
	if says := commands[len(commands)-1].Command.Says; says != "Painted 4 pixels of gloomy / grey 3 on Lantern" {
		t.Fatalf("a stroke is said as a whole so far: %q", says)
	}
	f.keys("G")
	f.click("swatch:gloomy / grey 4")
	f.click("pixel:6,6")
	commands = f.requests.Of(artplugin.PaintEdit)
	if says := commands[len(commands)-1].Command.Says; !strings.HasPrefix(says, "Filled ") ||
		!strings.HasSuffix(says, " of gloomy / grey 4 on Lantern") {
		t.Fatalf("a fill is said: %q", says)
	}
}
