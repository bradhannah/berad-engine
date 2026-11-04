package workflowview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The palette: its search, a result's lines and padding, how long the
// pointer rests on an entry before its tooltip shows, in frames, and how
// wide the tooltip is.
const (
	paletteSearchID = "palette-search"
	maxPaletteQuery = 64
	resultLines     = 2
	resultPad       = 5
	tooltipDelay    = 30
	tooltipWidth    = 260
	tooltipFrame    = 2
	hexMark         = 0xfbe3a6
	helpBackID      = "help-back"
	helpAddID       = "help-add"
	resultPart      = "result"
)

// palettePanel is the palette's search and the help it shows: the search
// field and its own focus, the result marked by the arrow keys, how far the
// entries are scrolled, the entry the pointer rests on and for how long,
// and the kind whose help page the inspector shows.
type palettePanel struct {
	search  *ui.TextField
	focus   ui.Focus
	marked  int
	scroll  int
	height  int
	hover   string
	rested  int
	help    string
	back    *ui.Button
	add     *ui.Button
	listTop int
}

func newPalettePanel(theme *ui.Theme) *palettePanel {
	search := ui.NewTextField(paletteSearchID, textedit.NewField("", false, maxPaletteQuery), false, theme)
	search.Placeholder, search.Unmarked = "Find a node", true
	panel := &palettePanel{
		search: search, focus: ui.Focus{}, marked: 0, scroll: 0, height: 0, hover: "", rested: 0, help: "",
		back: ui.NewButton(helpBackID, "‹ Back"), add: ui.NewButton(helpAddID, "Add"), listTop: 0,
	}
	panel.focus.Register(search)

	return panel
}

// query is what the palette's search holds.
func (s *State) query() string { return strings.TrimSpace(s.pal.search.Field.Value()) }

// results lists the palette entries a search matches, those matching by
// name first, each in palette order.
func (s *State) results() []int {
	query := s.query()
	var byName, byWords []int
	for index, entry := range s.palette {
		matches, named := matchesSearch(entry.kind, query)
		switch {
		case named:
			byName = append(byName, index)
		case matches:
			byWords = append(byWords, index)
		}
	}

	return append(byName, byWords...)
}

// paletteArea is where the palette's entries are, below its search.
func (s *State) paletteArea() image.Rectangle {
	return image.Rect(0, s.pal.listTop, PaletteWidth-ui.Outline, frame.Content(s.size).Max.Y)
}

// layoutPalette places the search and the entries: every entry under its
// group's heading, or, while searching, the results, each with its one
// line; scrolled when they don't fit.
func (s *State) layoutPalette() {
	panel := s.pal
	panel.search.Rect = image.Rect(ui.Inset-ui.Half, paletteTop, PaletteWidth-ui.Inset, paletteTop+ui.ButtonH)
	panel.listTop = panel.search.Rect.Max.Y + ui.Gap
	for index := range s.palette {
		s.palette[index].rect, s.palette[index].lines = ui.Zero(), nil
	}
	top, left, right := panel.listTop-panel.scroll, ui.Inset-ui.Half, PaletteWidth-ui.Inset
	if s.query() == "" {
		group := ""
		for index := range s.palette {
			entry := &s.palette[index]
			if entry.kind.Group != group {
				group = entry.kind.Group
				top += groupSpace
			}
			entry.rect = image.Rect(left, top, right, top+paletteItem)
			top += paletteItem + paletteGap
		}
	} else {
		top += ui.LineHeight
		for _, index := range s.results() {
			entry := &s.palette[index]
			entry.lines = s.wrap(hintFor(entry.kind.Name), right-left-ui.Half*resultPad)
			if len(entry.lines) > resultLines {
				entry.lines = entry.lines[:resultLines]
				entry.lines[resultLines-1] = s.clip(entry.lines[resultLines-1]+"…", right-left-ui.Half*resultPad)
			}
			height := ui.Half*resultPad + (1+len(entry.lines))*ui.LineHeight
			entry.rect = image.Rect(left, top, right, top+height)
			top += height + paletteGap
		}
	}
	panel.height = top + panel.scroll - panel.listTop
	panel.scroll = max(0, min(panel.scroll, panel.height-s.paletteArea().Dy()))
}

