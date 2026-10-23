package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

func theme(t *testing.T) *ui.Theme {
	t.Helper()
	loaded, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(loaded.Close)
	return loaded
}

func frame(presses ...string) *ui.Input {
	in := &ui.Input{}
	for _, press := range presses {
		in.Presses = append(in.Presses, ui.ParsePress(press))
	}
	return in
}

// TestFieldTypingAndSaving drives a multi-line field the way a window does:
// click into it, type, add a new line with Cmd+Enter, and press Enter to save.
func TestFieldTypingAndSaving(t *testing.T) {
	t.Parallel()
	field := ui.NewTextField("body", textedit.NewField("", true, 0), true, theme(t))
	field.Rect = image.Rect(10, 10, 210, 110)
	var focus ui.Focus
	focus.Register(field)

	click := &ui.Input{Cursor: image.Pt(50, 30), Pressed: true, Down: true}
	focus.Update(click, nil)
	if !field.Focused() {
		t.Fatal("click did not focus the field")
	}
	typed := frame()
	typed.Text = "first"
	focus.Update(typed, nil)
	focus.Update(frame("Cmd+Enter"), nil)
	typed = frame()
	typed.Text = "second"
	focus.Update(typed, nil)
	acted, outcome := focus.Update(frame("Enter"), nil)
	if acted != field || outcome != textedit.OutcomeCommit {
		t.Fatalf("Enter: %v %v", acted, outcome)
	}
	if value, ok := field.Field.Commit(); !ok || value != "first\nsecond" {
		t.Fatalf("committed %q %v", value, ok)
	}

	caret, before, after := field.Session()
	if before != "second" || after != "" || caret.Dy() <= 0 || !caret.In(field.Rect) && caret.Min.Y < field.Rect.Min.Y {
		t.Fatalf("session %v %q %q", caret, before, after)
	}
	focus.Update(frame("Up"), nil)
	if caret := field.Field.Buffer().Caret(); caret > len("first") {
		t.Fatalf("up moved to %d", caret)
	}
	outside := &ui.Input{Cursor: image.Pt(400, 400), Pressed: true, Down: true}
	focus.Update(outside, nil)
	if field.Focused() || !field.Field.Dirty() {
		t.Fatal("clicking away should leave the field and keep the unsaved draft")
	}
}

// TestFieldWrapsAndClicksPlaceTheCaret checks berad-text wrapping and hit
// testing: a click on the second visual line puts the caret there.
func TestFieldWrapsAndClicksPlaceTheCaret(t *testing.T) {
	t.Parallel()
	text := "one two three four five six seven eight nine ten eleven twelve"
	field := ui.NewTextField("body", textedit.NewField(text, true, 0), true, theme(t))
	field.Rect = image.Rect(0, 0, 120, 200)
	var focus ui.Focus
	focus.Register(field)
	focus.Update(&ui.Input{Cursor: image.Pt(8, 8+ui.LineHeight), Pressed: true, Down: true}, nil)
	caret := field.Field.Buffer().Caret()
	if caret == 0 || caret >= len(text) {
		t.Fatalf("click on the second line put the caret at %d", caret)
	}
	focus.Update(frame("Cmd+Left"), nil)
	start := field.Field.Buffer().Caret()
	if start == 0 || start > caret {
		t.Fatalf("Cmd+Left goes to the start of the visual line, got %d", start)
	}
}

// TestCaretAtAWrapStaysOnItsLine clicks at the start of each wrapped line:
// where one line ends and the next begins, the caret belongs to the line it
// is drawn on, so Cmd+Left keeps it there and Cmd+Right goes to that line's
// end, not the previous line's.
func TestCaretAtAWrapStaysOnItsLine(t *testing.T) {
	t.Parallel()
	text := "one two three four five six seven eight nine ten eleven twelve"
	field := ui.NewTextField("body", textedit.NewField(text, true, 0), true, theme(t))
	field.Rect = image.Rect(0, 0, 120, 200)
	var focus ui.Focus
	focus.Register(field)
	for line := 1; line <= 3; line++ {
		focus.Update(&ui.Input{Cursor: image.Pt(1, 8+line*ui.LineHeight), Pressed: true, Down: true}, nil)
		start := field.Field.Buffer().Caret()
		if start == 0 || text[start-1] != ' ' {
			t.Fatalf("line %d: a click at its left edge should land at a wrapped line's start, got %d", line, start)
		}
		focus.Update(frame("Cmd+Left"), nil)
		if got := field.Field.Buffer().Caret(); got != start {
			t.Fatalf("line %d: Cmd+Left moved from the line start %d to %d", line, start, got)
		}
		focus.Update(frame("Cmd+Right"), nil)
		if end := field.Field.Buffer().Caret(); end <= start {
			t.Fatalf("line %d: Cmd+Right went to %d, before the line start %d", line, end, start)
		}
	}
}

// TestSingleLineFieldHoldsStill clicks into a field one button high, whose
// line of text is taller than the room inside its padding, and checks the
// text stays put from frame to frame instead of scrolling back and forth.
func TestSingleLineFieldHoldsStill(t *testing.T) {
	t.Parallel()
	field := ui.NewTextField("name", textedit.NewField("Harbour", false, 0), false, theme(t))
	field.Rect = image.Rect(10, 10, 210, 10+ui.ButtonH)
	var focus ui.Focus
	focus.Register(field)
	focus.Update(&ui.Input{Cursor: image.Pt(50, 20), Pressed: true, Down: true}, nil)
	first, _, _ := field.Session()
	for range 3 {
		focus.Update(frame(), nil)
		if caret, _, _ := field.Session(); caret != first {
			t.Fatalf("the caret moved from %v to %v with no input", first, caret)
		}
	}
	if first.Min.Y < field.Rect.Min.Y || first.Max.Y > field.Rect.Max.Y {
		t.Fatalf("caret %v outside the field %v", first, field.Rect)
	}
}

func TestDialogKeys(t *testing.T) {
	t.Parallel()
	dialog := ui.NewDialog("Delete?", []string{"It is used in 2 places."},
		ui.DialogButton("delete", "Delete"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	window := image.Rect(0, 0, 800, 600)
	if got := dialog.Update(frame("Escape"), window); got != ui.DialogCancel {
		t.Fatalf("Escape chose %q", got)
	}
	if got := dialog.Update(frame("Enter"), window); got != "delete" {
		t.Fatalf("Enter chose %q", got)
	}
	point, ok := dialog.Locate(ui.DialogCancel, window)
	if !ok {
		t.Fatal("no cancel button")
	}
	if got := dialog.Update(&ui.Input{Cursor: point, Pressed: true, Down: true}, window); got != ui.DialogCancel {
		t.Fatalf("click chose %q", got)
	}
	blocked := frame("Cmd+Z")
	dialog.Update(blocked, window)
	if len(blocked.Presses) != 0 {
		t.Fatal("a dialog must take all input")
	}
}
