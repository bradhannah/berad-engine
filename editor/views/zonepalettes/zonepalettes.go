// Package zonepalettes is what a zone lists, as the project browser's
// inspector and the Map's both show it: the palettes it uses (increment 12)
// and its sounds, the music and ambient cues it plays throughout (increment
// 14). Each listed thing is a chip with a button that takes it off, and
// + Palette or + Sound lists another. The Pixel editor offers the palettes of
// the zones a sprite is placed in first; it is a convenience, not a limit.
package zonepalettes

import (
	"cmp"
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
	chipPad      = 8
	removeBox    = 18
	addWidth     = 104
	sectionSpace = 12
)

// Script names: "ui:<AddID>" is + Palette, and "ui:<RemovePrefix><name>"
// takes a palette off; "ui:<AddSoundID>" is + Sound, and
// "ui:<RemoveSoundPrefix><name>" takes a sound off.
const (
	AddID             = "add-zone-palette"
	RemovePrefix      = "remove-zone-palette:"
	AddSoundID        = "add-zone-sound"
	RemoveSoundPrefix = "remove-zone-sound:"
)

// soundMenuPrefix tells a sound's menu item from a palette's.
const soundMenuPrefix = "sound:"

// item is one thing a zone lists or could list: its key, a palette's ID or a
// cue's, and its name.
type item struct {
	key  string
	name string
}

// chip is one thing the zone lists: its name and the button that takes it
// off.
type chip struct {
	item

	rect   image.Rectangle
	remove *ui.Button
}

// section is one kind of thing a zone lists.
type section struct {
	heading, removePrefix string
	chips                 []chip
	add                   *ui.Button
	top                   int
}

// List is what a zone lists: its palettes, then its sounds.
type List struct {
	theme    *ui.Theme
	zone     ids.ULID
	palettes section
	sounds   section
	menu     ui.Menu
	window   image.Point
	// library is the audio library as last listed, for the cues' names and
	// for + Sound; asked says it has been asked for, once a zone that plays
	// sounds is shown.
	library *protocol.AudioLibrary
	asked   bool
}

// New returns an empty list.
func New(theme *ui.Theme) *List {
	list := &List{
		theme: theme, zone: ids.ULID{},
		palettes: section{
			heading: "PALETTES · OFFERED FIRST", removePrefix: RemovePrefix, chips: nil,
			add: ui.NewButton(AddID, "+ Palette"), top: 0,
		},
		sounds: section{
			heading: "SOUNDS · PLAYED THROUGHOUT", removePrefix: RemoveSoundPrefix, chips: nil,
			add: ui.NewButton(AddSoundID, "+ Sound"), top: 0,
		},
		menu: ui.Menu{Items: nil, Width: 0}, window: image.Pt(0, 0), library: nil, asked: false,
	}
	list.palettes.add.Caret, list.sounds.add.Caret = true, true

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

// listedSounds returns the cues a zone plays throughout, in the order they
// were added.
func (l *List) listedSounds(docs frame.Docs) []item {
	var cues []string
	docs.View(l.zone, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			cues = zoneDoc.Zone.Sounds()
		}
	})
	items := make([]item, 0, len(cues))
	for _, cue := range cues {
		items = append(items, item{key: cue, name: l.cueName(cue)})
	}

	return items
}

// cueName names a cue by its name in the library, or its ID.
func (l *List) cueName(cueID string) string {
	if l.library != nil {
		for _, cue := range l.library.Cues {
			if cue.ID == cueID && cue.Name != "" {
				return cue.Name
			}
		}
	}

	return cueID
}

// Layout places the list for a zone, from a top, in a width, and returns
// its foot. A zero zone hides it.
func (l *List) Layout(docs frame.Docs, zoneID ids.ULID, left, top, width int, window image.Point) int {
	l.zone, l.window = zoneID, window
	l.palettes.chips, l.sounds.chips = nil, nil
	l.palettes.add.Rect, l.sounds.add.Rect = ui.Zero(), ui.Zero()
	if zoneID.IsZero() {
		return top
	}
	listed := Listed(docs, zoneID)
	paletteItems := make([]item, 0, len(listed))
	for _, entry := range listed {
		paletteItems = append(paletteItems, item{key: entry.ID.String(), name: entry.Name})
	}
	top = l.palettes.layout(l.theme, paletteItems, left, top, width) + sectionSpace

	return l.sounds.layout(l.theme, l.listedSounds(docs), left, top, width)
}

