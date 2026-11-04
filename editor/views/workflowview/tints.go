package workflowview

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A tint is one of the colours an author can give a node, a container, an
// outcome or a line to keep a workflow organised: a wash for a node's face
// and a darker ink for a line, its port and an outcome's row. The layout
// holds a tint's name, never its colours, so these can change. There is no
// blue, which is selection, and no red, which is problems.
type tint struct {
	name string
	wash color.RGBA
	ink  color.RGBA
}

// The tints' colours: the editor's family hues, and amber.
const (
	hexGreenWash  = 0xe3f0ec
	hexGreenInk   = 0x3d7a4b
	hexAmberWash  = 0xfdf1dc
	hexAmberInk   = 0x9a6700
	hexTealWash   = 0xdff2f3
	hexTealInk    = 0x267378
	hexPurpleWash = 0xece6f5
	hexPurpleInk  = 0x6a4f9e
	hexPinkWash   = 0xf7e4ef
	hexPinkInk    = 0x8a4b72
	hexBrownWash  = 0xf1e9e1
	hexBrownInk   = 0x7a6248
	hexSlateWash  = 0xe6ebef
	hexSlateInk   = 0x4c6377
)

// tints lists the tints in the order the inspector offers them.
func tints() []tint {
	return []tint{
		{name: "green", wash: ui.Hex(hexGreenWash), ink: ui.Hex(hexGreenInk)},
		{name: "amber", wash: ui.Hex(hexAmberWash), ink: ui.Hex(hexAmberInk)},
		{name: "teal", wash: ui.Hex(hexTealWash), ink: ui.Hex(hexTealInk)},
		{name: "purple", wash: ui.Hex(hexPurpleWash), ink: ui.Hex(hexPurpleInk)},
		{name: "pink", wash: ui.Hex(hexPinkWash), ink: ui.Hex(hexPinkInk)},
		{name: "brown", wash: ui.Hex(hexBrownWash), ink: ui.Hex(hexBrownInk)},
		{name: "slate", wash: ui.Hex(hexSlateWash), ink: ui.Hex(hexSlateInk)},
	}
}

// tintNamed finds a tint by name; "" and TintNone name none.
func tintNamed(name string) (tint, bool) {
	for _, candidate := range tints() {
		if candidate.name == name {
			return candidate, true
		}
	}
	var none tint

	return none, false
}

// nodeTint is the tint of a node or container, if it has one.
func (s *State) nodeTint(id ids.ULID) (tint, bool) { return tintNamed(s.tints.Nodes[id]) }

// outcomeTint is the tint of an output, if it has one.
func (s *State) outcomeTint(key routeKey) (tint, bool) {
	return tintNamed(s.tints.Outcomes[flow.LineKey(key.from, key.output)])
}

// lineTint is a line's tint: its own when it has one, none when its own is
// TintNone, and its outcome's otherwise.
func (s *State) lineTint(key routeKey) (tint, bool) {
	if own, set := s.tints.Lines[flow.LineKey(key.from, key.output)]; set {
		return tintNamed(own)
	}

	return s.outcomeTint(key)
}

// Colour rows in the inspector, in screen pixels.
const (
	swatchSize = 18
	swatchGap  = 8
	swatchRing = 3
	swatchDot  = 4
)

// What a colour row sets, and the swatches every row has besides the tints.
const (
	rowNode       = "node"
	rowOutcome    = "outcome"
	rowLine       = "line"
	swatchNone    = "none"
	swatchInherit = "inherit"
)

// swatch is one choice in a colour row: a tint by name, none, or for a line
// inherit, its outcome's. value is what the tint edit sets: "" takes a tint
// away, TintNone gives a line no colour over its outcome's.
type swatch struct {
	label string
	value string
	rect  image.Rectangle
}

// colourRow sets one tint: a node's or container's, an outcome's, or a
// line's. current is the label of the swatch that is chosen.
type colourRow struct {
	title    string
	note     string
	kind     string
	node     ids.ULID
	key      string
	current  string
	swatches []swatch
	left     int
	label    int
}

