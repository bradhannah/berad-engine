package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// fieldAnswers clicks a wrapped field's second line and says where the
// caret went, where it is drawn, and how high the field must be.
func fieldAnswers(t *testing.T) (int, image.Rectangle, int) {
	t.Helper()
	text := "one two three four five six seven eight nine ten eleven twelve"
	field := ui.NewTextField("body", textedit.NewField(text, true, 0), true, theme(t))
	field.Rect = image.Rect(0, 0, 120, 200)
	var focus ui.Focus
	focus.Register(field)
	focus.Update(&ui.Input{Cursor: image.Pt(30, 8+ui.LineHeight), Pressed: true, Down: true}, nil)
	caret, _, _ := field.Session()

	return field.Field.Buffer().Caret(), caret, field.FitHeight(10)
}

// A field lays its text out with the font at the scale it is drawn at, and
// answers in points, so a click lands on the same letter and the field is
// as high at any scale (increment 56).
//
//nolint:paralleltest // It sets the window's scale.
func TestAFieldAnswersInPointsAtAnyScale(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	offset, caret, height := fieldAnswers(t)
	for _, scale := range [][2]float64{{2, 1}, {1, 1.5}, {2, 1.25}} {
		pen.SetScale(scale[0], scale[1])
		gotOffset, gotCaret, gotHeight := fieldAnswers(t)
		near := func(a, b int) bool { return max(a-b, b-a) <= 1 }
		if gotOffset != offset || !near(gotCaret.Min.X, caret.Min.X) || !near(gotCaret.Min.Y, caret.Min.Y) ||
			!near(gotHeight, height) {
			t.Fatalf("at %v: caret at %d drawn %v, %d high; at 100%%: %d drawn %v, %d high",
				scale, gotOffset, gotCaret, gotHeight, offset, caret, height)
		}
	}
}

// Text is measured in points at any scale, with its face opened at the
// scale it is drawn at.
//
//nolint:paralleltest // It sets the window's scale.
func TestTextIsMeasuredInPoints(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	loaded := theme(t)
	const words = "Text and controls at 150%"
	width := loaded.TextWidth(words)
	pen.SetScale(2, 1.5)
	if got := loaded.TextWidth(words); max(got-width, width-got) > 1 {
		t.Fatalf("at 300%% the text is %d points wide, at 100%% %d", got, width)
	}
}
