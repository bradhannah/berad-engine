package workflowview

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A Together on the canvas is a container whose lanes are bands one under
// another, each with its number and name at its left and the dot it starts
// from; "+ lane" runs along its foot, and all done leaves from its head.
const (
	// laneHead is the room at a band's left for its number and name, and
	// bandPad the room above and below a lane's steps.
	laneHead = 92
	bandPad  = 10
	laneMinH = nodeHeight + ui.Half*bandPad
	// laneBadge is a lane number's circle.
	laneBadge = 8
	// locateLane names a lane's band for input scripts.
	locateLane = "lane"
)

// Problems a node in a lane can have.
const (
	problemNotInLane = "A lane holds steps in a line: a Call or a Return can't go in one yet. Put it before or " +
		"after the Together."
	problemTwoLanes  = "It is in two lanes, %s and %s: give each lane steps of its own."
	problemTwoMovers = "Only one lane can move the NPC, and %s already does. Move it there, or out of the Together."
)

// togetherMinH is the least height a Together of a number of lanes needs.
func togetherMinH(lanes int) int {
	return containerHead + max(lanes, flow.MinLanes)*laneMinH + outputRow
}

// laneBands splits a Together into its lanes' bands, equal in height,
// between its head and its "+ lane" row.
func laneBands(rect image.Rectangle, count int) []image.Rectangle {
	bands := make([]image.Rectangle, 0, count)
	top, bottom := rect.Min.Y+containerHead, rect.Max.Y-outputRow
	for index := range count {
		bands = append(bands, image.Rect(rect.Min.X, top+(bottom-top)*index/count, rect.Max.X,
			top+(bottom-top)*(index+1)/count))
	}

	return bands
}

// laneStart is where a lane's dot sits in its band, level with the first
// entry dot of a step at the band's top.
func laneStart(band image.Rectangle) image.Point {
	return image.Pt(band.Min.X+laneHead, band.Min.Y+bandPad+entryTop)
}

// togetherShape lays out a Together: its lanes' dots in their bands, all
// done on the right of its head, and its "+ lane" row.
func togetherShape(made shape, box flow.Box, outputs []flow.Output, entries int) shape {
	lanes := len(outputs) - 1
	width, height := max(box.W, containerMinW), max(box.H, togetherMinH(lanes), entries)
	origin := image.Pt(box.X, box.Y)
	made.rect = image.Rectangle{Min: origin, Max: origin.Add(image.Pt(width, height))}
	bands := laneBands(made.rect, max(lanes, 1))
	for index, output := range outputs {
		at := image.Pt(made.rect.Max.X, made.rect.Min.Y+containerHead/ui.Half)
		if output.Name != flow.OutputDone && index < len(bands) {
			at = laneStart(bands[index])
		}
		made.ports = append(made.ports, port{output: output, at: at, row: image.Rect(0, 0, 0, 0)})
	}
	made.addOption = image.Rect(made.rect.Min.X, made.rect.Max.Y-outputRow, made.rect.Max.X, made.rect.Max.Y)

	return made
}

// innerOf is where a container's contents sit: inside its border, and in a
// Together right of its lanes' names and above its "+ lane" row.
func innerOf(drawn shape) image.Rectangle {
	area := inside(drawn.rect)
	if drawn.node.Kind == flow.KindTogether {
		area.Min.X = drawn.rect.Min.X + laneHead + containerPad
		area.Max.Y = drawn.rect.Max.Y - outputRow
	}

	return area
}

// containerMin is the least size a container can be resized to.
func containerMin(node flow.Node) image.Point {
	if node.Kind == flow.KindTogether {
		lanes, _ := node.Lanes() //nolint:errcheck // A Together that can't be read has no lanes.

		return image.Pt(containerMinW, togetherMinH(len(lanes)))
	}

	return image.Pt(containerMinW, containerMinH)
}