// plainSwatches are none and every tint, for a node's or an outcome's row.
func plainSwatches() []swatch {
	return tintSwatches([]swatch{{label: swatchNone, value: "", rect: ui.Zero()}})
}

// tintSwatches adds a swatch for every tint after the given ones.
func tintSwatches(first []swatch) []swatch {
	row := make([]swatch, 0, len(first)+len(tints()))
	row = append(row, first...)
	for _, choice := range tints() {
		row = append(row, swatch{label: choice.name, value: choice.name, rect: ui.Zero()})
	}

	return row
}

// nameOrNone is a tint's name as a swatch label: none when there is none.
func nameOrNone(name string) string {
	if name == "" {
		return swatchNone
	}

	return name
}

// colourRows builds the colour rows for the selection: a line's one row,
// whose colour may come from its outcome; a node's or container's, and one
// per outcome when it has several outputs.
func (s *State) colourRows() []colourRow {
	if s.lineSelected() {
		return []colourRow{s.lineColourRow()}
	}
	node, ok := s.selectedNode()
	if !ok || node.Kind == flow.KindNote {
		return nil
	}
	rows := []colourRow{{
		title: "Colour", note: "", kind: rowNode, node: node.ID, key: "",
		current: nameOrNone(s.tints.Nodes[node.ID]), swatches: plainSwatches(), left: 0, label: 0,
	}}
	var outcomes []flow.Output
	for _, out := range flow.Outputs(node) {
		if !flow.IsInner(node, out.Name) {
			outcomes = append(outcomes, out)
		}
	}
	if len(outcomes) < ui.Half {
		return rows
	}
	for _, out := range outcomes {
		key := flow.LineKey(node.ID, out.Name)
		rows = append(rows, colourRow{
			title: out.Label, note: "its lines take it", kind: rowOutcome, node: node.ID, key: key,
			current: nameOrNone(s.tints.Outcomes[key]), swatches: plainSwatches(), left: 0, label: 0,
		})
	}

	return rows
}

// lineColourRow is the selected line's row: inherit, none and the tints,
// with a note saying where its colour comes from.
func (s *State) lineColourRow() colourRow {
	key := flow.LineKey(s.selectedLink.from, s.selectedLink.output)
	current, note := swatchInherit, "from its outcome"
	if own, set := s.tints.Lines[key]; set {
		current, note = own, "set on this line"
	}
	if _, has := s.outcomeTint(s.selectedLink); !has && current == swatchInherit {
		note = "its outcome has none"
	}
	swatches := tintSwatches([]swatch{
		{label: swatchInherit, value: "", rect: ui.Zero()}, {label: swatchNone, value: flow.TintNone, rect: ui.Zero()},
	})

	return colourRow{
		title: "Colour", note: note, kind: rowLine, node: s.selectedLink.from, key: key, current: current,
		swatches: swatches, left: 0, label: 0,
	}
}

// layoutColours places the colour rows from a top, each its title then its
// swatches, wrapped to the width.
func (s *State) layoutColours(rows []colourRow, left, width, top int) []colourRow {
	for index := range rows {
		row := &rows[index]
		row.left, row.label = left, top
		top += ui.LineHeight
		column := left
		for which := range row.swatches {
			if column+swatchSize > left+width {
				column, top = left, top+swatchSize+swatchGap
			}
			row.swatches[which].rect = image.Rect(column, top, column+swatchSize, top+swatchSize)
			column += swatchSize + swatchGap
		}
		top += swatchSize + fieldSpace
	}

	return rows
}

// updateColours sets a tint when a swatch is clicked.
func (s *State) updateColours(input *ui.Input, ctx *frame.Context) {
	if !input.Pressed {
		return
	}
	for _, row := range s.colours {
		for _, choice := range row.swatches {
			if input.Cursor.In(choice.rect) {
				s.setTint(ctx, row, choice.value)

				return
			}
		}
	}
}