// layout places a section's heading, chips and add button from a top, and
// returns its foot.
func (s *section) layout(theme *ui.Theme, items []item, left, top, width int) int {
	s.top = top
	top += ui.LineHeight
	across := left
	for _, each := range items {
		chipWidth := theme.TextWidth(each.name) + ui.Half*chipPad + removeBox
		if across > left && across+chipWidth > left+width {
			across, top = left, top+ui.ButtonH+ui.Gap
		}
		rect := image.Rect(across, top, across+chipWidth, top+ui.ButtonH)
		remove := ui.NewButton(s.removePrefix+each.name, "×")
		remove.Rect = image.Rect(rect.Max.X-removeBox-ui.Gap, rect.Min.Y+ui.Gap, rect.Max.X-ui.Gap, rect.Max.Y-ui.Gap)
		s.chips = append(s.chips, chip{item: each, rect: rect, remove: remove})
		across = rect.Max.X + ui.Gap
	}
	if across > left && across+addWidth > left+width {
		across, top = left, top+ui.ButtonH+ui.Gap
	}
	s.add.Rect = image.Rect(across, top, across+addWidth, top+ui.ButtonH)

	return top + ui.ButtonH
}

// Active reports whether the list's menu is open; while it is, it takes all
// input.
func (l *List) Active() bool { return l.menu.IsOpen() }

// Update handles the list: a chip's × takes it off, + Palette offers the
// palettes the zone does not list, Default aside, and + Sound the music and
// ambient cues it does not play. It reports whether it handled the input.
func (l *List) Update(input *ui.Input, ctx *frame.Context) bool {
	if !l.asked && len(l.sounds.chips) > 0 {
		l.askAudio(ctx, false)
	}
	if l.menu.IsOpen() {
		if chosen := l.menu.Update(input); chosen != "" {
			l.insert(ctx, chosen)
		}

		return true
	}
	if l.removeClicked(input, ctx) {
		return true
	}
	switch {
	case l.palettes.add.Update(input):
		l.openPalettes(ctx.Docs)
	case l.sounds.add.Update(input):
		l.askAudio(ctx, true)
	default:
		return false
	}

	return true
}

// askAudio lists the audio library, and opens + Sound's menu once it is
// listed when open is set.
func (l *List) askAudio(ctx *frame.Context, open bool) {
	l.asked = true
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		if err != nil {
			ctx.Frame.Status("The audio library could not be listed: " + err.Error())

			return
		}
		l.library = &library
		if open {
			l.openSounds(ctx.Docs)
		}
	})
}

// removeClicked takes a palette or a sound off when its × is clicked.
func (l *List) removeClicked(input *ui.Input, ctx *frame.Context) bool {
	for _, each := range l.palettes.chips {
		if each.remove.Update(input) {
			if palette, err := ids.Parse(each.key); err == nil {
				args := zoneplugin.PaletteArgs{Palette: palette, Index: 0}
				command(ctx, l.zone, zoneplugin.RemovePaletteEdit, args, l.zoneName(ctx.Docs)+" no longer lists "+each.name)
			}

			return true
		}
	}
	for _, each := range l.sounds.chips {
		if each.remove.Update(input) {
			args := zoneplugin.SoundArgs{Cue: each.key, Index: 0}
			command(ctx, l.zone, zoneplugin.RemoveSoundEdit, args, l.zoneName(ctx.Docs)+" no longer plays "+each.name)

			return true
		}
	}

	return false
}

// zoneName names the zone from the project index.
func (l *List) zoneName(docs frame.Docs) string {
	name := l.zone.String()
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(l.zone, name)
		}
	})

	return name
}

// openPalettes offers the palettes the zone does not list yet.
func (l *List) openPalettes(docs frame.Docs) {
	listed := Listed(docs, l.zone)
	var items []ui.MenuItem
	for _, entry := range palettes(docs) {
		if entry.ID == art.DefaultPaletteID() || slices.ContainsFunc(listed, func(other project.Entry) bool {
			return other.ID == entry.ID
		}) {
			continue
		}
		items = append(items, menuItem(entry.ID.String(), entry.Name, ui.IconPalette, false))
	}
	if len(items) == 0 {
		items = append(items, menuItem("", "Every palette is listed", ui.IconNone, true))
	}
	l.openMenu(items, l.palettes.add.Rect)
}

