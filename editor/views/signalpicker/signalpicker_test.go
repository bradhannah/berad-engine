package signalpicker_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/signalpicker"
)

// TestGroupsFoldAndAClickPicks opens the picker on shop.bell_rung: its
// group is open and town's is closed until clicked; a click on a signal
// picks it and closes the picker.
func TestGroupsFoldAndAClickPicks(t *testing.T) {
	t.Parallel()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	window := image.Rect(0, 0, 1200, 800)
	ctx := frametest.Context(frametest.NewDocs(), &frametest.Requests{}, window.Size())
	var picked string
	options := []signalpicker.Option{
		{ID: frametest.ID(1), Path: "shop.bell_rung", Detail: "sent 1 · answered 1", Here: "sent here"},
		{ID: frametest.ID(2), Path: "shop.out_of_stock", Detail: "sent 1 · answered 1", Here: ""},
		{ID: frametest.ID(3), Path: "town.curfew", Detail: "sent 2 · answered 5", Here: ""},
	}
	config := signalpicker.Config{
		Title: "Choose a signal to send", Selected: frametest.ID(1),
		Pick: func(_ *frame.Context, option signalpicker.Option) { picked = option.Path },
	}
	picker := signalpicker.New(config, options, theme)
	picker.Draw(ebiten.NewImage(window.Dx(), window.Dy()), window)
	click := func(kind, value string) bool {
		t.Helper()
		point, found := picker.Locate(window, kind, value)
		if !found {
			t.Fatalf("cannot locate %s:%s", kind, value)
		}
		press := ui.NewInput(point)
		press.Pressed, press.Down = true, true

		return picker.Update(&press, ctx, window)
	}
	if _, shown := picker.Locate(window, signalpicker.LocateSignal, "shop.out_of_stock"); !shown {
		t.Fatal("the chosen signal's group opens with the picker")
	}
	if _, shown := picker.Locate(window, signalpicker.LocateSignal, "town.curfew"); shown {
		t.Fatal("another group starts closed")
	}
	click(signalpicker.LocateGroup, "town")
	if open := click(signalpicker.LocateSignal, "town.curfew"); open || picked != "town.curfew" {
		t.Fatalf("a click picks town.curfew and closes the picker: picked %q, open %v", picked, open)
	}
}
