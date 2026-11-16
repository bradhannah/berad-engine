package pixelview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
)

// TestAFirstColourIsRememberedNeverGuessed opens a sprite never painted:
// it has no colour, and the Pencil is refused until one is chosen. The
// colour chosen is kept for this person, and the sprite opens with it again
// (increment 60).
func TestAFirstColourIsRememberedNeverGuessed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.drag("pixel:1,1", "pixel:2,1")
	if pixels, _ := f.paints(); len(pixels) != 0 || f.status() != "Refused: Choose a colour to paint with" {
		t.Fatalf("with no colour the Pencil is refused: %d painted, %q", len(pixels), f.status())
	}
	f.click("swatch:gloomy / grey 3")
	if len(f.requests.Published) == 0 {
		t.Fatal("the colour chosen is kept")
	}
	kept := f.requests.Published[len(f.requests.Published)-1]
	if kept.Topic != protocol.TopicPixelColours {
		t.Fatalf("kept as this person's colours: %+v", kept)
	}
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	again := pixelview.New(frametest.ID(98), spriteID, theme)
	again.Start(f.ctx)
	again.Changed(client.Update{Session: &kept}, f.ctx)
	first := ui.NewInput(image.Pt(0, 0))
	again.Update(&first, f.ctx)
	if again.Color() != grey(3) {
		t.Fatalf("the sprite opens with the colour last used in it: %+v", again.Color())
	}
}
