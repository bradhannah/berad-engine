package frametest

import (
	"image"
	"slices"
	"strings"
)

// A Part is a named rectangle a view laid out: a button, a field, a line
// of text.
type Part struct {
	Name string
	Rect image.Rectangle
}

// Apart reports the first two parts that overlap, or parts outside the
// area, named as "Save over Default", or "" when every part is apart and
// inside: what a view lays out in a small window must not be drawn over
// anything else (increment 61). Empty parts, not shown, are left out.
func Apart(area image.Rectangle, parts ...Part) string {
	shown := slices.DeleteFunc(slices.Clone(parts), func(part Part) bool { return part.Rect.Empty() })
	var wrong []string
	for first, part := range shown {
		if !part.Rect.In(area) {
			wrong = append(wrong, part.Name+" outside "+area.String()+" at "+part.Rect.String())
		}
		for _, other := range shown[first+1:] {
			if part.Rect.Overlaps(other.Rect) {
				wrong = append(wrong, part.Name+" "+part.Rect.String()+" over "+other.Name+" "+other.Rect.String())
			}
		}
	}

	return strings.Join(wrong, "; ")
}
