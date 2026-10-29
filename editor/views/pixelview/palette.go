package pixelview

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Palette panel geometry, in screen pixels.
const (
	swatchSize   = 18
	swatchStep   = 21
	rampColumn   = 76
	foldColumn   = 14
	chosenWidth  = 2
	scrollBar    = 4
	chosenSwatch = 36
	editWidth    = 112
	tipPad       = 6
	tipOffset    = 14
	// chosenLines is how many lines of text the chosen colour shows beside
	// its swatch: its name, its label and its value.
	chosenLines = 3
)

// The panel picks colours only (increment 12): every palette's colours are
// edited in the Palette Editor, and the panel offers palettes in the agreed
// order. First come the palettes the zones the sprite is placed in list,
// A to Z; then Default; then every other palette, A to Z, folded until
// opened.

// swatch is one colour the panel offers, clear or a shade, where it is
// drawn, and what scripts call it: "clear" or its full name, such as
// "Default / grey 3".
type swatch struct {
	rect  image.Rectangle
	color art.ColorRef
	name  string
}

// rampLabel is a ramp's name in the panel, which chooses its first shade.
type rampLabel struct {
	rect    image.Rectangle
	palette ids.ULID
	ramp    art.Ramp
}

// paletteHeader is a palette's name in the panel, which folds it.
type paletteHeader struct {
	rect    image.Rectangle
	palette ids.ULID
	name    string
	note    string
	folded  bool
}

// panelLayout is where the palette panel's parts are this frame: the
// swatches, ramp names and palette headings, the area they scroll in and how
// tall they are, and the tops of the chosen colour and Shows.
type panelLayout struct {
	swatches  []swatch
	ramps     []rampLabel
	headers   []paletteHeader
	area      image.Rectangle
	content   int
	chosenTop int
	showsTop  int
}

func newPanelLayout() panelLayout {
	return panelLayout{
		swatches: nil, ramps: nil, headers: nil, area: ui.Zero(), content: 0, chosenTop: 0, showsTop: 0,
	}
}

// offer is one palette the panel offers, with its name, its colours, and
// whether a zone the sprite is placed in lists it.
type offer struct {
	id      ids.ULID
	name    string
	palette *art.Palette
	first   bool
}

// offers returns the palettes the panel shows, in the agreed order.
func (s *State) offers(docs frame.Docs) []offer {
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(art.PaletteFormatName)
		}
	})
	if !slices.ContainsFunc(entries, func(entry project.Entry) bool { return entry.ID == art.DefaultPaletteID() }) {
		entries = append(entries, project.Entry{
			ID: art.DefaultPaletteID(), Name: art.DefaultPaletteName, Format: art.PaletteFormatName, Folder: "", Tags: nil,
		})
	}
	offers := make([]offer, 0, len(entries))
	for _, entry := range entries {
		found, ok := palette(docs, entry.ID)
		if !ok {
			continue
		}
		offers = append(offers, offer{
			id: entry.ID, name: entry.Name, palette: found,
			first: entry.ID != art.DefaultPaletteID() && slices.Contains(s.first, entry.ID),
		})
	}
	rank := func(each offer) int {
		switch {
		case each.first:
			return 0
		case each.id == art.DefaultPaletteID():
			return 1
		}

		return ui.Half
	}
	slices.SortStableFunc(offers, func(left, right offer) int {
		return cmp.Or(rank(left)-rank(right), strings.Compare(strings.ToLower(left.name), strings.ToLower(right.name)))
	})

	return offers
}

// folded reports whether a palette is folded: as its author left it, or
// else folded unless it is Default or a zone of the sprite lists it.
func (s *State) folded(each offer) bool {
	if folded, chosen := s.folds[each.id]; chosen {
		return folded
	}

	return !each.first && each.id != art.DefaultPaletteID()
}

// layoutPanel places the palette panel: the palettes' colours from the top,
// scrolling when they do not fit; at the foot, Shows; and above it the chosen
// colour and Edit palette….
func (s *State) layoutPanel(docs frame.Docs) {
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	s.panel = newPanelLayout()
	s.layoutShows(docs, left, width, inspector.Max.Y-ui.Inset)
	editTop := s.panel.showsTop - ui.Inset - ui.ButtonH
	s.edit.Rect = image.Rect(left, editTop, left+editWidth, editTop+ui.ButtonH)
	s.edit.Disabled = s.color.IsClear() || s.color.IsLoose()
	s.panel.chosenTop = editTop - ui.Gap - max(chosenSwatch, chosenLines*ui.LineHeight) - ui.LineHeight
	area := image.Rect(left-ui.Half*chosenWidth, ui.Inset+ui.LineHeight, left+width, s.panel.chosenTop-ui.Inset)
	s.layoutSwatches(s.offers(docs), left, width, area)
}

