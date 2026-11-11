package ui

import "image/color"

// Tint is one of the colours that tell things apart: those an author gives
// a node or a line to keep a workflow organised, and those that tell open
// documents' tabs apart. Each has a wash for a background, a stronger wash
// for the tab in front, and a darker ink for a line or a mark. There is no
// blue, which is selection, and no red, which is problems.
type Tint struct {
	Name   string
	Wash   color.RGBA
	Strong color.RGBA
	Ink    color.RGBA
}

// The tints' colours: the editor's family hues, and amber. The washes are
// deep enough to hold against the desk and the page.
const (
	hexGreenWash    = 0xd7ead9
	hexGreenStrong  = 0xb5d9bd
	hexGreenInk     = 0x3d7a4b
	hexAmberWash    = 0xf8e8c4
	hexAmberStrong  = 0xf0d08a
	hexAmberInk     = 0x9a6700
	hexTealWash     = 0xd0eaec
	hexTealStrong   = 0xa9d9dd
	hexTealInk      = 0x267378
	hexPurpleWash   = 0xe2daf1
	hexPurpleStrong = 0xcabce6
	hexPurpleInk    = 0x6a4f9e
	hexPinkWash     = 0xf3dbe7
	hexPinkStrong   = 0xe8bbd2
	hexPinkInk      = 0x8a4b72
	hexBrownWash    = 0xebdfd1
	hexBrownStrong  = 0xdbc5ab
	hexBrownInk     = 0x7a6248
	hexSlateWash    = 0xdbe3ea
	hexSlateStrong  = 0xbccad6
	hexSlateInk     = 0x4c6377
)

// Tints lists the tints in the order they are offered and given out.
func Tints() []Tint {
	return []Tint{
		{Name: "green", Wash: Hex(hexGreenWash), Strong: Hex(hexGreenStrong), Ink: Hex(hexGreenInk)},
		{Name: "amber", Wash: Hex(hexAmberWash), Strong: Hex(hexAmberStrong), Ink: Hex(hexAmberInk)},
		{Name: "teal", Wash: Hex(hexTealWash), Strong: Hex(hexTealStrong), Ink: Hex(hexTealInk)},
		{Name: "purple", Wash: Hex(hexPurpleWash), Strong: Hex(hexPurpleStrong), Ink: Hex(hexPurpleInk)},
		{Name: "pink", Wash: Hex(hexPinkWash), Strong: Hex(hexPinkStrong), Ink: Hex(hexPinkInk)},
		{Name: "brown", Wash: Hex(hexBrownWash), Strong: Hex(hexBrownStrong), Ink: Hex(hexBrownInk)},
		{Name: "slate", Wash: Hex(hexSlateWash), Strong: Hex(hexSlateStrong), Ink: Hex(hexSlateInk)},
	}
}
