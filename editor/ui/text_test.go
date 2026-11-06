package ui_test

import (
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