// setTint sends the tint edit for one row.
func (s *State) setTint(ctx *frame.Context, row colourRow, value string) {
	args := workflowplugin.TintArgs{Tints: flow.Tints{Nodes: nil, Outcomes: nil, Lines: nil}}
	switch row.kind {
	case rowNode:
		args.Tints.Nodes = map[ids.ULID]string{row.node: value}
	case rowOutcome:
		args.Tints.Outcomes = map[string]string{row.key: value}
	default:
		args.Tints.Lines = map[string]string{row.key: value}
	}
	s.command(ctx, workflowplugin.TintEdit, "", args, nil)
}

// drawColours draws the colour rows: a title, a note on where a line's
// colour comes from, and a circle for each swatch, the chosen one ringed.
func (s *State) drawColours(screen *ebiten.Image) {
	for _, row := range s.colours {
		s.theme.DrawStrong(screen, row.title, row.left, row.label, ui.ColorInk())
		if row.note != "" {
			s.theme.DrawText(screen, row.note, row.left+s.theme.StrongWidth(row.title)+ui.Gap, row.label, ui.ColorMuted())
		}
		for _, choice := range row.swatches {
			s.drawSwatch(screen, row, choice)
		}
	}
}

// drawSwatch draws one swatch: a tint's wash ringed in its ink, none as an
// empty circle, inherit as a dot of the outcome's ink in an empty circle.
func (s *State) drawSwatch(screen *ebiten.Image, row colourRow, choice swatch) {
	centre := ui.Center(choice.rect)
	centreX, centreY := float32(centre.X), float32(centre.Y)
	fill, ring := ui.ColorBackground(), ui.ColorRule()
	if coloured, ok := tintNamed(choice.label); ok {
		fill, ring = coloured.wash, coloured.ink
	}
	vector.FillCircle(screen, centreX, centreY, swatchSize/ui.Half, fill, true)
	vector.StrokeCircle(screen, centreX, centreY, swatchSize/ui.Half, ui.Border, ring, true)
	if choice.label == swatchInherit {
		dot := ui.ColorRule()
		_, output, _ := flow.SplitLineKey(row.key)
		if from, ok := s.outcomeTint(routeKey{from: row.node, output: output}); ok {
			dot = from.ink
		}
		vector.FillCircle(screen, centreX, centreY, swatchDot, dot, true)
	}
	if choice.label == row.current {
		vector.StrokeCircle(screen, centreX, centreY, swatchSize/ui.Half+swatchRing, ui.Border, ui.ColorSelect(), true)
	}
}

// locateSwatch finds a swatch from "<row>:<label>": the row "node", "line"
// or "outcome:<output>".
func (s *State) locateSwatch(value string) (image.Point, bool) {
	kind, rest, _ := strings.Cut(value, ":")
	output, label := "", rest
	if kind == rowOutcome {
		output, label, _ = strings.Cut(rest, ":")
	}
	for _, row := range s.colours {
		if row.kind != kind || (kind == rowOutcome && !strings.HasSuffix(row.key, "/"+output)) {
			continue
		}
		for _, choice := range row.swatches {
			if choice.label == label {
				return ui.Center(choice.rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// hasTintState answers "tint:<kind>:<n>=<name>" for a node's tint,
// "outcome-tint:<kind>:<n>:<output>=<name>" for an outcome's and
// "line-tint:<kind>:<n>:<output>=<name>" for a line's, as drawn; the name
// is empty for none.
func (s *State) hasTintState(state string) (bool, bool) {
	which, name, found := strings.Cut(state, "=")
	kind, where, _ := strings.Cut(which, ":")
	if !found {
		return false, false
	}
	switch kind {
	case "tint":
		drawn, ok := s.nthNode(where)

		return ok && s.tints.Nodes[drawn.node.ID] == name, true
	case "outcome-tint":
		key, ok := s.lineNamed(where)
		coloured, has := s.outcomeTint(key)

		return ok && (has && coloured.name == name || !has && name == ""), true
	case "line-tint":
		key, ok := s.lineNamed(where)
		coloured, has := s.lineTint(key)

		return ok && (has && coloured.name == name || !has && name == ""), true
	default:
		return false, false
	}
}