// layoutSwatches places the clear colour and each palette's heading, and the
// ramps of each palette not folded, a ramp's shades wrapping onto further
// rows, scrolled by the panel's scroll, which it keeps within what there is to
// show.
func (s *State) layoutSwatches(offers []offer, left, width int, area image.Rectangle) {
	perRow := max(1, (width-rampColumn)/swatchStep)
	s.panel.area = area
	s.panel.content = swatchStep * (1 + len(offers))
	for _, each := range offers {
		if !s.folded(each) {
			for _, ramp := range each.palette.Ramps() {
				s.panel.content += max(1, (len(ramp.Shades)+perRow-1)/perRow) * swatchStep
			}
		}
	}
	s.scrollPanel(0)
	top := area.Min.Y - s.scroll
	s.panel.swatches = append(s.panel.swatches, swatch{
		rect: image.Rect(left, top, left+swatchSize, top+swatchSize), color: clearColor(), name: "clear",
	})
	top += swatchStep
	for _, each := range offers {
		header := paletteHeader{
			rect: image.Rect(left, top, left+width, top+swatchSize), palette: each.id, name: each.name, note: "",
			folded: s.folded(each),
		}
		if each.first {
			header.note = "listed by its zone"
		}
		s.panel.headers = append(s.panel.headers, header)
		top += swatchStep
		if header.folded {
			continue
		}
		for _, ramp := range each.palette.Ramps() {
			s.panel.ramps = append(s.panel.ramps, rampLabel{
				rect: image.Rect(left, top, left+rampColumn-ui.Gap, top+swatchSize), palette: each.id, ramp: ramp,
			})
			for index, shade := range ramp.Shades {
				if index > 0 && index%perRow == 0 {
					top += swatchStep
				}
				x := left + rampColumn + index%perRow*swatchStep
				s.panel.swatches = append(s.panel.swatches, swatch{
					rect:  image.Rect(x, top, x+swatchSize, top+swatchSize),
					color: art.Named(each.id, ramp.ID, shade.ID), name: each.name + " / " + ramp.Name + " " + strconv.Itoa(index+1),
				})
			}
			top += swatchStep
		}
	}
}

// scrollPanel scrolls the swatches by a distance, kept between the top and
// the last row.
func (s *State) scrollPanel(distance int) {
	s.scroll = min(max(s.scroll+distance, 0), max(s.panel.content-s.panel.area.Dy(), 0))
}

// reveal scrolls the swatches so a part of them is in view, and returns
// where it is then.
func (s *State) reveal(rect image.Rectangle) image.Rectangle {
	area, before := s.panel.area, s.scroll
	switch {
	case rect.Min.Y < area.Min.Y:
		s.scrollPanel(rect.Min.Y - area.Min.Y)
	case rect.Max.Y > area.Max.Y:
		s.scrollPanel(rect.Max.Y - area.Max.Y)
	}

	return rect.Sub(image.Pt(0, s.scroll-before))
}

// updatePanel handles Edit palette…, Shows, the wheel over the swatches,
// and a click on a swatch, a ramp's name or a palette's heading.
func (s *State) updatePanel(input *ui.Input, ctx *frame.Context) {
	switch {
	case s.edit.Update(input):
		s.editPalette(ctx)
	case s.shows.Update(input):
		s.openShows(ctx.Docs)
	case input.Cursor.In(s.panel.area) && input.WheelY != 0:
		s.scrollPanel(-input.WheelPan().Y)
		input.WheelX, input.WheelY = 0, 0
	case input.Pressed && input.Cursor.In(s.panel.area):
		s.choosePanel(input)
	}
}

