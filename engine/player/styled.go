package player

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/engine/player/fonts"
)

// Sizes of what a game shows, in points: its text, the small text of a
// box's count and hint, a line's height, the space a box keeps from the edge
// of its area, and a box's outline and corners.
const (
	TextSize            = 13
	SmallSize           = 11
	LineHeight          = 20
	inset               = 14
	border      float32 = 1.5
	radius      float32 = 6
	byteMask            = 0xff
	redShift            = 16
	greenShift          = 8
	fullOpacity         = 0xff
)

// Type is the faces what a game shows is written in: its text, bold, italic
// and both, for a message's markup, and the small bold of a box's count and
// hint.
type Type struct {
	Face         *text.GoTextFace
	Strong       *text.GoTextFace
	Italic       *text.GoTextFace
	StrongItalic *text.GoTextFace
	Small        *text.GoTextFace
	// sized holds the faces opened at each scale they are drawn at.
	sized *sizedFaces
}

// sizedFaces holds faces opened at a scale.
type sizedFaces struct {
	sync.Mutex

	faces map[sizedFace]*text.GoTextFace
}

// sizedFace is a face at a scale.
type sizedFace struct {
	face  *text.GoTextFace
	scale float64
}

// TypeOf is a type of faces already opened, as the editor's are.
func TypeOf(face, strong, italic, strongItalic, small *text.GoTextFace) *Type {
	return &Type{
		Face: face, Strong: strong, Italic: italic, StrongItalic: strongItalic, Small: small,
		sized: &sizedFaces{Mutex: sync.Mutex{}, faces: map[sizedFace]*text.GoTextFace{}},
	}
}

// NewType opens the player's own faces: IBM Plex Sans at the sizes a game's
// text is shown in.
func NewType() (*Type, error) {
	open := func(name string, data []byte, size float64) (*text.GoTextFace, error) {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("load font %s: %w", name, err)
		}
		var made text.GoTextFace
		made.Source = source
		made.Size = size

		return &made, nil
	}
	face, err := open("sans", fonts.SansRegular(), TextSize)
	if err != nil {
		return nil, err
	}
	strong, err := open("sans-bold", fonts.SansSemiBold(), TextSize)
	if err != nil {
		return nil, err
	}
	italic, err := open("sans-italic", fonts.SansItalic(), TextSize)
	if err != nil {
		return nil, err
	}
	strongItalic, err := open("sans-bold-italic", fonts.SansSemiBoldItalic(), TextSize)
	if err != nil {
		return nil, err
	}
	var small text.GoTextFace
	small.Source = strong.Source
	small.Size = SmallSize

	return TypeOf(face, strong, italic, strongItalic, &small), nil
}

// at returns a face opened at its size times a scale, so its text is drawn
// at the screen's own pixels rather than drawn small and enlarged.
func (t *Type) at(face *text.GoTextFace, scale float64) *text.GoTextFace {
	if scale == 1 {
		return face
	}
	t.sized.Lock()
	defer t.sized.Unlock()
	key := sizedFace{face: face, scale: scale}
	if sized, ok := t.sized.faces[key]; ok {
		return sized
	}
	var sized text.GoTextFace
	sized.Source = face.Source
	sized.Size = face.Size * scale
	t.sized.faces[key] = &sized

	return &sized
}

// look is the face a look is drawn in.
func (t *Type) look(bold, italic bool) *text.GoTextFace {
	switch {
	case bold && italic:
		return t.StrongItalic
	case bold:
		return t.Strong
	case italic:
		return t.Italic
	}

	return t.Face
}

// Writer writes what a game shows. It lays out in points and draws each
// point as Scale of the screen's pixels, so text and edges stay sharp at any
// scale.
type Writer struct {
	Type  *Type
	Scale float64
}

// Styled is a stretch of text drawn one way: bold or not, italic or not,
// and in a colour, or with none in the ink it is drawn with.
type Styled struct {
	Text   string
	Bold   bool
	Italic bool
	Colour color.Color
}

// Piece is part of a line of styled text: its words, the face and colour
// they are drawn in, and where the piece starts along the line.
type Piece struct {
	Text   string
	Face   *text.GoTextFace
	Colour color.Color
	X      int
}