// arrangeTogether lays out a Together's lanes relative to its top-left
// corner, each lane's steps in a band of its own, one under another, the
// bands all as tall as the tallest; a step two lanes reach goes with the
// first. Steps no lane reaches follow on the right. It returns the
// Together's size.
func arrangeTogether(
	workflow *flow.Workflow, together flow.Node, boxes map[ids.ULID]flow.Box, sized sizing, cycle map[routeKey]bool,
) image.Point {
	graph := levelOf(workflow, together.ID, cycle)
	lanes, _ := together.Lanes() //nolint:errcheck // A Together that can't be read has no lanes.
	taken := map[ids.ULID]bool{}
	placed := make([]map[ids.ULID]image.Point, 0, len(lanes))
	band, widest := laneMinH, 0
	for _, lane := range lanes {
		corners := map[ids.ULID]image.Point{}
		if first, linked := together.Next[lane.Port]; linked {
			columns := graph.laneColumns(first, taken)
			graph.reorder(columns)
			corners = graph.placeColumns(columns, nil, sized, image.Pt(0, 0))
		}
		right, bottom := extent(workflow, corners, sized)
		band, widest = max(band, bottom+ui.Half*bandPad), max(widest, right)
		placed = append(placed, corners)
	}
	left := laneHead + containerPad
	for index, corners := range placed {
		for nodeID, at := range corners {
			node, _ := workflow.Node(nodeID)
			boxes[nodeID] = boxAt(node, at.Add(image.Pt(left, containerHead+index*band+bandPad)), sizeOf(node, nil, sized))
		}
	}
	free := left + widest + columnGap
	top := containerHead + bandPad
	for _, node := range workflow.Nodes() {
		if _, done := boxes[node.ID]; done || node.Parent != together.ID {
			continue
		}
		size := sizeOf(node, nil, sized)
		boxes[node.ID] = boxAt(node, image.Pt(free, top), size)
		top += size.Y + rowGap
		widest = max(widest, free-left+size.X)
	}
	height := max(containerHead+max(len(lanes), 1)*band, top) + outputRow

	return image.Pt(max(containerMinW, left+widest+containerPad), max(height, togetherMinH(len(lanes))))
}

// extent is how far right and down a set of placed nodes reaches.
func extent(workflow *flow.Workflow, corners map[ids.ULID]image.Point, sized sizing) (int, int) {
	right, bottom := 0, 0
	for nodeID, at := range corners {
		node, _ := workflow.Node(nodeID)
		size := sizeOf(node, nil, sized)
		right, bottom = max(right, at.X+size.X), max(bottom, at.Y+size.Y)
	}

	return right, bottom
}

// laneColumns groups the steps a lane reaches from its first into columns,
// leaving out those an earlier lane took, and takes them.
func (g *level) laneColumns(first ids.ULID, taken map[ids.ULID]bool) [][]ids.ULID {
	depth := map[ids.ULID]int{}
	var met []ids.ULID
	deepest := g.layer([]ids.ULID{first}, 0, depth, &met)
	if deepest < 0 {
		return nil
	}
	columns := make([][]ids.ULID, deepest+1)
	for _, nodeID := range met {
		if !taken[nodeID] {
			taken[nodeID] = true
			columns[depth[nodeID]] = append(columns[depth[nodeID]], nodeID)
		}
	}

	return slices.DeleteFunc(columns, func(column []ids.ULID) bool { return len(column) == 0 })
}

