package textedit_test

import (
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

type memoryClipboard struct{ value string }

func (m *memoryClipboard) Read() (string, error) { return m.value, nil }

func (m *memoryClipboard) Write(value string) error {
	m.value = value
	return nil
}

type brokenClipboard struct{}

func (brokenClipboard) Read() (string, error) { return "", errors.New("no clipboard") }
func (brokenClipboard) Write(string) error    { return errors.New("no clipboard") }

func press(field *textedit.Field, key textedit.Key, mods textedit.Mods, clipboard textedit.Clipboard) textedit.Outcome {
	lines := textedit.HardLines{Buffer: field.Buffer()}
	if field.Buffer() == nil {
		field.Focus()
		lines.Buffer = field.Buffer()
	}
	return field.Key(textedit.Press{Key: key, Mods: mods}, lines, clipboard)
}

var (
	none     = textedit.Mods{}
	shift    = textedit.Mods{Shift: true}
	alt      = textedit.Mods{Alt: true}
	command  = textedit.Mods{Shortcut: true}
	control  = textedit.Mods{Control: true}
	shiftCmd = textedit.Mods{Shortcut: true, Shift: true}
)

func TestTypingAndGraphemes(t *testing.T) {
	t.Parallel()
	buffer := textedit.NewBuffer("")
	buffer.Insert("café ") // e + combining acute: one grapheme
	buffer.Insert("\U0001F44D\U0001F3FD")
	if buffer.Text() != "café \U0001F44D\U0001F3FD" {
		t.Fatalf("text %q", buffer.Text())
	}
	buffer.DeleteBackward(false)
	if buffer.Text() != "café " {
		t.Fatalf("emoji with skin tone is one grapheme: %q", buffer.Text())
	}
	buffer.MoveLeft(false, false)
	buffer.MoveLeft(false, false)
	if buffer.Caret() != len("caf") {
		t.Fatalf("caret %d skipped into the accent", buffer.Caret())
	}
	buffer.SetCaret(len("cafe"), false) // inside the cluster: snaps back
	if buffer.Caret() != len("caf") {
		t.Fatalf("snap %d", buffer.Caret())
	}
}

func TestWordsAndSelection(t *testing.T) {
	t.Parallel()
	buffer := textedit.NewBuffer("go to the desk.")
	buffer.MoveLeft(false, true)
	if buffer.Caret() != len("go to the ") {
		t.Fatalf("word left %d", buffer.Caret())
	}
	buffer.MoveLeft(true, true)
	if buffer.SelectedText() != "the " {
		t.Fatalf("selected %q", buffer.SelectedText())
	}
	buffer.Insert("a ")
	if buffer.Text() != "go to a desk." {
		t.Fatalf("replace selection %q", buffer.Text())
	}
	buffer.SelectWord(1)
	if buffer.SelectedText() != "go" {
		t.Fatalf("select word %q", buffer.SelectedText())
	}
	buffer.SetCaret(0, false)
	buffer.DeleteForward(true)
	if buffer.Text() != " to a desk." {
		t.Fatalf("delete word %q", buffer.Text())
	}
}

// TestPressAndHoldAccent replays what macOS does when a key is held: the plain
// letter is inserted, then replaced by the chosen accented letter.
func TestPressAndHoldAccent(t *testing.T) {
	t.Parallel()
	field := textedit.NewField("", false, 0)
	field.Focus()
	field.Type("caf")
	field.Type("e")
	field.ReplaceAround(len("e"), 0, "é", len("é"))
	field.Type("!")
	if field.Value() != "café!" {
		t.Fatalf("value %q", field.Value())
	}
}

func TestUndoGroupsTyping(t *testing.T) {
	t.Parallel()
	buffer := textedit.NewBuffer("")
	for _, letter := range []string{"h", "e", "l", "l", "o", " ", "y", "o", "u"} {
		buffer.Insert(letter)
	}
	buffer.Undo()
	if buffer.Text() != "hello " {
		t.Fatalf("after one undo %q", buffer.Text())
	}
	buffer.Undo()
	buffer.Undo()
	if buffer.Text() != "" || buffer.CanUndo() {
		t.Fatalf("after undoing everything %q", buffer.Text())
	}
	buffer.Redo()
	if buffer.Text() != "hello" {
		t.Fatalf("redo %q", buffer.Text())
	}
}

// TestEnterSavesAndNewLineKeys checks the keys Brad chose: in a multi-line
// field Enter, Cmd+Enter and Ctrl+J start a new line; in a one-line field
// Enter saves.
func TestEnterSavesAndNewLineKeys(t *testing.T) {
	t.Parallel()
	field := textedit.NewField("", true, 0)
	field.Focus()
	field.Type("one")
	if got := press(field, textedit.KeyEnter, command, nil); got != textedit.OutcomeNone {
		t.Fatalf("Cmd+Enter: %v", got)
	}
	field.Type("two")
	press(field, textedit.KeyJ, control, nil)
	field.Type("three")
	if got := press(field, textedit.KeyEnter, none, nil); got != textedit.OutcomeNone {
		t.Fatalf("Enter in a multi-line field: %v", got)
	}
	field.Type("four")
	if field.Value() != "one\ntwo\nthree\nfour" {
		t.Fatalf("value %q", field.Value())
	}
	single := textedit.NewField("Name", false, 0)
	single.Focus()
	if got := press(single, textedit.KeyEnter, none, nil); got != textedit.OutcomeCommit {
		t.Fatalf("Enter in a one-line field: %v", got)
	}
	press(single, textedit.KeyEnter, command, nil)
	press(single, textedit.KeyJ, control, nil)
	single.Type("\nmore")
	if single.Value() != "Name more" {
		t.Fatalf("single-line field took a line break: %q", single.Value())
	}
}

// TestDraftLifecycle follows strict updates: the draft stays until the
// document shows the saved value.
func TestDraftLifecycle(t *testing.T) {
	t.Parallel()
	field := textedit.NewField("old", true, 0)
	field.Focus()
	field.Type(" new")
	if !field.Unsaved() {
		t.Fatal("a typed change is unsaved")
	}
	value, ok := field.Commit()
	if !ok || value != "old new" || !field.Pending() {
		t.Fatalf("commit %q %v", value, ok)
	}
	if field.Unsaved() || !field.Dirty() {
		t.Fatal("a change on its way is dirty but no longer unsaved")
	}
	if field.Value() != "old new" {
		t.Fatal("draft disappeared before the event")
	}
	field.SetSaved("old new")
	if field.Editing() || field.Value() != "old new" || field.Pending() {
		t.Fatalf("after the event: editing %v value %q", field.Editing(), field.Value())
	}

	field.Focus()
	field.Type("!")
	field.Blur()
	if !field.Dirty() || field.Value() != "old new!" {
		t.Fatal("blur lost the unsaved draft")
	}
	field.SetSaved("changed elsewhere")
	if !field.Conflict() || field.Value() != "old new!" {
		t.Fatal("a change elsewhere replaced the draft")
	}
	if got := press(field, textedit.KeyEscape, none, nil); got != textedit.OutcomeNone || field.Value() != "changed elsewhere" {
		t.Fatalf("Escape reverts: %v %q", got, field.Value())
	}
	if got := press(field, textedit.KeyEscape, none, nil); got != textedit.OutcomeBlur {
		t.Fatalf("Escape with nothing to revert leaves the field: %v", got)
	}

	field.Focus()
	field.Type("x")
	if _, ok := field.Commit(); !ok {
		t.Fatal("no commit")
	}
	field.Failed()
	if field.Pending() || field.Value() != "changed elsewherex" {
		t.Fatal("a refused save lost the draft")
	}
	field.Revert()
	if _, ok := field.Commit(); ok {
		t.Fatal("nothing to save after revert")
	}
}

func TestClipboardAndUndoKeys(t *testing.T) {
	t.Parallel()
	clipboard := &memoryClipboard{value: ""}
	field := textedit.NewField("copy me", false, 0)
	press(field, textedit.KeyA, command, clipboard)
	press(field, textedit.KeyX, command, clipboard)
	if field.Value() != "" || clipboard.value != "copy me" {
		t.Fatalf("cut: %q %q", field.Value(), clipboard.value)
	}
	press(field, textedit.KeyV, command, clipboard)
	press(field, textedit.KeyV, command, clipboard)
	if field.Value() != "copy mecopy me" {
		t.Fatalf("paste: %q", field.Value())
	}
	if got := press(field, textedit.KeyC, command, brokenClipboard{}); got != textedit.OutcomeNone {
		t.Fatalf("broken clipboard: %v", got)
	}
	press(field, textedit.KeyZ, command, clipboard)
	press(field, textedit.KeyZ, command, clipboard)
	press(field, textedit.KeyZ, command, clipboard)
	if got := press(field, textedit.KeyZ, command, clipboard); got != textedit.OutcomePass {
		t.Fatalf("undo with no typing left passes to the window: %v %q", got, field.Value())
	}
	if got := press(field, textedit.KeyZ, shiftCmd, clipboard); got != textedit.OutcomeNone {
		t.Fatalf("redo: %v", got)
	}
}

func TestCaretMovementKeys(t *testing.T) {
	t.Parallel()
	field := textedit.NewField("first line\nsecond", true, 0)
	field.Focus()
	press(field, textedit.KeyUp, none, nil)
	if caret := field.Buffer().Caret(); caret != len("first ") {
		t.Fatalf("up keeps the column: %d", caret)
	}
	press(field, textedit.KeyEnd, shift, nil)
	if field.Buffer().SelectedText() != "line" {
		t.Fatalf("shift+end selected %q", field.Buffer().SelectedText())
	}
	press(field, textedit.KeyLeft, command, nil)
	if field.Buffer().Caret() != 0 {
		t.Fatalf("cmd+left %d", field.Buffer().Caret())
	}
	press(field, textedit.KeyDown, command, nil)
	if field.Buffer().Caret() != len(field.Value()) {
		t.Fatal("cmd+down goes to the end")
	}
	press(field, textedit.KeyBackspace, alt, nil)
	if field.Value() != "first line\n" {
		t.Fatalf("option+backspace %q", field.Value())
	}
	press(field, textedit.KeyBackspace, none, nil)
	press(field, textedit.KeyBackspace, command, nil)
	if field.Value() != "" {
		t.Fatalf("cmd+backspace %q", field.Value())
	}
}

func TestMaxBytes(t *testing.T) {
	t.Parallel()
	field := textedit.NewField("", false, 5)
	field.Focus()
	field.Type("abc")
	field.Type("def")
	field.Type("de")
	if field.Value() != "abcde" {
		t.Fatalf("value %q", field.Value())
	}
}

// TestRestoreKeepsADraftAcrossARebuild puts a draft back into a new field.
func TestRestoreKeepsADraftAcrossARebuild(t *testing.T) {
	t.Parallel()
	field := textedit.NewField("2", false, 0)
	field.Restore("2")
	if field.Editing() {
		t.Fatal("restoring the saved value starts no draft")
	}
	field.Restore("5")
	if !field.Unsaved() || field.Value() != "5" {
		t.Fatalf("restored draft: %q", field.Value())
	}
}
