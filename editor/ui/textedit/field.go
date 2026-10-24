package textedit

import "strings"

// Key is a key the field responds to.
type Key int

// Keys the field handles.
const (
	KeyNone Key = iota
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyDelete
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyA
	KeyC
	KeyX
	KeyV
	KeyZ
	KeyJ
	KeyTab
)

// Mods are the modifier keys held with a key. Shortcut is Command on macOS
// and Control elsewhere; Control is the physical Control key.
type Mods struct {
	Shortcut bool
	Shift    bool
	Alt      bool
	Control  bool
}

// Press is one key press, including repeats while the key is held.
type Press struct {
	Key  Key
	Mods Mods
}

// Outcome tells the window what a key press means beyond editing the draft.
type Outcome int

// Outcomes of a key press.
const (
	// OutcomeNone: the field handled the key, or ignored it.
	OutcomeNone Outcome = iota
	// OutcomeCommit: save the draft (Enter).
	OutcomeCommit
	// OutcomeBlur: leave the field (Escape with nothing to revert, or Tab).
	OutcomeBlur
	// OutcomePass: the field did not use the key; the window may (for
	// example Undo with no typing left to undo undoes the last saved change).
	OutcomePass
)

// Clipboard reads and writes the system clipboard.
type Clipboard interface {
	Read() (string, error)
	Write(value string) error
}

// Lines answers questions about how the text is laid out on screen, which
// only the widget drawing it knows: where visual lines start and end, and
// which offset lies above or below another.
type Lines interface {
	// LineBounds returns the visual line holding an offset.
	LineBounds(offset int) (int, int)
	// Vertical returns the offset delta lines away from offset, nearest the
	// horizontal position x; it sets x on first use so repeated moves keep a
	// column.
	Vertical(offset, delta int, x *float64) int
}

// Field is a text field that keeps a draft. The saved value comes from the
// document; typing changes only the draft, and Save, Cmd+S or Enter in a
// one-line field asks the window to save it with an ordinary command. The draft stays until the document shows the
// saved value, so the field never flickers back to old text (D1).
type Field struct {
	saved     string
	draft     *Buffer
	pending   *string
	multiline bool
	maxBytes  int
	conflict  bool
	column    *float64
}

// NewField returns a field showing a saved value. In a multi-line field Enter
// starts a new line (as do Cmd+Enter and Ctrl+J); in a one-line field Enter
// saves.
func NewField(saved string, multiline bool, maxBytes int) *Field {
	return &Field{
		saved: saved, draft: nil, pending: nil, multiline: multiline, maxBytes: maxBytes, conflict: false, column: nil,
	}
}

// Value returns what the field shows: the draft, or the saved value.
func (f *Field) Value() string {
	if f.draft != nil {
		return f.draft.Text()
	}

	return f.saved
}

// Saved returns the saved value.
func (f *Field) Saved() string { return f.saved }

// Editing reports whether the field holds a draft.
func (f *Field) Editing() bool { return f.draft != nil }

// Dirty reports whether the draft differs from the saved value.
func (f *Field) Dirty() bool { return f.draft != nil && f.draft.Text() != f.saved }

// Unsaved reports whether the draft holds changes no save has been asked for
// yet: dirty, and not the value already on its way.
func (f *Field) Unsaved() bool {
	return f.Dirty() && (f.pending == nil || *f.pending != f.draft.Text())
}

// Restore puts back a draft kept from before the field was rebuilt, for
// example when a form gains a row. It does nothing when the draft is the
// saved value.
func (f *Field) Restore(draft string) {
	if draft != f.saved {
		f.draft = NewBuffer(draft)
	}
}

// Conflict reports whether the saved value changed elsewhere while this field
// held a draft. Saving the draft replaces that change.
func (f *Field) Conflict() bool { return f.conflict && f.Dirty() }

// Pending reports whether a save was asked for and its event has not arrived.
func (f *Field) Pending() bool { return f.pending != nil }

// Buffer returns the draft, or nil when not editing.
func (f *Field) Buffer() *Buffer { return f.draft }