// drawTogether draws a Together: its head with its name and how it goes on,
// each lane's band with its number and name, and "+ lane" along its foot.
func (s *State) drawTogether(area *ebiten.Image, drawn shape) {
	s.drawContainerFrame(area, drawn, "Together")
	rect := s.screenRect(drawn.rect)
	lanes, _ := drawn.node.Lanes() //nolint:errcheck // A Together that can't be read has no lanes.
	waits := fmt.Sprintf("all done when all %d are", len(lanes))
	right := rect.Max.X - textPad - portRadius - s.markRoom(drawn)
	s.theme.DrawSmall(area, waits, right-s.theme.SmallWidth(waits), rect.Min.Y+(containerHead-ui.LabelSize)/ui.Half-1,
		ui.ColorMuted())
	for index, band := range laneBands(drawn.rect, max(len(lanes), 1)) {
		band = s.screenRect(band)
		if index > 0 {
			strokeDashed(area, image.Rect(band.Min.X, band.Min.Y, band.Max.X, band.Min.Y+1), ui.Hairline, ui.ColorRule())
		}
		if index >= len(lanes) {
			continue
		}
		centre := image.Pt(band.Min.X+textPad+laneBadge, band.Min.Y+bandPad+entryTop)
		vector.FillCircle(area, float32(centre.X), float32(centre.Y), laneBadge, ui.ColorInk(), true)
		number := strconv.Itoa(index + 1)
		s.theme.DrawSmall(area, number, centre.X-s.theme.SmallWidth(number)/ui.Half, centre.Y-ui.LabelSize/ui.Half-1,
			ui.ColorBackground())
		name := s.clip(lanes[index].Label, laneHead-textPad-ui.Half*laneBadge-textPad-portRadius)
		s.theme.DrawText(area, name, band.Min.X+textPad, centre.Y+laneBadge+ui.Gap, ui.ColorInk())
	}
	row := s.screenRect(drawn.addOption)
	ui.FillRect(area, image.Rect(row.Min.X+ui.Hairline, row.Min.Y, row.Max.X-ui.Hairline, row.Min.Y+ui.Hairline),
		ui.ColorRule())
	ink := ui.ColorMuted()
	if s.cursor.In(row) && len(lanes) < flow.MaxLanes {
		ink = ui.ColorSelect()
	}
	s.theme.DrawText(area, "+ lane", row.Min.X+textPad, row.Min.Y+(outputRow-ui.TextSize)/ui.Half-1, ink)
}

// locateLaneBand finds the middle of a lane's band, "lane:<n>", in the
// first Together on the canvas.
func (s *State) locateLaneBand(value string) (image.Point, bool) {
	number, err := strconv.Atoi(value)
	drawn, ok := s.nthNode(flow.KindTogether + ":1")
	if err != nil || !ok {
		return image.Pt(0, 0), false
	}
	lanes, _ := drawn.node.Lanes() //nolint:errcheck // A Together that can't be read has no lanes.
	if number < 1 || number > len(lanes) {
		return image.Pt(0, 0), false
	}
	band := laneBands(drawn.rect, len(lanes))[number-1]

	return image.Pt((innerOf(drawn).Min.X+band.Max.X)/ui.Half, (band.Min.Y+band.Max.Y)/ui.Half), true
}

// laneAt is the lane whose band holds a canvas point in a Together, and
// whether one does.
func laneAt(drawn shape, point image.Point) (flow.Lane, bool) {
	lanes, err := drawn.node.Lanes()
	if err != nil || drawn.node.Kind != flow.KindTogether {
		return flow.Lane{Port: "", Label: ""}, false
	}
	for index, band := range laneBands(drawn.rect, max(len(lanes), 1)) {
		if index < len(lanes) && point.In(band) {
			return lanes[index], true
		}
	}

	return flow.Lane{Port: "", Label: ""}, false
}

// firstEmptyLane is a Together's first lane with no first step, where a
// node added inside it goes.
func firstEmptyLane(drawn shape) (port, bool) {
	for _, candidate := range drawn.ports {
		if _, used := drawn.node.Next[candidate.output.Name]; !used && flow.IsInner(drawn.node, candidate.output.Name) {
			return candidate, true
		}
	}

	return port{output: flow.Output{Name: "", Label: ""}, at: image.Pt(0, 0), row: image.Rect(0, 0, 0, 0)}, false
}

// laneProblem says what is wrong with a node in a Together's lane: a kind a
// lane can't hold, being in two lanes, or moving the NPC when an earlier
// lane already does.
func (s *State) laneProblem(node flow.Node) string {
	together, ok := s.workflow.Node(node.Parent)
	if !ok || together.Kind != flow.KindTogether {
		return ""
	}
	if !flow.InLane(node.Kind) {
		return problemNotInLane
	}
	reached := s.workflow.LanesOf(together.ID)
	names := laneNames(together)
	if in := reached[node.ID]; len(in) > 1 {
		return fmt.Sprintf(problemTwoLanes, names[in[0]], names[in[1]])
	}
	if !flow.Moves(node.Kind) || len(reached[node.ID]) == 0 {
		return ""
	}
	if mover := movingLane(s.workflow, together, reached); mover != "" && mover != reached[node.ID][0] {
		return fmt.Sprintf(problemTwoMovers, names[mover])
	}

	return ""
}

