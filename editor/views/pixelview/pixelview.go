// Package pixelview is the Pixel editor: a sprite's tile, magnified by a
// whole number over a checkerboard, painted with a pencil, an eraser, a fill
// and an eyedropper in the shared palette's colours, whose ramps and shades
// its palette panel edits (increment 11).
//
// A stroke is one undo step, shared by every view of the sprite, and other
// views of the tile show it as it is drawn. The panel's typed fields wait for
// its save bar, as every inspector's do; adding and removing ramps and shades
// apply at once.
package pixelview

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ViewName is the Pixel editor's view name; its target is a sprite.
const ViewName = "sprite"

// shortID is how much of the window's ID its title shows.
const shortID = 6

// The panel's typed fields hold at most this many bytes.
const (
	colorBytes   = 16
	opacityBytes = 8
)

// stroke is the pencil or eraser drag in progress; count numbers the
// window's strokes and fills, for their gestures.
type stroke struct {
	active bool
	count  int
	last   image.Point
}

// State is one Pixel editor window.
type State struct {
	window ids.ULID
	sprite ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	// palettes are the palettes the window shows: the shared palette and
	// any other a tile's colours name.
	palettes []ids.ULID
	tool     Tool
	// color is what the pencil and the fill paint with and what the palette
	// panel shows: clear, a shade, or a ramp alone, chosen by its name.
	color art.ColorRef
	// zoom is the whole number each tile pixel is magnified by, zero until
	// the tile is first fitted; pan moves the tile from the canvas's centre.
	zoom   int
	pan    image.Point
	wheel  float64
	stroke stroke
	// hover is the tile pixel under the cursor, when hovering.
	hover    image.Point
	hovering bool

	tools   []*ui.Button
	zoomOut *ui.Button
	zoomIn  *ui.Button
	zoomFit *ui.Button
	grid    *ui.Toggle
	panel   panelLayout
	// scroll is how far the palette panel's swatches are scrolled.
	scroll int
	// named is the ramp and shade the typed fields hold.
	named        art.ColorRef
	rampName     *ui.TextField
	shadeColor   *ui.TextField
	shadeOpacity *ui.TextField
	addRamp      *ui.Button
	addShade     *ui.Button
	removeShade  *ui.Button
	removeRamp   *ui.Button
	// shows opens showsMenu, which chooses the stand-in the sprite shows.
	shows     *ui.Button
	showsMenu ui.Menu
	save      *ui.SaveBar
	guard     frame.Guard
	focus     ui.Focus
	picture   picture
}

// New returns the Pixel editor for a sprite.
func New(window, sprite ids.ULID, theme *ui.Theme) *State {
	field := func(id string, maxBytes int) *ui.TextField {
		return ui.NewTextField(id, textedit.NewField("", false, maxBytes), false, theme)
	}
	view := &State{
		window: window, sprite: sprite, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		palettes: []ids.ULID{art.SharedPaletteID()}, tool: ToolPencil, color: clearColor(),
		zoom: 0, pan: image.Pt(0, 0), wheel: 0, stroke: stroke{active: false, count: 0, last: image.Pt(0, 0)},
		hover: image.Pt(0, 0), hovering: false,
		tools: nil, zoomOut: ui.NewButton("zoom-out", "−"), zoomIn: ui.NewButton("zoom-in", "+"),
		zoomFit: ui.NewButton("zoom-fit", "Fit"),
		grid:    &ui.Toggle{ID: "grid", Label: "Grid", Rect: ui.Zero(), On: true},
		panel:   newPanelLayout(), scroll: 0, named: clearColor(),
		rampName:     field("ramp-name", art.MaxNameLength),
		shadeColor:   field("shade-color", colorBytes),
		shadeOpacity: field("shade-opacity", opacityBytes),
		addRamp:      ui.NewButton("add-ramp", "+ Ramp"), addShade: ui.NewButton("add-shade", "+ Shade"),
		removeShade: ui.NewButton("remove-shade", "Remove shade"),
		removeRamp:  ui.NewButton("remove-ramp", "Remove ramp"),
		shows:       ui.NewButton("shows", "Nothing"), showsMenu: ui.Menu{Items: nil, Width: 0},
		save: ui.NewSaveBar(), guard: frame.Guard{}, focus: ui.Focus{}, picture: newPicture(),
	}
	view.removeRamp.Danger = true
	view.shows.Caret = true
	for _, info := range tools() {
		view.tools = append(view.tools, ui.NewButton("tool-"+string(info.tool), info.label))
	}
	view.focus.Register(view.rampName, view.shadeColor, view.shadeOpacity)

	return view
}

