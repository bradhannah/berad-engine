package textedit

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

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
	// OutcomeBlur: leave the field (Escape, keeping what was typed).
	OutcomeBlur
	// OutcomePass: the field did not use the key; the window may.
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
	breaks    bool
	// limit is how many characters the field holds, or 0 for no limit, and
	// refusal says why the last typing or paste was refused for it, until
	// taken.
	limit   int
	refusal string
	// tab is the way Tab last moved, for the window's fields to take.
	tab      int
	conflict bool
	column   *float64
}

// NewField returns a field showing a saved value, holding at most limit
// characters, or any number when limit is 0. In a multi-line field Enter
// starts a new line (as do Cmd+Enter and Ctrl+J); in a one-line field Enter
// saves.
func NewField(saved string, multiline bool, limit int) *Field {
	return &Field{
		saved: saved, draft: nil, pending: nil, multiline: multiline, breaks: false, limit: limit, refusal: "",
		conflict: false, column: nil, tab: 0,
	}
}

// TakeRefusal returns why the field refused what was last typed or pasted
// into it, once: the window says it in its status bar. It is "" when
// nothing was refused.
func (f *Field) TakeRefusal() string {
	why := f.refusal
	f.refusal = ""

	return why
}

// KeepBreaks lets a one-line field hold line breaks: Enter still saves it,
// Shift+Enter starts a new line, and line breaks typed or pasted stay.
func (f *Field) KeepBreaks() { f.breaks = true }

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

// SetDraft replaces the draft with a value chosen another way than typing,
// such as with a colour picker. It waits for Save as typing does.
func (f *Field) SetDraft(value string) {
	f.draft = NewBuffer(value)
	f.column = nil
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

// SelectFrom starts editing, if it has not, with the text from a byte
// offset to its end selected, so typing replaces that part.
func (f *Field) SelectFrom(start int) {
	f.Focus()
	f.draft.Select(start, len(f.draft.text))
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

// Type inserts typed text. A one-line field drops line breaks, unless it
// keeps them.
func (f *Field) Type(value string) {
	if f.draft == nil || value == "" {
		return
	}
	switch {
	case f.breaks:
		value = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(value)
	case !f.multiline:
		value = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(value)
	}
	f.insert(value)
}

// insert puts text in place of the selection. Text that would take the
// field past the characters it holds is refused whole, not cut to fit, and
// the field keeps why for the window to say.
func (f *Field) insert(value string) {
	text := f.draft.Text()
	start, end := f.draft.Selection()
	total := utf8.RuneCountInString(text) - utf8.RuneCountInString(text[start:end]) + utf8.RuneCountInString(value)
	if f.limit > 0 && total > f.limit {
		f.refusal = fmt.Sprintf("Too long: this field holds %d characters, and that would make %d", f.limit, total)

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
	switch press.Key {
	case KeyEnter, KeyJ:
		return f.newLine(press)
	case KeyEscape:
		// A stray Esc changes nothing (increment 59): it leaves the field,
		// and what was typed waits for Save, as a click elsewhere leaves it.
		if f.Dirty() {
			f.refusal = "Left the field. What you typed waits for Save."
		}

		return OutcomeBlur
	case KeyTab:
		f.tab = 1
		if press.Mods.Shift {
			f.tab = -1
		}

		return OutcomeNone
	case KeyNone, KeyBackspace, KeyDelete, KeyLeft, KeyRight, KeyUp, KeyDown, KeyHome, KeyEnd:
		return f.edit(press, lines)
	case KeyA, KeyC, KeyX, KeyV, KeyZ:
		return f.command(press, clipboard)
	default:
		return OutcomePass
	}
}

// newLine handles Enter and Ctrl+J. Enter starts a new line in a field of
// several lines, as Shift+Enter does in a one-line field that keeps line
// breaks, and otherwise saves; Ctrl+J starts one in a field of several
// lines.
func (f *Field) newLine(press Press) Outcome {
	mods := press.Mods
	if press.Key == KeyJ {
		if mods.Control && !mods.Shortcut && f.multiline {
			f.insert("\n")
		}

		return OutcomeNone
	}
	if f.multiline || f.breaks && mods.Shift {
		f.insert("\n")

		return OutcomeNone
	}

	return OutcomeCommit
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

// undo undoes typing in the draft. With none left it stops at the field and
// says so, so it never takes back a saved change unseen (increment 59).
func (f *Field) undo(redo bool) Outcome {
	if redo {
		if !f.draft.Redo() {
			f.refusal = "Nothing more to redo in this field"
		}

		return OutcomeNone
	}
	if !f.draft.Undo() {
		f.refusal = "Nothing more to undo in this field. Esc leaves it, to undo saved changes."
	}

	return OutcomeNone
}

// TakeTab is the way Tab moved, once: 1 for Tab, -1 for Shift+Tab, 0 for
// none.
func (f *Field) TakeTab() int {
	step := f.tab
	f.tab = 0

	return step
}

// Say keeps words for the window to say about the field, as it says why
// the field refused what was typed.
func (f *Field) Say(words string) { f.refusal = words }

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