// laneNames names a Together's lanes by their outputs.
func laneNames(together flow.Node) map[string]string {
	names := map[string]string{}
	lanes, _ := together.Lanes() //nolint:errcheck // A Together that can't be read has no lanes.
	for _, lane := range lanes {
		names[lane.Port] = lane.Label
	}

	return names
}

// movingLane is the first lane, in lane order, holding a node that moves
// the NPC; "" when none does.
func movingLane(workflow *flow.Workflow, together flow.Node, reached map[ids.ULID][]string) string {
	lanes, _ := together.Lanes() //nolint:errcheck // A Together that can't be read has no lanes.
	for _, lane := range lanes {
		for _, child := range workflow.Children(together.ID) {
			node, _ := workflow.Node(child)
			if flow.Moves(node.Kind) && slices.Contains(reached[child], lane.Port) {
				return lane.Port
			}
		}
	}

	return ""
}

// endsAt says where the flow goes from an output with nothing connected: in
// a lane, the lane is done.
func (s *State) endsAt(node flow.Node, output string) string {
	if parent, ok := s.workflow.Node(node.Parent); ok && parent.Kind == flow.KindTogether {
		return "→ lane done"
	}

	return endsAt(node, output)
}

// addLane adds a lane to a Together, named for its number.
func (s *State) addLane(ctx *frame.Context, node flow.Node) {
	lanes, err := node.Lanes()
	if err != nil {
		return
	}
	if len(lanes) >= flow.MaxLanes {
		ctx.Frame.Status(fmt.Sprintf("A Together has at most %d lanes", flow.MaxLanes))

		return
	}
	port := newLanePort(lanes)
	lanes = append(lanes, flow.Lane{Port: port, Label: newLaneLabel(lanes)})
	s.saveParam(ctx, node, flow.ParamLanes, lanes)
}

// newLanePort returns an output name no lane uses yet.
func newLanePort(lanes []flow.Lane) string {
	for number := 1; ; number++ {
		name := "l" + strconv.Itoa(number)
		if !slices.ContainsFunc(lanes, func(lane flow.Lane) bool { return lane.Port == name }) {
			return name
		}
	}
}

// newLaneLabel returns "Lane N", unused among the lanes' names.
func newLaneLabel(lanes []flow.Lane) string {
	for number := len(lanes) + 1; ; number++ {
		name := "Lane " + strconv.Itoa(number)
		if !slices.ContainsFunc(lanes, func(lane flow.Lane) bool { return lane.Label == name }) {
			return name
		}
	}
}

// removeLane removes a lane and what is in it, the steps only it reaches,
// in one undo step; a Together keeps at least MinLanes lanes.
func (s *State) removeLane(ctx *frame.Context, node flow.Node, index int) {
	lanes, err := node.Lanes()
	if err != nil || index >= len(lanes) {
		return
	}
	if len(lanes) <= flow.MinLanes {
		ctx.Frame.Status(fmt.Sprintf("A Together has at least %d lanes", flow.MinLanes))

		return
	}
	gone := lanes[index]
	var steps []ids.ULID
	for nodeID, in := range s.workflow.LanesOf(node.ID) {
		if slices.Equal(in, []string{gone.Port}) {
			steps = append(steps, nodeID)
		}
	}
	slices.SortFunc(steps, func(a, b ids.ULID) int { return a.Compare(b) })
	changed, err := node.WithLink(gone.Port, ids.ULID{}).WithParam(flow.ParamLanes, slices.Delete(lanes, index, index+1))
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	gesture := s.gesture()
	replace := func() {
		s.command(ctx, workflowplugin.ReplaceEdit, gesture, workflowplugin.ReplaceArgs{Node: changed}, nil)
	}
	if len(steps) == 0 {
		replace()

		return
	}
	s.command(ctx, workflowplugin.RemoveEdit, gesture, workflowplugin.RemoveArgs{Nodes: steps}, func(protocol.Result) {
		replace()
		ctx.Frame.Status(gone.Label + " removed with what was in it. Undo puts it back.")
	})
}

