package browser_test

import (
	"encoding/json"
	"image"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/art"
)

// TestPicturesShowArt shows sprites as pictures by default: the folder
// Lights as a tile, which opens on a double-click to show Lantern, asking
// for its tile and palettes to draw it. A size and Rows are kept as the
// place's choice.
func TestPicturesShowArt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("place:Sprites", false)
	if !f.has("pane=pictures") {
		t.Fatal("sprites show as pictures")
	}
	f.click("row:Lights", true)
	if !f.has("place=Lights") {
		t.Fatal("a folder's tile opens it")
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !slices.Contains(f.requests.Subscribed, lanternTileID) ||
		!slices.Contains(f.requests.Subscribed, art.DefaultPaletteID()) {
		t.Fatal("the browser asks for the sprite's tile and palettes to draw it")
	}
	f.click("row:Lantern", false)
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Lantern" {
		t.Fatalf("a click selects a picture: %+v", row)
	}
	f.click("ui:size-2", false)
	f.click("ui:view-rows", false)
	var kept protocol.HomeKept
	published := f.requests.Published
	if len(published) == 0 || json.Unmarshal(published[len(published)-1].Value, &kept) != nil ||
		kept.Folders["folder:sprite:Lights"].Size != 3 || kept.Folders["folder:sprite:Lights"].View != "rows" {
		t.Fatalf("the folder keeps its size and view: %+v", kept)
	}
	if !f.has("pane=rows") {
		t.Fatal("Rows shows the folder as rows")
	}
}