// choosePanel chooses the colour of a clicked swatch, or a clicked ramp's
// first shade, or folds or opens a clicked palette.
func (s *State) choosePanel(input *ui.Input) {
	for _, offered := range s.panel.swatches {
		if input.Cursor.In(offered.rect) {
			s.color = offered.color
			input.ClearPointer()

			return
		}
	}
	for _, label := range s.panel.ramps {
		if input.Cursor.In(label.rect) && len(label.ramp.Shades) > 0 {
			s.color = art.Named(label.palette, label.ramp.ID, label.ramp.Shades[0].ID)
			input.ClearPointer()

			return
		}
	}
	for _, header := range s.panel.headers {
		if input.Cursor.In(header.rect) {
			s.folds[header.palette] = !header.folded
			input.ClearPointer()

			return
		}
	}
}

// editPalette opens the chosen colour's palette in the Palette Editor, with
// the colour chosen there.
func (s *State) editPalette(ctx *frame.Context) {
	if s.color.IsClear() || s.color.IsLoose() {
		return
	}
	ctx.Requests.OpenItem(PaletteEditorView, s.color.Palette, s.color.Shade.String())
}

// chosenShade returns the chosen colour's shade, found through a shadow
// record if it has moved.
func (s *State) chosenShade(docs frame.Docs) (art.Shade, bool) {
	var none art.Shade
	if s.color.IsClear() || s.color.IsLoose() {
		return none, false
	}
	found, ok := palette(docs, s.color.Palette)
	if !ok {
		return none, false
	}
	shade, _, resolved := found.Resolve(s.color.Ramp, s.color.Shade)

	return shade, resolved
}

// drawPanel draws the palette panel.
func (s *State) drawPanel(screen *ebiten.Image, docs frame.Docs) {
	frame.DrawInspector(screen)
	inspector := frame.Inspector(s.size)
	left := inspector.Min.X + ui.Inset
	s.theme.Heading(screen, "PALETTE", left, ui.Inset)
	s.drawSwatches(screen, docs)
	s.drawChosen(screen, docs, left, inspector.Dx()-ui.Half*ui.Inset)
	s.edit.Draw(screen, s.theme)
	s.drawShows(screen, left)
}

// drawChosen draws the chosen colour: its swatch, its name, its label and its
// value, which only the Palette Editor changes.
func (s *State) drawChosen(screen *ebiten.Image, docs frame.Docs, left, width int) {
	top := s.panel.chosenTop
	s.theme.Heading(screen, "CHOSEN", left, top)
	top += ui.LineHeight
	box := image.Rect(left, top, left+chosenSwatch, top+chosenSwatch)
	s.drawSwatch(screen, s.colors(docs), swatch{rect: box, color: s.color, name: ""}, false)
	textLeft := box.Max.X + ui.Inset
	room := left + width - textLeft
	name, _ := s.colorName(docs, s.color)
	lines := []string{name}
	if shade, ok := s.chosenShade(docs); ok {
		if shade.Label != "" {
			lines = append(lines, "“"+shade.Label+"”")
		}
		lines = append(lines, fmt.Sprintf("%s · %d%%", shade.Color, shade.Opacity))
	}
	for at, line := range lines {
		ink := ui.ColorInk()
		if at > 0 {
			ink = ui.ColorMuted()
		}
		s.theme.DrawText(screen, s.theme.FitText(line, room), textLeft, top+at*ui.LineHeight, ink)
	}
}

