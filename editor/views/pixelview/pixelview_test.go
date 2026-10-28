package pixelview_test

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const tileSize = 8

var (
	spriteID     = frametest.ID(1)
	tileID       = frametest.ID(2)
	glassRampID  = frametest.ID(3)
	glassShadeID = frametest.ID(4)
	chestID      = frametest.ID(5)
)

// grey is a shade of the starter ramp, by its number.
func grey(number int) art.ColorRef {
	return art.ColorRef{Palette: art.SharedPaletteID(), Ramp: art.StarterRampID(), Shade: art.StarterShadeID(number)}
}

func glass() art.ColorRef {
	return art.ColorRef{Palette: art.SharedPaletteID(), Ramp: glassRampID, Shade: glassShadeID}
}

type fixture struct {
	t        *testing.T
	state    *pixelview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
	// applied counts the commands already applied to the documents, and
	// made the IDs given to new ramps and shades.
	applied int
	made    byte
}

// newFixture opens the sprite Lantern, whose tile is 8 pixels square, with
// the shared palette's grey ramp and a ramp "glass" of one shade at 40%. The
// sprite Chest shows the medium pickup.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	palette, err := art.StarterPalette()
	if err != nil {
		t.Fatal(err)
	}
	glassRamp := art.Ramp{ID: glassRampID, Name: "glass", Shades: []art.Shade{{ID: glassShadeID, Color: 0x3060c0, Opacity: 40}}}
	if err := palette.InsertRamp(1, glassRamp); err != nil {
		t.Fatal(err)
	}
	tile, err := art.NewTile(tileSize)
	if err != nil {
		t.Fatal(err)
	}
	docs := frametest.NewDocs()
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: spriteID, Name: "Lantern", Format: art.SpriteFormatName},
		{ID: tileID, Name: "Tile 1", Format: art.TileFormatName},
		{ID: art.SharedPaletteID(), Name: art.SharedPaletteName, Format: art.PaletteFormatName},
		{ID: art.SettingsID(), Name: art.SettingsName, Format: art.SettingsFormatName},
		{ID: chestID, Name: "Chest", Format: art.SpriteFormatName},
	}))
	standIns := art.NewStandInSprites()
	if _, err := standIns.Set("pickup.medium", chestID); err != nil {
		t.Fatal(err)
	}
	docs.Put(art.StandInsID(), art.StandInsName, &artplugin.StandInsDocument{Sprites: standIns})
	docs.Put(art.SharedPaletteID(), art.SharedPaletteName, &artplugin.PaletteDocument{Palette: palette})
	docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: tileSize}})
	docs.Put(tileID, "Tile 1", &artplugin.TileDocument{Tile: tile})
	docs.Put(spriteID, "Lantern", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID}})
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := pixelview.New(frametest.ID(99), spriteID, theme)
	state.Start(ctx)
	f := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func (f *fixture) frame(input ui.Input) { f.state.Update(&input, f.ctx) }

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

func (f *fixture) click(name string) {
	f.t.Helper()
	point := f.at(name)
	press := ui.NewInput(point)
	press.Pressed, press.Down = true, true
	f.frame(press)
	release := ui.NewInput(point)
	release.Released = true
	f.frame(release)
}

// drag presses on the first part, moves through the rest with the button
// held, applying what each frame sends, and releases.
func (f *fixture) drag(names ...string) {
	f.t.Helper()
	for index, name := range names {
		input := ui.NewInput(f.at(name))
		input.Down, input.Pressed = true, index == 0
		f.frame(input)
		f.applyAll()
	}
	release := ui.NewInput(f.at(names[len(names)-1]))
	release.Released = true
	f.frame(release)
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) typeText(text string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

// retype replaces a field's text.
func (f *fixture) retype(field, text string) {
	f.t.Helper()
	f.click(field)
	f.keys("Cmd+A")
	f.typeText(text)
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.docs)

	return ok
}

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

