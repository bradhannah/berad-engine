package zonepicker

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// The look: the ground floor drawn at most this tall, the width of its
// labels, and how many names a line lists before saying how many more.
const (
	groundHeight = 132
	labelWidth   = 76
	namesShown   = 4
)

// A cell's pixel: its bytes, red, green, blue and alpha.
const (
	bytesPerCell = 4
	red          = 0
	green        = 1
	blue         = 2
	alpha        = 3
	opaque       = 0xff
)

// looks keeps each zone's ground floor drawn a pixel a cell, until the
// floor changes.
type looks struct {
	drawn map[ids.ULID]drawnFloor
}

type drawnFloor struct {
	revision uint64
	image    *ebiten.Image
}

func newLooks() *looks { return &looks{drawn: map[ids.ULID]drawnFloor{}} }

// ground returns a zone's ground floor drawn a pixel a cell in its tiles'
// stand-in colours, the top layer over the ground.
func (l *looks) ground(each Zone) *ebiten.Image {
	if each.Ground == nil {
		return nil
	}
	if kept, ok := l.drawn[each.ID]; ok && kept.revision == each.Drawn {
		return kept.image
	}
	bounds := each.Ground.Bounds()
	if bounds.Empty() {
		return nil
	}
	pixels := make([]byte, bounds.Dx()*bounds.Dy()*bytesPerCell)
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for col := bounds.Min.X; col < bounds.Max.X; col++ {
			colour := ui.Hex(cellColour(each.Ground, col, row))
			at := ((row-bounds.Min.Y)*bounds.Dx() + col - bounds.Min.X) * bytesPerCell
			pixels[at+red], pixels[at+green], pixels[at+blue], pixels[at+alpha] = colour.R, colour.G, colour.B, opaque
		}
	}
	drawn := ebiten.NewImage(bounds.Dx(), bounds.Dy())
	drawn.WritePixels(pixels)
	if kept, ok := l.drawn[each.ID]; ok {
		kept.image.Deallocate()
	}
	l.drawn[each.ID] = drawnFloor{revision: each.Drawn, image: drawn}

	return drawn
}

// cellColour is a cell's colour: its top tile's, else its ground's.
func cellColour(ground *floor.Floor, col, row int) uint32 {
	if top, err := ground.Top(col, row); err == nil && top != "" {
		return tiles.ColorOf(top)
	}
	groundTile, _ := ground.Ground(col, row) //nolint:errcheck // Inside the floor's bounds.

	return tiles.ColorOf(groundTile)
}

// looked is the zone the look shows: the one under the cursor, else the
// one selected, else the first listed.
func (p *Picker) looked() (Zone, bool) {
	for _, each := range p.shown {
		if each.ID == p.hover {
			return each, true
		}
	}

	return p.pick()
}

// drawLook draws the look at a zone: its name, its ground floor drawn
// small, and what it holds.
func (p *Picker) drawLook(screen *ebiten.Image) {
	ui.FillRound(screen, p.look, ui.Radius, ui.ColorDesk())
	each, found := p.looked()
	if !found {
		return
	}
	left, top := p.look.Min.X+ui.Inset, p.look.Min.Y+ui.Gap
	p.theme.DrawStrong(screen, p.theme.FitText(each.Name, p.look.Dx()-ui.Half*ui.Inset), left, top, ui.ColorInk())
	top += ui.LineHeight + ui.Gap
	frame := image.Rect(left, top, p.look.Max.X-ui.Inset, top+groundHeight)
	ui.FillRect(screen, frame, ui.ColorPage())
	if drawn := p.looks.ground(each); drawn != nil {
		p.drawGround(screen, drawn, frame)
		caption := each.Floors[0] + ", the ground floor"
		p.theme.DrawSmall(screen, p.theme.FitText(caption, frame.Dx()-ui.Gap), frame.Min.X+ui.Half, frame.Min.Y+ui.Half,
			ui.ColorInk())
	}
	ui.StrokeRound(screen, frame, ui.Half, ui.Hairline, ui.ColorRule())
	top = frame.Max.Y + ui.Gap
	for _, line := range p.lookLines(each) {
		p.theme.DrawText(screen, line[0], left, top, ui.ColorMuted())
		for index, text := range p.theme.Wrap(line[1], p.look.Max.X-ui.Inset-left-labelWidth) {
			if top+(index+1)*ui.LineHeight > p.look.Max.Y {
				break
			}
			p.theme.DrawText(screen, text, left+labelWidth, top+index*ui.LineHeight, ui.ColorInk())
		}
		top += max(1, len(p.theme.Wrap(line[1], p.look.Max.X-ui.Inset-left-labelWidth))) * ui.LineHeight
		if top > p.look.Max.Y-ui.LineHeight {
			return
		}
	}
}

// drawGround draws a ground floor drawn a pixel a cell, scaled to fit a
// frame and centred in it.
func (p *Picker) drawGround(screen *ebiten.Image, drawn *ebiten.Image, frame image.Rectangle) {
	size := drawn.Bounds().Size()
	scale := min(float64(frame.Dx())/float64(size.X), float64(frame.Dy())/float64(size.Y))
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(
		float64(frame.Min.X)+(float64(frame.Dx())-scale*float64(size.X))/ui.Half,
		float64(frame.Min.Y)+(float64(frame.Dy())-scale*float64(size.Y))/ui.Half,
	)
	options.Filter = ebiten.FilterNearest
	screen.DrawImage(drawn, options)
}

// lookLines are the look's lines: a label and what it says.
func (p *Picker) lookLines(each Zone) [][2]string {
	none := func(names []string, empty string) string {
		if len(names) == 0 {
			return empty
		}
		if len(names) > namesShown {
			return fmt.Sprintf("%s and %d more", strings.Join(names[:namesShown], ", "), len(names)-namesShown)
		}

		return strings.Join(names, ", ")
	}
	size := "no floors yet"
	if len(each.Floors) > 0 {
		size = strconv.Itoa(each.Size.X) + " × " + strconv.Itoa(each.Size.Y) + " tiles"
	}
	lines := [][2]string{
		{"Floors", none(each.Floors, "none yet")}, {"Size", size}, {"Lives here", none(each.Living, "no one yet")},
		{"Places", none(each.Places, "none yet")},
	}
	if len(each.Tags) > 0 {
		lines = append(lines, [2]string{"Tags", strings.Join(each.Tags, ", ")})
	}
	if notes := strings.TrimSpace(each.Notes); notes != "" {
		lines = append(lines, [2]string{"Notes", strings.ReplaceAll(notes, "\n", " ")})
	}
	if each.Refused != "" {
		lines = append(lines, [2]string{"Can't", each.Refused})
	}

	return lines
}