// StyledLine is one line of styled text, its pieces from left to right,
// and how wide it is.
type StyledLine struct {
	Pieces []Piece
	Width  int
}

// tokenKind is what a stretch of styled text is, for wrapping: a word,
// the space between words, or a line break.
type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenSpace
	tokenBreak
)

// styledToken is a word, a space or a line break, in the pieces of styled
// text it spans; a word can change look partway, as in [b]Mar[/b]ta.
type styledToken struct {
	kind  tokenKind
	parts []Styled
}

// Width measures text in a face, in whole points, at the scale it is drawn
// at.
func (w Writer) Width(value string, face *text.GoTextFace) int {
	width, _ := text.Measure(value, w.Type.at(face, w.Scale), LineHeight*w.Scale)

	return int(width / w.Scale)
}

// Wrap breaks styled text into lines that fit a width, keeping its own
// line breaks; a word wider than the width has a line of its own.
func (w Writer) Wrap(runs []Styled, width int) []StyledLine {
	lines := []StyledLine{{Pieces: nil, Width: 0}}
	var space []Styled
	for _, token := range tokenizeStyled(runs) {
		current := &lines[len(lines)-1]
		switch token.kind {
		case tokenBreak:
			lines, space = append(lines, StyledLine{Pieces: nil, Width: 0}), nil
		case tokenSpace:
			space = token.parts
		case tokenWord:
			gap := w.styledWidth(space)
			if len(current.Pieces) > 0 && current.Width+gap+w.styledWidth(token.parts) > width {
				lines = append(lines, StyledLine{Pieces: nil, Width: 0})
				current, space = &lines[len(lines)-1], nil
			}
			if len(current.Pieces) > 0 {
				w.place(current, space)
			}
			w.place(current, token.parts)
			space = nil
		}
	}

	return lines
}

// place adds styled pieces at the end of a line.
func (w Writer) place(line *StyledLine, parts []Styled) {
	for _, part := range parts {
		face := w.Type.look(part.Bold, part.Italic)
		line.Pieces = append(line.Pieces, Piece{Text: part.Text, Face: face, Colour: part.Colour, X: line.Width})
		line.Width += w.Width(part.Text, face)
	}
}

// styledWidth measures pieces of styled text side by side.
func (w Writer) styledWidth(parts []Styled) int {
	width := 0
	for _, part := range parts {
		width += w.Width(part.Text, w.Type.look(part.Bold, part.Italic))
	}

	return width
}

// tokenizeStyled splits styled text into words, spaces and line breaks,
// each in the looks it spans.
func tokenizeStyled(runs []Styled) []styledToken {
	var tokens []styledToken
	for _, run := range runs {
		for _, char := range run.Text {
			kind := tokenWord
			switch {
			case char == '\n':
				kind = tokenBreak
			case unicode.IsSpace(char):
				kind = tokenSpace
			}
			last := len(tokens) - 1
			if kind == tokenBreak || last < 0 || tokens[last].kind != kind {
				tokens = append(tokens, styledToken{kind: kind, parts: nil})
				last = len(tokens) - 1
			}
			if kind == tokenBreak {
				continue
			}
			parts := tokens[last].parts
			look := Styled{Text: "", Bold: run.Bold, Italic: run.Italic, Colour: run.Colour}
			if len(parts) == 0 || !sameLook(parts[len(parts)-1], look) {
				parts = append(parts, look)
			}
			parts[len(parts)-1].Text += string(char)
			tokens[last].parts = parts
		}
	}

	return tokens
}

func sameLook(a, b Styled) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Colour == b.Colour
}

// StyledWidth is how wide the widest of some styled lines is.
func StyledWidth(lines []StyledLine) int {
	widest := 0
	for _, line := range lines {
		widest = max(widest, line.Width)
	}

	return widest
}

// DrawStyled draws styled lines from a top-left point, a line apart, each
// piece in its own colour or else in ink.
func (w Writer) DrawStyled(screen *ebiten.Image, lines []StyledLine, left, top int, ink color.Color) {
	for index, line := range lines {
		for _, piece := range line.Pieces {
			colour := piece.Colour
			if colour == nil {
				colour = ink
			}
			w.Draw(screen, strings.TrimRight(piece.Text, "\n"), piece.Face, left+piece.X, top+index*LineHeight, colour)
		}
	}
}