// SetSaved updates the saved value from the document. Without a draft the
// field shows it; with one, the draft is kept, unless it is the value just
// saved, which ends editing.
func (f *Field) SetSaved(value string) {
	if f.pending != nil && *f.pending == value {
		f.pending = nil
		f.saved = value
		if f.draft != nil && f.draft.Text() == value {
			f.draft = nil
		}
		f.conflict = false

		return
	}
	if f.draft != nil && value != f.saved {
		f.conflict = true
	}
	f.saved = value
}

// Focus starts editing, keeping an existing draft.
func (f *Field) Focus() {
	if f.draft == nil {
		f.draft = NewBuffer(f.saved)
	}
}

// Blur stops editing. An unsaved draft is kept and shown as unsaved.
func (f *Field) Blur() {
	if f.draft != nil && !f.Dirty() && f.pending == nil {
		f.draft = nil
	}
	f.column = nil
}

// Revert drops the draft and shows the saved value.
func (f *Field) Revert() {
	f.draft = nil
	f.pending = nil
	f.conflict = false
	f.column = nil
}

// Commit returns the draft to save, if it differs from the saved value. The
// window sends it as a command; the draft stays until its event arrives.
func (f *Field) Commit() (string, bool) {
	if !f.Dirty() {
		if f.draft != nil && f.pending == nil {
			f.draft = nil
		}

		return "", false
	}
	value := f.draft.Text()
	f.pending = &value

	return value, true
}

// Failed reports that a save was refused, so the draft stays editable.
func (f *Field) Failed() { f.pending = nil }

// Type inserts typed text. Single-line fields drop line breaks.
func (f *Field) Type(value string) {
	if f.draft == nil || value == "" {
		return
	}
	if !f.multiline {
		value = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(value)
	}
	f.insert(value)
}

func (f *Field) insert(value string) {
	start, end := f.draft.Selection()
	if room := f.maxBytes - (len(f.draft.Text()) - (end - start)); f.maxBytes > 0 && len(value) > room {
		return
	}
	f.draft.Insert(value)
	f.column = nil
}

// ReplaceAround applies an input method commit that replaces text around the
// caret.
func (f *Field) ReplaceAround(before, after int, replacement string, caretInReplacement int) {
	if f.draft == nil {
		return
	}
	f.draft.ReplaceAround(before, after, replacement, caretInReplacement)
	f.column = nil
}

// Key handles one key press while the field has focus.
func (f *Field) Key(press Press, lines Lines, clipboard Clipboard) Outcome {
	if f.draft == nil {
		f.Focus()
	}
	mods := press.Mods
	switch press.Key {
	case KeyEnter:
		if f.multiline {
			f.insert("\n")

			return OutcomeNone
		}

		return OutcomeCommit
	case KeyJ:
		if mods.Control && !mods.Shortcut && f.multiline {
			f.insert("\n")
		}

		return OutcomeNone
	case KeyEscape:
		if f.Dirty() {
			f.Revert()

			return OutcomeNone
		}

		return OutcomeBlur
	case KeyTab:
		return OutcomeBlur
	case KeyNone, KeyBackspace, KeyDelete, KeyLeft, KeyRight, KeyUp, KeyDown, KeyHome, KeyEnd:
		return f.edit(press, lines)
	case KeyA, KeyC, KeyX, KeyV, KeyZ:
		return f.command(press, clipboard)
	default:
		return OutcomePass
	}
}

