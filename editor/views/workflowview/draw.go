package workflowview

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/loudness"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Drawing sizes and colours.
const (
	dotSpacing    = 16
	lineWidth     = 2
	arrowSize     = 5
	selectWidth   = 2.5
	selectGap     = 3
	textPad       = 9
	titleDrop     = 21
	labelDrop     = 6
	dashLength    = 6
	dashGap       = 4
	containerLine = 2
	freeLabelGap  = 8
	anchorDot     = 6
	anchorRing    = 2
	paletteRadius = 5
	// Washes over the canvas, and the corner grip of a container.
	hexWhite       = 0xffffff
	ghostAlpha     = 0xd0
	containerAlpha = 0xd8
	gripStep       = 4
	gripInset      = 2
	// portParts is "<kind>:<n>:<output>".
	portParts = 3
)

// textParam returns a text parameter, or "" for a node without it.
func textParam(node flow.Node, name string) string {
	text, err := node.Text(name)
	if err != nil {
		return ""
	}

	return text
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	if s.live != nil {
		s.drawLive(screen, docs)

		return
	}
	s.size = screen.Bounds().Size()
	s.load(docs)
	s.layoutWidgets(docs)
	screen.Fill(ui.ColorBackground())
	s.drawZoomed(screen, docs)
	if s.workflow != nil {
		s.zoomCard.draw(screen, s.theme)
		s.drawLegend(screen)
	}
	s.drawToolbar(screen, docs)
	s.drawPalette(screen)
	s.drawInspector(screen, docs)
	if dialog := s.picker.Dialog(); dialog != nil {
		dialog.Draw(screen, s.theme)
	}
	s.contextMenu.Draw(screen, s.theme)
	s.guard.Draw(screen, s.theme)
	if s.values != nil {
		s.drawValuePicker(screen, docs)
	}
}

func (s *State) drawToolbar(screen *ebiten.Image, docs frame.Docs) {
	content := frame.Content(s.size)
	textY := (toolbarHeight - ui.TextSize) / ui.Half
	trail := "Workflows › "
	s.theme.DrawText(screen, trail, ui.Inset, textY, ui.ColorMuted())
	s.theme.DrawStrong(screen, s.name(docs), ui.Inset+s.theme.TextWidth(trail), textY, ui.ColorInk())
	s.arrange.Draw(screen, s.theme)
	s.recentre.Draw(screen, s.theme)
	s.legend.Draw(screen, s.theme)
	ui.Divider(screen, image.Pt(0, toolbarHeight-ui.Outline), image.Pt(content.Max.X, toolbarHeight-ui.Outline))
}

func (s *State) drawPalette(screen *ebiten.Image) {
	content := frame.Content(s.size)
	ui.Divider(screen, image.Pt(PaletteWidth-ui.Outline, toolbarHeight), image.Pt(PaletteWidth-ui.Outline, content.Max.Y))
	group := ""
	for _, entry := range s.palette {
		if entry.kind.Group != group {
			group = entry.kind.Group
			s.theme.Heading(screen, group, ui.Inset, entry.rect.Min.Y-groupSpace/ui.Half-ui.Gap-1)
		}
		fill := ui.ColorBackground()
		if s.cursor.In(entry.rect) {
			fill = ui.ColorHover()
		}
		ui.FillRound(screen, entry.rect, paletteRadius, fill)
		if entry.kind.Name == flow.KindNote {
			strokeDashed(screen, entry.rect, ui.Border, ui.ColorInk())
		} else {
			ui.StrokeRound(screen, entry.rect, paletteRadius, ui.Border, ui.ColorInk())
		}
		s.theme.DrawTextIn(screen, entry.kind.Label, entry.rect, ui.ColorInk())
	}
	if s.adding != nil && !s.adding.at.In(frame.Content(s.size).Intersect(image.Rect(0, 0, PaletteWidth, s.size.Y))) {
		corner := s.adding.at.Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half))
		ghost := image.Rect(corner.X, corner.Y, corner.X+nodeWidth, corner.Y+nodeHeight)
		ui.FillRound(screen, ghost, ui.Radius, ui.HexAlpha(hexWhite, ghostAlpha))
		ui.StrokeRound(screen, ghost, ui.Radius, ui.Border, ui.ColorSelect())
		if kind, ok := flow.KindOf(s.adding.kind); ok {
			s.theme.DrawTextIn(screen, kind.Label, ghost, ui.ColorSelect())
		}
	}
}

// drawCanvas draws the dotted ground, then connections behind the nodes.
func (s *State) drawCanvas(area *ebiten.Image, docs frame.Docs) {
	area.Fill(ui.ColorCanvas())
	canvas := area.Bounds()
	origin := canvas.Min.Add(s.offset)
	dot := ui.ColorRule()
	startX := canvas.Min.X + mod(origin.X-canvas.Min.X, dotSpacing)
	startY := canvas.Min.Y + mod(origin.Y-canvas.Min.Y, dotSpacing)
	for y := startY; y < canvas.Max.Y; y += dotSpacing {
		for x := startX; x < canvas.Max.X; x += dotSpacing {
			vector.FillRect(area, float32(x), float32(y), 1, 1, dot, false)
		}
	}
	if s.workflow == nil {
		s.theme.DrawText(area, "Loading the workflow…", canvas.Min.X+ui.Inset, canvas.Min.Y+ui.Inset, ui.ColorMuted())

		return
	}
	order := s.drawOrder()
	for _, nodeID := range order {
		switch s.shapes[nodeID].node.Kind {
		case flow.KindContainer:
			s.drawContainer(area, s.shapes[nodeID])
		case flow.KindNote:
			s.drawNode(area, s.shapes[nodeID], docs)
		}
	}
	s.drawLines(area)
	for _, nodeID := range order {
		if runsBehindLines(s.shapes[nodeID].node.Kind) {
			s.drawNode(area, s.shapes[nodeID], docs)
		}
	}
	for _, nodeID := range order {
		s.drawPorts(area, s.shapes[nodeID])
		s.drawEntries(area, s.shapes[nodeID])
	}
	if selected, ok := s.shapes[s.selected]; ok {
		ui.StrokeRound(area, s.screenRect(selected.rect).Inset(-selectGap), ui.Radius+selectGap, selectWidth,
			ui.ColorSelect())
	}
	s.drawLiveNode(area)
	if s.link != nil {
		if from, ok := s.shapes[s.link.from].portOf(s.link.output); ok {
			ui.Line(area, s.toScreen(from.at), s.toScreen(s.link.to), lineWidth, ui.ColorSelect())
		}
	}
}