// shownEntry reports whether an entry is laid out and in view.
func (s *State) shownEntry(entry paletteEntry) bool {
	return !entry.rect.Empty() && entry.rect.Overlaps(s.paletteArea())
}

// paletteEntryAt returns the kind of the palette entry under a point, if one is in
// view.
func (s *State) paletteEntryAt(point image.Point) (string, bool) {
	for _, entry := range s.palette {
		if s.shownEntry(entry) && point.In(entry.rect) && point.In(s.paletteArea()) {
			return entry.kind.Name, true
		}
	}

	return "", false
}

// updateSearch runs the palette's search before the inspector's fields:
// typing searches, Up and Down mark a result, Enter shows the marked
// result's help and Escape clears the search.
func (s *State) updateSearch(input *ui.Input, ctx *frame.Context) {
	panel := s.pal
	if panel.focus.Current() == panel.search {
		results := s.results()
		plain := ui.Plain()
		switch {
		case input.Consume(ui.KeyEscape, plain):
			panel.search.Field.Revert()
			panel.focus.Set(nil)
			panel.marked, panel.scroll = 0, 0
		case input.Consume(ui.KeyDown, plain):
			panel.marked = min(panel.marked+1, max(0, len(results)-1))
		case input.Consume(ui.KeyUp, plain):
			panel.marked = max(panel.marked-1, 0)
		case input.Consume(ui.KeyEnter, plain):
			if panel.marked < len(results) {
				s.showHelp(s.palette[results[panel.marked]].kind.Name)
			}
		}
	}
	before := s.query()
	panel.focus.Update(input, ctx.Clipboard)
	if s.query() != before {
		panel.marked, panel.scroll = 0, 0
	}
}

// showHelp shows a kind's help page in the inspector, the selection staying
// as it is underneath.
func (s *State) showHelp(kind string) {
	s.pal.help = kind
	s.focus.Set(nil)
}

// updatePalette shows an entry's help when it is clicked, places a node when
// one is dragged onto the canvas, putting it in the flow when dropped onto a
// line, scrolls the palette, and times the pointer resting on an entry.
func (s *State) updatePalette(input *ui.Input, ctx *frame.Context) {
	if s.adding != nil {
		s.adding.at = input.Cursor
		if !input.Released && input.Down {
			return
		}
		adding := s.adding
		s.adding = nil
		moved := adding.at.Sub(adding.start)
		dragged := max(abs(moved.X), abs(moved.Y)) > dragStart
		switch {
		case dragged && adding.at.In(s.Canvas()):
			s.drop(ctx, adding.kind, s.toCanvas(adding.at))
		case !dragged:
			s.showHelp(adding.kind)
		}

		return
	}
	if input.Pressed && input.Cursor.In(s.Canvas()) {
		s.pal.help = ""
	}
	if input.Cursor.In(s.paletteArea()) && input.WheelY != 0 {
		s.pal.scroll -= input.WheelPan().Y
		input.WheelY = 0
	}
	kind, onEntry := s.paletteEntryAt(input.Cursor)
	s.restOn(kind, onEntry && !input.Down)
	if input.Pressed && onEntry {
		s.adding = &paletteDrag{kind: kind, start: input.Cursor, at: input.Cursor}
		s.pal.rested = 0
		input.ClearPointer()
	}
}

// restOn counts the frames the pointer has rested on an entry.
func (s *State) restOn(kind string, resting bool) {
	switch {
	case !resting:
		s.pal.hover, s.pal.rested = "", 0
	case kind != s.pal.hover:
		s.pal.hover, s.pal.rested = kind, 0
	default:
		s.pal.rested++
	}
}

// tooltipShown is the kind whose tooltip shows, if any.
func (s *State) tooltipShown() (string, bool) {
	return s.pal.hover, s.pal.hover != "" && s.pal.rested >= tooltipDelay && s.adding == nil
}

