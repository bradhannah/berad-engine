// Package ui is the editor's small widget kit, in the agreed plain white look:
// buttons, lists, dialogs and a text field. Widgets turn one frame of Input
// into results in Update, which needs no screen, and draw themselves in Draw.
//
// Windows build Input from Ebitengine each frame, or from an input script in
// tests, so every widget can be driven without a person at the keyboard.
package ui

import (
	"image"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// Key names a key, as written in input scripts ("Enter", "Z", "Left").
type Key string

// Keys windows react to.
const (
	KeyEnter     Key = "Enter"
	KeyEscape    Key = "Escape"
	KeyBackspace Key = "Backspace"
	KeyDelete    Key = "Delete"
	KeyTab       Key = "Tab"
	KeySpace     Key = "Space"
	KeyLeft      Key = "Left"
	KeyRight     Key = "Right"
	KeyUp        Key = "Up"
	KeyDown      Key = "Down"
	KeyHome      Key = "Home"
	KeyEnd       Key = "End"
	KeyPageUp    Key = "PageUp"
	KeyPageDown  Key = "PageDown"
	KeyEqual     Key = "Equal"
	KeyMinus     Key = "Minus"
	KeySlash     Key = "Slash"
	KeyA         Key = "A"
	KeyC         Key = "C"
	KeyF         Key = "F"
	KeyJ         Key = "J"
	KeyK         Key = "K"
	KeyN         Key = "N"
	KeyO         Key = "O"
	KeyP         Key = "P"
	KeyQ         Key = "Q"
	KeyS         Key = "S"
	KeyV         Key = "V"
	KeyW         Key = "W"
	KeyX         Key = "X"
	KeyZ         Key = "Z"
	Key0         Key = "0"
	Key1         Key = "1"
	Key9         Key = "9"
)

// Mods are the modifier keys held with a key press.
type Mods = textedit.Mods

// Press is one key press, including repeats while the key is held.
type Press struct {
	Key  Key
	Mods Mods
}

// ParsePress reads a press written as in input scripts: "Cmd+Shift+Z",
// "Ctrl+J" (the physical Control key), "Alt+Left", "Enter".
func ParsePress(value string) Press {
	parts := strings.Split(value, "+")
	press := Press{Key: Key(parts[len(parts)-1]), Mods: Plain()}
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "Cmd":
			press.Mods.Shortcut = true
		case "Shift":
			press.Mods.Shift = true
		case "Alt":
			press.Mods.Alt = true
		case "Ctrl":
			press.Mods.Control = true
		}
	}

	return press
}

// Replacement is an input method commit that replaces text around the caret.
type Replacement struct {
	Before int    `json:"before"`
	After  int    `json:"after"`
	Text   string `json:"text"`
	Caret  int    `json:"caret"`
}

// Plain is no modifier.
func Plain() Mods { return Mods{Shortcut: false, Shift: false, Alt: false, Control: false} }

// Cmd is the shortcut modifier alone: Command on macOS, Control elsewhere.
func Cmd() Mods { return Mods{Shortcut: true, Shift: false, Alt: false, Control: false} }

// CmdShift is the shortcut modifier with Shift.
func CmdShift() Mods { return Mods{Shortcut: true, Shift: true, Alt: false, Control: false} }

// Center returns the middle of a rectangle.
func Center(rect image.Rectangle) image.Point {
	return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
}

// Zero is the empty rectangle, for widgets that are hidden.
func Zero() image.Rectangle { return image.Rect(0, 0, 0, 0) }

// Input is one frame of user input.
type Input struct {
	Cursor image.Point
	// Down is true while the main button is held; Pressed and Released mark
	// the frames it changed. DoubleClick marks a second press close to the
	// first.
	Down, Pressed, Released, DoubleClick bool
	// Secondary marks a right-button press this frame, or on macOS a
	// Control-click, which never also counts as a main-button press.
	Secondary bool
	// PanDown is true while the middle button is held.
	PanDown bool
	// WheelX and WheelY are scroll amounts this frame; trackpads give
	// fractions.
	WheelX, WheelY float64
	// Presses are the keys pressed this frame, with repeats.
	Presses []Press
	// Held are the modifiers held now; Space reports the space bar held.
	Held  Mods
	Space bool
	// Keys are the keys held down now, for actions that last as long as a
	// key is held, such as walking in a preview.
	Keys []Key
	// Text is text typed this frame through the input method.
	Text string
	// Replace is an input method commit that replaces text near the caret.
	Replace *Replacement
	// Composition is the input method's unfinished text, shown at the caret.
	Composition string
}

// NewInput returns a frame of input with the cursor at a point and nothing
// else happening.
func NewInput(cursor image.Point) Input {
	return Input{
		Cursor: cursor, Down: false, Pressed: false, Released: false, DoubleClick: false, Secondary: false, PanDown: false,
		WheelX: 0, WheelY: 0, Presses: nil, Held: Plain(), Space: false, Keys: nil, Text: "", Replace: nil,
		Composition: "",
	}
}

// wheelStep is how far a canvas pans for one step of the scroll wheel, in
// screen pixels.
const wheelStep = 24

// WheelPan returns how far a canvas's offset moves for this frame's
// scrolling, so the Map, Watch, Workflow and Pixel editor all scroll the same
// way. The content moves with the fingers on a trackpad, as macOS scrolls
// pages: a positive WheelY, fingers moving down, moves it down.
func (input *Input) WheelPan() image.Point {
	return image.Pt(int(input.WheelX*wheelStep), int(input.WheelY*wheelStep))
}

// IsDown reports whether a key is held down now.
func (input *Input) IsDown(key Key) bool {
	return slices.Contains(input.Keys, key)
}

// Has reports whether a key was pressed this frame with exactly the given
// Shortcut and Shift modifiers.
func (input *Input) Has(key Key, mods Mods) bool {
	return slices.ContainsFunc(input.Presses, func(press Press) bool {
		return press.Key == key && press.Mods.Shortcut == mods.Shortcut && press.Mods.Shift == mods.Shift
	})
}

// Consume removes a key press so nothing else reacts to it.
func (input *Input) Consume(key Key, mods Mods) bool {
	index := slices.IndexFunc(input.Presses, func(press Press) bool {
		return press.Key == key && press.Mods.Shortcut == mods.Shortcut && press.Mods.Shift == mods.Shift
	})
	if index < 0 {
		return false
	}
	input.Presses = slices.Delete(input.Presses, index, index+1)

	return true
}

// ClearPointer removes the button presses, so widgets underneath a dialog or
// a menu do not react to them.
func (input *Input) ClearPointer() {
	input.Pressed, input.Released, input.DoubleClick = false, false, false
	input.Down, input.Secondary = false, false
}

// ClearAll removes every key, text and button event.
func (input *Input) ClearAll() {
	input.ClearPointer()
	input.Presses = nil
	input.Keys = nil
	input.Text = ""
	input.Replace = nil
	input.WheelX, input.WheelY = 0, 0
}