// applyAll applies every command not yet applied with the art plug-in's own
// edit kinds, as the orchestrator would, then answers it; the window sees
// the change before the answer, as it does from the orchestrator. It runs no
// frame, so a drag goes on.
func (f *fixture) applyAll() {
	f.t.Helper()
	for ; f.applied < len(f.requests.Commands); f.applied++ {
		command := f.requests.Commands[f.applied]
		kind := editKind(f.t, command.Command.Edit)
		args := command.Command.Args
		var err error
		f.docs.View(command.Command.Document, func(document client.Document) {
			if preparer, ok := kind.(model.Preparer); ok {
				args, err = preparer.Prepare(document.Model, args, f.newID)
			}
			if err == nil {
				_, err = kind.Apply(document.Model, args)
			}
		})
		if err != nil {
			f.t.Fatalf("%s: %v", command.Command.Edit, err)
		}
		f.docs.Touch(command.Command.Document)
		f.state.Changed(client.Update{Event: &protocol.Event{Document: command.Command.Document}}, f.ctx)
		command.Succeed(ids.ULID{})
	}
}

func (f *fixture) newID() (ids.ULID, error) {
	f.made++

	return frametest.ID(100 + f.made), nil
}

func editKind(t *testing.T, name string) model.EditKind {
	t.Helper()
	for _, kind := range artplugin.Plugin().Edits {
		if kind.Name() == name {
			return kind
		}
	}
	t.Fatalf("no art edit %s", name)

	return nil
}

// paints returns the painted pixels of every Paint command sent, and their
// gestures.
func (f *fixture) paints() ([]artplugin.Pixel, []string) {
	f.t.Helper()
	var pixels []artplugin.Pixel
	var gestures []string
	for _, command := range f.requests.Of(artplugin.PaintEdit) {
		var args artplugin.PaintArgs
		if err := command.Decode(&args); err != nil {
			f.t.Fatal(err)
		}
		pixels = append(pixels, args.Pixels...)
		if !slices.Contains(gestures, command.Command.Gesture) {
			gestures = append(gestures, command.Command.Gesture)
		}
	}

	return pixels, gestures
}

// setPixels paints the tile directly, as if drawn before the window opened.
func (f *fixture) setPixels(color art.ColorRef, pixels ...image.Point) {
	f.t.Helper()
	f.docs.View(tileID, func(document client.Document) {
		tile := document.Model.(*artplugin.TileDocument).Tile
		for _, pixel := range pixels {
			if _, err := tile.Set(pixel.X, pixel.Y, color); err != nil {
				f.t.Fatal(err)
			}
		}
	})
	f.docs.Touch(tileID)
}

func (f *fixture) pixel(col, row int) art.ColorRef {
	f.t.Helper()
	var ref art.ColorRef
	f.docs.View(tileID, func(document client.Document) {
		var err error
		if ref, err = document.Model.(*artplugin.TileDocument).Tile.At(col, row); err != nil {
			f.t.Fatal(err)
		}
	})

	return ref
}

// TestOpensTheSpriteAndItsColours opens a sprite: the window subscribes to
// its tile, the shared palette and the settings, names the sprite in its
// title, and fits the tile at a whole magnification.
func TestOpensTheSpriteAndItsColours(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, want := range []ids.ULID{project.IndexID(), art.SettingsID(), tileID, art.SharedPaletteID()} {
		if !slices.Contains(f.requests.Subscribed, want) {
			t.Fatalf("subscribed to %v, not %v", f.requests.Subscribed, want)
		}
	}
	if title := f.state.Title(f.docs); !strings.HasPrefix(title, "Sprite: Lantern  (window ") {
		t.Fatalf("title %q", title)
	}
	if !f.has("loaded") || f.state.Zoom() != 64 || f.state.Tool() != pixelview.ToolPencil || !f.has("color:clear") {
		t.Fatalf("loaded %v, zoom %d, tool %s", f.has("loaded"), f.state.Zoom(), f.state.Tool())
	}
}

