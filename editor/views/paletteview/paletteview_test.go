package paletteview_test

import (
	"encoding/json"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/paletteview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

var (
	gloomyID = frametest.ID(6)
	zoneID   = frametest.ID(7)
	spriteID = frametest.ID(8)
	tileID   = frametest.ID(9)
)

// shadeID numbers gloomy's shades: smoke 1 is shadeID(11), murk 1
// shadeID(21), and so on.
func shadeID(number int) ids.ULID { return frametest.ID(byte(number)) }

func rampID(number int) ids.ULID { return frametest.ID(byte(100 + number)) }

// gloomy is a mood palette: smoke of four greys, murk of three greens, ember
// of four oranges, the last labelled "glow", and fog of two see-through
// greys.
func gloomy(t *testing.T) *art.Palette {
	t.Helper()
	palette := art.NewPalette()
	for at, ramp := range []struct {
		name   string
		colors []art.RGB
	}{
		{"smoke", []art.RGB{0x2E2C33, 0x5D5A63, 0x87838D, 0xB3AFB8}},
		{"murk", []art.RGB{0x1D2620, 0x34443A, 0x52665A}},
		{"ember", []art.RGB{0x3A1408, 0x6B2A10, 0xA8481C, 0xE07A30}},
		{"fog", []art.RGB{0xC7C9CC, 0xE1E3E6}},
	} {
		made := art.Ramp{ID: rampID(at + 1), Name: ramp.name, Shades: nil}
		for number, value := range ramp.colors {
			shade := art.Shade{ID: shadeID((at+1)*10 + number + 1), Color: value, Opacity: 100}
			if ramp.name == "fog" {
				shade.Opacity = 60 - 20*number
			}
			if ramp.name == "ember" && number == 3 {
				shade.Label = "glow"
			}
			made.Shades = append(made.Shades, shade)
		}
		if err := palette.InsertRamp(at, made); err != nil {
			t.Fatal(err)
		}
	}

	return palette
}

type fixture struct {
	t        *testing.T
	state    *paletteview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
	applied  int
	made     byte
	screen   *ebiten.Image
}

// newFixture opens the Palette Editor on a palette: gloomy, or Default.
func newFixture(t *testing.T, target ids.ULID) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	defaults, err := art.DefaultPalette()
	if err != nil {
		t.Fatal(err)
	}
	tile, err := art.NewTile(8)
	if err != nil {
		t.Fatal(err)
	}
	docs := frametest.NewDocs()
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: art.DefaultPaletteID(), Name: art.DefaultPaletteName, Format: art.PaletteFormatName},
		{ID: gloomyID, Name: "gloomy", Format: art.PaletteFormatName},
		{ID: zoneID, Name: "Test zone", Format: "zone"},
		{ID: spriteID, Name: "Chimney top", Format: art.SpriteFormatName},
		{ID: tileID, Name: "Tile 1", Format: art.TileFormatName},
	}))
	docs.Put(art.DefaultPaletteID(), art.DefaultPaletteName, &artplugin.PaletteDocument{Palette: defaults})
	docs.Put(gloomyID, "gloomy", &artplugin.PaletteDocument{Palette: gloomy(t)})
	zone := zoneplugin.NewDocument()
	if err := zone.Zone.InsertPalette(0, gloomyID); err != nil {
		t.Fatal(err)
	}
	docs.Put(zoneID, "Test zone", zone)
	docs.Put(tileID, "Tile 1", &artplugin.TileDocument{Tile: tile})
	docs.Put(spriteID, "Chimney top", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID}})
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := paletteview.New(frametest.ID(99), target, theme)
	state.Start(ctx)
	f := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx, screen: ebiten.NewImage(1200, 800)}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

// frame runs one frame of input, then draws the window, as every frame does.
func (f *fixture) frame(input ui.Input) {
	f.state.Update(&input, f.ctx)
	f.state.Draw(f.screen, f.docs)
}

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

func (f *fixture) found(name string) bool {
	_, ok := f.state.Locate(name, f.docs)

	return ok
}

