package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
)

// TestAStrokeIsSaidInTheMapsWords says each command of a stroke as the
// stroke so far: how many cells, of which tile, on which floor
// (increment 51).
func TestAStrokeIsSaidInTheMapsWords(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("3")
	f.press("cell:2,8", false)
	f.hold("cell:5,8")
	f.release("cell:5,8")
	commands := f.requests.Of(tilefloor.PaintEdit)
	first, last := commands[0].Command.Says, commands[len(commands)-1].Command.Says
	if !strings.HasPrefix(first, "Painted 1 cell of ") || !strings.HasPrefix(last, "Painted 4 cells of ") ||
		!strings.Contains(last, " on ") {
		t.Fatalf("each command says the stroke so far: %q, then %q", first, last)
	}
}