func (s *State) screenRect(rect image.Rectangle) image.Rectangle {
	return rect.Add(s.Canvas().Min).Add(s.offset)
}

// drawEntries draws a node's entry dots: filled for each link coming in, and
// the hollow spare for the next one, washed blue while a link being drawn is
// over a node that can take it.
func (s *State) drawEntries(area *ebiten.Image, drawn shape) {
	if !takesEntries(drawn.node.Kind) {
		return
	}
	for _, in := range drawn.entries {
		centre := s.toScreen(in.at)
		fill := ui.ColorInk()
		if s.lineLit(routeKey{from: in.from, output: in.output}) {
			fill = ui.ColorSelect()
		}
		vector.FillCircle(area, float32(centre.X), float32(centre.Y), portRadius, fill, true)
	}
	centre := s.toScreen(drawn.spare)
	fill, ring := ui.ColorBackground(), ui.ColorInk()
	if s.link != nil && s.linkTarget() == drawn.node.ID {
		fill, ring = ui.ColorSelectWash(), ui.ColorSelect()
	}
	vector.FillCircle(area, float32(centre.X), float32(centre.Y), portRadius, fill, true)
	vector.StrokeCircle(area, float32(centre.X), float32(centre.Y), portRadius, ui.Border, ring, true)
}

// drawPorts draws each output's port: filled when connected, hollow with
// where the flow goes instead when not.
func (s *State) drawPorts(area *ebiten.Image, from shape) {
	for _, out := range from.ports {
		centre := s.toScreen(out.at)
		key := routeKey{from: from.node.ID, output: out.output.Name}
		_, linked := from.node.Next[out.output.Name]
		fill, ring := ui.ColorBackground(), ui.ColorInk()
		if coloured, ok := s.outcomeTint(key); ok {
			ring = coloured.ink
		}
		switch {
		case linked && s.lineLit(key):
			fill, ring = ui.ColorSelect(), ui.ColorSelect()
		case linked:
			fill = ring
		}
		vector.FillCircle(area, float32(centre.X), float32(centre.Y), portRadius, fill, true)
		vector.StrokeCircle(area, float32(centre.X), float32(centre.Y), portRadius, ui.Border, ring, true)
		if linked || out.output.Name == flow.OutputInside {
			continue
		}
		labelY := centre.Y - ui.LabelSize/ui.Half - ui.Half
		s.theme.DrawSmall(area, endsAt(from.node, out.output.Name), centre.X+freeLabelGap, labelY, ui.ColorMuted())
	}
}

// endsAt says where the flow goes from an output with nothing connected.
func endsAt(node flow.Node, output string) string {
	switch {
	case node.Kind == flow.KindContainer && output == flow.OutputExit:
		return "→ start"
	case !node.Parent.IsZero():
		return "→ repeat"
	default:
		return "→ start"
	}
}

