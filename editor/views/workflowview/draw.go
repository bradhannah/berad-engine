package workflowview

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
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
	s.size = screen.Bounds().Size()
	s.load(docs)
	s.layoutWidgets(docs)
	screen.Fill(ui.ColorBackground())
	canvas := s.Canvas()
	if area, ok := screen.SubImage(canvas).(*ebiten.Image); ok {
		s.drawCanvas(area, docs)
	}
	s.drawToolbar(screen, docs)
	s.drawPalette(screen)
	s.drawInspector(screen, docs)
	if dialog := s.picker.Dialog(); dialog != nil {
		dialog.Draw(screen, s.theme)
	}
}

func (s *State) drawToolbar(screen *ebiten.Image, docs frame.Docs) {
	content := frame.Content(s.size)
	textY := (toolbarHeight - ui.TextSize) / ui.Half
	trail := "Workflows › "
	s.theme.DrawText(screen, trail, ui.Inset, textY, ui.ColorMuted())
	s.theme.DrawStrong(screen, s.name(docs), ui.Inset+s.theme.TextWidth(trail), textY, ui.ColorInk())
	s.arrange.Draw(screen, s.theme)
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
	canvas := s.Canvas()
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
		if s.shapes[nodeID].node.Kind == flow.KindContainer {
			s.drawContainer(area, s.shapes[nodeID])
		}
	}
	for _, nodeID := range order {
		s.drawLinks(area, s.shapes[nodeID])
	}
	for _, nodeID := range order {
		if s.shapes[nodeID].node.Kind != flow.KindContainer {
			s.drawNode(area, s.shapes[nodeID], docs)
		}
	}
	for _, nodeID := range order {
		s.drawPorts(area, s.shapes[nodeID])
	}
	if selected, ok := s.shapes[s.selected]; ok {
		ui.StrokeRound(area, s.screenRect(selected.rect).Inset(-selectGap), ui.Radius+selectGap, selectWidth,
			ui.ColorSelect())
	}
	if s.link != nil {
		if from, ok := s.shapes[s.link.from].portOf(s.link.output); ok {
			ui.Line(area, s.toScreen(from.at), s.toScreen(s.link.to), lineWidth, ui.ColorSelect())
		}
	}
}

func (s *State) screenRect(rect image.Rectangle) image.Rectangle {
	return rect.Add(s.Canvas().Min).Add(s.offset)
}

// drawLinks draws a node's connections: square-cornered lines from each
// connected output to the node it leads to, with an arrowhead.
func (s *State) drawLinks(area *ebiten.Image, from shape) {
	for _, out := range from.ports {
		to, linked := from.node.Next[out.output.Name]
		target, ok := s.shapes[to]
		if !linked || !ok {
			continue
		}
		var corners []image.Point
		if out.output.Name == flow.OutputInside {
			end := image.Pt(target.rect.Min.X, target.rect.Min.Y+min(target.rect.Dy(), nodeHeight)/ui.Half)
			corners = []image.Point{out.at, {X: out.at.X, Y: end.Y}, end}
		} else {
			corners = route(out.at, target.rect, from.rect)
		}
		for index := 1; index < len(corners); index++ {
			ui.Line(area, s.toScreen(corners[index-1]), s.toScreen(corners[index]), lineWidth, ui.ColorInk())
		}
		tip := s.toScreen(corners[len(corners)-1])
		var arrow vector.Path
		arrow.MoveTo(float32(tip.X), float32(tip.Y))
		arrow.LineTo(float32(tip.X-arrowSize*ui.Half), float32(tip.Y-arrowSize))
		arrow.LineTo(float32(tip.X-arrowSize*ui.Half), float32(tip.Y+arrowSize))
		arrow.Close()
		fill := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
		fill.ColorScale.ScaleWithColor(ui.ColorInk())
		vector.FillPath(area, &arrow, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, fill)
	}
}