// updateLanes adds a lane to a Together, or removes one with what is in it.
func (s *State) updateLanes(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	if made.addOption.Update(input) {
		s.addLane(ctx, node)

		return
	}
	for index, row := range made.options {
		if row.remove.Update(input) {
			s.removeLane(ctx, node, index)

			return
		}
	}
}

// parseLaneField reads a lane's new name.
func parseLaneField(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	lanes, err := node.Lanes()
	if err != nil {
		return nil, false, ""
	}
	for index, row := range made.options {
		if field != row.label || index >= len(lanes) {
			continue
		}
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, true, "Give the lane a name"
		}
		lanes[index].Label = name

		return lanes, true, ""
	}

	return nil, false, ""
}

// newLaneRows makes a Together's lane rows, each a name and Remove, and Add
// lane.
func newLaneRows(made *control, rows int, text func(string, bool) *ui.TextField) {
	for index := range rows {
		prefix := made.param.Name + "-" + strconv.Itoa(index+1)
		made.options = append(made.options, optionRow{
			port: "", label: text(prefix+"-label", false), share: nil, remove: ui.NewButton(prefix+"-remove", "Remove"),
		})
	}
	made.addOption = ui.NewButton("add-lane", "Add lane")
}

// setSavedLanes shows a Together's lanes' names.
func setSavedLanes(made *control, node flow.Node) {
	lanes, err := node.Lanes()
	if err != nil || len(lanes) != len(made.options) {
		return
	}
	for index, lane := range lanes {
		made.options[index].port = lane.Port
		made.options[index].label.Field.SetSaved(lane.Label)
		made.options[index].remove.Disabled = len(lanes) <= flow.MinLanes
	}
	made.addOption.Disabled = len(lanes) >= flow.MaxLanes
}

// layoutLanes places a Together's lanes, a number, a name and Remove to a
// row, with Add lane under them, and returns where the next control goes.
func layoutLanes(made *control, left, width, top int) int {
	nameLeft := left + ui.Half*laneBadge + rowSpace
	for _, row := range made.options {
		row.label.Rect = image.Rect(nameLeft, top, left+width-removeWidth-rowSpace, top+ui.ButtonH)
		row.remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + rowSpace
	}
	made.addOption.Rect = image.Rect(left, top, left+width/ui.Half, top+ui.ButtonH)

	return top + ui.ButtonH + rowSpace + ui.Gap + ui.Half*noteLine
}

// drawLanes draws a Together's lane rows, each numbered, and Add lane.
func (s *State) drawLanes(screen *ebiten.Image, made *control) {
	for index, row := range made.options {
		centre := image.Pt(row.label.Rect.Min.X-rowSpace-laneBadge, (row.label.Rect.Min.Y+row.label.Rect.Max.Y)/ui.Half)
		vector.FillCircle(screen, float32(centre.X), float32(centre.Y), laneBadge, ui.ColorInk(), true)
		number := strconv.Itoa(index + 1)
		s.theme.DrawSmall(screen, number, centre.X-s.theme.SmallWidth(number)/ui.Half, centre.Y-ui.LabelSize/ui.Half-1,
			ui.ColorBackground())
		row.label.Draw(screen)
		row.remove.Draw(screen, s.theme)
	}
	made.addOption.Draw(screen, s.theme)
	note := "Removing a lane removes what is in it, in one undo step."
	for line, text := range s.wrap(note, made.addOption.Rect.Dx()*ui.Half) {
		s.theme.DrawSmall(screen, text, made.addOption.Rect.Min.X, made.addOption.Rect.Max.Y+ui.Gap+line*noteLine,
			ui.ColorMuted())
	}
}

// hasLaneState answers "lanes:<kind>:<n>=<count>", how many lanes the nth
// node of a kind has, and whether the state is that one.
func (s *State) hasLaneState(state string) (bool, bool) {
	asked, ok := strings.CutPrefix(state, "lanes:")
	if !ok {
		return false, false
	}
	which, count, _ := strings.Cut(asked, "=")
	drawn, found := s.nthNode(which)
	if !found {
		return false, true
	}
	lanes, err := drawn.node.Lanes()

	return err == nil && strconv.Itoa(len(lanes)) == count, true
}