// TestPencilPaintsAStrokeAsOneStep paints a drag across a row in grey 3: the
// pixels between the frames' positions are filled in, every command shares
// one gesture, and a pixel already grey 3 is not sent again. The next stroke
// has a gesture of its own.
func TestPencilPaintsAStrokeAsOneStep(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("swatch:grey 3")
	if f.state.Color() != grey(3) || !f.has("color:grey 3") {
		t.Fatalf("chosen %+v", f.state.Color())
	}
	f.drag("pixel:1,1", "pixel:4,1", "pixel:4,3")
	pixels, gestures := f.paints()
	if len(gestures) != 1 || len(pixels) != 6 {
		t.Fatalf("one stroke paints 6 pixels in one gesture: %v %v", pixels, gestures)
	}
	for _, want := range []image.Point{{1, 1}, {2, 1}, {3, 1}, {4, 1}, {4, 2}, {4, 3}} {
		if f.pixel(want.X, want.Y) != grey(3) || !f.has("pixel:"+itoa(want.X)+","+itoa(want.Y)+":grey 3") {
			t.Fatalf("pixel %v is %+v", want, f.pixel(want.X, want.Y))
		}
	}
	before := len(f.requests.Commands)
	f.drag("pixel:2,1", "pixel:1,1")
	if len(f.requests.Commands) != before {
		t.Fatal("pixels already grey 3 are not painted again")
	}
	f.drag("pixel:0,0")
	if _, gestures := f.paints(); len(gestures) != 2 || f.pixel(0, 0) != grey(3) {
		t.Fatalf("a second stroke has its own gesture: %v", gestures)
	}
}

// TestEraserAndClearErase clears pixels with the eraser, and with the pencil
// when clear is the chosen colour.
func TestEraserAndClearErase(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.setPixels(grey(2), image.Pt(0, 0), image.Pt(1, 0), image.Pt(2, 0), image.Pt(3, 0))
	f.click("swatch:grey 3")
	f.keys("E")
	if f.state.Tool() != pixelview.ToolEraser {
		t.Fatalf("E chooses the eraser, not %s", f.state.Tool())
	}
	f.drag("pixel:0,0", "pixel:1,0")
	if !f.pixel(0, 0).IsClear() || !f.pixel(1, 0).IsClear() || f.pixel(2, 0) != grey(2) {
		t.Fatal("the eraser clears what it is dragged over")
	}
	f.keys("B")
	f.click("swatch:clear")
	f.drag("pixel:2,0")
	if !f.pixel(2, 0).IsClear() || f.pixel(3, 0) != grey(2) {
		t.Fatal("the pencil paints clear when clear is chosen")
	}
}

// TestFillStaysInItsRegion fills the pixels of one colour joined side by
// side: a column of grey 1 bounds it, as do the tile's edges, and pixels
// joined only at a corner are left alone. Filling with the colour already
// there sends nothing.
func TestFillStaysInItsRegion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	var wall []image.Point
	for row := range tileSize {
		wall = append(wall, image.Pt(3, row))
	}
	f.setPixels(grey(1), wall...)
	f.click("swatch:grey 4")
	f.keys("G")
	f.click("pixel:1,1")
	f.applyAll()
	pixels, gestures := f.paints()
	if len(pixels) != 3*tileSize || len(gestures) != 1 {
		t.Fatalf("the fill paints the 24 pixels left of the wall, as one step: %d %v", len(pixels), gestures)
	}
	for _, pixel := range pixels {
		if pixel.Col >= 3 || pixel.Color != grey(4) {
			t.Fatalf("the fill crossed the wall at %+v", pixel)
		}
	}
	if f.pixel(4, 4) != (art.ColorRef{}) || f.pixel(3, 4) != grey(1) {
		t.Fatal("the wall and what lies beyond it are unchanged")
	}
	before := len(f.requests.Commands)
	f.click("pixel:0,7")
	if len(f.requests.Commands) != before {
		t.Fatal("filling with the colour already there sends nothing")
	}

	f.setPixels(grey(2), image.Pt(6, 0), image.Pt(5, 1), image.Pt(4, 2))
	f.click("swatch:glass 1")
	f.click("pixel:7,0")
	f.applyAll()
	if f.pixel(7, 0) != glass() || f.pixel(7, 1) != glass() || f.pixel(5, 0) == glass() {
		t.Fatal("a fill passes side by side but never through a corner")
	}
}