// edit handles deleting and moving the caret.
func (f *Field) edit(press Press, lines Lines) Outcome {
	buffer, mods := f.draft, press.Mods
	start, end := lines.LineBounds(buffer.Caret())
	keepColumn := false
	switch press.Key {
	case KeyBackspace:
		if mods.Shortcut {
			buffer.DeleteToLineStart(start)
		} else {
			buffer.DeleteBackward(mods.Alt)
		}
	case KeyDelete:
		buffer.DeleteForward(mods.Alt)
	case KeyLeft:
		if mods.Shortcut {
			buffer.SetCaret(start, mods.Shift)
		} else {
			buffer.MoveLeft(mods.Shift, mods.Alt)
		}
	case KeyRight:
		if mods.Shortcut {
			buffer.SetCaret(end, mods.Shift)
		} else {
			buffer.MoveRight(mods.Shift, mods.Alt)
		}
	case KeyHome:
		buffer.SetCaret(start, mods.Shift)
	case KeyEnd:
		buffer.SetCaret(end, mods.Shift)
	case KeyUp, KeyDown:
		keepColumn = true
		f.vertical(press, lines)
	case KeyNone, KeyEnter, KeyEscape, KeyA, KeyC, KeyX, KeyV, KeyZ, KeyJ, KeyTab:
		return OutcomePass
	}
	if !keepColumn {
		f.column = nil
	}

	return OutcomeNone
}

func (f *Field) vertical(press Press, lines Lines) {
	buffer := f.draft
	delta := 1
	if press.Key == KeyUp {
		delta = -1
	}
	switch {
	case press.Mods.Shortcut && delta < 0:
		buffer.SetCaret(0, press.Mods.Shift)
	case press.Mods.Shortcut:
		buffer.SetCaret(len(buffer.Text()), press.Mods.Shift)
	default:
		if f.column == nil {
			var column float64
			f.column = &column
			*f.column = -1
		}
		buffer.SetCaret(lines.Vertical(buffer.Caret(), delta, f.column), press.Mods.Shift)
	}
}

// command handles select all, the clipboard and undo.
func (f *Field) command(press Press, clipboard Clipboard) Outcome {
	buffer, mods := f.draft, press.Mods
	if !mods.Shortcut {
		return OutcomeNone
	}
	switch press.Key {
	case KeyA:
		buffer.SelectAll()
	case KeyC, KeyX:
		if !buffer.HasSelection() || clipboard == nil {
			return OutcomeNone
		}
		if err := clipboard.Write(buffer.SelectedText()); err != nil {
			return OutcomeNone
		}
		if press.Key == KeyX {
			buffer.DeleteBackward(false)
		}
	case KeyV:
		if clipboard == nil {
			return OutcomeNone
		}
		if pasted, err := clipboard.Read(); err == nil {
			f.Type(pasted)
		}
	case KeyZ:
		return f.undo(mods.Shift)
	case KeyNone, KeyEnter, KeyEscape, KeyBackspace, KeyDelete, KeyLeft, KeyRight, KeyUp, KeyDown,
		KeyHome, KeyEnd, KeyJ, KeyTab:
		return OutcomePass
	}
	f.column = nil

	return OutcomeNone
}

// undo undoes typing in the draft; with none left, the window undoes its last
// saved change instead.
func (f *Field) undo(redo bool) Outcome {
	if redo {
		if f.draft.Redo() {
			return OutcomeNone
		}

		return OutcomePass
	}
	if f.draft.Undo() {
		return OutcomeNone
	}

	return OutcomePass
}

// HardLines lays text out as its hard lines only, for fields that do not wrap
// and for tests.
type HardLines struct {
	Buffer *Buffer
}

// LineBounds returns the hard line holding an offset.
func (h HardLines) LineBounds(offset int) (int, int) {
	return h.Buffer.LineStart(offset), h.Buffer.LineEnd(offset)
}

// Vertical moves by hard lines, keeping a column counted in bytes.
func (h HardLines) Vertical(offset, delta int, column *float64) int {
	start := h.Buffer.LineStart(offset)
	if *column < 0 {
		*column = float64(offset - start)
	}
	text := h.Buffer.Text()
	var target int
	switch {
	case delta < 0 && start == 0:
		return 0
	case delta < 0:
		target = h.Buffer.LineStart(start - 1)
	default:
		end := h.Buffer.LineEnd(offset)
		if end == len(text) {
			return len(text)
		}
		target = end + 1
	}

	return min(target+int(*column), h.Buffer.LineEnd(target))
}