// drop places a dragged node where it is dropped: into the flow when it is
// dropped onto a line it can go in, else free.
func (s *State) drop(ctx *frame.Context, kind string, point image.Point) {
	if key, ok := s.spliceTarget(kind, point); ok {
		s.splice(ctx, kind, key, point.Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half)))

		return
	}
	s.addAt(ctx, kind, point)
}

// splices reports whether a kind of node can go into a line: one that runs
// in the flow and has an output to carry it on.
func splices(kind string) bool {
	node, err := flow.NewNode(kind)
	if err != nil || !flow.Runs(kind) || flow.IsContainer(kind) || kind == flow.KindStart {
		return false
	}

	return len(flow.Outputs(node)) > 0
}

// spliceTarget returns the line a node dropped at a canvas point would go
// into, if any.
func (s *State) spliceTarget(kind string, point image.Point) (routeKey, bool) {
	if s.workflow == nil || !splices(kind) {
		return noRoute(), false
	}
	key, onLine := s.linkAt(point)
	if !onLine {
		return noRoute(), false
	}
	if _, linked := s.targetOf(key); !linked {
		return noRoute(), false
	}

	return key, true
}

// splice adds a node into a line: the line ends at the new node, and its
// first output goes on to the line's old target, in one undo step.
func (s *State) splice(ctx *frame.Context, kind string, key routeKey, place image.Point) {
	from, found := s.workflow.Node(key.from)
	target, linked := s.targetOf(key)
	node, err := flow.NewNode(kind)
	if !found || !linked || err != nil {
		return
	}
	node.Parent = from.Parent
	if outputs := flow.Outputs(node); len(outputs) > 0 {
		node = node.WithLink(outputs[0].Name, target)
	}
	s.addNode(ctx, node, s.clearOf(place, from.Parent), &flow.Link{From: from.ID, Output: key.output, To: ids.ULID{}})
	ctx.Frame.Status("Put " + labelOf(kind) + " between " + s.kindLabel(from.ID) + " and " + s.kindLabel(target) +
		". Undo takes it out.")
}

// labelOf names a kind of node.
func labelOf(kind string) string {
	declared, _ := flow.KindOf(kind)

	return declared.Label
}

// kindLabel names a node of the workflow by its kind.
func (s *State) kindLabel(nodeID ids.ULID) string {
	node, ok := s.workflow.Node(nodeID)
	if !ok {
		return "a node"
	}

	return labelOf(node.Kind)
}

// drawPalette draws the search, the entries or the results, the zone loop's
// note of what it lacks, the node being dragged and the tooltip.
func (s *State) drawPalette(screen *ebiten.Image) {
	content := frame.Content(s.size)
	ui.Divider(screen, image.Pt(PaletteWidth-ui.Outline, toolbarHeight), image.Pt(PaletteWidth-ui.Outline, content.Max.Y))
	s.pal.search.Draw(screen)
	area, ok := screen.SubImage(s.paletteArea()).(*ebiten.Image)
	if !ok {
		return
	}
	if s.query() == "" {
		s.drawPaletteEntries(area)
	} else {
		s.drawResults(area)
	}
	s.drawDragged(screen)
	s.drawTooltip(screen)
}

// drawPaletteEntries draws every entry under its group's heading.
func (s *State) drawPaletteEntries(area *ebiten.Image) {
	group := ""
	for _, entry := range s.palette {
		if entry.kind.Group != group {
			group = entry.kind.Group
			s.theme.Heading(area, group, ui.Inset, entry.rect.Min.Y-groupSpace/ui.Half-ui.Gap-1)
		}
		s.drawEntryBox(area, entry)
		s.theme.DrawTextIn(area, entry.kind.Label, entry.rect, ui.ColorInk())
	}
	if s.workflow != nil && s.workflow.Purpose() == flow.PurposeZoneLoop && len(s.palette) > 0 {
		top := s.palette[len(s.palette)-1].rect.Max.Y + groupSpace
		for index, line := range s.theme.Wrap(zoneLoopLacks, PaletteWidth-ui.Inset*ui.Half) {
			s.theme.DrawText(area, line, ui.Inset, top+index*ui.LineHeight, ui.ColorMuted())
		}
	}
}

