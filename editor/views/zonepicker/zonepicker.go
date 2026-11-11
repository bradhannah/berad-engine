// Package zonepicker is the zone picker: a dialog over a window that finds
// one of a project's zones among many, shared by whatever asks for a zone.
// It has a search, focused on open, that matches a zone's name, folder or
// tags; the project's zone tags as chips; a rail of All zones, Recent and
// the zone folders, with how many zones match in each; the zones with their
// folder, floors, the NPCs living in them and their places; and a look at
// the zone under the cursor: its ground floor drawn small, its floors, size,
// who lives there, its places, tags and notes. While it is open it takes
// all input.
//
// The asker says what the zone is for, the zone it is now, the zones it
// cannot take and why, and whether No zone is offered; it is told what was
// chosen.
package zonepicker

import (
	"cmp"
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Layout, in screen pixels.
const (
	width     = 980
	height    = 620
	radius    = 8
	shadow    = 6
	shade     = 0x40
	buttonW   = 110
	third     = 3
	textPad   = 9
	railWidth = 176
	listWidth = 380
	maxSearch = 200
	title     = "Choose a zone"
	tagsLabel = "Tags"
)

// Locate names for input scripts, besides "zone:<name>" for a zone's row:
// the search, the buttons, a tag chip as "zone-tag:<tag>" and a rail entry
// as "zone-rail:all", "zone-rail:recent" or "zone-rail:<folder>".
const (
	SearchID = "zone-search"
	ChooseID = "zone-choose"
	NoneID   = "zone-none"
	tagID    = "zone-tag:"
	railID   = "zone-rail:"
	rowName  = "zone:"
)

// Rail entries besides the folders.
const (
	railAll    = "all"
	railRecent = "recent"
)

// Config is what a picker is for: what the zone is for, as "for Clerk to
// live in"; the zone it is now, marked, or none; the zones chosen last,
// most recent first; whether No zone is offered, and what it is called, as
// Any zone, "" being No zone; whether choosing goes on to ask something
// more, so its button ends in an ellipsis; and how many of the asker's own
// places are in each zone, shown beside it.
type Config struct {
	For       string
	Now       ids.ULID
	Recent    []ids.ULID
	NoZone    bool
	NoneLabel string
	GoesOn    bool
	Mine      map[ids.ULID]int
}

// Offer is what a picker lists: the zones, as Gather finds them with the
// asker's refusals, and the zone folders, as Folders finds them.
type Offer struct {
	Zones   []Zone
	Folders []string
}

// Choice is what was chosen: a zone, or None for No zone.
type Choice struct {
	Zone ids.ULID
	None bool
}

// Picker is an open zone picker.
type Picker struct {
	config Config
	theme  *ui.Theme
	search *ui.TextField
	focus  ui.Focus
	list   *ui.List
	tags   []*ui.Button
	rail   []railEntry
	scope  string
	counts map[string]int
	shown  []Zone
	// hover is the zone under the cursor, the one the look shows before the
	// one selected; note says why the last zone chosen could not be.
	hover  ids.ULID
	note   string
	looks  *looks
	choose *ui.Button
	cancel *ui.Button
	none   *ui.Button
	box    image.Rectangle
	look   image.Rectangle
	closed bool
	chosen Choice
}

// railEntry is one entry of the rail: its ID, its label and how deep a
// folder it is, and where it is drawn.
type railEntry struct {
	id, label string
	depth     int
	rect      image.Rectangle
}

// New opens a picker with the search focused and the zone it is now
// selected.
func New(config Config, theme *ui.Theme) *Picker {
	search := ui.NewTextField(SearchID, textedit.NewField("", false, maxSearch), false, theme)
	search.Placeholder, search.Unmarked = "type to narrow, by a zone's name, folder or tag", true
	selected := ""
	if !config.Now.IsZero() {
		selected = config.Now.String()
	}
	picker := &Picker{
		config: config, theme: theme, search: search, focus: ui.Focus{}, list: ui.NewList("zones", selected),
		tags: nil, rail: nil, scope: railAll, counts: map[string]int{}, shown: nil, hover: ids.ULID{}, note: "",
		looks: newLooks(), choose: ui.NewButton(ChooseID, "Choose"), cancel: ui.NewButton(ui.DialogCancel, "Cancel"),
		none: ui.NewButton(NoneID, cmp.Or(config.NoneLabel, "No zone")), box: ui.Zero(), look: ui.Zero(), closed: false,
		chosen: Choice{Zone: ids.ULID{}, None: false},
	}
	picker.choose.Primary = true
	picker.focus.Register(search)
	picker.focus.Set(search)

	return picker
}

// Focused returns the search, which takes typing while the picker is open.
func (p *Picker) Focused() *ui.TextField { return p.search }

// layout centres the picker in the window and places its parts for the
// zones given: the title, the search and the tag chips, the rail, the list
// and the look beside it, and the buttons at the foot.
func (p *Picker) layout(window image.Rectangle, offer Offer) {
	boxWidth, boxHeight := min(width, window.Dx()-ui.Half*ui.Inset), min(height, window.Dy()-ui.Half*ui.Inset)
	left := window.Min.X + (window.Dx()-boxWidth)/ui.Half
	top := window.Min.Y + max((window.Dy()-boxHeight)/third, ui.Inset)
	p.box = image.Rect(left, top, left+boxWidth, top+boxHeight)
	right := left + boxWidth - ui.Inset
	line := top + ui.Inset + ui.LineHeight + ui.Gap
	p.layoutTags(offer.Zones, line, right)
	searchRight := right
	if len(p.tags) > 0 {
		searchRight = p.tags[0].Rect.Min.X - ui.Half*ui.Gap - p.theme.StrongWidth(tagsLabel) - ui.Half*ui.Gap
	}
	p.search.Rect = image.Rect(left+ui.Inset, line, searchRight, line+ui.ButtonH)
	line += ui.ButtonH + ui.Half*ui.Gap
	foot := top + boxHeight - ui.Inset - ui.ButtonH
	p.layoutRail(offer.Folders, left+ui.Inset, line)
	listLeft := left + ui.Inset + railWidth + ui.Inset
	p.list.Rect = image.Rect(listLeft, line+ui.RowHeight, min(listLeft+listWidth, right), foot-ui.Half*ui.Gap-ui.ButtonH)
	p.look = image.Rect(p.list.Rect.Max.X+ui.Inset, line, right, p.list.Rect.Max.Y)
	p.list.Rows = p.rows(offer.Zones)
	place := func(button *ui.Button, wide int, show bool) {
		button.Rect = ui.Zero()
		if show {
			button.Rect = image.Rect(right-wide, foot, right, foot+ui.ButtonH)
			right -= wide + ui.Gap
		}
	}
	p.choose.Label = p.chooseLabel()
	place(p.choose, max(buttonW, p.theme.TextWidth(p.choose.Label)+ui.Inset), true)
	place(p.cancel, buttonW, true)
	place(p.none, buttonW, p.config.NoZone)
}

// layoutTags places a chip for each tag the project's zones carry, from the
// right edge leftwards, keeping those already made so they stay on.
func (p *Picker) layoutTags(zones []Zone, top, right int) {
	var tags []string
	for _, each := range zones {
		for _, tag := range each.Tags {
			if !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
	}
	slices.Sort(tags)
	kept := make([]*ui.Button, 0, len(tags))
	for _, tag := range tags {
		chip := ui.NewButton(tagID+tag, tag)
		for _, made := range p.tags {
			if made.ID == chip.ID {
				chip = made
			}
		}
		kept = append(kept, chip)
	}
	p.tags = kept
	for index := len(p.tags) - 1; index >= 0; index-- {
		chip := p.tags[index]
		chip.Rect = image.Rect(right-p.theme.TextWidth(chip.Label)-ui.Half*ui.Inset, top, right, top+ui.ButtonH)
		right = chip.Rect.Min.X - ui.Gap
	}
}

// chooseLabel is the choose button's label: Choose and the zone it would
// choose, with an ellipsis when choosing goes on to ask more.
func (p *Picker) chooseLabel() string {
	label := "Choose"
	if picked, found := p.pick(); found {
		label += " " + picked.Name
	}
	if p.config.GoesOn {
		label += "…"
	}

	return p.theme.FitText(label, buttonW*ui.Half-ui.Inset)
}

// pick is the zone Enter or the choose button chooses: the one selected,
// else the first listed.
func (p *Picker) pick() (Zone, bool) {
	for _, each := range p.shown {
		if each.ID.String() == p.list.Selected {
			return each, true
		}
	}
	if len(p.shown) > 0 {
		return p.shown[0], true
	}

	return Zone{
		ID: ids.ULID{}, Name: "", Folder: "", Tags: nil, Floors: nil, Size: image.Pt(0, 0), Living: nil, Places: nil,
		Notes: "", Ground: nil, Drawn: 0, Refused: "",
	}, false
}

// Update runs the picker for a frame in a window over the zones given,
// taking all input: typing searches, a chip narrows by tag and a rail entry
// by folder, arrows move, a click selects a zone and shows it, and a
// double-click, Enter or the choose button chooses it; No zone chooses
// none; Escape, a click outside or Cancel closes. It returns what was
// chosen, once, and whether the picker is still open.
func (p *Picker) Update(
	input *ui.Input, clipboard textedit.Clipboard, window image.Rectangle, offer Offer,
) (Choice, bool, bool) {
	p.layout(window, offer)
	defer input.ClearAll()
	enter, escape := input.Has(ui.KeyEnter, ui.Plain()), input.Has(ui.KeyEscape, ui.Plain())
	upward, downward := input.Has(ui.KeyUp, ui.Plain()), input.Has(ui.KeyDown, ui.Plain())
	if p.cancel.Update(input) || escape || (input.Pressed && !input.Cursor.In(p.box)) {
		return p.chosen, false, false
	}
	if input.Pressed {
		p.note = ""
	}
	p.focus.Update(input, clipboard)
	if p.focus.Current() == nil {
		p.focus.Set(p.search)
	}
	if p.updateFilters(input) {
		p.layout(window, offer)
	}
	p.hover = p.zoneAt(input.Cursor)
	switch {
	case upward:
		p.move(-1)
	case downward:
		p.move(1)
	}
	if p.none.Update(input) {
		p.closed, p.chosen = true, Choice{Zone: ids.ULID{}, None: true}

		return p.chosen, true, false
	}
	result := p.list.Update(input)
	if p.choose.Update(input) || result.Activated || enter {
		p.chooseSelected()
	}

	return p.chosen, p.closed, !p.closed
}

// chooseSelected chooses the zone selected, or says why it cannot be.
func (p *Picker) chooseSelected() {
	picked, found := p.pick()
	switch {
	case !found:
		p.note = "No zone matches."
	case picked.Refused != "":
		p.note = picked.Name + ": " + picked.Refused
	default:
		p.closed, p.chosen = true, Choice{Zone: picked.ID, None: false}
	}
}

// updateFilters handles the tag chips and the rail, and reports whether one
// was clicked.
func (p *Picker) updateFilters(input *ui.Input) bool {
	for _, chip := range p.tags {
		if chip.Update(input) {
			chip.On = !chip.On

			return true
		}
	}
	for _, entry := range p.rail {
		if input.Pressed && input.Cursor.In(entry.rect) {
			input.ClearPointer()
			p.scope = entry.id

			return true
		}
	}

	return false
}

// move selects the next or previous zone listed.
func (p *Picker) move(step int) {
	if len(p.shown) == 0 {
		return
	}
	current := -1
	for index, each := range p.shown {
		if each.ID.String() == p.list.Selected {
			current = index
		}
	}
	p.list.Selected = p.shown[min(max(current+step, 0), len(p.shown)-1)].ID.String()
	p.list.Reveal(p.list.Selected)
}

// zoneAt returns the zone whose row is under a point, or zero.
func (p *Picker) zoneAt(point image.Point) ids.ULID {
	if !point.In(p.list.Rect) {
		return ids.ULID{}
	}
	for index, row := range p.list.Rows {
		if point.In(p.list.RowRect(index)) {
			if zoneID, err := ids.Parse(row.ID); err == nil {
				return zoneID
			}
		}
	}

	return ids.ULID{}
}

// Draw draws the picker over the shaded window.
func (p *Picker) Draw(screen *ebiten.Image, window image.Rectangle, offer Offer) {
	p.layout(window, offer)
	ui.FillRect(screen, window, ui.ColorShade())
	ui.FillRound(screen, p.box.Add(image.Pt(0, shadow)), radius, ui.HexAlpha(0, shade))
	ui.Float(screen, p.box, radius)
	ui.FillRound(screen, p.box, radius, ui.ColorPage())
	ui.StrokeRound(screen, p.box, radius, ui.Border, ui.ColorInk())
	left, top := p.box.Min.X+ui.Inset, p.box.Min.Y+ui.Inset
	p.theme.DrawStrong(screen, title, left, top, ui.ColorInk())
	room := p.box.Dx() - ui.Half*ui.Inset - p.theme.StrongWidth(title)
	p.theme.DrawText(screen, p.theme.FitText(p.forLine(offer.Zones), room), left+p.theme.StrongWidth(title)+ui.Half*ui.Gap,
		top, ui.ColorMuted())
	p.search.Draw(screen)
	if len(p.tags) > 0 {
		chipsLeft := p.tags[0].Rect.Min.X - ui.Half*ui.Gap - p.theme.StrongWidth(tagsLabel)
		p.theme.DrawStrong(screen, tagsLabel, chipsLeft, centred(p.tags[0].Rect), ui.ColorInk())
	}
	for _, chip := range p.tags {
		chip.Draw(screen, p.theme)
	}
	p.drawRail(screen)
	p.drawList(screen)
	p.drawLook(screen)
	hint := "↑ ↓ move · Enter chooses · Esc closes"
	ink := ui.ColorMuted()
	if p.note != "" {
		hint, ink = p.note, ui.ColorWarn()
	}
	leftmost := p.cancel.Rect.Min.X
	if !p.none.Rect.Empty() {
		leftmost = p.none.Rect.Min.X
	}
	p.theme.DrawText(screen, p.theme.FitText(hint, leftmost-left-ui.Inset), left, centred(p.cancel.Rect), ink)
	for _, button := range []*ui.Button{p.none, p.cancel, p.choose} {
		button.Draw(screen, p.theme)
	}
}

// forLine says what the zone is for and what it is now, as "for Clerk to
// live in · now Market".
func (p *Picker) forLine(zones []Zone) string {
	for _, each := range zones {
		if each.ID == p.config.Now {
			return strings.TrimPrefix(p.config.For+" · now "+each.Name, " · ")
		}
	}

	return p.config.For
}

// centred is the top of a line of text centred on a row as high as a
// button.
func centred(rect image.Rectangle) int { return rect.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1 }
