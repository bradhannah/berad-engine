package window

import (
	"image"
	"runtime"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Key repeat, in ticks, and double-click limits.
const (
	repeatDelay    = 30
	repeatInterval = 3
	doubleClick    = 400 * time.Millisecond
	doubleDistance = 4
)

// keyNames maps Ebitengine keys to the names views and input scripts use.
func keyNames() map[ebiten.Key]ui.Key {
	names := map[ebiten.Key]ui.Key{
		ebiten.KeyEnter: ui.KeyEnter, ebiten.KeyNumpadEnter: ui.KeyEnter, ebiten.KeyEscape: ui.KeyEscape,
		ebiten.KeyBackspace: ui.KeyBackspace, ebiten.KeyDelete: ui.KeyDelete, ebiten.KeyTab: ui.KeyTab,
		ebiten.KeySpace: ui.KeySpace, ebiten.KeyArrowLeft: ui.KeyLeft, ebiten.KeyArrowRight: ui.KeyRight,
		ebiten.KeyArrowUp: ui.KeyUp, ebiten.KeyArrowDown: ui.KeyDown, ebiten.KeyHome: ui.KeyHome,
		ebiten.KeyEnd: ui.KeyEnd, ebiten.KeyPageUp: ui.KeyPageUp, ebiten.KeyPageDown: ui.KeyPageDown,
		ebiten.KeyEqual: ui.KeyEqual, ebiten.KeyMinus: ui.KeyMinus,
	}
	for offset := range 26 {
		names[ebiten.KeyA+ebiten.Key(offset)] = ui.Key(string(rune('A' + offset)))
	}
	for offset := range 10 {
		names[ebiten.KeyDigit0+ebiten.Key(offset)] = ui.Key(string(rune('0' + offset)))
	}

	return names
}

// inputReader builds each frame's ui.Input from Ebitengine, running the input
// method (IME) for the focused text field so typed text, accents from
// press-and-hold and composed characters arrive as text.
type inputReader struct {
	names     map[ebiten.Key]ui.Key
	composer  textinput.Composer
	field     *ui.TextField
	before    string
	after     string
	text      strings.Builder
	replace   *ui.Replacement
	compose   string
	lastPress time.Time
	lastPoint image.Point
	// controlClick is true from a Control-click on macOS until its release:
	// the whole click is a right-click.
	controlClick bool
}

func newInputReader() *inputReader {
	reader := &inputReader{
		names: keyNames(), composer: newComposer(), field: nil, before: "", after: "",
		text: strings.Builder{}, replace: nil, compose: "", lastPress: time.Time{}, lastPoint: image.Pt(0, 0),
		controlClick: false,
	}
	reader.composer.OnNewSession = reader.newSession
	reader.composer.OnComposition = func(composition *textinput.Composition) { reader.compose = composition.Text() }
	reader.composer.OnCommit = reader.commit
	reader.composer.OnEndByUser = func() {}

	return reader
}

func newComposer() textinput.Composer {
	return textinput.Composer{OnNewSession: nil, OnComposition: nil, OnCommit: nil, OnEndByUser: nil}
}

// newSession tells the input method where the caret is and what surrounds it.
func (r *inputReader) newSession() *textinput.SessionOptions {
	if r.field == nil {
		return nil
	}
	caret, before, after := r.field.Session()
	r.before, r.after = before, after

	return &textinput.SessionOptions{CaretBounds: caret, TextBeforeCaret: before, TextAfterCaret: after}
}

// commit queues committed text, or a replacement of the text around the
// caret, as press-and-hold accents produce.
func (r *inputReader) commit(commit *textinput.Commit) {
	if before, after := commit.IsSurroundingTextReplaced(); before || after {
		prefix, suffix := commit.SurroundingText()
		r.flushText()
		r.replace = &ui.Replacement{
			Before: len(r.before), After: len(r.after),
			Text: prefix + commit.Text() + suffix, Caret: len(prefix) + len(commit.Text()),
		}

		return
	}
	r.text.WriteString(commit.Text())
}

// flushText keeps typed text ahead of a replacement in the same frame by
// folding it into the replacement's surroundings.
func (r *inputReader) flushText() {
	if r.text.Len() == 0 {
		return
	}
	r.before += r.text.String()
	r.text.Reset()
}

// read returns this frame's input. focused is the view's focused field.
func (r *inputReader) read(focused *ui.TextField) ui.Input {
	input := ui.NewInput(image.Pt(ebiten.CursorPosition()))
	input.Down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	input.Pressed = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	input.Released = inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	input.PanDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	input.WheelX, input.WheelY = ebiten.Wheel()
	input.Held = heldMods()
	input.Space = ebiten.IsKeyPressed(ebiten.KeySpace)
	r.readSecondary(&input)
	if input.Pressed {
		now := time.Now()
		distance := input.Cursor.Sub(r.lastPoint)
		input.DoubleClick = now.Sub(r.lastPress) < doubleClick &&
			abs(distance.X) <= doubleDistance && abs(distance.Y) <= doubleDistance
		r.lastPress, r.lastPoint = now, input.Cursor
		if input.DoubleClick {
			r.lastPress = time.Time{}
		}
	}
	for key, name := range r.names {
		duration := inpututil.KeyPressDuration(key)
		if duration == 1 || (duration >= repeatDelay && (duration-repeatDelay)%repeatInterval == 0) {
			input.Presses = append(input.Presses, ui.Press{Key: name, Mods: input.Held})
		}
		if duration > 0 {
			input.Keys = append(input.Keys, name)
		}
	}
	r.runInputMethod(&input, focused)

	return input
}

// macOS is runtime.GOOS on a Mac.
const macOS = "darwin"

// readSecondary marks a right-button press. On macOS a Control-click is a
// right-click too, so its press, hold and release never reach the main
// button.
func (r *inputReader) readSecondary(input *ui.Input) {
	input.Secondary = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if runtime.GOOS == macOS && input.Pressed && input.Held.Control {
		r.controlClick, input.Secondary = true, true
	}
	if !r.controlClick {
		return
	}
	if input.Released || !input.Down {
		r.controlClick = false
	}
	input.Pressed, input.Down, input.Released = false, false, false
}

// runInputMethod feeds the focused field's input method. While the input
// method is composing it owns the keyboard; otherwise any live session is
// confirmed before keys edit the text, so its idea of the caret stays right.
func (r *inputReader) runInputMethod(input *ui.Input, focused *ui.TextField) {
	if focused != r.field {
		r.composer.Confirm()
		r.field = focused
	}
	if r.field == nil {
		r.take(input)

		return
	}
	handled, err := r.composer.Update()
	if err != nil || handled {
		input.Presses = nil
		r.take(input)

		return
	}
	if len(input.Presses) > 0 {
		r.composer.Confirm()
	}
	r.take(input)
}

// take moves queued text into the frame's input. Typing a space while a text
// field has focus is text, not a key.
func (r *inputReader) take(input *ui.Input) {
	input.Text = r.text.String()
	input.Replace = r.replace
	input.Composition = r.compose
	r.text.Reset()
	r.replace = nil
	if r.field != nil {
		kept := input.Presses[:0]
		for _, press := range input.Presses {
			if press.Key != ui.KeySpace {
				kept = append(kept, press)
			}
		}
		input.Presses = kept
	}
}

// heldMods returns the modifiers held now. The shortcut modifier is Command
// on macOS and Control elsewhere.
func heldMods() ui.Mods {
	meta := ebiten.IsKeyPressed(ebiten.KeyMeta)
	control := ebiten.IsKeyPressed(ebiten.KeyControl)
	shortcut := meta
	if runtime.GOOS != macOS {
		shortcut = control
	}

	return ui.Mods{
		Shortcut: shortcut,
		Shift:    ebiten.IsKeyPressed(ebiten.KeyShift),
		Alt:      ebiten.IsKeyPressed(ebiten.KeyAlt),
		Control:  control,
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
