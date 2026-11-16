package ui_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// TestEscapeKeepsWhatWasTyped types into a field, then presses Escape: it
// leaves the field, keeps what was typed waiting for Save and says so; a
// click in it again edits on from there (increment 59).
func TestEscapeKeepsWhatWasTyped(t *testing.T) {
	t.Parallel()
	field := ui.NewTextField("name", textedit.NewField("", false, 0), false, theme(t))
	field.Field.SetSaved("Town")
	field.Rect = image.Rect(10, 10, 210, 36)
	var focus ui.Focus
	focus.Register(field)
	focus.Update(&ui.Input{Cursor: image.Pt(50, 20), Pressed: true, Down: true}, nil)
	typed := frame()
	typed.Text = "x"
	focus.Update(typed, nil)
	focus.Update(frame("Escape"), nil)
	if field.Focused() || field.Field.Value() != "Townx" || !field.Field.Dirty() {
		t.Fatalf("Escape leaves the field and keeps the typing: %v %q", field.Focused(), field.Field.Value())
	}
	if said := field.Field.TakeRefusal(); said != "Left the field. What you typed waits for Save." {
		t.Fatalf("it says so: %q", said)
	}
	focus.Update(&ui.Input{Cursor: image.Pt(60, 20), Pressed: true, Down: true}, nil)
	typed = frame()
	typed.Text = "!"
	focus.Update(typed, nil)
	if !field.Field.Editing() || !strings.HasPrefix(field.Field.Value(), "Town") || !strings.Contains(field.Field.Value(), "x") {
		t.Fatalf("a click edits on from what was kept: %q", field.Field.Value())
	}
}

// TestTabMovesBetweenFields moves with Tab and Shift+Tab through a window's
// fields shown, skipping one that is not, and stays at the ends saying so
// (increment 59).
func TestTabMovesBetweenFields(t *testing.T) {
	t.Parallel()
	first := ui.NewTextField("first", textedit.NewField("", false, 0), false, theme(t))
	hidden := ui.NewTextField("hidden", textedit.NewField("", false, 0), false, theme(t))
	last := ui.NewTextField("last", textedit.NewField("", false, 0), false, theme(t))
	first.Rect, last.Rect = image.Rect(10, 10, 210, 36), image.Rect(10, 50, 210, 76)
	var focus ui.Focus
	focus.Register(first, hidden, last)
	revealed := 0
	focus.SetReveal(func(*ui.TextField) { revealed++ })
	focus.Update(&ui.Input{Cursor: image.Pt(50, 20), Pressed: true, Down: true}, nil)
	typed := frame()
	typed.Text = "kept"
	focus.Update(typed, nil)
	focus.Update(frame("Tab"), nil)
	if focus.Current() != last || revealed != 1 || first.Field.Value() != "kept" {
		t.Fatalf("Tab moves to the next field shown: %v %d %q", focus.Current() == last, revealed, first.Field.Value())
	}
	focus.Update(frame("Tab"), nil)
	if focus.Current() != last || last.Field.TakeRefusal() != "This is the last field. Esc leaves it." {
		t.Fatal("at the last field Tab stays and says so")
	}
	focus.Update(frame("Shift+Tab"), nil)
	if focus.Current() != first {
		t.Fatal("Shift+Tab moves back")
	}
}
