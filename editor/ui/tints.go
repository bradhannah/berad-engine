package ui

import "image/color"

// Tint is one of the colours that tell things apart: those an author gives
// a node or a line to keep a workflow organised, and those that tell open
// tabs of one kind apart. Each has a wash for a background and a darker ink
// for a line or a mark. There is no blue, which is selection, and no red,
// which is problems.
type Tint struct {
	Name string
	Wash color.RGBA
	Ink  color.RGBA
}

// The tints' colours: the editor's family hues, and amber.
const (
	hexGreenWash  = 0xe3f0ec
	hexGreenInk   = 0x3d7a4b
	hexAmberWash  = 0xfdf1dc
	hexAmberInk   = 0x9a6700
	hexTealWash   = 0xdff2f3
	hexTealInk    = 0x267378
	hexPurpleWash = 0xece6f5
	hexPurpleInk  = 0x6a4f9e
	hexPinkWash   = 0xf7e4ef
	hexPinkInk    = 0x8a4b72
	hexBrownWash  = 0xf1e9e1
	hexBrownInk   = 0x7a6248
	hexSlateWash  = 0xe6ebef
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