// clearColor is the clear colour, which erases.
func clearColor() art.ColorRef {
	return art.ColorRef{Palette: ids.ULID{}, Ramp: ids.ULID{}, Shade: ids.ULID{}}
}

// Tool returns the current tool.
func (s *State) Tool() Tool { return s.tool }

// Color returns the colour the pencil and the fill paint with.
func (s *State) Color() art.ColorRef { return s.color }

// Zoom returns how many screen pixels show each tile pixel.
func (s *State) Zoom() int { return s.zoom }

// Title is the window title: the sprite's name.
func (s *State) Title(docs frame.Docs) string {
	name := "Sprite"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(s.sprite, name)
		}
	})

	return "Sprite: " + name + "  (window " + s.window.Short(shortID) + ")"
}

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Start subscribes to what the window shows. The window has already
// subscribed to the sprite; this adds the project index for names, the art
// settings for the tile size, the stand-in sprites for Shows, the shared
// palette, and the sprite's tile.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
	s.subscribe(art.SettingsID(), ctx)
	s.subscribe(art.StandInsID(), ctx)
	s.syncSubscriptions(ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] {
		return
	}
	s.subs[document] = true
	ctx.Requests.Subscribe(document, func(err error) {
		if err != nil {
			delete(s.subs, document)
			ctx.Frame.Status(err.Error())
		}
	})
}

// Changed keeps the window subscribed to the sprite's tile and to every
// palette the tile's colours name.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.Event != nil || update.Snapshot != nil {
		s.syncSubscriptions(ctx)
	}
}

// syncSubscriptions subscribes to the sprite's tile, the shared palette and
// every palette the tile is painted from.
func (s *State) syncSubscriptions(ctx *frame.Context) {
	tileID, ok := s.tileID(ctx.Docs)
	if ok {
		s.subscribe(tileID, ctx)
		ctx.Docs.View(tileID, func(document client.Document) {
			if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
				for _, ref := range tileDoc.Tile.Colors() {
					if !slices.Contains(s.palettes, ref.Palette) {
						s.palettes = append(s.palettes, ref.Palette)
					}
				}
			}
		})
	}
	for _, palette := range s.palettes {
		s.subscribe(palette, ctx)
	}
}

// tileID returns the sprite's tile.
func (s *State) tileID(docs frame.Docs) (ids.ULID, bool) {
	var tile ids.ULID
	docs.View(s.sprite, func(document client.Document) {
		if sprite, ok := document.Model.(*artplugin.SpriteDocument); ok {
			tile = sprite.Sprite.Tile
		}
	})

	return tile, !tile.IsZero()
}

// tileSize returns the size of the sprite's tile, once it is loaded.
func (s *State) tileSize(docs frame.Docs) (int, bool) {
	size := 0
	if tileID, ok := s.tileID(docs); ok {
		docs.View(tileID, func(document client.Document) {
			if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
				size = tileDoc.Tile.Size()
			}
		})
	}

	return size, size > 0
}

// pixelColor returns the colour of one of the tile's pixels.
func (s *State) pixelColor(docs frame.Docs, pixel image.Point) (art.ColorRef, bool) {
	ref, found := clearColor(), false
	if tileID, ok := s.tileID(docs); ok {
		docs.View(tileID, func(document client.Document) {
			if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
				at, err := tileDoc.Tile.At(pixel.X, pixel.Y)
				ref, found = at, err == nil
			}
		})
	}

	return ref, found
}

// projectTileSize returns the project's tile size, from its art settings.
func projectTileSize(docs frame.Docs) (int, bool) {
	size := 0
	docs.View(art.SettingsID(), func(document client.Document) {
		if settings, ok := document.Model.(*artplugin.SettingsDocument); ok {
			size = settings.Settings.TileSize
		}
	})

	return size, size > 0
}

