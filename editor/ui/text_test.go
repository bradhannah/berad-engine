package ui_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// TestFitText keeps text that fits, and cuts text that does not, ending it
// with an ellipsis inside the width.
func TestFitText(t *testing.T) {
	t.Parallel()
	loaded := theme(t)
	short := "Ready"
	if got := loaded.FitText(short, 200); got != short {
		t.Fatalf("text that fits is kept: %q", got)
	}
	long := "Jumped to 09:00. NPCs start their workflows again."
	got := loaded.FitText(long, 120)
	if !strings.HasSuffix(got, "…") || loaded.TextWidth(got) > 120 || !strings.HasPrefix(long, strings.TrimSuffix(got, "…")) {
		t.Fatalf("cut to %q (%d px)", got, loaded.TextWidth(got))
	}
	if got := loaded.FitText(long, 2); got != "" {
		t.Fatalf("nothing fits: %q", got)
	}
}

// TestFieldPaints draws the stretches a field's Paints names in their
// paints and the rest in ink, leaving out one out of order or out of the
// text.
func TestFieldPaints(t *testing.T) {
	t.Parallel()
	field := ui.NewTextField("line", textedit.NewField("", false, 0), false, theme(t))
	field.Paints = func(string) []ui.TextPaint {
		return []ui.TextPaint{
			{Start: 3, End: 8, Paint: ui.PaintConstant}, {Start: 1, End: 2, Paint: ui.PaintTag},
			{Start: 9, End: 99, Paint: ui.PaintValue},
		}
	}
	got := field.PaintsOf("Hi {gold} there")
	want := [][2]string{{"Hi ", "ink"}, {"{gold", ui.PaintConstant}, {"} there", "ink"}}
	if !slices.Equal(got, want) {
		t.Fatalf("painted %q, want %q", got, want)
	}
	field.Paints = nil
	if got := field.PaintsOf("plain"); !slices.Equal(got, [][2]string{{"plain", "ink"}}) {
		t.Fatalf("without paints, all ink: %q", got)
	}
}

// TestFitHeight grows a wrapping field a row for each line its text wraps
// to, from a button's height, and no further than the rows it may have.
func TestFitHeight(t *testing.T) {
	t.Parallel()
	field := ui.NewTextField("line", textedit.NewField("Hello", false, 0), true, theme(t))
	field.Rect = image.Rect(0, 0, 200, ui.ButtonH)
	if height := field.FitHeight(6); height != ui.ButtonH {
		t.Fatalf("one line is a button high, not %d", height)
	}
	field.Field = textedit.NewField(strings.Repeat("Fresh bread, still warm! ", 4), false, 0)
	two := field.FitHeight(6)
	if two <= ui.ButtonH {
		t.Fatalf("a line that wraps grows the field: %d", two)
	}
	field.Field = textedit.NewField(strings.Repeat("Fresh bread, still warm! ", 40), false, 0)
	if most, tallest := field.FitHeight(6), field.FitHeight(60); most >= tallest || most <= two {
		t.Fatalf("six rows at most: %d, against %d for sixty", most, tallest)
	}
}
