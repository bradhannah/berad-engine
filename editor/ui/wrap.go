package ui

import "image"

// WrapRow places a row of controls across an area's width, a line of
// ButtonH at a time from the area's top: the lead controls from the left
// edge and the trail controls against the right edge, on one line while
// both fit with Inset between them. When they do not, the trail starts a
// line of its own below, and a control that does not fit on its group's
// line carries on at the start of the next, so a row too wide for its room
// wraps rather than run under another control (increment 61). Each width
// is a control's, or a few controls' kept together; a width wider than the
// area is cut to it. It returns each control's rectangle, the lead's then
// the trail's, and the bottom of the last line.
func WrapRow(area image.Rectangle, lead, trail []int, gap int) ([]image.Rectangle, []image.Rectangle, int) {
	room := max(area.Dx(), 0)
	if span(lead, gap)+Inset+span(trail, gap) <= room || len(lead) == 0 || len(trail) == 0 {
		leads, leadLast := flow(area, lead, gap, area.Min.Y, false)
		trails, trailLast := flow(area, trail, gap, area.Min.Y, true)

		return leads, trails, max(leadLast, trailLast) + ButtonH
	}
	leads, last := flow(area, lead, gap, area.Min.Y, false)
	trails, last := flow(area, trail, gap, last+ButtonH+Gap, true)

	return leads, trails, last + ButtonH
}

// span is the width of controls side by side with a gap between each.
func span(widths []int, gap int) int {
	total := 0
	for index, width := range widths {
		if index > 0 {
			total += gap
		}
		total += width
	}

	return total
}

// flow places controls on lines from top, each line starting a control
// that would not fit on the one before, against the left edge or, with
// right, the right edge. It returns the rectangles and the last line's top.
func flow(area image.Rectangle, widths []int, gap, top int, right bool) ([]image.Rectangle, int) {
	room := max(area.Dx(), 0)
	rects := make([]image.Rectangle, len(widths))
	line, used := make([]int, 0, len(widths)), 0
	place := func() {
		across := area.Min.X
		if right {
			across = area.Max.X - span(lineWidths(widths, line, room), gap)
		}
		for _, index := range line {
			width := min(widths[index], room)
			rects[index] = image.Rect(across, top, across+width, top+ButtonH)
			across += width + gap
		}
	}
	for index, width := range widths {
		width = min(width, room)
		if len(line) > 0 && used+gap+width > room {
			place()
			line, used, top = line[:0], 0, top+ButtonH+Gap
		}
		if len(line) > 0 {
			used += gap
		}
		line, used = append(line, index), used+width
	}
	if len(line) > 0 {
		place()
	}

	return rects, top
}

// lineWidths are the widths of the controls on one line, cut to the room.
func lineWidths(widths, line []int, room int) []int {
	picked := make([]int, 0, len(line))
	for _, index := range line {
		picked = append(picked, min(widths[index], room))
	}

	return picked
}