// openSounds offers the music and ambient cues the zone does not play yet,
// by name.
func (l *List) openSounds(docs frame.Docs) {
	listed := l.listedSounds(docs)
	var cues []protocol.AudioCue
	if l.library != nil {
		for _, cue := range l.library.Cues {
			throughout := cue.Category == protocol.CategoryMusic || cue.Category == protocol.CategoryAmbient
			if throughout && !slices.ContainsFunc(listed, func(other item) bool { return other.key == cue.ID }) {
				cues = append(cues, cue)
			}
		}
	}
	slices.SortFunc(cues, func(a, b protocol.AudioCue) int { return cmp.Compare(a.Name, b.Name) })
	items := make([]ui.MenuItem, 0, len(cues))
	for _, cue := range cues {
		items = append(items, menuItem(soundMenuPrefix+cue.ID, cue.Name, ui.IconSound, false))
	}
	if len(items) == 0 {
		items = append(items, menuItem("", "No music or ambient cues to add", ui.IconNone, true))
	}
	l.openMenu(items, l.sounds.add.Rect)
}

func menuItem(id, label string, icon ui.Icon, disabled bool) ui.MenuItem {
	return ui.MenuItem{ID: id, Label: label, Icon: icon, Disabled: disabled, Danger: false, Rule: false}
}

// openMenu opens the list's menu under a button.
func (l *List) openMenu(items []ui.MenuItem, under image.Rectangle) {
	window := frame.AboveStatus(l.window)
	l.menu.Items = items
	l.menu.Width = min(ui.MenuWidth(items, l.theme), window.Dx()-ui.Half*ui.Inset)
	l.menu.Open(under, window)
}

// insert lists a palette or a sound for the zone, at the end.
func (l *List) insert(ctx *frame.Context, chosen string) {
	if cue, isSound := strings.CutPrefix(chosen, soundMenuPrefix); isSound {
		args := zoneplugin.SoundArgs{Cue: cue, Index: len(l.sounds.chips)}
		command(ctx, l.zone, zoneplugin.InsertSoundEdit, args, l.zoneName(ctx.Docs)+" plays "+l.cueName(cue)+" throughout")

		return
	}
	if palette, err := ids.Parse(chosen); err == nil {
		args := zoneplugin.PaletteArgs{Palette: palette, Index: len(l.palettes.chips)}
		command(ctx, l.zone, zoneplugin.InsertPaletteEdit, args, "")
	}
}

// command sends a zone edit, reporting a refusal in the status bar.
func command(ctx *frame.Context, zoneID ids.ULID, edit string, args any, done string) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	ctx.Requests.Command(protocol.Command{Document: zoneID, Edit: edit, Gesture: "", Args: data, Says: ""},
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

// Locate finds + Palette, + Sound, a chip's × or an item of the open menu.
func (l *List) Locate(name string) (image.Point, bool) {
	kind, value, _ := strings.Cut(name, ":")
	if kind == "menu" {
		return l.menu.Locate(value)
	}
	if kind != "ui" {
		return image.Pt(0, 0), false
	}
	for _, each := range []*section{&l.palettes, &l.sounds} {
		if value == each.add.ID && !each.add.Rect.Empty() {
			return ui.Center(each.add.Rect), true
		}
		for _, listed := range each.chips {
			if listed.remove.ID == value {
				return ui.Center(listed.remove.Rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// Draw draws each section's heading, its chips and its add button.
func (l *List) Draw(screen *ebiten.Image) {
	if l.zone.IsZero() {
		return
	}
	l.palettes.draw(screen, l.theme)
	l.sounds.draw(screen, l.theme)
}

func (s *section) draw(screen *ebiten.Image, theme *ui.Theme) {
	left := s.add.Rect.Min.X
	if len(s.chips) > 0 {
		left = s.chips[0].rect.Min.X
	}
	theme.Heading(screen, s.heading, left, s.top)
	for _, each := range s.chips {
		ui.StrokeRound(screen, each.rect, ui.Radius, ui.Hairline, ui.ColorRule())
		theme.DrawText(screen, each.name, each.rect.Min.X+chipPad, each.rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1,
			ui.ColorInk())
		each.remove.Draw(screen, theme)
	}
	s.add.Draw(screen, theme)
}

// DrawMenu draws the open menu, over everything else.
func (l *List) DrawMenu(screen *ebiten.Image) { l.menu.Draw(screen, l.theme) }