// Draw draws text in a face with its top-left corner at a point.
func (w Writer) Draw(screen *ebiten.Image, value string, face *text.GoTextFace, x, y int, ink color.Color) {
	options := &text.DrawOptions{}
	options.GeoM.Translate(float64(x)*w.Scale, math.Round(float64(y)*w.Scale))
	options.ColorScale.ScaleWithColor(ink)
	text.Draw(screen, value, w.Type.at(face, w.Scale), options)
}

// px is a length in points in the screen's pixels.
func (w Writer) px(value float32) float32 { return value * float32(w.Scale) }

// edge is the edge of a box in points in the screen's pixels, a whole pixel
// so that boxes that meet still meet.
func (w Writer) edge(value int) float32 {
	if w.Scale == 1 {
		return float32(value)
	}

	return float32(math.Round(float64(value) * w.Scale))
}

// line is the width of a line drawn without smoothing in the screen's
// pixels: as wide as asked at one pixel to a point, and otherwise a whole
// number of pixels, at least one.
func (w Writer) line(width float32) float32 {
	if w.Scale == 1 {
		return width
	}

	return max(1, float32(math.Round(float64(width)*w.Scale)))
}

// fillRect fills a rectangle.
func (w Writer) fillRect(screen *ebiten.Image, rect image.Rectangle, fill color.Color) {
	minX, minY, maxX, maxY := w.edge(rect.Min.X), w.edge(rect.Min.Y), w.edge(rect.Max.X), w.edge(rect.Max.Y)
	vector.FillRect(screen, minX, minY, maxX-minX, maxY-minY, fill, false)
}

// fillRound fills a rectangle with rounded corners.
func (w Writer) fillRound(screen *ebiten.Image, rect image.Rectangle, fill color.Color) {
	if rect.Empty() {
		return
	}
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(fill)
	vector.FillPath(screen, w.rounded(rect, 0), &vector.FillOptions{FillRule: vector.FillRuleNonZero}, options)
}

// strokeRound outlines a rectangle with rounded corners, keeping the line
// inside the rectangle.
func (w Writer) strokeRound(screen *ebiten.Image, rect image.Rectangle, stroke color.Color) {
	if rect.Empty() {
		return
	}
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(stroke)
	var line vector.StrokeOptions
	line.Width = w.px(border)
	vector.StrokePath(screen, w.rounded(rect, border/half), &line, options)
}

// rounded returns the path of a rounded rectangle, inset by some points, in
// the screen's pixels.
func (w Writer) rounded(rect image.Rectangle, inside float32) *vector.Path {
	inside = w.px(inside)
	corner := w.px(radius)
	left, top := w.edge(rect.Min.X)+inside, w.edge(rect.Min.Y)+inside
	right, bottom := w.edge(rect.Max.X)-inside, w.edge(rect.Max.Y)-inside
	corner = max(min(corner, (right-left)/half, (bottom-top)/half), 0)
	var path vector.Path
	path.MoveTo(left+corner, top)
	path.LineTo(right-corner, top)
	path.ArcTo(right, top, right, top+corner, corner)
	path.LineTo(right, bottom-corner)
	path.ArcTo(right, bottom, right-corner, bottom, corner)
	path.LineTo(left+corner, bottom)
	path.ArcTo(left, bottom, left, bottom-corner, corner)
	path.LineTo(left, top+corner)
	path.ArcTo(left, top, left+corner, top, corner)
	path.Close()

	return &path
}

// strokeLine draws a straight line.
func (w Writer) strokeLine(screen *ebiten.Image, from, to image.Point, ink color.Color) {
	vector.StrokeLine(screen, w.edge(from.X), w.edge(from.Y), w.edge(to.X), w.edge(to.Y), w.line(border), ink, false)
}

// hexAlpha returns a colour from a 0xRRGGBB value and an alpha. Go colours
// carry premultiplied alpha, so each channel is scaled by it.
func hexAlpha(rgb uint32, alpha uint8) color.RGBA {
	scale := func(channel uint32) uint8 {
		return uint8(channel & byteMask * uint32(alpha) / fullOpacity) //nolint:gosec // At most 255 by construction.
	}

	return color.RGBA{R: scale(rgb >> redShift), G: scale(rgb >> greenShift), B: scale(rgb), A: alpha}
}
