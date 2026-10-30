// Package zonepalettes is the list of palettes a zone uses, as the project
// browser's inspector and the Map's both show it (increment 12): each palette
// A to Z with a button that takes it off, and + Palette, which lists another.
// The Pixel editor offers the palettes of the zones a sprite is placed in
// first; it is a convenience, not a limit.
package zonepalettes

import (
	"encoding/json"
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Geometry, in screen pixels.
const (
	chipPad   = 8
	removeBox = 18
	addWidth  = 104
)

// Script names: "ui:<AddID>" is + Palette, and "ui:<RemovePrefix><name>"
// takes a palette off.
const (
	AddID        = "add-zone-palette"
	RemovePrefix = "remove-zone-palette:"
)

// chip is one palette the zone lists: its name and the button that takes it
// off.
type chip struct {
	palette ids.ULID
	name    string
	rect    image.Rectangle
	remove  *ui.Button
}

// List is a zone's palettes.
type List struct {
	theme   *ui.Theme
	zone    ids.ULID
	chips   []chip
	add     *ui.Button
	menu    ui.Menu
	heading int
	window  image.Point
}

// New returns an empty list.
func New(theme *ui.Theme) *List {
	list := &List{
		theme: theme, zone: ids.ULID{}, chips: nil, add: ui.NewButton(AddID, "+ Palette"),
		menu: ui.Menu{Items: nil, Width: 0}, heading: 0, window: image.Pt(0, 0),
	}
	list.add.Caret = true

	return list
}

// Listed returns the palettes a zone lists, A to Z by name.
func Listed(docs frame.Docs, zoneID ids.ULID) []project.Entry {
	var listed []ids.ULID
	docs.View(zoneID, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			listed = zoneDoc.Zone.Palettes()
		}
	})
	entries := make([]project.Entry, 0, len(listed))
	for _, entry := range palettes(docs) {
		if slices.Contains(listed, entry.ID) {
			entries = append(entries, entry)
		}
	}

	return entries
}

// palettes returns the project's palettes, A to Z by name.
func palettes(docs frame.Docs) []project.Entry {
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(art.PaletteFormatName)
		}
	})
	slices.SortStableFunc(entries, func(left, right project.Entry) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})

	return entries
}

// Layout places the list for a zone, from a top, in a width, and returns
// its foot. A zero zone hides it.
func (l *List) Layout(docs frame.Docs, zoneID ids.ULID, left, top, width int, window image.Point) int {
	l.zone, l.window, l.chips = zoneID, window, nil
	l.add.Rect = ui.Zero()
	if zoneID.IsZero() {
		return top
	}
	l.heading = top
	top += ui.LineHeight
	across := left
	for _, entry := range Listed(docs, zoneID) {
		chipWidth := l.theme.TextWidth(entry.Name) + ui.Half*chipPad + removeBox
		if across > left && across+chipWidth > left+width {
			across, top = left, top+ui.ButtonH+ui.Gap
		}
		rect := image.Rect(across, top, across+chipWidth, top+ui.ButtonH)
		remove := ui.NewButton(RemovePrefix+entry.Name, "×")
		remove.Rect = image.Rect(rect.Max.X-removeBox-ui.Gap, rect.Min.Y+ui.Gap, rect.Max.X-ui.Gap, rect.Max.Y-ui.Gap)
		l.chips = append(l.chips, chip{palette: entry.ID, name: entry.Name, rect: rect, remove: remove})
		across = rect.Max.X + ui.Gap
	}
	if across > left && across+addWidth > left+width {
		across, top = left, top+ui.ButtonH+ui.Gap
	}
	l.add.Rect = image.Rect(across, top, across+addWidth, top+ui.ButtonH)

	return top + ui.ButtonH
}

// Active reports whether the list's menu is open; while it is, it takes all
// input.
func (l *List) Active() bool { return l.menu.IsOpen() }