// drawEntryBox draws an entry's box: hovered, marked by the arrow keys or
// showing its help, and a Note's dashed.
func (s *State) drawEntryBox(area *ebiten.Image, entry paletteEntry) {
	fill := ui.ColorBackground()
	if s.cursor.In(entry.rect) {
		fill = ui.ColorHover()
	}
	ui.FillRound(area, entry.rect, paletteRadius, fill)
	switch entry.kind.Name {
	case s.pal.help:
		ui.StrokeRound(area, entry.rect, paletteRadius, ui.Half*ui.Border, ui.ColorSelect())
	case flow.KindNote:
		strokeDashed(area, entry.rect, ui.Border, ui.ColorInk())
	default:
		ui.StrokeRound(area, entry.rect, paletteRadius, ui.Border, ui.ColorInk())
	}
}

// drawResults draws how many entries match and each result: its name, its
// group and its one line, the words searched for marked.
func (s *State) drawResults(area *ebiten.Image) {
	results, query := s.results(), s.query()
	count := "No node matches. Try what it does: walk, wait, sound."
	if len(results) > 0 {
		count = strconv.Itoa(len(results)) + " " + plural(len(results), "node matches", "nodes match")
	}
	for index, line := range s.theme.Wrap(count, PaletteWidth-ui.Half*ui.Inset) {
		s.theme.DrawText(area, line, ui.Inset, s.pal.listTop-s.pal.scroll+index*ui.LineHeight, ui.ColorMuted())
	}
	for place, index := range results {
		entry := s.palette[index]
		s.drawEntryBox(area, entry)
		if place == s.pal.marked && s.pal.focus.Current() == s.pal.search {
			ui.StrokeRound(area, entry.rect, paletteRadius, ui.Half*ui.Border, ui.ColorSelect())
		}
		left, top := entry.rect.Min.X+resultPad, entry.rect.Min.Y+resultPad
		s.drawMarked(area, entry.kind.Label, left, top, query, true)
		s.theme.DrawLabel(area, entry.kind.Group, entry.rect.Max.X-resultPad-s.theme.LabelWidth(entry.kind.Group), top,
			ui.ColorMuted())
		for line, text := range entry.lines {
			s.drawMarked(area, text, left, top+(1+line)*ui.LineHeight, query, false)
		}
	}
}

// drawMarked writes text with the first place a search matches it marked,
// in the strong face for a name or muted for a description.
func (s *State) drawMarked(area *ebiten.Image, text string, left, top int, query string, strong bool) {
	width := s.theme.TextWidth
	if strong {
		width = s.theme.StrongWidth
	}
	if at := strings.Index(strings.ToLower(text), strings.ToLower(query)); query != "" && at >= 0 {
		from := left + width(text[:at])
		ui.FillRect(area, image.Rect(from, top-1, from+width(text[at:at+len(query)]), top+ui.TextSize+ui.Half),
			ui.Hex(hexMark))
	}
	if strong {
		s.theme.DrawStrong(area, text, left, top, ui.ColorInk())

		return
	}
	s.theme.DrawText(area, text, left, top, ui.ColorMuted())
}

