// Package paletteview is the Palette Editor (increment 12): one palette's
// ramps and colours, where all palette editing happens. Ramps are added,
// renamed, reordered and removed; colours are added, moved within and
// between ramps, recoloured, given an opacity and labelled; a gradient fills
// a ramp; colours are copied between palettes; and the palette is renamed
// and deleted. Default, the engine's own palette, shows read only: its
// colours can be chosen and copied, and nothing else.
//
// Typed fields wait for the save bar, as every inspector's do; adding,
// moving, removing, the gradient and copying apply at once and undo.
package paletteview

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

// ViewName is the Palette Editor's view name; its target is a palette.
const ViewName = "palette"

// ItemDelete is the item that opens Delete palette… when the window is
// opened at it.
const ItemDelete = "delete"

// UndoGroup is the undo list the Palette Editors of one palette share.
func UndoGroup(palette ids.ULID) string { return "palette:" + palette.String() }

// shortID is how much of the window's ID its title shows.
const shortID = 6

// The typed fields hold at most this many bytes: a label's fifteen
// characters can take four bytes each.
const (
	nameBytes    = 200
	labelBytes   = 4 * art.MaxLabelLength
	colorBytes   = 16
	opacityBytes = 8
	stepsBytes   = 4
)

// choice is a colour of the palette, by its ramp and shade, or a ramp alone
// when the shade is zero.
type choice struct {
	ramp  ids.ULID
	shade ids.ULID
}

func none() choice { return choice{ramp: ids.ULID{}, shade: ids.ULID{}} }

// State is one Palette Editor window.
type State struct {
	window ids.ULID
	target ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	// chosen is the colour the inspector shows; picked are the colours
	// chosen together, for copying, in the order they were chosen.
	chosen choice
	picked []ids.ULID
	rows   rowsLayout
	scroll int
	// reveal scrolls the chosen colour into view at the next layout.
	reveal bool
	drag   drag
	cursor image.Point
	// rampTop is where the inspector's ramp heading is.
	rampTop int
	// named is what the typed fields hold.
	named    choice
	name     *ui.TextField
	label    *ui.TextField
	hex      *ui.TextField
	opacity  *ui.TextField
	rampName *ui.TextField
	// picker chooses the colour by eye, into the hex and opacity fields.
	picker   *ui.ColorPicker
	focus    ui.Focus
	save     *ui.SaveBar
	guard    frame.Guard
	bar      toolbar
	inspect  inspectorButtons
	menu     ui.Menu
	menuFor  string
	gradient *gradientPanel
	copying  *copyPanel
	deleting *deletePanel
	// uses are the documents that refer to an item of the palette, asked
	// once per item until something changes.
	uses  map[string][]protocol.Use
	asked map[string]bool
	steps int
}

// toolbar is the row of buttons above the ramps.
type toolbar struct {
	addRamp, gradient, copyFrom, remove, copyTo, newPalette *ui.Button
}

// inspectorButtons are the inspector's buttons.
type inspectorButtons struct {
	addColor, removeColor, gradientInto, removeRamp *ui.Button
}

// New returns the Palette Editor for a palette.
func New(window, palette ids.ULID, theme *ui.Theme) *State {
	field := func(id string, maxBytes int) *ui.TextField {
		return ui.NewTextField(id, textedit.NewField("", false, maxBytes), false, theme)
	}
	view := &State{
		window: window, target: palette, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		chosen: none(), picked: nil, rows: newRowsLayout(), scroll: 0, reveal: false, drag: noDrag(),
		cursor: image.Pt(0, 0), rampTop: 0, named: none(), name: field("palette-name", nameBytes),
		label: field("label", labelBytes),
		hex:   field("colour", colorBytes), opacity: field("opacity", opacityBytes),
		rampName: field("ramp-name", art.MaxNameLength), picker: ui.NewColorPicker("picker"), focus: ui.Focus{},
		save: ui.NewSaveBar(), guard: frame.Guard{},
		bar: toolbar{
			addRamp: ui.NewButton("add-ramp", "+ Ramp"), gradient: ui.NewButton("gradient", "Gradient…"),
			copyFrom: ui.NewButton("copy-from", "Copy colours from…"), remove: ui.NewButton("delete", "Delete palette…"),
			copyTo:     ui.NewButton("copy-to", "Copy to palette"),
			newPalette: ui.NewButton("new-palette", "New palette from these"),
		},
		inspect: inspectorButtons{
			addColor: ui.NewButton("add-colour", "+ Colour after"), removeColor: ui.NewButton("remove-colour", "Remove colour"),
			gradientInto: ui.NewButton("gradient-into", "Gradient…"),
			removeRamp:   ui.NewButton("remove-ramp", "Remove ramp"),
		},
		menu: ui.Menu{Items: nil, Width: 0}, menuFor: "", gradient: newGradientPanel(theme), copying: newCopyPanel(theme),
		deleting: nil,
		uses:     map[string][]protocol.Use{}, asked: map[string]bool{}, steps: 0,
	}
	view.bar.remove.Danger = true
	view.inspect.removeRamp.Danger = true
	view.inspect.removeColor.Danger = true
	view.bar.copyTo.Caret = true
	view.focus.Register(view.name, view.label, view.hex, view.opacity, view.rampName)
	view.focus.Register(view.gradient.fields()...)
	for _, field := range append(view.gradient.fields(), view.copying.name) {
		field.Unmarked = true
	}
	view.focus.Register(view.copying.name)

	return view
}