// drawPorts draws each output's port: filled when connected, hollow with
// where the flow goes instead when not.
func (s *State) drawPorts(area *ebiten.Image, from shape) {
	for _, out := range from.ports {
		centre := s.toScreen(out.at)
		_, linked := from.node.Next[out.output.Name]
		fill := ui.ColorBackground()
		if linked {
			fill = ui.ColorInk()
		}
		vector.FillCircle(area, float32(centre.X), float32(centre.Y), portRadius, fill, true)
		vector.StrokeCircle(area, float32(centre.X), float32(centre.Y), portRadius, ui.Border, ui.ColorInk(), true)
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
	ui.Box(area, rect, ui.ColorBackground())
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
		for index, out := range drawn.ports {
			row := s.screenRect(out.row)
			ui.FillRect(area, image.Rect(row.Min.X+ui.Hairline, row.Min.Y, row.Max.X-ui.Hairline, row.Min.Y+ui.Hairline),
				ui.ColorRule())
			label, ink := outputLabel(drawn.node, out.output), ui.ColorInk()
			if out.output.Name == flow.OutputOtherwise {
				ink = ui.ColorMuted()
			}
			textY := row.Min.Y + (outputRow-ui.MonoSize)/ui.Half - 1
			if drawn.node.Kind == flow.KindTimeSwitch && index < len(drawn.ports)-1 {
				s.theme.DrawMono(area, label, row.Min.X+textPad, textY, ink)
			} else {
				s.theme.DrawText(area, label, row.Min.X+textPad, row.Min.Y+(outputRow-ui.TextSize)/ui.Half-1, ink)
			}
		}
	default:
		s.theme.DrawLabel(area, kind.Group, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		title := kind.Label + " · " + s.summary(drawn.node, docs)
		clipped := s.clip(title, drawn.rect.Dx()-ui.Half*textPad)
		s.theme.DrawText(area, clipped, rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorInk())
	}
}

// outputLabel names an output in a node's list of outputs.
func outputLabel(node flow.Node, output flow.Output) string {
	if node.Kind == flow.KindRandom && output.Name == flow.OutputYes {
		chance, err := node.Number("chance")
		if err == nil {
			return "chance " + strconv.Itoa(chance) + "%"
		}
	}

	return output.Label
}

// summary describes a node's parameters in a few words.
func (s *State) summary(node flow.Node, docs frame.Docs) string {
	switch node.Kind {
	case flow.KindGoTo:
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
		minutes, err := node.Number("minutes")
		if err != nil {
			return "?"
		}

		return strconv.Itoa(minutes) + " min"
	default:
		return ""
	}
}

// drawContainer draws a time-bound container: a dashed box with a tinted
// heading that names it and gives its times.
func (s *State) drawContainer(area *ebiten.Image, drawn shape) {
	rect := s.screenRect(drawn.rect)
	ui.FillRect(area, rect, ui.HexAlpha(hexWhite, containerAlpha))
	head := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+containerHead)
	ui.FillRect(area, head, ui.ColorGroup())
	strokeDashed(area, rect, containerLine, ui.ColorInk())
	strokeDashed(area, image.Rect(head.Min.X, head.Max.Y, head.Max.X, head.Max.Y+1), ui.Border, ui.ColorInk())
	textY := head.Min.Y + (containerHead-ui.TextSize)/ui.Half - 1
	s.theme.DrawLabel(area, "Container", head.Min.X+textPad, head.Min.Y+(containerHead-ui.LabelSize)/ui.Half-1,
		ui.ColorMuted())
	nameX := head.Min.X + textPad + s.theme.LabelWidth("Container") + textPad
	s.theme.DrawStrong(area, textParam(drawn.node, "name"), nameX, textY, ui.ColorInk())
	if bound, err := drawn.node.TimeRange("bound"); err == nil {
		times := flow.FormatRange(bound) + " → exit"
		s.theme.DrawMono(area, times, head.Max.X-textPad-s.theme.MonoWidth(times)-portRadius,
			head.Min.Y+(containerHead-ui.MonoSize)/ui.Half-1, ui.ColorInk())
	}
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
	node, ok := s.selectedNode()
	if !ok || s.form == nil {
		s.theme.Heading(screen, "Inspector", left, top)
		lines := []string{
			"Select a node to change it, or", "add one from the palette: click", "to add after the selected node,",
			"or drag onto the canvas.",
		}
		for index, line := range lines {
			s.theme.DrawText(screen, line, left, top+(index+1)*ui.LineHeight, ui.ColorMuted())
		}

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
	for index := range s.form.controls {
		s.drawControl(screen, docs, &s.form.controls[index], node, left)
	}
	if s.form.remove != nil {
		s.form.remove.Draw(screen, s.theme)
	}
	hint := s.wrap(hintFor(node.Kind), panel.Dx()-ui.Half*ui.Inset)
	bottom := panel.Max.Y - ui.Inset - ui.ButtonH - ui.Gap
	for index, line := range hint {
		s.theme.DrawText(screen, line, left, bottom-(len(hint)-index)*ui.LineHeight, ui.ColorMuted())
	}
}

func (s *State) titleWidth(value string) int {
	return int(float64(s.theme.StrongWidth(value)) * ui.TitleSize / ui.TextSize)
}

func (s *State) drawControl(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node, left int) {
	s.theme.DrawStrong(screen, made.param.Label, left, made.label, ui.ColorInk())
	switch made.param.Type {
	case flow.TypeMinutes, flow.TypePercent, flow.TypeText:
		made.field.Draw(screen)
		unit := map[flow.ParamType]string{flow.TypeMinutes: "game minutes", flow.TypePercent: "per cent"}[made.param.Type]
		if unit != "" {
			s.theme.DrawText(screen, unit, made.field.Rect.Max.X-s.theme.TextWidth(unit)-textPad,
				made.field.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorSubdued())
		}
	case flow.TypeTimeRange:
		s.drawTimes(screen, made.from, made.to)
	case flow.TypeTimeRanges:
		for _, row := range made.rows {
			s.drawTimes(screen, row.from, row.to)
			row.remove.Draw(screen, s.theme)
		}
		made.addRange.Draw(screen, s.theme)
	case flow.TypeAnchor:
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
}

func (s *State) drawTimes(screen *ebiten.Image, from, to *ui.TextField) {
	from.Draw(screen)
	to.Draw(screen)
	s.theme.DrawText(screen, "–", from.Rect.Max.X+(dashWidth-s.theme.TextWidth("–"))/ui.Half,
		from.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorInk())
}

// hintFor explains what a kind of node does.
func hintFor(kind string) string {
	return map[string]string{
		flow.KindStart:      "Where the workflow begins. A branch with nothing after it comes back here.",
		flow.KindTimeSwitch: "Takes the output for the time now on the game clock; if no time matches, “otherwise”.",
		flow.KindRandom:     "Takes “chance” that often, at random; otherwise the other output.",
		flow.KindContainer: "Repeats what is inside while its time lasts, then leaves by its exit, even partway " +
			"through a step.",
		flow.KindGoTo: "Walks to the anchor.",
		flow.KindWait: "Waits this many game minutes.",
		flow.KindNote: "A note for authors. It is not part of the flow.",
	}[kind]
}

// noteHeight is how tall a note of a width must be to show its text.
func (s *State) noteHeight(node flow.Node, width int) int {
	lines := s.wrap(textParam(node, "text"), width-ui.Half*textPad)

	return max(noteMinHeight, titleDrop+len(lines)*noteLine+textPad)
}

// wrap breaks text into lines that fit a width, keeping its own line breaks.
func (s *State) wrap(text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := strings.TrimSpace(line + " " + word)
			if line != "" && s.theme.TextWidth(candidate) > width {
				lines = append(lines, line)
				candidate = word
			}
			line = candidate
		}
		lines = append(lines, line)
	}

	return lines
}

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
// "canvas:X,Y" for a canvas point, "ui:<id>" for a button or field, a dialog
// button, or "state:..." (see hasState).
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.load(docs)
	s.layoutWidgets(docs)
	kind, value, _ := strings.Cut(name, ":")
	if s.picker.Dialog() != nil {
		return s.picker.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	}
	switch kind {
	case "state":
		return image.Pt(0, 0), s.hasState(value)
	case "palette":
		for _, entry := range s.palette {
			if entry.kind.Name == value {
				return ui.Center(entry.rect), true
			}
		}
	case "node":
		if drawn, ok := s.nthNode(value); ok {
			return s.toScreen(drawn.rect.Min.Add(image.Pt(textPad*ui.Half, textPad*ui.Half))), true
		}
	case "port":
		where := strings.Split(value, ":")
		if len(where) == portParts {
			if drawn, ok := s.nthNode(where[0] + ":" + where[1]); ok {
				if out, found := drawn.portOf(where[2]); found {
					return s.toScreen(out.at), true
				}
			}
		}
	case "canvas":
		var x, y int
		if _, err := fmt.Sscanf(value, "%d,%d", &x, &y); err == nil {
			return s.toScreen(image.Pt(x, y)), true
		}
	case "ui":
		return s.locateWidget(value)
	}

	return image.Pt(0, 0), false
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

func (s *State) locateWidget(widget string) (image.Point, bool) {
	if widget == s.arrange.ID {
		return ui.Center(s.arrange.Rect), true
	}
	if s.form == nil {
		return image.Pt(0, 0), false
	}
	if s.form.remove != nil && widget == s.form.remove.ID {
		return ui.Center(s.form.remove.Rect), true
	}
	for _, made := range s.form.controls {
		var fields []*ui.TextField
		var buttons []*ui.Button
		fields = append(fields, made.field, made.from, made.to)
		buttons = append(buttons, made.addRange, made.pick, made.clear)
		for _, row := range made.rows {
			fields = append(fields, row.from, row.to)
			buttons = append(buttons, row.remove)
		}
		for _, field := range fields {
			if field != nil && field.ID == widget {
				return field.Center(), true
			}
		}
		for _, button := range buttons {
			if button != nil && button.ID == widget {
				return ui.Center(button.Rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts waiting for the window: "loaded", "nodes:N"
// when the workflow holds N nodes, "selected:<kind>" when a node of that kind
// is selected, "picking" while a pick is in progress, and "destination" once
// the selected Go to has one.
func (s *State) hasState(state string) bool {
	switch {
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
