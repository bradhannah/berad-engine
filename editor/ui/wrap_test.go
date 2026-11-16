package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestARowWrapsWhenTooWide lays out a toolbar's tools at the left and its
// zoom at the right: on one line in a wide area, and in a narrow one the
// zoom goes on a line of its own below, against the right edge, and tools
// that do not fit carry on below them, nothing over anything else
// (increment 61).
func TestARowWrapsWhenTooWide(t *testing.T) {
	t.Parallel()
	tools, zoom := []int{88, 88, 88, 88}, []int{140, 60}
	leads, trails, bottom := ui.WrapRow(image.Rect(10, 20, 810, 20), tools, zoom, ui.Gap)
	if bottom != 20+ui.ButtonH || leads[0].Min != image.Pt(10, 20) || trails[1].Max.X != 810 {
		t.Fatalf("a wide row was not one line: %v %v, bottom %d", leads, trails, bottom)
	}
	leads, trails, bottom = ui.WrapRow(image.Rect(10, 20, 510, 20), tools, zoom, ui.Gap)
	line := ui.ButtonH + ui.Gap
	if leads[3].Min.Y != 20 || trails[0].Min.Y != 20+line || trails[1].Max.X != 510 || bottom != 20+line+ui.ButtonH {
		t.Fatalf("the zoom did not go on its own line below: %v %v, bottom %d", leads, trails, bottom)
	}
	leads, trails, bottom = ui.WrapRow(image.Rect(0, 0, 200, 0), tools, zoom, ui.Gap)
	if leads[1].Min.Y != 0 || leads[2].Min != image.Pt(0, line) || bottom != 3*line+ui.ButtonH {
		t.Fatalf("tools did not carry on below: %v, bottom %d", leads, bottom)
	}
	all := append(leads, trails...) //nolint:gocritic // A new slice of both.
	for first, rect := range all {
		if !rect.In(image.Rect(0, 0, 200, bottom)) {
			t.Fatalf("control %d at %v is outside the row", first, rect)
		}
		for second := first + 1; second < len(all); second++ {
			if rect.Overlaps(all[second]) {
				t.Fatalf("controls %d and %d overlap: %v %v", first, second, rect, all[second])
			}
		}
	}
	leads, _, _ = ui.WrapRow(image.Rect(0, 0, 50, 0), []int{80}, nil, ui.Gap)
	if leads[0].Dx() != 50 {
		t.Fatalf("a control wider than the row was not cut to it: %v", leads[0])
	}
}