// drawDragged draws the node being dragged from the palette, with its
// outputs, and lights the line it would go into, saying so.
func (s *State) drawDragged(screen *ebiten.Image) {
	if s.adding == nil || s.adding.at.In(image.Rect(0, 0, PaletteWidth, s.size.Y)) {
		return
	}
	if key, ok := s.spliceTarget(s.adding.kind, s.toCanvas(s.adding.at)); ok {
		corners := s.routes[key]
		for index := 1; index < len(corners); index++ {
			ui.Line(screen, s.onScreen(corners[index-1]), s.onScreen(corners[index]), ui.Half*lineWidth,
				ui.ColorSelect())
		}
		target, _ := s.targetOf(key)
		note := "Drop to put it between " + s.kindLabel(key.from) + " and " + s.kindLabel(target)
		at := s.adding.at.Add(image.Pt(-s.theme.TextWidth(note)/ui.Half, -nodeHeight))
		pill := image.Rect(at.X-ui.Gap, at.Y-ui.Gap/ui.Half, at.X+s.theme.TextWidth(note)+ui.Gap,
			at.Y+ui.TextSize+ui.Gap)
		ui.FillRound(screen, pill, ui.Radius, ui.ColorSelect())
		s.theme.DrawText(screen, note, at.X, at.Y, ui.ColorBackground())
	}
	corner := s.adding.at.Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half))
	ghost := image.Rect(corner.X, corner.Y, corner.X+nodeWidth, corner.Y+nodeHeight+ui.LineHeight)
	ui.FillRound(screen, ghost, ui.Radius, ui.HexAlpha(hexWhite, ghostAlpha))
	ui.StrokeRound(screen, ghost, ui.Radius, ui.Border, ui.ColorSelect())
	s.theme.DrawStrong(screen, labelOf(s.adding.kind), ghost.Min.X+textPad, ghost.Min.Y+textPad, ui.ColorSelect())
	if node, err := flow.NewNode(s.adding.kind); err == nil {
		var names []string
		for _, output := range flow.Outputs(node) {
			names = append(names, output.Name)
		}
		s.theme.DrawText(screen, s.clip(strings.Join(names, " · "), nodeWidth-ui.Half*textPad), ghost.Min.X+textPad,
			ghost.Min.Y+textPad+ui.LineHeight, ui.ColorMuted())
	}
}

// drawTooltip draws the one line of the entry the pointer rests on, beside
// the palette.
func (s *State) drawTooltip(screen *ebiten.Image) {
	kind, shown := s.tooltipShown()
	if !shown {
		return
	}
	var entry paletteEntry
	for _, each := range s.palette {
		if each.kind.Name == kind {
			entry = each
		}
	}
	lines := s.wrap(hintFor(kind), tooltipWidth-ui.Half*ui.Inset)
	height := ui.Half*ui.Gap + (tooltipFrame+len(lines))*ui.LineHeight
	box := image.Rect(PaletteWidth+ui.Gap, entry.rect.Min.Y, PaletteWidth+ui.Gap+tooltipWidth, entry.rect.Min.Y+height)
	ui.FillRound(screen, box, ui.Radius, ui.ColorInk())
	left, top := box.Min.X+ui.Inset, box.Min.Y+ui.Gap
	s.theme.DrawStrong(screen, labelOf(kind), left, top, ui.ColorBackground())
	for index, line := range lines {
		s.theme.DrawText(screen, line, left, top+(1+index)*ui.LineHeight, ui.ColorBackground())
	}
	s.theme.DrawText(screen, "Click for more · drag to place", left, top+(1+len(lines))*ui.LineHeight, ui.ColorRule())
}

// locatePalette finds an entry, "palette:<kind>", or a result while
// searching, "result:<kind>".
func (s *State) locatePalette(kind string) (image.Point, bool) {
	for _, entry := range s.palette {
		if entry.kind.Name == kind && s.shownEntry(entry) {
			return ui.Center(entry.rect), true
		}
	}

	return image.Pt(0, 0), false
}

// hasPaletteState answers "results=<count>", how many entries a search
// matches, "help=<kind>", whose help page the inspector shows, and
// "tooltip=<kind>", whose tooltip shows.
func (s *State) hasPaletteState(state string, _ frame.Docs) (bool, bool) {
	switch {
	case strings.HasPrefix(state, "results="):
		return strconv.Itoa(len(s.results())) == strings.TrimPrefix(state, "results="), true
	case strings.HasPrefix(state, "help="):
		return s.pal.help == strings.TrimPrefix(state, "help="), true
	case strings.HasPrefix(state, "tooltip="):
		kind, shown := s.tooltipShown()

		return shown && kind == strings.TrimPrefix(state, "tooltip="), true
	}

	return false, false
}
