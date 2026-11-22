package browser_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestASetFacetHasACrossThatClearsIt leaves a facet's button quiet, with
// its triangle, until a value is ticked: then it is set, its triangle gives
// way to a cross, and a click on the cross takes that facet's ticks away
// and nothing else (increment 66).
func TestASetFacetHasACrossThatClearsIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	if quiet, caret := f.state.FacetLook("kind"); !quiet || !caret {
		t.Fatalf("an unset facet is quiet, with a triangle: %v %v", quiet, caret)
	}
	if _, crossed := f.state.Locate("clear:Kind", f.ctx.Docs); crossed {
		t.Fatal("an unset facet has no cross")
	}
	f.click("ui:facet-kind", false)
	f.click("facet:Kind/Workflows", false)
	f.press("Escape")
	if !f.has("narrowed") || !f.has("ticked:Kind/Workflows") {
		t.Fatal("ticking Workflows narrows the window")
	}
	cross, set := f.state.FacetCross("kind")
	if _, caret := f.state.FacetLook("kind"); !set || caret {
		t.Fatalf("a set facet has a cross where its triangle was: %v %v", set, caret)
	}
	if named, crossed := f.state.Locate("clear:Kind", f.ctx.Docs); !crossed || named != cross {
		t.Fatalf("a script finds the cross by its facet's name: %v %v, want %v", named, crossed, cross)
	}
	input := ui.NewInput(cross)
	input.Pressed, input.Down = true, true
	f.state.Update(&input, f.ctx)
	if f.has("narrowed") || f.has("ticked:Kind/Workflows") {
		t.Fatal("the cross clears the facet")
	}
	if _, listed := f.state.Locate("facet:Kind/Workflows", f.ctx.Docs); listed {
		t.Fatal("the cross does not open the facet's values")
	}
}

// TestTheViewsAreOneControl stands a place's view buttons against one
// another on one line, so they are drawn as one control.
func TestTheViewsAreOneControl(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("place:Zones", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	if count, joined := f.state.ViewsJoined(); count < 2 || !joined {
		t.Fatalf("the views of Zones are one control: %d %v", count, joined)
	}
	f.click("ui:view-rows", false)
	if got := f.labels(); len(got) == 0 {
		t.Fatal("each view's button still takes its own press: Rows lists the zones")
	}
	if count, joined := f.state.ViewsJoined(); count < 2 || !joined {
		t.Fatalf("they stay one control whichever is chosen: %d %v", count, joined)
	}
}