// clickWith clicks a part with modifiers held.
func (f *fixture) clickWith(name string, held ui.Mods) {
	f.t.Helper()
	point := f.at(name)
	press := ui.NewInput(point)
	press.Pressed, press.Down, press.Held = true, true, held
	f.frame(press)
	release := ui.NewInput(point)
	release.Released, release.Held = true, held
	f.frame(release)
}

func (f *fixture) click(name string) { f.clickWith(name, ui.Plain()) }

// drag presses on one part, moves to another with the button held, and
// releases there.
func (f *fixture) drag(from, to string) {
	f.t.Helper()
	start, end := f.at(from), f.at(to)
	press := ui.NewInput(start)
	press.Pressed, press.Down = true, true
	f.frame(press)
	for _, point := range []image.Point{start.Add(image.Pt(6, 6)), end} {
		move := ui.NewInput(point)
		move.Down = true
		f.frame(move)
	}
	release := ui.NewInput(end)
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

// retype replaces a field's text.
func (f *fixture) retype(field, text string) {
	f.t.Helper()
	f.click(field)
	f.keys("Cmd+A")
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

func (f *fixture) has(state string) bool { return f.found("state:" + state) }

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

// applyAll applies every art and zone command not yet applied, as the
// orchestrator would, and answers each; project commands are answered
// without being applied, giving a created document the next ID.
func (f *fixture) applyAll() {
	f.t.Helper()
	for ; f.applied < len(f.requests.Commands); f.applied++ {
		command := f.requests.Commands[f.applied]
		if command.Command.Document == project.IndexID() {
			command.Succeed(frametest.ID(200))

			continue
		}
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
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

func (f *fixture) newID() (ids.ULID, error) {
	f.made++

	return frametest.ID(150 + f.made), nil
}

func editKind(t *testing.T, name string) model.EditKind {
	t.Helper()
	for _, kind := range append(artplugin.Plugin().Edits, zoneplugin.Plugin().Edits...) {
		if kind.Name() == name {
			return kind
		}
	}
	t.Fatalf("no edit %s", name)

	return nil
}

// sent decodes the arguments of the commands of an edit sent so far.
func sent[A any](f *fixture, edit string) []A {
	f.t.Helper()
	var all []A
	for _, command := range f.requests.Of(edit) {
		var args A
		if err := command.Decode(&args); err != nil {
			f.t.Fatal(err)
		}
		all = append(all, args)
	}

	return all
}

// answerUses answers every where-used question not yet answered: about a
// document, or an item in it, with what uses says, or none.
func (f *fixture) answerUses(uses func(question frametest.UsesQuestion) []protocol.Use) {
	pending := f.requests.Uses
	questions := f.requests.UsesOf
	f.requests.Uses, f.requests.UsesOf = nil, nil
	for at, done := range pending {
		done(uses(questions[at]), nil)
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

func noUses(frametest.UsesQuestion) []protocol.Use { return nil }

func (f *fixture) palette(id ids.ULID) *art.Palette {
	var found *art.Palette
	f.docs.View(id, func(document client.Document) { found = document.Model.(*artplugin.PaletteDocument).Palette })

	return found
}

// TestOpensAPaletteAndChoosesColours opens gloomy: the title names it, a
// click chooses a colour, Shift-click picks a run, Cmd-click adds or takes
// away one, and a ramp's name picks the whole ramp.
func TestOpensAPaletteAndChoosesColours(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	if title := f.state.Title(f.docs); !strings.HasPrefix(title, "Palette Editor: gloomy  (window ") {
		t.Fatalf("title %q", title)
	}
	if !f.has("loaded") || !f.has("ramps:smoke,murk,ember,fog") || f.state.ReadOnly() {
		t.Fatal("gloomy loads, editable")
	}
	f.click("swatch:smoke 2")
	if !f.has("chosen:smoke 2") || !f.has("picked:1") {
		t.Fatal("a click chooses smoke 2")
	}
	f.clickWith("swatch:murk 2", ui.Mods{Shift: true})
	if !f.has("chosen:smoke 2") || !f.has("picked:5") {
		t.Fatal("Shift-click picks smoke 2 to murk 2")
	}
	f.clickWith("swatch:smoke 3", ui.Cmd())
	if !f.has("picked:4") {
		t.Fatal("Cmd-click takes smoke 3 away")
	}
	f.click("ramp:ember")
	if !f.has("chosen:ember 1") || !f.has("picked:4") {
		t.Fatal("a ramp's name picks the whole ramp")
	}
	f.state.Show(f.docs, shadeID(42).String())
	if !f.has("chosen:fog 2") {
		t.Fatal("Edit palette… opens at the colour it names")
	}
}

// TestAddsRenamesAndRemovesRamps adds a ramp with one mid grey and chooses
// it, renames it with Enter, adds a copy of its colour after it, removes that
// colour, then the ramp.
func TestAddsRenamesAndRemovesRamps(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("ui:add-ramp")
	adds := sent[artplugin.AddRampArgs](f, artplugin.AddRampEdit)
	if len(adds) != 1 || adds[0].Index != 4 || adds[0].Ramp.Name != "new ramp" || len(adds[0].Ramp.Shades) != 1 {
		t.Fatalf("add ramp sent %+v", adds)
	}
	f.applyAll()
	if !f.has("chosen:new ramp 1") {
		t.Fatal("the new ramp's colour is chosen")
	}
	f.retype("ui:ramp-name", "ash")
	f.keys("Enter")
	f.applyAll()
	if !f.has("ramps:smoke,murk,ember,fog,ash") || !f.has("saved") {
		t.Fatal("Enter renames the ramp")
	}
	f.click("ui:duplicate-colour")
	f.applyAll()
	if !f.has("chosen:ash 2") || !f.has("colour:ash 2:#808080") {
		t.Fatal("Duplicate colour copies the chosen colour")
	}
	f.click("ui:remove-colour")
	f.applyAll()
	if !f.has("chosen:ash 1") {
		t.Fatal("removing a colour chooses its neighbour")
	}
	f.click("ui:remove-ramp")
	f.applyAll()
	if !f.has("ramps:smoke,murk,ember,fog") || f.found("ramp:ash") {
		t.Fatal("the ramp is removed")
	}
}

// TestTypedColourWaitsForSave types a label, a colour and an opacity:
// nothing is sent until Save, which sends all three at once. A label of more
// than 15 characters is refused and kept to correct, and choosing another
// colour asks first.
func TestTypedColourWaitsForSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("swatch:smoke 2")
	f.retype("ui:label", "a chimney's soot")
	f.retype("ui:colour", "#5A5662")
	if len(f.requests.Commands) != 0 || f.state.Unsaved(f.docs) != 2 {
		t.Fatalf("typing waits for Save: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	if len(f.requests.Commands) != 0 || !strings.HasPrefix(f.status(), "A label is at most 15 characters") {
		t.Fatalf("sixteen characters are refused: %q", f.status())
	}
	f.retype("ui:label", "chimney")
	f.retype("ui:opacity", "75%")
	f.click("ui:save")
	sets := sent[artplugin.SetShadeArgs](f, artplugin.SetShadeEdit)
	if len(sets) != 1 || sets[0].Shade.Label != "chimney" || sets[0].Shade.Color != 0x5A5662 ||
		sets[0].Shade.Opacity != 75 || sets[0].Shade.ID != shadeID(12) {
		t.Fatalf("Save sends the label, colour and opacity at once: %+v", sets)
	}
	f.applyAll()
	if !f.has("label:smoke 2:chimney") || !f.has("saved") {
		t.Fatal("the label is saved")
	}
	f.retype("ui:colour", "#445566")
	f.click("swatch:murk 1")
	if !f.has("chosen:smoke 2") || !f.found("ui:"+frame.GuardDiscard) {
		t.Fatal("choosing another colour asks about the typed one")
	}
	f.click("ui:" + frame.GuardDiscard)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("chosen:murk 1") {
		t.Fatal("Don't save moves on")
	}
}

// TestDragMovesColoursAndRamps drags ember 1 to the end of murk, where it
// becomes murk 4 and the shadow record lists it while a tile still names its
// old place, and drags fog above smoke.
func TestDragMovesColoursAndRamps(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.drag("swatch:ember 1", "slot:murk 4")
	moves := sent[artplugin.MoveShadeArgs](f, artplugin.MoveShadeEdit)
	if len(moves) != 1 || moves[0].Shade != shadeID(31) || moves[0].Ramp != rampID(2) || moves[0].Index != 3 {
		t.Fatalf("the drag moves ember 1 to murk 4: %+v", moves)
	}
	f.applyAll()
	if !f.has("chosen:murk 4") || !strings.Contains(f.status(), "Moved ember 1 to murk 4") {
		t.Fatalf("the moved colour is chosen: %q", f.status())
	}
	f.answerUses(func(question frametest.UsesQuestion) []protocol.Use {
		if question.Item == artplugin.RampItem(rampID(3)) || question.Item == artplugin.ShadeItem(shadeID(31)) {
			return []protocol.Use{{Document: tileID, Name: "Tile 1", Format: art.TileFormatName, Label: "painted with it"}}
		}

		return nil
	})
	if !f.has("moved:ember 1") {
		t.Fatal("Moved colours lists ember 1 while a tile names it")
	}
	f.drag("swatch:smoke 2", "slot:smoke 4")
	moves = sent[artplugin.MoveShadeArgs](f, artplugin.MoveShadeEdit)
	if len(moves) != 2 || moves[1].Ramp != rampID(1) || moves[1].Index != 2 {
		t.Fatalf("a move within a ramp counts the place without the colour: %+v", moves)
	}
	f.drag("ramp:fog", "rampslot:1")
	ramps := sent[artplugin.MoveRampArgs](f, artplugin.MoveRampEdit)
	if len(ramps) != 1 || ramps[0].Ramp != rampID(4) || ramps[0].Index != 0 {
		t.Fatalf("fog moves to the top: %+v", ramps)
	}
	f.applyAll()
	if !f.has("ramps:fog,smoke,murk,ember") {
		t.Fatal("fog is first")
	}
}

// TestDefaultIsReadOnly opens Default: it has no fields, no ramp is added and
// nothing is dragged, but picked colours copy into a palette of the game's
// own, without labels, and make a new palette.
func TestDefaultIsReadOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t, art.DefaultPaletteID())
	if !f.state.ReadOnly() || f.found("ui:label") || f.found("ui:add-ramp") || f.found("ui:palette-name") ||
		f.found("ui:save") {
		t.Fatal("Default offers nothing to change")
	}
	f.drag("swatch:grey 3", "slot:red 1")
	if len(f.requests.Commands) != 0 {
		t.Fatal("nothing in Default is dragged")
	}
	f.click("swatch:grey 3")
	f.clickWith("swatch:grey 5", ui.Cmd())
	f.click("ui:copy-to")
	f.click("menu:" + gloomyID.String())
	adds := sent[artplugin.AddRampArgs](f, artplugin.AddRampEdit)
	if len(adds) != 1 || adds[0].Ramp.Name != "grey" || len(adds[0].Ramp.Shades) != 2 ||
		adds[0].Ramp.Shades[0].Color != 0x222222 || f.requests.Commands[0].Command.Document != gloomyID {
		t.Fatalf("grey 3 and 5 are copied into a new grey ramp in gloomy: %+v", adds)
	}
	f.click("ramp:red")
	f.clickWith("swatch:grey 3", ui.Cmd())
	f.click("ui:new-palette")
	creates := sent[project.DocumentArgs](f, project.CreateEdit)
	if len(creates) != 1 || creates[0].Name != "New palette" || creates[0].Format != art.PaletteFormatName {
		t.Fatalf("a new palette is made: %+v", creates)
	}
	var wire struct {
		Ramps []art.Ramp `json:"ramps"`
	}
	if err := json.Unmarshal(creates[0].Data, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Ramps) != 2 || wire.Ramps[0].Name != "default" || len(wire.Ramps[0].Shades) != 1 ||
		wire.Ramps[1].Name != "red" || len(wire.Ramps[1].Shades) != 15 || !wire.Ramps[1].Shades[0].ID.IsZero() {
		t.Fatalf("single colours go into “default” first, whole ramps keep their names: %+v", wire.Ramps)
	}
	f.applyAll()
	if len(f.requests.Opened) != 1 || f.requests.Opened[0].View != paletteview.ViewName {
		t.Fatalf("the new palette opens: %+v", f.requests.Opened)
	}
}

// TestGradientFillsRamps adds a ramp of five colours stepped in OKLab, then
// fills smoke with six, setting its four and adding two, as one step; a
// gradient that would remove a colour a tile uses is refused.
func TestGradientFillsRamps(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("ui:gradient")
	f.retype("ui:left", "#2E2C33")
	f.retype("ui:right", "#E1E3E6")
	f.retype("ui:steps", "5")
	f.click("ui:apply-gradient")
	adds := sent[artplugin.AddRampArgs](f, artplugin.AddRampEdit)
	if len(adds) != 1 || adds[0].Ramp.Name != "gradient" || len(adds[0].Ramp.Shades) != 5 ||
		adds[0].Ramp.Shades[2].Color != 0x828187 {
		t.Fatalf("a new ramp of five: %+v", adds)
	}
	f.applyAll()
	f.click("ui:gradient-target")
	f.click("menu:" + rampID(1).String())
	f.retype("ui:steps", "6")
	f.click("ui:apply-gradient")
	sets := f.requests.Of(artplugin.SetShadeEdit)
	added := f.requests.Of(artplugin.AddShadeEdit)
	if len(sets) != 4 || len(added) != 2 || sets[0].Command.Gesture == "" || sets[0].Command.Gesture != added[1].Command.Gesture {
		t.Fatalf("smoke's four are set and two added, as one step: %d %d", len(sets), len(added))
	}
	f.applyAll()
	f.click("ui:gradient-target")
	f.click("menu:" + rampID(2).String())
	f.retype("ui:steps", "2")
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.answerUses(func(question frametest.UsesQuestion) []protocol.Use {
		if question.Item == artplugin.ShadeItem(shadeID(23)) {
			return []protocol.Use{{Document: tileID, Name: "Tile 1", Format: art.TileFormatName, Label: "painted with it"}}
		}

		return nil
	})
	before := len(f.requests.Commands)
	f.click("ui:apply-gradient")
	if len(f.requests.Commands) != before || !strings.Contains(f.status(), "murk 3 is used by Tile 1") {
		t.Fatalf("removing a used colour is refused: %q", f.status())
	}
}

// TestCopiesColoursFromDefault picks Default's slate 2 to 6 and copies them
// into a new ramp named slate, without labels.
func TestCopiesColoursFromDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("ui:copy-from")
	f.click("source:slate 2")
	f.clickWith("source:slate 6", ui.Mods{Shift: true})
	f.click("ui:copy")
	adds := sent[artplugin.AddRampArgs](f, artplugin.AddRampEdit)
	if len(adds) != 1 || adds[0].Ramp.Name != "slate" || len(adds[0].Ramp.Shades) != 5 ||
		adds[0].Ramp.Shades[0].Color != 0x131927 || !adds[0].Ramp.Shades[0].ID.IsZero() {
		t.Fatalf("five copies in a new ramp: %+v", adds)
	}
	f.applyAll()
	if !f.has("ramps:smoke,murk,ember,fog,slate") || !f.has("label:slate 1:") {
		t.Fatal("the copies are independent, without labels")
	}
	f.click("sourceramp:earth")
	f.click("ui:copy")
	adds = sent[artplugin.AddRampArgs](f, artplugin.AddRampEdit)
	if len(adds) != 2 || adds[1].Ramp.Name != "earth" || len(adds[1].Ramp.Shades) != 15 {
		t.Fatalf("a ramp's name, scrolled to, copies it whole: %+v", adds)
	}
}

// TestDeleteRemovesReferencesFirst lists the zone and the tile that refer to
// gloomy: Delete waits until each is removed; removing it from the tile
// keeps each pixel's colour, as Default's exact match or a loose colour.
func TestDeleteRemovesReferencesFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.docs.View(tileID, func(document client.Document) {
		tile := document.Model.(*artplugin.TileDocument).Tile
		for col, ref := range []art.ColorRef{
			art.Named(gloomyID, rampID(1), shadeID(11)), art.Named(gloomyID, rampID(2), shadeID(21)),
		} {
			if _, err := tile.Set(col, 0, ref); err != nil {
				t.Fatal(err)
			}
		}
	})
	f.docs.View(gloomyID, func(document client.Document) {
		if _, err := document.Model.(*artplugin.PaletteDocument).Palette.SetShade(rampID(2), art.Shade{
			ID: shadeID(21), Color: 0x2B3449, Opacity: 100,
		}); err != nil {
			t.Fatal(err)
		}
	})
	f.click("ui:delete")
	uses := []protocol.Use{
		{Document: zoneID, Name: "Test zone", Format: "zone", Label: "listed by the zone"},
		{Document: tileID, Name: "Tile 1", Format: art.TileFormatName, Label: "painted with it"},
	}
	f.answerUses(func(question frametest.UsesQuestion) []protocol.Use {
		if question.Item == "" {
			return uses
		}

		return nil
	})
	f.click("ui:confirm-delete")
	if len(f.requests.Of(project.DeleteEdit)) != 0 || f.has("deletable") {
		t.Fatal("Delete waits while the palette is used")
	}
	f.click("ui:remove-use-1")
	f.click("ui:remove-use-2")
	if removes := sent[zoneplugin.PaletteArgs](f, zoneplugin.RemovePaletteEdit); len(removes) != 1 ||
		removes[0].Palette != gloomyID || f.requests.Of(zoneplugin.RemovePaletteEdit)[0].Command.Document != zoneID {
		t.Fatalf("the zone stops listing gloomy: %+v", removes)
	}
	paints := sent[artplugin.PaintArgs](f, artplugin.PaintEdit)
	if len(paints) != 1 || len(paints[0].Pixels) != 2 {
		t.Fatalf("the tile's two pixels are repainted: %+v", paints)
	}
	for _, pixel := range paints[0].Pixels {
		want := art.Loose(0x2E2C33, 100)
		if pixel.Col == 1 {
			want = art.Named(art.DefaultPaletteID(), art.DefaultRampID(17), art.DefaultShadeID(17, 4))
		}
		if pixel.Color != want {
			t.Fatalf("pixel %d keeps its colour: %+v", pixel.Col, pixel.Color)
		}
	}
	f.applyAll()
	f.answerUses(noUses)
	if !f.has("deletable") {
		t.Fatal("with nothing left, the palette can be deleted")
	}
	f.click("ui:confirm-delete")
	deletes := sent[project.DocumentArgs](f, project.DeleteEdit)
	if len(deletes) != 1 || deletes[0].ID != gloomyID {
		t.Fatalf("Delete deletes gloomy: %+v", deletes)
	}
	if !slices.Contains(f.requests.Subscribed, tileID) {
		t.Fatal("the tile is held, to work out its colours")
	}
}

// TestPickerWaitsForSave picks a colour and an opacity by eye: they wait in
// the colour and opacity fields, as typing does, and Save sends them.
func TestPickerWaitsForSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("swatch:smoke 2")
	f.click("picker:hue 0")
	f.click("picker:square 100 100")
	if len(f.requests.Commands) != 0 || f.state.Unsaved(f.docs) != 1 {
		t.Fatalf("a picked colour waits for Save: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.drag("picker:opacity 100", "picker:opacity 50")
	if f.state.Unsaved(f.docs) != 2 {
		t.Fatalf("a picked opacity waits too: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	sets := sent[artplugin.SetShadeArgs](f, artplugin.SetShadeEdit)
	if len(sets) != 1 || sets[0].Shade.Color != 0xFF0000 || sets[0].Shade.Opacity != 50 ||
		sets[0].Shade.ID != shadeID(12) {
		t.Fatalf("Save sends the picked colour and opacity: %+v", sets)
	}
	f.applyAll()
	f.retype("ui:colour", "#00FF00")
	f.click("picker:opacity 100")
	if sets := sent[artplugin.SetShadeArgs](f, artplugin.SetShadeEdit); len(sets) != 1 {
		t.Fatalf("picking sends nothing: %+v", sets)
	}
	f.click("ui:save")
	sets = sent[artplugin.SetShadeArgs](f, artplugin.SetShadeEdit)
	if len(sets) != 2 || sets[1].Shade.Color != 0x00FF00 || sets[1].Shade.Opacity != 100 {
		t.Fatalf("the picker starts from the typed colour: %+v", sets)
	}
}

// TestMovesAColourToARamp drops ember 1 on murk's name, which puts it at
// murk's end, and moves smoke 2 to fog with Move to ramp.
func TestMovesAColourToARamp(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.drag("swatch:ember 1", "ramp:murk")
	moves := sent[artplugin.MoveShadeArgs](f, artplugin.MoveShadeEdit)
	if len(moves) != 1 || moves[0].Shade != shadeID(31) || moves[0].Ramp != rampID(2) || moves[0].Index != 3 {
		t.Fatalf("dropped on murk's name, ember 1 goes to murk's end: %+v", moves)
	}
	f.applyAll()
	f.click("swatch:smoke 2")
	f.click("ui:move-to")
	if f.found("menu:" + rampID(1).String()) {
		t.Fatal("Move to ramp does not offer the colour's own ramp")
	}
	f.click("menu:" + rampID(4).String())
	moves = sent[artplugin.MoveShadeArgs](f, artplugin.MoveShadeEdit)
	if len(moves) != 2 || moves[1].Shade != shadeID(12) || moves[1].Ramp != rampID(4) || moves[1].Index != 2 {
		t.Fatalf("Move to ramp puts smoke 2 at fog's end: %+v", moves)
	}
	f.applyAll()
	if !f.has("chosen:fog 3") {
		t.Fatal("the moved colour stays chosen")
	}
}

// TestOpacitySliderWaitsForSave sets an opacity with the slider: it waits
// in the opacity field for Save.
func TestOpacitySliderWaitsForSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("swatch:smoke 2")
	f.drag("slider:100", "slider:40")
	if len(f.requests.Commands) != 0 || f.state.Unsaved(f.docs) != 1 {
		t.Fatalf("the slider's opacity waits for Save: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	sets := sent[artplugin.SetShadeArgs](f, artplugin.SetShadeEdit)
	if len(sets) != 1 || sets[0].Shade.Opacity != 40 || sets[0].Shade.Color != 0x5D5A63 {
		t.Fatalf("Save sends the slider's opacity: %+v", sets)
	}
}

// TestGradientPicksItsEnds picks the right end's colour, and with Opacity
// mixed its opacity, in the gradient's own picker; the left end is kept.
func TestGradientPicksItsEnds(t *testing.T) {
	t.Parallel()
	f := newFixture(t, gloomyID)
	f.click("ui:gradient")
	f.retype("ui:left", "#2E2C33")
	f.click("end:right")
	if f.found("gradient-picker:opacity 50") {
		t.Fatal("the picker has no opacity until Opacity is mixed")
	}
	f.click("gradient-picker:hue 0")
	f.click("gradient-picker:square 100 100")
	f.click("ui:mix-opacity")
	f.click("gradient-picker:opacity 50")
	f.retype("ui:steps", "3")
	f.click("ui:apply-gradient")
	adds := sent[artplugin.AddRampArgs](f, artplugin.AddRampEdit)
	if len(adds) != 1 || len(adds[0].Ramp.Shades) != 3 || adds[0].Ramp.Shades[0].Color != 0x2E2C33 ||
		adds[0].Ramp.Shades[2].Color != 0xFF0000 || adds[0].Ramp.Shades[2].Opacity != 50 {
		t.Fatalf("the gradient ends at the picked red, half see-through: %+v", adds)
	}
	f.click("end:left")
	f.click("gradient-picker:square 0 100")
	if f.state.Unsaved(f.docs) != 0 {
		t.Fatal("the gradient's settings never wait for Save")
	}
}