// drawSwatches draws the clear colour, each palette's heading, and the ramps
// of every palette not folded, clipped to their area, with a scroll bar when
// they do not fit, and the name of the swatch under the cursor.
func (s *State) drawSwatches(screen *ebiten.Image, docs frame.Docs) {
	area, clipped := screen.SubImage(s.panel.area).(*ebiten.Image)
	if !clipped || len(s.panel.swatches) == 0 {
		return
	}
	empty := s.panel.swatches[0].rect
	s.theme.DrawText(area, "clear", empty.Max.X+ui.Gap*ui.Half, empty.Min.Y+ui.Gap/ui.Half, ui.ColorMuted())
	for _, header := range s.panel.headers {
		text := header.rect.Min.Y + ui.Gap/ui.Half
		mark := image.Rect(header.rect.Min.X, header.rect.Min.Y, header.rect.Min.X+foldColumn-ui.Gap, header.rect.Max.Y)
		ui.DrawTriangle(area, mark, !header.folded, ui.ColorMuted())
		s.theme.DrawText(area, header.name, header.rect.Min.X+foldColumn, text, ui.ColorInk())
		if header.note != "" {
			s.theme.DrawText(area, header.note, header.rect.Min.X+foldColumn+s.theme.TextWidth(header.name)+ui.Gap, text,
				ui.ColorMuted())
		}
	}
	for _, label := range s.panel.ramps {
		s.theme.DrawText(area, s.theme.FitText(label.ramp.Name, label.rect.Dx()), label.rect.Min.X,
			label.rect.Min.Y+ui.Gap/ui.Half, ui.ColorInk())
	}
	colors := s.colors(docs)
	for _, offered := range s.panel.swatches {
		if offered.rect.Overlaps(s.panel.area) {
			s.drawSwatch(area, colors, offered, true)
		}
	}
	if view := s.panel.area.Dy(); s.panel.content > view {
		top := s.panel.area.Min.Y + s.scroll*view/s.panel.content
		bar := image.Rect(s.panel.area.Max.X-scrollBar, top, s.panel.area.Max.X, top+view*view/s.panel.content)
		ui.FillRound(screen, bar, scrollBar/ui.Half, ui.ColorSubdued())
	}
}

// drawSwatch draws one colour: see-through shades over a checkerboard, and,
// when asked, the chosen colour outlined in blue.
func (s *State) drawSwatch(screen *ebiten.Image, colors art.Palettes, offered swatch, outline bool) {
	rect := offered.rect
	half := rect.Dx() / ui.Half
	ui.FillRect(screen, rect, ui.ColorBackground())
	ui.FillRect(screen, image.Rect(rect.Min.X+half, rect.Min.Y, rect.Max.X, rect.Min.Y+half), ui.Hex(hexChecker))
	ui.FillRect(screen, image.Rect(rect.Min.X, rect.Min.Y+half, rect.Min.X+half, rect.Max.Y), ui.Hex(hexChecker))
	if value, known := colors.Color(offered.color); known && !offered.color.IsClear() {
		var ink color.Color = value
		ui.FillRect(screen, rect, ink)
	}
	ui.StrokeRect(screen, rect, ui.Hairline, ui.ColorRule())
	if outline && offered.color == s.color {
		ui.StrokeRect(screen, rect.Inset(-chosenWidth), chosenWidth, ui.ColorSelect())
	}
}

// drawTip names the swatch under the cursor, with its label and value, in a
// tip beside it.
func (s *State) drawTip(screen *ebiten.Image, docs frame.Docs, cursor image.Point) {
	if !cursor.In(s.panel.area) {
		return
	}
	for _, offered := range s.panel.swatches {
		if !cursor.In(offered.rect) || offered.color.IsClear() {
			continue
		}
		text := offered.name
		if found, ok := palette(docs, offered.color.Palette); ok {
			if shade, _, resolved := found.Resolve(offered.color.Ramp, offered.color.Shade); resolved {
				if shade.Label != "" {
					text += " · " + shade.Label
				}
				text += " · " + shade.Color.String()
			}
		}
		width := s.theme.TextWidth(text) + ui.Half*tipPad
		corner := image.Pt(min(cursor.X+tipOffset, s.size.X-width-ui.Gap), cursor.Y+tipOffset)
		tip := image.Rectangle{Min: corner, Max: corner.Add(image.Pt(width, ui.LineHeight+tipPad))}
		ui.FillRound(screen, tip, ui.Radius, ui.ColorInk())
		s.theme.DrawText(screen, text, tip.Min.X+tipPad, tip.Min.Y+tipPad/ui.Half, ui.ColorBackground())

		return
	}
}

// colors resolves colours from every palette the window holds.
func (s *State) colors(docs frame.Docs) art.Palettes {
	colors := art.Palettes{}
	for _, paletteID := range s.palettes {
		if found, ok := palette(docs, paletteID); ok {
			colors[paletteID] = found
		}
	}

	return colors
}

// syncPalettes keeps the window holding every palette of the project, and
// the palettes the zones the sprite is placed in list, which come first.
func (s *State) syncPalettes(docs frame.Docs) {
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(art.PaletteFormatName) {
				if !slices.Contains(s.palettes, entry.ID) {
					s.palettes = append(s.palettes, entry.ID)
				}
			}
		}
	})
	s.first = placedPalettes(docs, s.shownStandIn(docs))
}