// drawNode draws a node as in the wireframes: a box with its group as a
// small label and a summary under it; nodes with several outputs list them.
func (s *State) drawNode(area *ebiten.Image, drawn shape, docs frame.Docs) {
	rect := s.screenRect(drawn.rect)
	kind, _ := flow.KindOf(drawn.node.Kind)
	if drawn.node.Kind == flow.KindNote {
		ui.FillRound(area, rect, ui.Radius, ui.ColorBackground())
		strokeDashed(area, rect, ui.Border, ui.ColorInk())
		s.theme.DrawLabel(area, "Note", rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		for index, line := range s.wrap(textParam(drawn.node, "text"), drawn.rect.Dx()-ui.Half*textPad) {
			s.theme.DrawText(area, line, rect.Min.X+textPad, rect.Min.Y+titleDrop+index*noteLine, ui.ColorInk())
		}

		return
	}
	face := ui.ColorBackground()
	if coloured, ok := s.nodeTint(drawn.node.ID); ok {
		face = coloured.wash
	}
	ui.Box(area, rect, face)
	switch drawn.node.Kind {
	case flow.KindStart:
		s.theme.DrawLabel(area, flow.GroupFlow, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		s.theme.DrawStrong(area, "Start", rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorInk())
	case flow.KindTimeSwitch, flow.KindRandom:
		if drawn.node.Kind == flow.KindTimeSwitch {
			s.theme.DrawStrong(area, kind.Label, rect.Min.X+textPad, rect.Min.Y+(switchHead-ui.TextSize)/ui.Half-1,
				ui.ColorInk())
		} else {
			s.theme.DrawLabel(area, kind.Group, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
			s.theme.DrawStrong(area, kind.Label, rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorInk())
		}
		s.drawOutputRows(area, drawn)
	case flow.KindCall:
		s.drawCallNode(area, drawn, docs)
	case flow.KindReturn:
		s.theme.DrawLabel(area, kind.Label, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		s.theme.DrawStrong(area, s.clip(wayLabel(drawn.node), drawn.rect.Dx()-ui.Half*textPad), rect.Min.X+textPad,
			rect.Min.Y+titleDrop, ui.ColorInk())
	case flow.KindCheckValue, flow.KindFindNPC:
		s.theme.DrawLabel(area, kind.Group, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		s.theme.DrawText(area, s.clip(s.valueTitle(drawn.node, docs), drawn.rect.Dx()-ui.Half*textPad),
			rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorInk())
		s.drawOutputRows(area, drawn)
	default:
		s.theme.DrawLabel(area, kind.Group, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		title := kind.Label + " · " + s.summary(drawn.node, docs)
		if drawn.node.Kind == flow.KindSetValue {
			title = s.valueTitle(drawn.node, docs)
		}
		clipped := s.clip(title, drawn.rect.Dx()-ui.Half*textPad)
		s.theme.DrawText(area, clipped, rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorInk())
	}
	s.drawMark(area, drawn)
}

// drawOutputRows lists a time switch's times or a random choice's options,
// one row per output, the options with what their shares work out to and
// "+ option" under them.
func (s *State) drawOutputRows(area *ebiten.Image, drawn shape) {
	bottom := s.screenRect(drawn.rect).Max.Y
	for index, out := range drawn.ports {
		row := s.screenRect(out.row)
		if coloured, ok := s.outcomeTint(routeKey{from: drawn.node.ID, output: out.output.Name}); ok {
			washRow(area, row, row.Max.Y >= bottom-ui.Half, coloured.wash)
		}
		ui.FillRect(area, image.Rect(row.Min.X+ui.Hairline, row.Min.Y, row.Max.X-ui.Hairline, row.Min.Y+ui.Hairline),
			ui.ColorRule())
		label, ink := out.output.Label, ui.ColorInk()
		missing := s.missingWays[drawn.node.ID][out.output.Name]
		switch {
		case missing:
			ink = ui.ColorWarn()
		case drawn.node.Kind == flow.KindTimeSwitch && out.output.Name == flow.OutputOtherwise,
			drawn.node.Kind == flow.KindCall && out.output.Name == flow.OutputDone:
			ink = ui.ColorMuted()
		}
		textY := row.Min.Y + (outputRow-ui.MonoSize)/ui.Half - 1
		switch {
		case drawn.node.Kind == flow.KindTimeSwitch && index < len(drawn.ports)-1:
			s.theme.DrawMono(area, label, row.Min.X+textPad, textY, ink)
		case drawn.node.Kind == flow.KindRandom:
			share := shareLabel(drawn.node, index)
			shareX := row.Max.X - textPad - portRadius - s.theme.MonoWidth(share)
			s.theme.DrawText(area, s.clip(label, shareX-row.Min.X-ui.Half*textPad), row.Min.X+textPad,
				row.Min.Y+(outputRow-ui.TextSize)/ui.Half-1, ink)
			s.theme.DrawMono(area, share, shareX, textY, ui.ColorInk())
		default:
			s.theme.DrawText(area, label, row.Min.X+textPad, row.Min.Y+(outputRow-ui.TextSize)/ui.Half-1, ink)
		}
		if missing {
			middle := row.Min.Y + outputRow/ui.Half
			ui.FillRect(area, image.Rect(row.Min.X+textPad, middle, row.Min.X+textPad+s.theme.TextWidth(label),
				middle+ui.Hairline), ink)
		}
	}
	if drawn.node.Kind == flow.KindRandom {
		s.drawAddOption(area, drawn)
	}
}

// washRow fills an outcome's row with its tint's wash, inside the node's
// border; a row at the node's foot follows its rounded corners.
func washRow(area *ebiten.Image, row image.Rectangle, last bool, wash color.Color) {
	washed := image.Rect(row.Min.X+ui.Half, row.Min.Y, row.Max.X-ui.Half, row.Max.Y)
	if !last {
		ui.FillRect(area, washed, wash)

		return
	}
	washed.Max.Y -= ui.Half
	ui.FillRound(area, washed, ui.Radius, wash)
	ui.FillRect(area, image.Rect(washed.Min.X, washed.Min.Y, washed.Max.X, washed.Max.Y-int(ui.Radius)), wash)
}

// shareLabel is what a random choice's option works out to, as "25%".
func shareLabel(node flow.Node, index int) string {
	options, err := node.Shares("options")
	if err != nil || index >= len(options) {
		return ""
	}

	return strconv.Itoa(flow.Percentages(options)[index]) + "%"
}

// drawAddOption draws a random choice's "+ option" row, under its options.
func (s *State) drawAddOption(area *ebiten.Image, drawn shape) {
	row := s.screenRect(drawn.addOption)
	ui.FillRect(area, image.Rect(row.Min.X+ui.Hairline, row.Min.Y, row.Max.X-ui.Hairline, row.Min.Y+ui.Hairline),
		ui.ColorRule())
	ink := ui.ColorMuted()
	if s.cursor.In(row) {
		ink = ui.ColorSelect()
	}
	s.theme.DrawText(area, "+ option", row.Min.X+textPad, row.Min.Y+(outputRow-ui.TextSize)/ui.Half-1, ink)
}

// summary describes a node's parameters in a few words.
func (s *State) summary(node flow.Node, docs frame.Docs) string {
	switch node.Kind {
	case flow.KindGoTo:
		if choiceOf(node, flow.ParamGoes) == flow.GoesToValue {
			return s.placeValueLabel(node, docs)
		}
		destination, err := node.Anchor("destination")
		if err != nil {
			return "?"
		}
		name, _ := s.anchorLabel(docs, destination)
		if destination.IsZero() {
			return "no destination"
		}

		return name
	case flow.KindWait:
		wait, err := node.Duration("duration")
		if err != nil {
			return "?"
		}

		return formatDuration(wait)
	case flow.KindMove:
		direction, directionErr := node.Choice("direction")
		tiles, tilesErr := node.Number("tiles")
		if directionErr != nil || tilesErr != nil {
			return "?"
		}

		return direction + " " + strconv.Itoa(tiles) + " " + plural(tiles, "tile", "tiles")
	case flow.KindPlaySound:
		return s.playSummary(node)
	case flow.KindReturn:
		return wayLabel(node)
	case flow.KindCall:
		return s.callTitle(node, docs)
	case flow.KindSetValue, flow.KindCheckValue, flow.KindFindNPC:
		return s.valueSummary(node, docs)
	default:
		return ""
	}
}

// playSummary describes a Play sound: its cue, and how loud this time.
func (s *State) playSummary(node flow.Node) string {
	cue, err := node.Cue("cue")
	if err != nil {
		return "?"
	}
	if cue == "" {
		return "no cue"
	}
	if listed, found := s.listedCue(cue); found {
		cue = listed.Name
	}
	if step, found := loudness.Find(choiceOf(node, "loudness")); found {
		return cue + " · " + strings.ToLower(step.Label) + " this time"
	}

	return cue
}

// formatDuration writes a wait as "2 s", "30 min" or "1 h", adding "real"
// for a wait on a real clock.
func formatDuration(wait flow.Duration) string {
	unit := map[string]string{flow.UnitSeconds: "s", flow.UnitMinutes: "min", flow.UnitHours: "h"}[wait.Unit]
	written := strconv.Itoa(wait.Amount) + " " + unit
	if wait.Clock == flow.ClockReal {
		written += " real time"
	}

	return written
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

// drawContainer draws a time-bound container: a dashed box with a tinted
// heading that names it and gives its times.
func (s *State) drawContainer(area *ebiten.Image, drawn shape) {
	rect := s.screenRect(drawn.rect)
	ui.FillRect(area, rect, ui.HexAlpha(hexWhite, containerAlpha))
	head := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+containerHead)
	wash := ui.ColorGroup()
	if coloured, ok := s.nodeTint(drawn.node.ID); ok {
		wash = coloured.wash
	}
	ui.FillRect(area, head, wash)
	strokeDashed(area, rect, containerLine, ui.ColorInk())
	strokeDashed(area, image.Rect(head.Min.X, head.Max.Y, head.Max.X, head.Max.Y+1), ui.Border, ui.ColorInk())
	textY := head.Min.Y + (containerHead-ui.TextSize)/ui.Half - 1
	s.theme.DrawLabel(area, "Container", head.Min.X+textPad, head.Min.Y+(containerHead-ui.LabelSize)/ui.Half-1,
		ui.ColorMuted())
	nameX := head.Min.X + textPad + s.theme.LabelWidth("Container") + textPad
	s.theme.DrawStrong(area, textParam(drawn.node, "name"), nameX, textY, ui.ColorInk())
	if bound, err := drawn.node.TimeRange("bound"); err == nil {
		times := flow.FormatRange(bound) + " → exit"
		right := head.Max.X - textPad - portRadius
		if _, problem := s.problems[drawn.node.ID]; problem {
			right -= markInset + ui.Half*markRadius
		}
		s.theme.DrawMono(area, times, right-s.theme.MonoWidth(times), head.Min.Y+(containerHead-ui.MonoSize)/ui.Half-1,
			ui.ColorInk())
	}
	s.drawMark(area, drawn)
	corner := rect.Max
	for step := 0; step < resizeGrab; step += gripStep {
		ui.Line(area, image.Pt(corner.X-resizeGrab+step, corner.Y-gripInset),
			image.Pt(corner.X-gripInset, corner.Y-resizeGrab+step), ui.Hairline, ui.ColorMuted())
	}
}

// drawInspector draws the selected node's parameters, built from its kind's
// declarations.
func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	left, top := panel.Min.X+ui.Inset, panel.Min.Y+ui.Inset
	node, nodeSelected := s.selectedNode()
	if s.lineSelected() {
		s.drawLineInspector(screen, docs, left, top)

		return
	}
	if s.localsActive() {
		s.drawLocals(screen)

		return
	}
	if !nodeSelected || s.form == nil {
		s.theme.Heading(screen, "Inspector", left, top)
		s.drawHowToSelect(screen, left, top+ui.LineHeight)

		return
	}
	s.theme.Heading(screen, "Selected node", left, top)
	group := s.form.kind.Group
	if group == "" {
		group = flow.GroupFlow
	}
	s.theme.DrawTitle(screen, s.form.kind.Label, left, top+ui.LineHeight, ui.ColorInk())
	s.theme.DrawText(screen, "("+group+")", left+s.titleWidth(s.form.kind.Label)+ui.Gap*ui.Half,
		top+ui.LineHeight+ui.Half, ui.ColorMuted())
	s.drawControls(screen, docs, node, left)
	s.drawColours(screen)
	if s.form.remove != nil {
		s.form.remove.Draw(screen, s.theme)
	}
	s.save.Draw(screen, s.theme)
	hint := s.wrap(hintFor(node.Kind), panel.Dx()-ui.Half*ui.Inset)
	bottom := panel.Max.Y - ui.Inset
	switch {
	case !s.save.Rect.Empty():
		bottom = s.save.Rect.Min.Y - ui.Gap
	case s.form.remove != nil:
		bottom = s.form.remove.Rect.Min.Y - ui.Gap
	}
	for index, line := range hint {
		s.theme.DrawText(screen, line, left, bottom-(len(hint)-index)*ui.LineHeight, ui.ColorMuted())
	}
	if problem, has := s.problems[node.ID]; has {
		lines := s.wrap(problem, panel.Dx()-ui.Half*ui.Inset)
		above := bottom - (len(hint)+len(lines))*ui.LineHeight - ui.Gap
		for index, line := range lines {
			s.theme.DrawText(screen, line, left, above+index*ui.LineHeight, ui.ColorWarn())
		}
	}
}

// hasPartState answers the states about lines, problems, cues and local
// values, and whether the state is one of them.
func (s *State) hasPartState(state string) (bool, bool) {
	if found, handled := s.hasLineState(state); handled {
		return found, true
	}
	if found, handled := s.hasCueState(state); handled {
		return found, true
	}
	if found, handled := s.hasTintState(state); handled {
		return found, true
	}
	if state == "legend" {
		return s.legend.On, true
	}
	if strings.HasPrefix(state, localState) {
		return s.hasLocal(state), true
	}
	if found, handled := s.hasCallState(state); handled {
		return found, true
	}

	return false, false
}

// hasLineState answers the states about lines and problems, and whether the
// state is one of them.
func (s *State) hasLineState(state string) (bool, bool) {
	switch {
	case state == "line-selected":
		return s.lineSelected(), true
	case strings.HasPrefix(state, "bent:"):
		key, ok := s.lineNamed(strings.TrimPrefix(state, "bent:"))
		_, bent := s.workflow.Bend(key.from, key.output)

		return ok && bent, true
	case strings.HasPrefix(state, "far-label:"):
		key, ok := s.lineNamed(strings.TrimPrefix(state, "far-label:"))
		_, far := s.farLabel(key)

		return ok && far, true
	case strings.HasPrefix(state, "lit:"):
		where := strings.Split(strings.TrimPrefix(state, "lit:"), ":")
		if len(where) != portParts {
			return false, true
		}
		drawn, ok := s.nthNode(where[0] + ":" + where[1])

		return ok && s.lineLit(routeKey{from: drawn.node.ID, output: where[2]}), true
	case strings.HasPrefix(state, "problems:"):
		return strconv.Itoa(len(s.problems)) == strings.TrimPrefix(state, "problems:"), true
	case strings.HasPrefix(state, "problem:"):
		which, says, saying := strings.Cut(strings.TrimPrefix(state, "problem:"), "=")
		drawn, ok := s.nthNode(which)
		problem, marked := s.problems[drawn.node.ID]

		return ok && marked && (!saying || problem == says), true
	default:
		return false, false
	}
}

// drawLineInspector describes the selected line: the node and output it
// leaves from and the node it leads to.
func (s *State) drawLineInspector(screen *ebiten.Image, docs frame.Docs, left, top int) {
	s.theme.Heading(screen, "Selected line", left, top)
	s.theme.DrawTitle(screen, "Line", left, top+ui.LineHeight, ui.ColorInk())
	from, _ := s.workflow.Node(s.selectedLink.from)
	to, _ := s.targetOf(s.selectedLink)
	target, _ := s.workflow.Node(to)
	output := s.selectedLink.output
	for _, declared := range flow.Outputs(from) {
		if declared.Name == output {
			output = declared.Label
		}
	}
	textTop := panelTextTop(top)
	s.theme.DrawStrong(screen, "From", left, textTop, ui.ColorInk())
	s.theme.DrawText(screen, s.nodeLabel(from, docs)+" · "+output, left, textTop+ui.LineHeight, ui.ColorInk())
	s.theme.DrawStrong(screen, "To", left, textTop+ui.Half*ui.LineHeight+fieldSpace, ui.ColorInk())
	s.theme.DrawText(screen, s.nodeLabel(target, docs), left, textTop+3*ui.LineHeight+fieldSpace, ui.ColorInk())
	if s.loops[s.selectedLink] {
		s.theme.DrawText(screen, "It loops back, so it is dashed.", left, textTop+4*ui.LineHeight+ui.Half*fieldSpace,
			ui.ColorMuted())
	}
	s.drawColours(screen)
	s.deleteLine.Draw(screen, s.theme)
	s.resetLine.Draw(screen, s.theme)
	panel := frame.Inspector(s.size)
	text := "Drag a handle on the line to move that part of it. Delete or Backspace removes it. Undo puts it back."
	hint := s.wrap(text, panel.Dx()-ui.Half*ui.Inset)
	bottom := s.deleteLine.Rect.Min.Y - ui.Gap
	if !s.resetLine.Rect.Empty() {
		bottom = s.resetLine.Rect.Min.Y - ui.Gap
	}
	for index, line := range hint {
		s.theme.DrawText(screen, line, left, bottom-(len(hint)-index)*ui.LineHeight, ui.ColorMuted())
	}
}

// panelTextTop is where the inspector's text starts below its title.
func panelTextTop(top int) int { return top + ui.LineHeight + ui.TitleSize + ui.Inset }

// nodeLabel names a node in a few words: its kind and its summary.
func (s *State) nodeLabel(node flow.Node, docs frame.Docs) string {
	kind, _ := flow.KindOf(node.Kind)
	if summary := s.summary(node, docs); summary != "" {
		return kind.Label + " · " + summary
	}

	return kind.Label
}

func (s *State) titleWidth(value string) int {
	return int(float64(s.theme.StrongWidth(value)) * ui.TitleSize / ui.TextSize)
}

func (s *State) drawControl(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node, left int) {
	if made.hidden {
		return
	}
	s.theme.DrawStrong(screen, made.param.Label, left, made.label, ui.ColorInk())
	switch made.param.Type {
	case flow.TypeTiles, flow.TypeText:
		made.field.Draw(screen)
		if made.param.Type == flow.TypeTiles {
			s.drawUnit(screen, made.field, "tiles")
		}
	case flow.TypeChoice, flow.TypeDuration, flow.TypeShares:
		s.drawChoices(screen, made, node)
	case flow.TypeTimeRange:
		s.drawTimes(screen, made.from, made.to)
	case flow.TypeTimeRanges:
		for _, row := range made.rows {
			s.drawTimes(screen, row.from, row.to)
			row.remove.Draw(screen, s.theme)
		}
		made.addRange.Draw(screen, s.theme)
	case flow.TypeAnchor:
		s.drawAnchor(screen, docs, made, node, left)
	case flow.TypeCue:
		s.drawCue(screen, made, node)
	case flow.TypeValue:
		s.drawValue(screen, docs, made, node)
	case flow.TypeOperand:
		s.drawOperand(screen, docs, made, node, left)
	case flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort:
	}
}

// drawControls draws the selected node's parameters, and a Call's own part.
func (s *State) drawControls(screen *ebiten.Image, docs frame.Docs, node flow.Node, left int) {
	for index := range s.form.controls {
		s.drawControl(screen, docs, &s.form.controls[index], node, left)
	}
	if s.form.call != nil {
		s.drawCall(screen, docs, node, left)
	}
}

// drawAnchor draws an anchor parameter: its destination, where it is, and
// Pick on map and Clear.
func (s *State) drawAnchor(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node, left int) {
	destination, err := node.Anchor(made.param.Name)
	if err != nil {
		destination = floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}
	}
	name, detail := s.anchorLabel(docs, destination)
	ui.Box(screen, made.anchor, ui.ColorBackground())
	centre := image.Pt(made.anchor.Min.X+textPad+anchorDot, made.anchor.Min.Y+made.anchor.Dy()/ui.Half)
	vector.StrokeCircle(screen, float32(centre.X), float32(centre.Y), anchorDot, anchorRing, ui.ColorSelect(), true)
	ink := ui.ColorInk()
	if destination.IsZero() || strings.HasPrefix(name, "Broken") {
		ink = ui.ColorMuted()
	}
	s.theme.DrawText(screen, name, centre.X+anchorDot+textPad, made.anchor.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ink)
	s.theme.DrawText(screen, detail, left, made.anchor.Max.Y+ui.Gap, ui.ColorMuted())
	made.pick.Draw(screen, s.theme)
	made.clear.Draw(screen, s.theme)
}

// drawUnit writes a field's unit, faintly, at its right end.
func (s *State) drawUnit(screen *ebiten.Image, field *ui.TextField, unit string) {
	s.theme.DrawText(screen, unit, field.Rect.Max.X-s.theme.TextWidth(unit)-textPad,
		field.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorSubdued())
}

// drawChoices draws a choice's buttons, a wait's amount, units and clocks,
// or a random choice's options.
func (s *State) drawChoices(screen *ebiten.Image, made *control, node flow.Node) {
	for _, button := range slices.Concat(made.choices, made.units, made.clocks) {
		button.Draw(screen, s.theme)
	}
	switch made.param.Type {
	case flow.TypeDuration:
		made.field.Draw(screen)
		if wait, err := node.Duration(made.param.Name); err == nil {
			s.drawUnit(screen, made.field, wait.Unit)
		}
	case flow.TypeShares:
		top := made.label + ui.LineHeight
		if len(made.options) > 0 {
			s.theme.DrawSmall(screen, "Name", made.options[0].label.Rect.Min.X, top, ui.ColorMuted())
			s.theme.DrawSmall(screen, "Share", made.options[0].share.Rect.Min.X, top, ui.ColorMuted())
		}
		for _, option := range made.options {
			option.label.Draw(screen)
			option.share.Draw(screen)
			option.remove.Draw(screen, s.theme)
		}
		made.addOption.Draw(screen, s.theme)
	case flow.TypeChoice:
		if made.note {
			s.drawCompareNote(screen, made, node)
		}
	case flow.TypeAnchor, flow.TypeTiles, flow.TypeTimeRange, flow.TypeTimeRanges, flow.TypeText,
		flow.TypeCue, flow.TypeValue, flow.TypeOperand,
		flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort:
	}
}

func (s *State) drawTimes(screen *ebiten.Image, from, to *ui.TextField) {
	from.Draw(screen)
	to.Draw(screen)
	s.theme.DrawText(screen, "–", from.Rect.Max.X+(dashWidth-s.theme.TextWidth("–"))/ui.Half,
		from.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorInk())
}

// noteLines counts the lines of a node's problem and hint, as the inspector
// writes them above its footer.
func (s *State) noteLines(node flow.Node, width int) int {
	lines := len(s.wrap(hintFor(node.Kind), width))
	if problem, has := s.problems[node.ID]; has {
		lines += len(s.wrap(problem, width))
	}

	return lines
}

// hintFor explains what a kind of node does.
func hintFor(kind string) string {
	return map[string]string{
		flow.KindStart:      "Where the workflow begins. A branch with nothing after it comes back here.",
		flow.KindTimeSwitch: "Takes the output for the time now on the game clock; if no time matches, “otherwise”.",
		flow.KindRandom: "Takes one option at random. Each option is taken its share of the total; the node " +
			"shows what that works out to.",
		flow.KindContainer: "Repeats what is inside while its time lasts, then leaves by its exit, even partway " +
			"through a step.",
		flow.KindGoTo: "Walks to the anchor picked on a map, or to the place a place value holds as it starts.",
		flow.KindMove: "Moves this many tiles one way. If something is in the way, it stops there and carries on.",
		flow.KindWait: "Waits this long. Game time pauses and speeds up with the preview clock; real time " +
			"ignores it.",
		flow.KindPlaySound: "Plays the cue once, then carries on to the next node straight away. Leave its " +
			"loudness as the cue is, unless a moment must stand out or sit back.",
		flow.KindNote: "A note for authors. It is not part of the flow.",
		flow.KindCall: "Runs another workflow, then carries on from the output named for the way out it took, " +
			"or done. Its inputs are read once, as the call starts.",
		flow.KindReturn: "Goes back to whatever called this workflow, out of the Call's output of this name. " +
			"Two Returns with one name are one way out. Run as a behaviour, it starts again.",
		flow.KindSetValue: "Sets one of this workflow's own values: its NPC's, its zone's, the world's or a " +
			"local one. Add counts a number up, or down with a minus.",
		flow.KindCheckValue: "Takes “yes” when the value compares as asked, and “otherwise” when not. It can " +
			"read another NPC's values: one a local value holds, or one named by its path name.",
		flow.KindFindNPC: "Looks for the nearest other NPC on its floor within this many tiles, optionally " +
			"only one whose value matches, and keeps it in a local value for later nodes to read.",
	}[kind]
}

// noteHeight is how tall a note of a width must be to show its text.
func (s *State) noteHeight(node flow.Node, width int) int {
	lines := s.wrap(textParam(node, "text"), width-ui.Half*textPad)

	return max(noteMinHeight, titleDrop+len(lines)*noteLine+textPad)
}

// wrap breaks text into lines that fit a width, keeping its own line breaks.
func (s *State) wrap(text string, width int) []string { return s.theme.Wrap(text, width) }

// clip shortens text with an ellipsis to fit a width.
func (s *State) clip(text string, width int) string {
	if s.theme.TextWidth(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && s.theme.TextWidth(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}

	return string(runes) + "…"
}

// strokeDashed outlines a rectangle with a dashed line, as the wireframes
// draw containers and notes.
func strokeDashed(screen *ebiten.Image, rect image.Rectangle, width float32, ink color.Color) {
	edges := [][2]image.Point{
		{rect.Min, {X: rect.Max.X, Y: rect.Min.Y}}, {{X: rect.Max.X, Y: rect.Min.Y}, rect.Max},
		{rect.Max, {X: rect.Min.X, Y: rect.Max.Y}}, {{X: rect.Min.X, Y: rect.Max.Y}, rect.Min},
	}
	for _, edge := range edges {
		from, to := edge[0], edge[1]
		length := max(abs(to.X-from.X), abs(to.Y-from.Y))
		for start := 0; start < length; start += dashLength + dashGap {
			end := min(start+dashLength, length)
			ui.Line(screen, lerp(from, to, start, length), lerp(from, to, end, length), width, ink)
		}
	}
}

func lerp(from, to image.Point, step, length int) image.Point {
	if length == 0 {
		return from
	}

	return image.Pt(from.X+(to.X-from.X)*step/length, from.Y+(to.Y-from.Y)*step/length)
}

func mod(value, divisor int) int {
	return ((value % divisor) + divisor) % divisor
}

// Locate finds a named part of the window for input scripts: "palette:<kind>",
// "node:<kind>:<n>" for the nth node of a kind, "port:<kind>:<n>:<output>",
// "bend:<kind>:<n>:<output>" for the middle of a line's upright part or
// run, or with ":drop" or ":end" a back line's drop or end leg,
// "entry:<kind>:<n>" for its spare entry dot or "entry:<kind>:<n>:<i>" for
// its i-th filled one, "add-option:<kind>:<n>" for a
// random choice's "+ option", "canvas:X,Y" for a canvas point, "ui:<id>" for
// a button or field, "ui:swatch:<row>:<label>" for a colour swatch (see
// locateSwatch), a dialog button, "menu:<id>" for an item of the open
// right-click menu, or "state:..." (see hasState).
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.load(docs)
	s.layoutWidgets(docs)
	kind, value, _ := strings.Cut(name, ":")
	if point, found, handled := s.locateInOverlay(docs, kind, value); handled {
		return point, found
	}
	switch kind {
	case "state":
		if found, handled := s.hasValueState(value, docs); handled {
			return image.Pt(0, 0), found
		}

		return image.Pt(0, 0), s.hasState(value)
	case locateValue, locateDeclare:
		return image.Pt(0, 0), false
	case locateCue, locatePlay:
		return s.locateCueRow(kind, value)
	case "palette":
		for _, entry := range s.palette {
			if entry.kind.Name == value {
				return ui.Center(entry.rect), true
			}
		}
	case "ui":
		return s.locateWidget(value)
	case locateMenu:
		return s.contextMenu.Locate(value)
	case localPrefix:
		return s.locateLocal(value)
	default:
		if at, found := s.locateOnCanvas(kind, value); found {
			return s.onScreen(at), true
		}
	}

	return image.Pt(0, 0), false
}

// locateOnCanvas finds a node, a port, a spare entry dot, a "+ option" row
// or a canvas point, in canvas pixels.
func (s *State) locateOnCanvas(kind, value string) (image.Point, bool) {
	switch kind {
	case "node":
		if drawn, ok := s.nthNode(value); ok {
			return drawn.rect.Min.Add(image.Pt(textPad*ui.Half, textPad*ui.Half)), true
		}
	case "port":
		return s.locatePort(value)
	case "entry":
		return s.locateEntry(value)
	case "add-option":
		if drawn, ok := s.nthNode(value); ok && !drawn.addOption.Empty() {
			return ui.Center(drawn.addOption), true
		}
	case "line":
		return s.locateLine(value)
	case "bend":
		return s.locateBend(value)
	case "canvas":
		var x, y int
		if _, err := fmt.Sscanf(value, "%d,%d", &x, &y); err == nil {
			return image.Pt(x, y), true
		}
	}

	return image.Pt(0, 0), false
}

// locateEntry finds the spare entry dot of "<kind>:<n>", or the i-th filled
// one of "<kind>:<n>:<i>", counting from 1.
func (s *State) locateEntry(value string) (image.Point, bool) {
	where := strings.Split(value, ":")
	drawn, ok := s.nthNode(strings.Join(where[:min(len(where), ui.Half)], ":"))
	if !ok || !takesEntries(drawn.node.Kind) {
		return image.Pt(0, 0), false
	}
	if len(where) != portParts {
		return drawn.spare, true
	}
	number, err := strconv.Atoi(where[2])
	if err != nil || number < 1 || number > len(drawn.entries) {
		return image.Pt(0, 0), false
	}

	return drawn.entries[number-1].at, true
}

// locatePort finds the port of "<kind>:<n>:<output>".
func (s *State) locatePort(value string) (image.Point, bool) {
	where := strings.Split(value, ":")
	if len(where) != portParts {
		return image.Pt(0, 0), false
	}
	drawn, ok := s.nthNode(where[0] + ":" + where[1])
	if !ok {
		return image.Pt(0, 0), false
	}
	out, found := drawn.portOf(where[2])

	return out.at, found
}

// nthNode finds "<kind>:<n>", the nth node of a kind in creation order.
func (s *State) nthNode(value string) (shape, bool) {
	kind, number, _ := strings.Cut(value, ":")
	wanted, err := strconv.Atoi(number)
	if err != nil || s.workflow == nil {
		return noShape(), false
	}
	for _, node := range s.workflow.Nodes() {
		if node.Kind == kind {
			wanted--
			if wanted == 0 {
				return s.shapes[node.ID], true
			}
		}
	}

	return noShape(), false
}

// locateInOverlay finds a part of whatever is over the window and takes its
// input: the guard's dialog, the map picker's dialog or the value picker;
// states are still the window's own. It reports whether one was open.
func (s *State) locateInOverlay(docs frame.Docs, kind, value string) (image.Point, bool, bool) {
	switch {
	case s.guard.Active():
		point, found := s.guard.Locate(value, s.size)

		return point, found, true
	case s.picker.Dialog() != nil:
		point, found := s.picker.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})

		return point, found, true
	case s.values != nil && kind != "state":
		point, found := s.locateInPicker(docs, kind, value)

		return point, found, true
	default:
		return image.Pt(0, 0), false, false
	}
}

func (s *State) locateWidget(widget string) (image.Point, bool) {
	if rest, isSwatch := strings.CutPrefix(widget, "swatch:"); isSwatch {
		return s.locateSwatch(rest)
	}
	for _, button := range slices.Concat([]*ui.Button{s.arrange, s.recentre, s.legend, s.deleteLine, s.resetLine},
		s.zoomCard.buttons()) {
		if widget == button.ID && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}
	if point, found := s.locateLocalsWidget(widget); found {
		return point, true
	}

	return s.locateFormWidget(widget)
}

// locateFormWidget finds a field or button of the inspector's form.
func (s *State) locateFormWidget(widget string) (image.Point, bool) {
	if s.form == nil {
		return image.Pt(0, 0), false
	}
	if s.form.call != nil {
		if point, found := s.form.call.locate(widget); found {
			return point, true
		}
	}
	if s.form.remove != nil && widget == s.form.remove.ID {
		return ui.Center(s.form.remove.Rect), true
	}
	if point, found := s.save.Locate(widget); found {
		return point, true
	}
	for _, made := range s.form.controls {
		if point, found := made.locate(widget); found {
			return point, true
		}
	}

	return image.Pt(0, 0), false
}

// locate finds one of a control's fields or buttons, its cue as "cue", or
// a value parameter's box as "pick-<param>".
func (made control) locate(widget string) (image.Point, bool) {
	fields, buttons := made.widgets()
	for _, field := range fields {
		if field != nil && field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	if made.isBox(widget) && !made.cueBox.Empty() {
		return ui.Center(made.cueBox), true
	}
	for _, button := range slices.Concat(buttons, made.sources) {
		if button != nil && button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// widgets lists a control's text fields and buttons, some of them nil.
func (made control) widgets() ([]*ui.TextField, []*ui.Button) {
	fields := make([]*ui.TextField, 0, ui.Half*(ui.Half+len(made.rows)+len(made.options)))
	fields = append(fields, made.field, made.from, made.to, made.search)
	buttons := slices.Concat([]*ui.Button{made.addRange, made.pick, made.clear, made.addOption, made.listen},
		made.choices, made.units, made.clocks)
	for _, row := range made.rows {
		fields = append(fields, row.from, row.to)
		buttons = append(buttons, row.remove)
	}
	for _, option := range made.options {
		fields = append(fields, option.label, option.share)
		buttons = append(buttons, option.remove)
	}

	return fields, buttons
}

// hasState answers input scripts waiting for the window: "loaded", "nodes:N"
// when the workflow holds N nodes, "selected:<kind>" when a node of that kind
// is selected, "picking" while a pick is in progress, "destination" once
// the selected Go to has one, "entries:<kind>:<n>=C" when C links come into
// the nth node of a kind, "line-selected" while a line is selected,
// "lit:<kind>:<n>:<output>" while that line is drawn blue,
// "problems:N" when N nodes have problems and "problem:<kind>:<n>", or
// "problem:<kind>:<n>=<problem>" for what it says, when the
// nth node of a kind has one.
func (s *State) hasState(state string) bool {
	if found, handled := s.hasPartState(state); handled {
		return found
	}
	switch {
	case strings.HasPrefix(state, "entries:"):
		which, count, _ := strings.Cut(strings.TrimPrefix(state, "entries:"), "=")
		drawn, ok := s.nthNode(which)

		return ok && strconv.Itoa(len(drawn.entries)) == count
	case state == "loaded":
		return s.workflow != nil
	case strings.HasPrefix(state, "nodes:"):
		count, err := strconv.Atoi(strings.TrimPrefix(state, "nodes:"))

		return err == nil && s.workflow != nil && len(s.workflow.Nodes()) == count
	case strings.HasPrefix(state, "selected:"):
		node, ok := s.selectedNode()

		return ok && node.Kind == strings.TrimPrefix(state, "selected:")
	case state == "picking":
		return s.picker.Picking()
	case state == "destination":
		node, ok := s.selectedNode()
		destination, err := node.Anchor("destination")

		return ok && err == nil && !destination.IsZero()
	}

	return false
}

// choiceOf is a choice parameter's value, empty when it has none.
func choiceOf(node flow.Node, name string) string {
	chosen, err := node.Choice(name)
	if err != nil {
		return ""
	}

	return chosen
}
