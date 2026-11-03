package valueform_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/valueform"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestAPlaceShowsItsRing finds the place type's button marked by the
// anchor's ring, and a place's default as a box that opens the list of
// places, with Pick on map, and Clear once it has one.
func TestAPlaceShowsItsRing(t *testing.T) {
	t.Parallel()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	form := valueform.New(theme)
	place := worldstate.New(ids.ULID{}, "place", worldstate.ScopeLocal, worldstate.TypePlace)
	form.Show(place, "local.place")
	form.Layout(0, 0, 260)
	for _, button := range form.Buttons() {
		if button.ID == "value-type-place" && button.Icon != ui.IconPlace {
			t.Fatal("the place type's button has the ring")
		}
	}
	if form.PlaceBox.Empty() || form.PickPlace.Rect.Empty() || !form.ClearPlace.Rect.Empty() {
		t.Fatal("a place with none yet has its box and Pick on map, and no Clear")
	}
	form.PlaceName = "Shelf"
	form.Layout(0, 0, 260)
	if form.ClearPlace.Rect.Empty() {
		t.Fatal("a place has Clear")
	}
	input := ui.NewInput(ui.Center(form.PlaceBox))
	input.Pressed = true
	if _, change := form.Update(&input); change != valueform.ChangeChoosePlace {
		t.Fatalf("its box opens the list of places: %v", change)
	}
}