// ReadOnly reports whether the window shows Default, which nothing changes.
func (s *State) ReadOnly() bool { return s.target == art.DefaultPaletteID() }

// Title is the window title: the palette's name.
func (s *State) Title(docs frame.Docs) string {
	return "Palette Editor: " + s.paletteName(docs) + "  (window " + s.window.Short(shortID) + ")"
}

// paletteName is the palette's name, from the project index.
func (s *State) paletteName(docs frame.Docs) string {
	return documentName(docs, s.target, art.DefaultPaletteName)
}

// documentName is a document's name, from the project index.
func documentName(docs frame.Docs, id ids.ULID, fallback string) string {
	name := fallback
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(id, fallback)
		}
	})

	return name
}

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Start subscribes to what the window shows. The window has already
// subscribed to the palette; this adds the project index, for names and the
// other palettes, and Default, which copies and removals look colours up in.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
	s.subscribe(art.DefaultPaletteID(), ctx)
	s.subscribe(s.target, ctx)
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

// Changed forgets where things are used once anything changes, so it is
// asked again, and chooses the colour the window was asked to show.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.Event != nil || update.Snapshot != nil {
		s.uses, s.asked = map[string][]protocol.Use{}, map[string]bool{}
	}
	switch {
	case update.Attention && update.Item == ItemDelete:
		s.openDeleting(ctx)
	case update.Attention && update.Item != "":
		s.Show(ctx.Docs, update.Item)
	}
}

// Show chooses a colour by its shade's ID, as Edit palette… asks, and
// scrolls it into view.
func (s *State) Show(docs frame.Docs, item string) {
	shadeID, err := ids.Parse(item)
	palette, ok := s.palette(docs)
	if err != nil || !ok {
		return
	}
	if ramp, placed := palette.Place(shadeID); placed {
		s.choose(choice{ramp: ramp, shade: shadeID})
		s.reveal = true
	}
}

// palette returns a copy of the window's palette.
func (s *State) palette(docs frame.Docs) (*art.Palette, bool) {
	return paletteOf(docs, s.target)
}

// paletteOf returns a copy of a palette.
func paletteOf(docs frame.Docs, paletteID ids.ULID) (*art.Palette, bool) {
	var found *art.Palette
	docs.View(paletteID, func(document client.Document) {
		if paletteDoc, ok := document.Model.(*artplugin.PaletteDocument); ok {
			found = paletteDoc.Palette.Clone()
		}
	})

	return found, found != nil
}

// choose makes a colour the chosen one, and the only one picked.
func (s *State) choose(chosen choice) {
	s.chosen = chosen
	s.picked = nil
	if !chosen.shade.IsZero() {
		s.picked = []ids.ULID{chosen.shade}
	}
}

// picks reports whether a shade is among the picked colours.
func (s *State) picks(shade ids.ULID) bool { return slices.Contains(s.picked, shade) }

// gesture names a new step of several commands, so they undo as one.
func (s *State) gesture() string {
	s.steps++

	return s.window.String() + "-" + strconv.Itoa(s.steps)
}

// command sends an edit, reporting a refusal in the status bar; done runs
// once it is applied, and failed once it is refused.
func (s *State) command(
	ctx *frame.Context, document ids.ULID, edit, gesture string, args any, done func(protocol.Result), failed func(),
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
			done(result)
		}
	})
}

// usesOf returns the documents that refer to an item of the palette, asking
// the first time, and whether they are known yet.
func (s *State) usesOf(ctx *frame.Context, item string) ([]protocol.Use, bool) {
	if found, known := s.uses[item]; known {
		return found, true
	}
	if !s.asked[item] {
		s.asked[item] = true
		ctx.Requests.WhereUsed(s.target, item, func(uses []protocol.Use, err error) {
			if err == nil {
				s.uses[item] = uses
			}
		})
	}

	return nil, false
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.cursor = input.Cursor
	s.syncFields(ctx.Docs)
	s.layout(ctx.Docs)
	switch {
	case s.guard.Active():
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discard)
	case s.menu.IsOpen():
		s.updateMenu(input, ctx)
	case s.deleting != nil:
		s.updateDeleting(input, ctx)
	default:
		s.updateFields(input, ctx)
		s.updatePicker(input)
		frame.CommonKeys(input, ctx)
		s.updateButtons(input, ctx)
		s.updatePanels(input, ctx)
		s.updateRows(input, ctx)
		s.guardChoice()
	}
	s.askUses(ctx)
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout(docs)
	screen.Fill(ui.ColorBackground())
	palette, loaded := s.palette(docs)
	if !loaded {
		s.theme.DrawText(screen, "Loading the palette…", ui.Inset, ui.Inset, ui.ColorMuted())

		return
	}
	s.drawToolbar(screen)
	s.drawRows(screen, palette)
	s.drawPanels(screen)
	s.drawInspector(screen, docs, palette)
	s.drawTip(screen, palette)
	s.drawDeleting(screen, docs)
	s.menu.Draw(screen, s.theme)
	s.guard.Draw(screen, s.theme)
}

// layout places everything for the window's size.
func (s *State) layout(docs frame.Docs) {
	palette, ok := s.palette(docs)
	if !ok {
		return
	}
	s.layoutToolbar()
	s.layoutPanels(docs)
	s.layoutRows(palette)
	s.layoutInspector(palette)
	s.layoutDeleting()
}
