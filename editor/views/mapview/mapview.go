// Package mapview is the Map window's behaviour, kept apart from drawing and
// from Ebitengine so it can be tested without a screen: it turns input into
// actions (paint, undo, open a view) and tracks what is hovered and selected.
package mapview

import (
	"image"
	"image/color"
	"strconv"
)

// Layout constants, in screen pixels.
const (
	PaletteWidth    = 150
	InspectorWidth  = 280
	StatusHeight    = 30
	DefaultCellSize = 24
	paletteRowSize  = 34
	paletteTop      = 44
	paletteSwatch   = 22
	paletteInset    = 12
	palettePad      = paletteInset / 2
	opaque          = 0xff
	byteMask        = 0xff
	redShift        = 16
	greenShift      = 8
	half            = 2
	hexGrass        = 0x8fc06a
	hexFloor        = 0xe8dcc4
	hexWall         = 0x4a4f58
	hexRoad         = 0xa8aba4
	hexWater        = 0x5c8cc8
	hexUnassigned   = 0xffffff
	hexUnknown      = 0xc9cbc5
)

// Tile is a paintable tile name and its stand-in colour.
type Tile struct {
	Name  string
	Color color.RGBA
}

// Palette returns the tiles offered in the Map window. The empty name is the
// unassigned stand-in.
func Palette() []Tile {
	return []Tile{
		{Name: "grass", Color: Hex(hexGrass)},
		{Name: "floor", Color: Hex(hexFloor)},
		{Name: "wall", Color: Hex(hexWall)},
		{Name: "road", Color: Hex(hexRoad)},
		{Name: "water", Color: Hex(hexWater)},
		{Name: "", Color: Hex(hexUnassigned)},
	}
}

// ColorOf returns a tile's colour; unknown names get a neutral grey.
func ColorOf(name string) color.RGBA {
	for _, tile := range Palette() {
		if tile.Name == name {
			return tile.Color
		}
	}

	return Hex(hexUnknown)
}

// Layout is the window's current size.
type Layout struct {
	Width  int
	Height int
}

// Grid returns the screen area that shows the floor.
func (l Layout) Grid() image.Rectangle {
	return image.Rect(PaletteWidth, 0, l.Width-InspectorWidth, l.Height-StatusHeight)
}

// Input is one frame of user input.
type Input struct {
	CursorX, CursorY int
	// Down is true while the main button is held; Pressed and Released mark
	// the frames it changed.
	Down, Pressed, Released bool
	Undo, Redo, NewView     bool
	Quit, SimulateCrash     bool
	// TileKey selects palette entry TileKey-1; zero selects nothing.
	TileKey int
}

// Paint asks for one cell to be painted as part of a stroke.
type Paint struct {
	Gesture string
	Col     int
	Row     int
	Tile    string
}

// Actions are what the window should do this frame.
type Actions struct {
	Paints        []Paint
	Undo, Redo    bool
	NewView, Quit bool
	SimulateCrash bool
}

// Cell is a floor position; OK is false when no cell applies.
type Cell struct {
	Col, Row int
	OK       bool
}

// State is the Map window's behaviour.
type State struct {
	window   string
	cellSize int
	floorW   int
	floorH   int
	tile     int
	hover    Cell
	selected Cell
	painting bool
	strokes  int
	last     Cell
}

// New returns the state for one Map window. The window ID makes stroke
// gestures unique across windows.
func New(window string) *State {
	none := Cell{Col: 0, Row: 0, OK: false}

	return &State{
		window: window, cellSize: DefaultCellSize, floorW: 0, floorH: 0, tile: 0,
		hover: none, selected: none, painting: false, strokes: 0, last: none,
	}
}

// SetFloorSize records the floor's dimensions in cells.
func (s *State) SetFloorSize(width, height int) {
	s.floorW, s.floorH = width, height
}

// CellSize returns the size of one cell on screen.
func (s *State) CellSize() int { return s.cellSize }

