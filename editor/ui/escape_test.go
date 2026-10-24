package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// TestTypingAfterEscape types into a field, reverts with Escape while it
// keeps focus, then types and clicks in it again: neither is lost.
func TestTypingAfterEscape(t *testing.T) {
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
	if !field.Focused() || field.Field.Value() != "Town" {
		t.Fatalf("Escape reverts and keeps focus: %v %q", field.Focused(), field.Field.Value())
	}
	typed = frame()
	typed.Text = "!"
	focus.Update(typed, nil)
	if field.Field.Value() == "Town" {
		t.Fatal("typing after Escape is kept")
	}
	focus.Update(frame("Escape"), nil)
	focus.Update(&ui.Input{Cursor: image.Pt(60, 20), Pressed: true, Down: true}, nil)
	if !field.Field.Editing() {
		t.Fatal("a click after Escape edits again")
	}
}
