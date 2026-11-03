package zonepicker

import (
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Locate finds a part of the picker for input scripts: a zone's row,
// "zone:<name>"; the search, "zone-search"; a button, "zone-choose",
// "zone-none" or "cancel"; a tag chip, "zone-tag:<tag>"; or a rail entry,
// "zone-rail:all", "zone-rail:recent" or "zone-rail:<folder>".
func (p *Picker) Locate(window image.Rectangle, offer Offer, name string) (image.Point, bool) {
	p.layout(window, offer)
	if zoneName, isRow := strings.CutPrefix(name, rowName); isRow {
		for index, row := range p.list.Rows {
			if row.Label == zoneName && p.list.RowRect(index).Overlaps(p.list.Rect) {
				return ui.Center(p.list.RowRect(index)), true
			}
		}

		return image.Pt(0, 0), false
	}
	if scope, isRail := strings.CutPrefix(name, railID); isRail {
		for _, entry := range p.rail {
			if entry.id == scope {
				return ui.Center(entry.rect), true
			}
		}
	}
	if name == SearchID {
		return p.search.Center(), true
	}
	for _, button := range append([]*ui.Button{p.choose, p.cancel, p.none}, p.tags...) {
		if button.ID == name && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// HasState answers "zones-shown=<n>", how many zones are listed;
// "zone-count:<rail entry>=<n>", how many match in a rail entry;
// "zone-look=<name>", the zone the look shows; "zone-detail:<name>=<detail>",
// what a zone's row says after its name; and "zone-note=<text>", why the
// last zone chosen could not be. It reports whether the state was one of
// these.
func (p *Picker) HasState(window image.Rectangle, offer Offer, state string) (bool, bool) {
	p.layout(window, offer)
	key, want, _ := strings.Cut(state, "=")
	switch {
	case key == "zones-shown":
		return strconv.Itoa(len(p.shown)) == want, true
	case key == "zone-look":
		looked, found := p.looked()

		return found && looked.Name == want, true
	case key == "zone-note":
		return p.note == want, true
	case strings.HasPrefix(key, "zone-count:"):
		return strconv.Itoa(p.counts[strings.TrimPrefix(key, "zone-count:")]) == want, true
	case strings.HasPrefix(key, "zone-detail:"):
		for _, row := range p.list.Rows {
			if row.Label == strings.TrimPrefix(key, "zone-detail:") {
				return row.Detail == want, true
			}
		}

		return false, true
	}

	return false, false
}