// Update handles the list: a chip's × takes its palette off, and + Palette
// offers the palettes the zone does not list, Default aside, which is always
// offered after them. It reports whether it handled the input.
func (l *List) Update(input *ui.Input, ctx *frame.Context) bool {
	if l.menu.IsOpen() {
		if chosen := l.menu.Update(input); chosen != "" {
			if palette, err := ids.Parse(chosen); err == nil {
				l.insert(ctx, palette)
			}
		}

		return true
	}
	for _, each := range l.chips {
		if each.remove.Update(input) {
			args := zoneplugin.PaletteArgs{Palette: each.palette, Index: 0}
			zoneName := l.zone.String()
			ctx.Docs.View(project.IndexID(), func(document client.Document) {
				if index, ok := document.Model.(*project.Index); ok {
					zoneName = index.Name(l.zone, zoneName)
				}
			})
			command(ctx, l.zone, zoneplugin.RemovePaletteEdit, args, zoneName+" no longer lists "+each.name)

			return true
		}
	}
	if l.add.Update(input) {
		l.open(ctx.Docs)

		return true
	}

	return false
}

// open offers the palettes the zone does not list yet.
func (l *List) open(docs frame.Docs) {
	listed := Listed(docs, l.zone)
	var items []ui.MenuItem
	for _, entry := range palettes(docs) {
		if entry.ID == art.DefaultPaletteID() || slices.ContainsFunc(listed, func(other project.Entry) bool {
			return other.ID == entry.ID
		}) {
			continue
		}
		items = append(items, ui.MenuItem{
			ID: entry.ID.String(), Label: entry.Name, Icon: ui.IconPalette, Disabled: false, Danger: false, Rule: false,
		})
	}
	if len(items) == 0 {
		items = append(items, ui.MenuItem{
			ID: "", Label: "Every palette is listed", Icon: ui.IconNone, Disabled: true, Danger: false, Rule: false,
		})
	}
	window := image.Rectangle{Min: image.Pt(0, 0), Max: l.window}
	l.menu.Items = items
	l.menu.Width = min(ui.MenuWidth(items, l.theme), window.Dx()-ui.Half*ui.Inset)
	l.menu.Open(l.add.Rect, window)
}

// insert lists a palette for the zone, at the end.
func (l *List) insert(ctx *frame.Context, palette ids.ULID) {
	count := len(l.chips)
	args := zoneplugin.PaletteArgs{Palette: palette, Index: count}
	command(ctx, l.zone, zoneplugin.InsertPaletteEdit, args, "")
}

// command sends a zone edit, reporting a refusal in the status bar.
func command(ctx *frame.Context, zoneID ids.ULID, edit string, args zoneplugin.PaletteArgs, done string) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	ctx.Requests.Command(protocol.Command{Document: zoneID, Edit: edit, Gesture: "", Args: data},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)

				return
			}
			if done != "" {
				ctx.Frame.Status(done)
			}
		})
}

// Locate finds + Palette, a chip's × or an item of the open menu.
func (l *List) Locate(name string) (image.Point, bool) {
	kind, value, _ := strings.Cut(name, ":")
	if kind == "menu" {
		return l.menu.Locate(value)
	}
	if kind != "ui" {
		return image.Pt(0, 0), false
	}
	if value == AddID && !l.add.Rect.Empty() {
		return ui.Center(l.add.Rect), true
	}
	for _, each := range l.chips {
		if each.remove.ID == value {
			return ui.Center(each.remove.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// Draw draws the list's heading, its chips and + Palette.
func (l *List) Draw(screen *ebiten.Image) {
	if l.zone.IsZero() {
		return
	}
	left := l.add.Rect.Min.X
	if len(l.chips) > 0 {
		left = l.chips[0].rect.Min.X
	}
	l.theme.Heading(screen, "PALETTES · OFFERED FIRST", left, l.heading)
	for _, each := range l.chips {
		ui.StrokeRound(screen, each.rect, ui.Radius, ui.Hairline, ui.ColorRule())
		l.theme.DrawText(screen, each.name, each.rect.Min.X+chipPad, each.rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1,
			ui.ColorInk())
		each.remove.Draw(screen, l.theme)
	}
	l.add.Draw(screen, l.theme)
}

// DrawMenu draws the open menu, over everything else.
func (l *List) DrawMenu(screen *ebiten.Image) { l.menu.Draw(screen, l.theme) }
