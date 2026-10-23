package window

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
)

const (
	shortID     = 6
	lineHeight  = 20
	panelInset  = 14
	outlineWide = 2
	hairline    = 1
	dotSize     = 2
	labelGap    = 9
	labelDrop   = 3
	selectPad   = 3
)

// The plain white look agreed for the editor, with blue for selection.
const (
	hexBackground = 0xffffff
	hexInk        = 0x1f2328
	hexMuted      = 0x5a6270
	hexSelect     = 0x1f5fbf
	hexPanel      = 0xf7f7f5
	gridAlpha     = 0x22
)

func colorBackground() color.RGBA { return mapview.Hex(hexBackground) }
func colorInk() color.RGBA        { return mapview.Hex(hexInk) }
func colorMuted() color.RGBA      { return mapview.Hex(hexMuted) }
func colorRule() color.RGBA       { return mapview.Hex(hexInk) }
func colorGrid() color.RGBA       { return mapview.HexAlpha(0, gridAlpha) }
func colorSelect() color.RGBA     { return mapview.Hex(hexSelect) }
func colorPanel() color.RGBA      { return mapview.Hex(hexPanel) }

// Draw renders the window.
func (g *mapGame) Draw(screen *ebiten.Image) {
	screen.Fill(colorBackground())
	var floorInfo client.Document
	loaded := g.conn.View(g.target, func(document client.Document) {
		floorInfo = document
		floorInfo.Model = nil
		g.drawFloor(screen, document)
	})
	g.drawPalette(screen)
	g.drawInspector(screen, floorInfo, loaded)
	g.drawStatus(screen)
	g.frames++
	if g.screenshot != "" && g.frames == screenshotFrame {
		if err := saveScreenshot(screen, g.screenshot); err != nil {
			g.setStatus(err.Error())
		}
	}
}

func (g *mapGame) drawFloor(screen *ebiten.Image, document client.Document) {
	floorDoc, ok := document.Model.(*tilefloor.Document)
	if !ok {
		return
	}
	floor := floorDoc.Floor
	g.state.SetFloorSize(floor.Width(), floor.Height())
	grid := g.layout.Grid()
	for row := range floor.Height() {
		for col := range floor.Width() {
			rect := g.state.CellRect(col, row, g.layout)
			if !rect.Overlaps(grid) {
				continue
			}
			tile, err := floor.At(col, row)
			if err != nil {
				continue
			}
			fillRect(screen, rect, mapview.ColorOf(tile))
			if tile == "" {
				center := g.state.CellCenter(col, row, g.layout)
				fillRect(screen, image.Rect(center.X, center.Y, center.X+dotSize, center.Y+dotSize), colorMuted())
			}
			strokeRect(screen, rect, hairline, colorGrid())
		}
	}
	if hover := g.state.Hover(); hover.OK {
		strokeRect(screen, g.state.CellRect(hover.Col, hover.Row, g.layout), outlineWide, colorSelect())
	}
}

func (g *mapGame) drawPalette(screen *ebiten.Image) {
	fillRect(screen, image.Rect(0, 0, mapview.PaletteWidth, g.layout.Height-mapview.StatusHeight), colorPanel())
	fillRect(screen, image.Rect(mapview.PaletteWidth-hairline, 0, mapview.PaletteWidth, g.layout.Height), colorRule())
	g.drawText(screen, "TILES", panelInset, panelInset, colorMuted())
	for index, tile := range mapview.Palette() {
		rect := mapview.PaletteRect(index)
		fillRect(screen, rect, tile.Color)
		strokeRect(screen, rect, hairline, colorRule())
		name := tile.Name
		if name == "" {
			name = "unassigned"
		}
		label := strconv.Itoa(index+1) + "  " + name
		labelColor := colorInk()
		if tile.Name == g.state.Tile().Name {
			strokeRect(screen, rect.Inset(-selectPad), outlineWide, colorSelect())
			labelColor = colorSelect()
		}
		g.drawText(screen, label, rect.Max.X+labelGap, rect.Min.Y+labelDrop, labelColor)
	}
}