// palette returns a copy of a palette.
func palette(docs frame.Docs, paletteID ids.ULID) (*art.Palette, bool) {
	var found *art.Palette
	docs.View(paletteID, func(document client.Document) {
		if paletteDoc, ok := document.Model.(*artplugin.PaletteDocument); ok {
			found = paletteDoc.Palette.Clone()
		}
	})

	return found, found != nil
}

// colorName names a colour as the panel and the readout show it: "clear",
// "grey 3" in the shared palette, or with its palette's name before it in
// any other.
func (s *State) colorName(docs frame.Docs, ref art.ColorRef) (string, bool) {
	if ref.IsClear() {
		return "clear", true
	}
	found, ok := palette(docs, ref.Palette)
	if !ok {
		return "", false
	}
	name, ok := found.ShadeName(ref.Ramp, ref.Shade)
	if !ok {
		return "", false
	}
	if ref.Palette == art.SharedPaletteID() {
		return name, true
	}
	scope := art.SharedPaletteName
	docs.View(project.IndexID(), func(document client.Document) {
		if index, isIndex := document.Model.(*project.Index); isIndex {
			scope = index.Name(ref.Palette, scope)
		}
	})

	return scope + " / " + name, true
}

// gesture names the current stroke or fill, so it undoes as one step.
func (s *State) gesture() string {
	return s.window.String() + "-" + strconv.Itoa(s.stroke.count)
}

// command sends an edit, reporting a refusal in the status bar; done runs
// once it is applied, and failed once it is refused.
func (s *State) command(
	ctx *frame.Context, document ids.ULID, edit, gesture string, args any, done func(), failed func(),
) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: document, Edit: edit, Gesture: gesture, Args: data}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			ctx.Frame.Report("")(result, err)
			if failed != nil {
				failed()
			}

			return
		}
		if done != nil {
			done()
		}
	})
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	shared, _ := palette(ctx.Docs, art.SharedPaletteID())
	s.syncFields(shared)
	s.layout(ctx.Docs, shared)
	if s.guard.Active() {
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discard)

		return
	}
	if s.showsMenu.IsOpen() {
		s.updateShows(input, ctx)

		return
	}
	s.updateFields(input, ctx)
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	s.updateToolbar(input, ctx)
	if shared != nil {
		s.updatePanel(input, ctx, shared)
	}
	s.updateCanvas(input, ctx)
	s.guardShade()
}

// keys handles the window's own keys while no text field has focus: a
// tool's letter, and zooming.
func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	for _, info := range tools() {
		if input.Consume(info.key, ui.Plain()) {
			s.setTool(info.tool)
		}
	}
	switch {
	case input.Consume(ui.KeyEqual, ui.Cmd()):
		s.zoomBy(1)
	case input.Consume(ui.KeyMinus, ui.Cmd()):
		s.zoomBy(-1)
	case input.Consume(ui.Key0, ui.Cmd()):
		s.fit(ctx.Docs)
	}
}

// updateToolbar handles the tool buttons, the zoom buttons and the grid.
func (s *State) updateToolbar(input *ui.Input, ctx *frame.Context) {
	for index, button := range s.tools {
		if button.Update(input) {
			s.setTool(tools()[index].tool)
		}
	}
	switch {
	case s.zoomOut.Update(input):
		s.zoomBy(-1)
	case s.zoomIn.Update(input):
		s.zoomBy(1)
	case s.zoomFit.Update(input):
		s.fit(ctx.Docs)
	}
	s.grid.Update(input)
}

func (s *State) setTool(tool Tool) {
	s.tool = tool
	s.stroke.active = false
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	shared, _ := palette(docs, art.SharedPaletteID())
	s.layout(docs, shared)
	screen.Fill(ui.ColorBackground())
	s.drawCanvas(screen, docs)
	s.drawToolbar(screen)
	s.drawReadout(screen, docs)
	s.drawPanel(screen, shared)
	s.showsMenu.Draw(screen, s.theme)
	s.guard.Draw(screen, s.theme)
}

// layout places everything for the window's size: the toolbar, the canvas
// and the palette panel.
func (s *State) layout(docs frame.Docs, shared *art.Palette) {
	s.layoutToolbar()
	s.layoutPanel(docs, shared)
}