// TestEyedropperPicksAColour picks a pixel's colour to paint with, and clear
// from a clear pixel.
func TestEyedropperPicksAColour(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.setPixels(glass(), image.Pt(5, 5))
	f.keys("I")
	f.click("pixel:5,5")
	if f.state.Color() != glass() || !f.has("color:glass 1") || f.status() != "Painting with glass 1" {
		t.Fatalf("picked %+v, status %q", f.state.Color(), f.status())
	}
	f.click("pixel:0,0")
	if !f.state.Color().IsClear() {
		t.Fatal("a clear pixel picks clear")
	}
	if len(f.requests.Commands) != 0 {
		t.Fatal("picking a colour changes nothing")
	}
}

// TestZoomIsAWholeNumber steps the magnification through whole numbers,
// fits it again, and keeps pixels where the tools find them.
func TestZoomIsAWholeNumber(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("Cmd+Minus")
	f.keys("Cmd+Minus")
	if f.state.Zoom() != 40 || !f.has("zoom:40") {
		t.Fatalf("zoom %d after two steps out", f.state.Zoom())
	}
	f.click("ui:zoom-in")
	if f.state.Zoom() != 48 {
		t.Fatalf("zoom %d after one step in", f.state.Zoom())
	}
	f.click("swatch:grey 5")
	f.drag("pixel:6,6")
	if f.pixel(6, 6) != grey(5) {
		t.Fatal("the pixel under the cursor is painted at any zoom")
	}
	f.click("ui:zoom-fit")
	if f.state.Zoom() != 64 {
		t.Fatalf("Fit gives %d", f.state.Zoom())
	}
}

// TestPaletteAddsRampsAndShades adds a ramp, named so no other ramp has its
// name, with one shade, and chooses it; + Shade copies the chosen shade into
// the place after it.
func TestPaletteAddsRampsAndShades(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:add-ramp")
	adds := f.requests.Of(artplugin.AddRampEdit)
	var added artplugin.AddRampArgs
	if len(adds) != 1 || adds[0].Decode(&added) != nil || added.Index != 2 || added.Ramp.Name != "new ramp" ||
		len(added.Ramp.Shades) != 1 {
		t.Fatalf("add ramp sent %+v", added)
	}
	f.applyAll()
	if !f.has("color:new ramp 1") {
		t.Fatal("the new ramp's shade is chosen")
	}
	f.click("ui:add-shade")
	f.applyAll()
	var shade artplugin.AddShadeArgs
	if err := f.requests.Of(artplugin.AddShadeEdit)[0].Decode(&shade); err != nil || shade.Index != 1 ||
		shade.Shade.Color != added.Ramp.Shades[0].Color {
		t.Fatalf("add shade sent %+v", shade)
	}
	if !f.has("color:new ramp 2") {
		t.Fatal("the new shade is chosen")
	}
	f.click("ui:add-ramp")
	if err := f.requests.Of(artplugin.AddRampEdit)[1].Decode(&added); err != nil || added.Ramp.Name != "new ramp 2" {
		t.Fatalf("a second new ramp is named %q", added.Ramp.Name)
	}
}