func (g *mapGame) drawInspector(screen *ebiten.Image, document client.Document, loaded bool) {
	left := g.layout.Width - mapview.InspectorWidth
	fillRect(screen, image.Rect(left, 0, left+hairline, g.layout.Height), colorRule())
	textLeft := left + panelInset
	lines := []struct {
		text  string
		color color.Color
	}{
		{"INSPECTOR", colorMuted()},
		{document.Name, colorInk()},
		{"revision " + strconv.FormatUint(document.Revision, 10), colorMuted()},
		{"", colorInk()},
		{"Selected: " + g.cellText(g.state.Selected(), document), colorInk()},
		{"Hovered:  " + g.cellText(g.state.Hover(), document), colorInk()},
		{"Painting with: " + displayTile(g.state.Tile().Name), colorInk()},
		{"", colorInk()},
		{"SPEED (command to event)", colorMuted()},
		{"last " + formatLatency(g.lastLatency()) + ", median " + formatLatency(g.medianLatency()), colorInk()},
		{"", colorInk()},
		{"KEYS", colorMuted()},
		{"drag: paint   1-6: tile", colorInk()},
		{"Cmd+Z / Cmd+Shift+Z: undo / redo", colorInk()},
		{"Cmd+N: new view of this map", colorInk()},
		{"Cmd+W: close   Cmd+Shift+K: test crash", colorInk()},
		{"", colorInk()},
		{"window " + g.conn.Window().Short(shortID), colorMuted()},
	}
	if !loaded {
		lines[1].text = "document not loaded"
	}
	for index, line := range lines {
		g.drawText(screen, line.text, textLeft, panelInset+index*lineHeight, line.color)
	}
}

func (g *mapGame) cellText(cell mapview.Cell, document client.Document) string {
	if !cell.OK {
		return "-"
	}
	tile := ""
	g.conn.View(document.ID, func(current client.Document) {
		if floorDoc, ok := current.Model.(*tilefloor.Document); ok {
			tile, _ = floorDoc.Floor.At(cell.Col, cell.Row) //nolint:errcheck // An out-of-range cell shows as unassigned.
		}
	})

	return fmt.Sprintf("%d, %d  %s", cell.Col, cell.Row, displayTile(tile))
}

func (g *mapGame) drawStatus(screen *ebiten.Image) {
	top := g.layout.Height - mapview.StatusHeight
	fillRect(screen, image.Rect(0, top, g.layout.Width, g.layout.Height), colorPanel())
	fillRect(screen, image.Rect(0, top, g.layout.Width, top+hairline), colorRule())
	message := "Ready. Every change is saved automatically."
	messageColor := colorMuted()
	if g.notice != "" {
		message, messageColor = "Notice: "+g.notice, colorSelect()
	}
	if g.status != "" && time.Now().Before(g.statusUntil) {
		message, messageColor = g.status, colorInk()
	}
	g.drawText(screen, message, panelInset, top+(mapview.StatusHeight-textSize)/2-1, messageColor)
}

func (g *mapGame) lastLatency() time.Duration {
	if len(g.latencies) == 0 {
		return 0
	}

	return g.latencies[len(g.latencies)-1]
}

func (g *mapGame) drawText(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	options := &text.DrawOptions{}
	options.GeoM.Translate(float64(x), float64(y))
	options.ColorScale.ScaleWithColor(ink)
	text.Draw(screen, value, g.face, options)
}

func displayTile(name string) string {
	if name == "" {
		return "unassigned"
	}

	return name
}

func fillRect(screen *ebiten.Image, rect image.Rectangle, fill color.Color) {
	minX, minY := float32(rect.Min.X), float32(rect.Min.Y)
	vector.FillRect(screen, minX, minY, float32(rect.Dx()), float32(rect.Dy()), fill, false)
}

func strokeRect(screen *ebiten.Image, rect image.Rectangle, width float32, stroke color.Color) {
	minX, minY := float32(rect.Min.X), float32(rect.Min.Y)
	vector.StrokeRect(screen, minX, minY, float32(rect.Dx()), float32(rect.Dy()), width, stroke, false)
}

// saveScreenshot writes the window's pixels to a PNG once, for automated visual
// checks. It is enabled by the BERAD_SCREENSHOT environment variable.
func saveScreenshot(screen *ebiten.Image, path string) error {
	bounds := screen.Bounds()
	pixels := image.NewRGBA(bounds)
	screen.ReadPixels(pixels.Pix)
	file, err := os.Create(path) //nolint:gosec // The path comes from the developer's own environment.
	if err != nil {
		return fmt.Errorf("create screenshot: %w", err)
	}
	encodeErr := png.Encode(file, pixels)
	closeErr := file.Close()
	if err := errors.Join(encodeErr, closeErr); err != nil {
		return fmt.Errorf("write screenshot: %w", err)
	}

	return nil
}