// Tile returns the selected palette tile.
func (s *State) Tile() Tile { return Palette()[s.tile] }

// Hover returns the cell under the cursor.
func (s *State) Hover() Cell { return s.hover }

// Selected returns the last clicked cell.
func (s *State) Selected() Cell { return s.selected }

// Painting reports whether a stroke is in progress.
func (s *State) Painting() bool { return s.painting }

// CellAt returns the floor cell at a screen position.
func (s *State) CellAt(posX, posY int, layout Layout) Cell {
	grid := layout.Grid()
	if !image.Pt(posX, posY).In(grid) {
		return Cell{Col: 0, Row: 0, OK: false}
	}
	col := (posX - grid.Min.X) / s.cellSize
	row := (posY - grid.Min.Y) / s.cellSize
	if col >= s.floorW || row >= s.floorH {
		return Cell{Col: 0, Row: 0, OK: false}
	}

	return Cell{Col: col, Row: row, OK: true}
}

// CellRect returns a cell's screen rectangle.
func (s *State) CellRect(col, row int, layout Layout) image.Rectangle {
	grid := layout.Grid()
	x := grid.Min.X + col*s.cellSize
	y := grid.Min.Y + row*s.cellSize

	return image.Rect(x, y, x+s.cellSize, y+s.cellSize)
}

// PaletteRect returns the screen rectangle of palette entry index.
func PaletteRect(index int) image.Rectangle {
	top := paletteTop + index*paletteRowSize

	return image.Rect(paletteInset, top, paletteInset+paletteSwatch, top+paletteSwatch)
}

// Update turns one frame of input into actions.
func (s *State) Update(input Input, layout Layout) Actions {
	actions := Actions{Paints: nil, Undo: input.Undo, Redo: input.Redo, NewView: input.NewView,
		Quit: input.Quit, SimulateCrash: input.SimulateCrash}
	if input.TileKey > 0 && input.TileKey <= len(Palette()) {
		s.tile = input.TileKey - 1
	}
	s.hover = s.CellAt(input.CursorX, input.CursorY, layout)
	if input.Pressed && s.pickPalette(input.CursorX, input.CursorY) {
		return actions
	}
	if input.Pressed && s.hover.OK {
		s.painting = true
		s.strokes++
		s.selected = s.hover
		s.last = Cell{Col: 0, Row: 0, OK: false}
	}
	if s.painting && input.Down && s.hover.OK && s.hover != s.last {
		actions.Paints = append(actions.Paints, Paint{
			Gesture: s.window + "-" + strconv.Itoa(s.strokes), Col: s.hover.Col, Row: s.hover.Row, Tile: s.Tile().Name,
		})
		s.last = s.hover
	}
	if input.Released || !input.Down {
		s.painting = false
	}

	return actions
}

func (s *State) pickPalette(x, y int) bool {
	for index := range Palette() {
		if image.Pt(x, y).In(PaletteRect(index).Inset(-palettePad)) {
			s.tile = index

			return true
		}
	}

	return false
}

// Hex returns an opaque colour from a 0xRRGGBB value.
func Hex(rgb uint32) color.RGBA {
	return HexAlpha(rgb, opaque)
}

// HexAlpha returns a colour from a 0xRRGGBB value and an alpha.
func HexAlpha(rgb uint32, alpha uint8) color.RGBA {
	return color.RGBA{
		R: uint8(rgb >> redShift & byteMask),
		G: uint8(rgb >> greenShift & byteMask),
		B: uint8(rgb & byteMask),
		A: alpha,
	}
}

// CellCenter returns the screen position at the middle of a cell.
func (s *State) CellCenter(col, row int, layout Layout) image.Point {
	rect := s.CellRect(col, row, layout)

	return image.Pt(rect.Min.X+rect.Dx()/half, rect.Min.Y+rect.Dy()/half)
}
