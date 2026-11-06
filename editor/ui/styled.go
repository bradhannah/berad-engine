package ui

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
)

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
	Face   *etext.GoTextFace
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

// face is the face a look is drawn in.
func (t *Theme) face(bold, italic bool) *etext.GoTextFace {
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

// WrapStyled breaks styled text into lines that fit a width, keeping its
// own line breaks; a word wider than the width has a line of its own.
func (t *Theme) WrapStyled(runs []Styled, width int) []StyledLine {
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
			gap := t.styledWidth(space)
			if len(current.Pieces) > 0 && current.Width+gap+t.styledWidth(token.parts) > width {
				lines = append(lines, StyledLine{Pieces: nil, Width: 0})
				current, space = &lines[len(lines)-1], nil
			}
			if len(current.Pieces) > 0 {
				t.place(current, space)
			}
			t.place(current, token.parts)
			space = nil
		}
	}

	return lines
}

// place adds styled pieces at the end of a line.
func (t *Theme) place(line *StyledLine, parts []Styled) {
	for _, part := range parts {
		face := t.face(part.Bold, part.Italic)
		line.Pieces = append(line.Pieces, Piece{Text: part.Text, Face: face, Colour: part.Colour, X: line.Width})
		line.Width += measure(part.Text, face)
	}
}

// styledWidth measures pieces of styled text side by side.
func (t *Theme) styledWidth(parts []Styled) int {
	width := 0
	for _, part := range parts {
		width += measure(part.Text, t.face(part.Bold, part.Italic))
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

// DrawStyled draws styled lines from a top-left point, a line apart, each
// piece in its own colour or else in ink.
func (t *Theme) DrawStyled(screen *ebiten.Image, lines []StyledLine, left, top int, ink color.Color) {
	for index, line := range lines {
		for _, piece := range line.Pieces {
			colour := piece.Colour
			if colour == nil {
				colour = ink
			}
			draw(screen, strings.TrimRight(piece.Text, "\n"), piece.Face, float64(left+piece.X),
				float64(top+index*LineHeight), colour)
		}
	}
}

// StyledWidth is how wide the widest of some styled lines is.
func StyledWidth(lines []StyledLine) int {
	widest := 0
	for _, line := range lines {
		widest = max(widest, line.Width)
	}

	return widest
}
