package ui

import "image/color"

// Tint is one of the colours that tell things apart: those an author gives
// a node or a line to keep a workflow organised, and those that tell open
// documents' tabs apart. Each has a wash for a background and a darker ink
// for a line or a mark. There is no
// blue, which is selection, and no red, which is problems.
type Tint struct {
	Name string
	Wash color.RGBA
	Ink  color.RGBA
}

// The tints' colours: the editor's family hues, and amber. The washes are
// deep enough to hold against the desk and the page.
const (
	hexGreenWash  = 0xd7ead9
	hexGreenInk   = 0x3d7a4b
	hexAmberWash  = 0xf8e8c4
	hexAmberInk   = 0x9a6700
	hexTealWash   = 0xd0eaec
	hexTealInk    = 0x267378
	hexPurpleWash = 0xe2daf1
	hexPurpleInk  = 0x6a4f9e
	hexPinkWash   = 0xf3dbe7
	hexPinkInk    = 0x8a4b72
	hexBrownWash  = 0xebdfd1
	hexBrownInk   = 0x7a6248
	hexSlateWash  = 0xdbe3ea
	hexSlateInk   = 0x4c6377
)

// Tints lists the tints in the order they are offered and given out.
func Tints() []Tint {
	return []Tint{
		{Name: "green", Wash: Hex(hexGreenWash), Ink: Hex(hexGreenInk)},
		{Name: "amber", Wash: Hex(hexAmberWash), Ink: Hex(hexAmberInk)},
		{Name: "teal", Wash: Hex(hexTealWash), Ink: Hex(hexTealInk)},
		{Name: "purple", Wash: Hex(hexPurpleWash), Ink: Hex(hexPurpleInk)},
		{Name: "pink", Wash: Hex(hexPinkWash), Ink: Hex(hexPinkInk)},
		{Name: "brown", Wash: Hex(hexBrownWash), Ink: Hex(hexBrownInk)},
		{Name: "slate", Wash: Hex(hexSlateWash), Ink: Hex(hexSlateInk)},
	}
}