// TestTypedShadeWaitsForSave types a shade's colour and opacity: nothing is
// sent until Save, which sends both at once; a colour or opacity that cannot
// be saved is refused and kept to correct.
func TestTypedShadeWaitsForSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("swatch:grey 3")
	f.retype("ui:shade-color", "#12")
	if len(f.requests.Commands) != 0 || f.state.Unsaved(f.docs) != 1 {
		t.Fatalf("typing waits for Save: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	if len(f.requests.Commands) != 0 || !strings.HasPrefix(f.status(), "Enter a colour as six hexadecimal digits") ||
		f.state.Unsaved(f.docs) != 1 {
		t.Fatalf("a short colour is refused and kept: %q", f.status())
	}
	f.retype("ui:shade-color", "#112233")
	f.retype("ui:shade-opacity", "0")
	f.click("ui:save")
	if len(f.requests.Commands) != 0 || f.status() != "Enter an opacity from 1% to 100%" {
		t.Fatalf("0%% is refused: %q", f.status())
	}
	f.retype("ui:shade-opacity", "75%")
	f.click("ui:save")
	var set artplugin.SetShadeArgs
	sets := f.requests.Of(artplugin.SetShadeEdit)
	if len(sets) != 1 || sets[0].Decode(&set) != nil || set.Shade.Color != 0x112233 || set.Shade.Opacity != 75 ||
		set.Shade.ID != art.StarterShadeID(3) {
		t.Fatalf("Save sends the colour and opacity at once: %+v", set)
	}
	f.applyAll()
	if !f.has("saved") || f.state.Unsaved(f.docs) != 0 {
		t.Fatal("the saved shade leaves nothing unsaved")
	}
}

// TestRampNameAndRemovals renames a ramp with Enter, and removes a shade and
// then its ramp; a removal refused because a tile uses the colour says why
// and keeps the choice.
func TestRampNameAndRemovals(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ramp:glass")
	if f.state.Color() != glass() {
		t.Fatal("a ramp's name chooses its first shade")
	}
	f.retype("ui:ramp-name", "window")
	f.keys("Enter")
	var renamed artplugin.RenameRampArgs
	if err := f.requests.Of(artplugin.RenameRampEdit)[0].Decode(&renamed); err != nil || renamed.Name != "window" {
		t.Fatalf("Enter saves the name: %+v", renamed)
	}
	f.applyAll()
	if !f.has("color:window 1") || !f.has("saved") {
		t.Fatal("the renamed ramp keeps its shade")
	}
	f.click("ui:remove-shade")
	refused := f.requests.Commands[len(f.requests.Commands)-1]
	refused.Done(protocol.Result{OK: false, Reason: "art.shade.remove refused: used by Lantern", Revision: 0,
		Created: ids.ULID{}, Item: ""}, errors.New("refused"))
	f.applied++
	if !strings.Contains(f.status(), "used by Lantern") || f.state.Color() != glass() {
		t.Fatalf("a refused removal says why: %q", f.status())
	}
	f.click("ui:remove-shade")
	f.applyAll()
	if f.state.Color().Ramp != glassRampID || !f.state.Color().Shade.IsZero() {
		t.Fatalf("removing a ramp's last shade leaves the ramp chosen: %+v", f.state.Color())
	}
	f.click("ui:remove-ramp")
	f.applyAll()
	if !f.state.Color().IsClear() {
		t.Fatal("removing the ramp chooses clear")
	}
	if _, ok := f.state.Locate("ramp:window", f.docs); ok {
		t.Fatal("the removed ramp is gone from the panel")
	}
}

// TestUnsavedShadeAsksBeforeChoosingAnother asks about a typed colour before
// another colour is chosen: Cancel keeps it, Don't save drops it and moves
// on.
func TestUnsavedShadeAsksBeforeChoosingAnother(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("swatch:grey 3")
	f.retype("ui:shade-color", "#445566")
	f.click("swatch:grey 1")
	if f.state.Color() != grey(3) {
		t.Fatal("the choice waits for the author")
	}
	f.click("ui:" + "cancel")
	if f.state.Color() != grey(3) || f.state.Unsaved(f.docs) != 1 {
		t.Fatal("Cancel keeps the typed colour")
	}
	f.click("swatch:grey 1")
	f.click("ui:" + frame.GuardDiscard)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.state.Color() != grey(1) || f.state.Unsaved(f.docs) != 0 || len(f.requests.Commands) != 0 {
		t.Fatalf("Don't save moves on: %+v", f.state.Color())
	}
	if !f.has("tool:pencil") {
		t.Fatal("painting is untouched by the question")
	}
}

func itoa(value int) string {
	return string(rune('0' + value))
}

// TestShowsChoosesTheStandInItReplaces makes the sprite show the small
// pickup, then the medium pickup, taking it over from Chest in one step that
// also puts the small pickup back to its flat colour, then nothing. Walls
// are not offered.
func TestShowsChoosesTheStandInItReplaces(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if !f.has("shows:") {
		t.Fatal("a new sprite shows nothing")
	}
	f.click("ui:shows")
	if _, offered := f.state.Locate("menu:standin:wall.a", f.docs); offered {
		t.Fatal("a wall family cannot be shown by a sprite")
	}
	f.click("menu:standin:pickup.small")
	f.applyAll()
	if !f.has("shows:pickup.small") || f.status() != "Shows Small pickup on every map" {
		t.Fatalf("shows the small pickup: %q", f.status())
	}
	before := len(f.requests.Commands)
	f.click("ui:shows")
	f.click("menu:standin:pickup.medium")
	sent := f.requests.Commands[before:]
	var cleared, set artplugin.StandInArgs
	if len(sent) != 2 || sent[0].Command.Gesture != sent[1].Command.Gesture ||
		sent[0].Decode(&cleared) != nil || sent[1].Decode(&set) != nil {
		t.Fatalf("one step of two commands: %+v", sent)
	}
	if cleared.StandIn != "pickup.small" || !cleared.Sprite.IsZero() || set.StandIn != "pickup.medium" ||
		set.Sprite != spriteID {
		t.Fatalf("cleared %+v, set %+v", cleared, set)
	}
	f.applyAll()
	if !f.has("shows:pickup.medium") || !strings.Contains(f.status(), "in place of Chest") {
		t.Fatalf("takes over the medium pickup: %q", f.status())
	}
	f.click("ui:shows")
	f.click("menu:standin:")
	f.applyAll()
	if !f.has("shows:") || f.status() != "Shows nothing: Medium pickup is its flat colour again" {
		t.Fatalf("shows nothing: %q", f.status())
	}
}

// TestALongPaletteScrolls gives the shared palette sixteen more ramps of
// sixteen shades, too many to fit. The swatches scroll with the wheel, between
// the top and their last row, while + Ramp, the chosen shade's fields, Shows
// and the save bar stay below them; a swatch out of view is scrolled to and
// chosen.
func TestALongPaletteScrolls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.docs.View(art.SharedPaletteID(), func(document client.Document) {
		shared := document.Model.(*artplugin.PaletteDocument).Palette
		for number := range 16 {
			ramp := art.Ramp{ID: longID(number * 20), Name: "r" + strconv.Itoa(number), Shades: nil}
			for shade := range 16 {
				ramp.Shades = append(ramp.Shades, art.Shade{
					ID: longID(number*20 + shade + 1), Color: art.RGB(0x0f0f0f * (shade + 1)), Opacity: 100,
				})
			}
			if err := shared.InsertRamp(len(shared.Ramps()), ramp); err != nil {
				t.Fatal(err)
			}
		}
	})
	f.docs.Touch(art.SharedPaletteID())
	f.frame(ui.NewInput(image.Pt(0, 0)))
	area := f.state.PanelArea()
	if f.state.PanelScroll() != 0 || area.Empty() {
		t.Fatalf("the swatches start at the top: %d in %v", f.state.PanelScroll(), area)
	}
	last := f.at("swatch:r15 16")
	if f.state.PanelScroll() == 0 || !last.In(area) {
		t.Fatalf("the last swatch is scrolled into view: %v in %v, scrolled %d", last, area, f.state.PanelScroll())
	}
	f.click("swatch:r15 16")
	if !f.has("color:r15 16") {
		t.Fatal("the last swatch is chosen")
	}
	area = f.state.PanelArea()
	for _, widget := range []string{"ui:add-ramp", "ui:shade-color", "ui:shade-opacity", "ui:shows"} {
		if at := f.at(widget); at.Y <= area.Max.Y {
			t.Fatalf("%s stays below the swatches: %v, area %v", widget, at, area)
		}
	}
	wheel := func(amount float64) int {
		input := ui.NewInput(ui.Center(area))
		input.WheelY = amount
		f.frame(input)

		return f.state.PanelScroll()
	}
	if top := wheel(-1000); top != 0 {
		t.Fatalf("scrolling up stops at the top: %d", top)
	}
	bottom := wheel(1000)
	if bottom <= 0 || bottom != wheel(10) {
		t.Fatalf("scrolling down stops at the last row: %d", bottom)
	}
}

// longID is a test ID for a long palette's ramps and shades, beyond the
// fixture's.
func longID(number int) ids.ULID {
	return ids.MustParse(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G%04d", number))
}
