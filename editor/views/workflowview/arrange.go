package workflowview

import (
	"cmp"
	"image"
	"slices"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// sweeps is how many times auto-arrange reorders the columns, alternating
// forwards and backwards, to cut crossings.
const sweeps = 5

// sizing is what a node's size depends on besides the node: how tall a note
// must be, and how many links come into each node.
type sizing struct {
	noteHeight func(flow.Node, int) int
	incoming   map[ids.ULID]int
}

// edge is one link between two nodes of a level that is not a loop: the
// output it leaves by, and where that output sits among the node's outputs.
type edge struct {
	from, to ids.ULID
	output   string
	place    float64
}

// level is the nodes sharing one parent, the top level or one container,
// with the links between them that are not loops.
type level struct {
	nodes map[ids.ULID]flow.Node
	order []ids.ULID
	into  map[ids.ULID][]edge
	outOf map[ids.ULID][]edge
}

// arrange lays a workflow out as columns from the start, the way layered
// graph layouts do: loops are left out so they do not stretch the columns; a
// node's column is its longest distance from the start, so every other line
// points forward; nodes in each column are ordered by where their lines come
// from, to cut crossings; and each node is lined up with the output it comes
// from. Each container's contents are laid out the same way inside it. Ties
// are broken by node, so the same workflow always arranges the same way.
func arrange(workflow *flow.Workflow, noteHeight func(flow.Node, int) int) map[ids.ULID]flow.Box {
	boxes := map[ids.ULID]flow.Box{}
	sizes := map[ids.ULID]image.Point{}
	incoming := map[ids.ULID]int{}
	for target, links := range incomingLinks(workflow) {
		incoming[target] = len(links)
	}
	sized := sizing{noteHeight: noteHeight, incoming: incoming}
	cycle := loops(workflow)
	for _, node := range workflow.Nodes() {
		if node.Kind == flow.KindContainer {
			sizes[node.ID] = arrangeInside(workflow, node, boxes, sized, cycle)
		}
	}
	placed := arrangeLevel(workflow, ids.ULID{}, workflow.Start(), sizes, sized, cycle, image.Pt(arrangeX, arrangeY))
	for nodeID, at := range placed {
		node, _ := workflow.Node(nodeID)
		boxes[nodeID] = boxAt(node, at, sizeOf(node, sizes, sized))
		if node.Kind == flow.KindContainer {
			offsetChildren(workflow, nodeID, boxes, at)
		}
	}

	return boxes
}

// arrangeInside lays out a container's contents relative to the container's
// top-left corner and returns the container's size.
func arrangeInside(
	workflow *flow.Workflow, container flow.Node, boxes map[ids.ULID]flow.Box, sized sizing, cycle map[routeKey]bool,
) image.Point {
	origin := image.Pt(containerPad, containerHead+containerPad)
	placed := arrangeLevel(workflow, container.ID, container.Next[flow.OutputInside], nil, sized, cycle, origin)
	right, bottom := containerMinW, containerMinH
	for nodeID, at := range placed {
		node, _ := workflow.Node(nodeID)
		size := sizeOf(node, nil, sized)
		boxes[nodeID] = boxAt(node, at, size)
		right = max(right, at.X+size.X+containerPad)
		bottom = max(bottom, at.Y+size.Y+rowGap)
	}

	return image.Pt(right, bottom)
}

// arrangeLevel places one level's nodes, starting from first, and returns
// each node's top-left corner.
func arrangeLevel(
	workflow *flow.Workflow, parent, first ids.ULID, sizes map[ids.ULID]image.Point, sized sizing,
	cycle map[routeKey]bool, origin image.Point,
) map[ids.ULID]image.Point {
	graph := levelOf(workflow, parent, cycle)
	columns := graph.columns(first)
	graph.reorder(columns)
	corners := map[ids.ULID]image.Point{}
	left := origin.X
	for _, column := range columns {
		top, width, leaving := origin.Y, 0, 0
		for _, nodeID := range column {
			node := graph.nodes[nodeID]
			size := sizeOf(node, sizes, sized)
			want := top
			if aligned, ok := graph.alignedTop(nodeID, corners, sizes, sized); ok {
				want = max(want, aligned)
			}
			corners[nodeID] = image.Pt(left, want)
			top = want + size.Y + rowGap
			width = max(width, size.X)
			leaving += len(flow.Outputs(node))
		}
		left += width + max(columnGap, ui.Half*laneMargin+(leaving+1)*laneGap)
	}
	placeNotes(workflow, parent, corners, sizes, sized, origin)

	return corners
}

// placeNotes puts a level's notes in a row under its other nodes. Notes are
// not part of the flow, so they never take a place in its columns; placed
// notes keep their own place when the workflow is arranged.
func placeNotes(
	workflow *flow.Workflow, parent ids.ULID, corners, sizes map[ids.ULID]image.Point, sized sizing, origin image.Point,
) {
	top := origin.Y
	for nodeID, at := range corners {
		node, _ := workflow.Node(nodeID)
		top = max(top, at.Y+sizeOf(node, sizes, sized).Y+rowGap)
	}
	left := origin.X
	for _, node := range workflow.Nodes() {
		if node.Parent == parent && node.Kind == flow.KindNote {
			corners[node.ID] = image.Pt(left, top)
			left += sizeOf(node, nil, sized).X + rowGap
		}
	}
}

// levelOf collects the nodes with one parent and the links between them
// that are not loops; a container's link to its first step belongs to the
// level inside it, not this one.
func levelOf(workflow *flow.Workflow, parent ids.ULID, cycle map[routeKey]bool) *level {
	graph := &level{nodes: map[ids.ULID]flow.Node{}, order: nil, into: map[ids.ULID][]edge{}, outOf: map[ids.ULID][]edge{}}
	for _, node := range workflow.Nodes() {
		if node.Parent == parent && node.Kind != flow.KindNote {
			graph.nodes[node.ID] = node
			graph.order = append(graph.order, node.ID)
		}
	}
	for _, nodeID := range graph.order {
		node := graph.nodes[nodeID]
		outputs := flow.Outputs(node)
		for index, output := range outputs {
			target, linked := node.Next[output.Name]
			_, here := graph.nodes[target]
			if !linked || !here || output.Name == flow.OutputInside || cycle[routeKey{from: nodeID, output: output.Name}] {
				continue
			}
			link := edge{
				from: nodeID, to: target, output: output.Name, place: float64(index+1) / float64(len(outputs)+1),
			}
			graph.outOf[nodeID] = append(graph.outOf[nodeID], link)
			graph.into[target] = append(graph.into[target], link)
		}
	}

	return graph
}

// columns groups the level into columns by longest distance from first,
// each in the order a walk from first meets them; nodes first does not reach
// follow in their own columns, in creation order.
func (g *level) columns(first ids.ULID) [][]ids.ULID {
	depth := map[ids.ULID]int{}
	var met []ids.ULID
	deepest := g.layer([]ids.ULID{first}, 0, depth, &met)
	var rest []ids.ULID
	for _, nodeID := range g.order {
		if _, placed := depth[nodeID]; !placed && !g.reachedOnlyByOthers(nodeID, depth) {
			rest = append(rest, nodeID)
		}
	}
	if len(rest) > 0 {
		deepest = g.layer(rest, deepest+1, depth, &met)
	}
	for _, nodeID := range g.order {
		if _, placed := depth[nodeID]; !placed {
			deepest++
			depth[nodeID] = deepest
			met = append(met, nodeID)
		}
	}
	if deepest < 0 {
		return nil
	}
	columns := make([][]ids.ULID, deepest+1)
	for _, nodeID := range met {
		columns[depth[nodeID]] = append(columns[depth[nodeID]], nodeID)
	}

	return columns
}

// reachedOnlyByOthers reports whether a node not yet placed has a link in
// from another node not yet placed, so it is laid out after that one.
func (g *level) reachedOnlyByOthers(nodeID ids.ULID, depth map[ids.ULID]int) bool {
	for _, link := range g.into[nodeID] {
		if _, placed := depth[link.from]; !placed {
			return true
		}
	}

	return false
}

// layer gives the nodes reachable from roots their longest distance from
// them, starting at base, and returns the deepest column used. Without
// loops the level has no cycles, so each node is settled once every link
// into it has been.
func (g *level) layer(roots []ids.ULID, base int, depth map[ids.ULID]int, met *[]ids.ULID) int {
	reach := map[ids.ULID]bool{}
	var visit func(nodeID ids.ULID)
	visit = func(nodeID ids.ULID) {
		if _, here := g.nodes[nodeID]; !here || reach[nodeID] {
			return
		}
		reach[nodeID] = true
		for _, link := range g.outOf[nodeID] {
			visit(link.to)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	waiting := map[ids.ULID]int{}
	for nodeID := range reach {
		for _, link := range g.into[nodeID] {
			if reach[link.from] {
				waiting[nodeID]++
			}
		}
	}
	deepest := base - 1
	var ready []ids.ULID
	for _, root := range roots {
		if reach[root] && waiting[root] == 0 {
			depth[root] = base
			ready = append(ready, root)
		}
	}
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		*met = append(*met, current)
		deepest = max(deepest, depth[current])
		for _, link := range g.outOf[current] {
			depth[link.to] = max(depth[link.to], depth[current]+1)
			waiting[link.to]--
			if waiting[link.to] == 0 {
				ready = append(ready, link.to)
			}
		}
	}

	return deepest
}

// reorder orders each column by the average position of the lines coming in
// from earlier columns, then of the lines going out to later ones, a few
// times over. An output's place among its node's outputs counts, so the node
// after a lower output goes lower.
func (g *level) reorder(columns [][]ids.ULID) {
	position := map[ids.ULID]float64{}
	remember := func() {
		for _, column := range columns {
			for index, nodeID := range column {
				position[nodeID] = float64(index)
			}
		}
	}
	remember()
	for sweep := range sweeps {
		forward := sweep%ui.Half == 0
		for step := range columns {
			index := step
			if !forward {
				index = len(columns) - 1 - step
			}
			g.sortColumn(columns[index], position, forward)
			remember()
		}
	}
}

// sortColumn sorts one column by the average position of its nodes' lines
// in, or out; a node with none keeps its place.
func (g *level) sortColumn(column []ids.ULID, position map[ids.ULID]float64, forward bool) {
	key := map[ids.ULID]float64{}
	for _, nodeID := range column {
		sum, count := 0.0, 0
		if forward {
			for _, link := range g.into[nodeID] {
				sum += position[link.from] + link.place
				count++
			}
		} else {
			for _, link := range g.outOf[nodeID] {
				sum += position[link.to]
				count++
			}
		}
		key[nodeID] = position[nodeID]
		if count > 0 {
			key[nodeID] = sum / float64(count)
		}
	}
	slices.SortStableFunc(column, func(a, b ids.ULID) int {
		return cmp.Or(cmp.Compare(key[a], key[b]), cmp.Compare(position[a], position[b]), a.Compare(b))
	})
}

// alignedTop is the top that puts a node's first entry dot level with the
// output it comes from, the middle one when several lead in from columns
// already placed.
func (g *level) alignedTop(
	nodeID ids.ULID, at map[ids.ULID]image.Point, sizes map[ids.ULID]image.Point, sized sizing,
) (int, bool) {
	var ports []int
	for _, link := range g.into[nodeID] {
		from, placed := at[link.from]
		if !placed {
			continue
		}
		made := shapeOf(g.nodes[link.from], flow.Box{X: 0, Y: 0, W: sizes[link.from].X, H: sizes[link.from].Y},
			sized.noteHeight, sized.incoming[link.from])
		if out, found := made.portOf(link.output); found {
			ports = append(ports, from.Y+out.at.Y)
		}
	}
	if len(ports) == 0 {
		return 0, false
	}
	slices.Sort(ports)

	return ports[(len(ports)-1)/2] - entryTop, true
}

// offsetChildren moves a container's contents, laid out relative to it, to
// where the container now sits.
func offsetChildren(workflow *flow.Workflow, container ids.ULID, boxes map[ids.ULID]flow.Box, by image.Point) {
	for _, child := range workflow.Children(container) {
		box := boxes[child]
		box.X += by.X
		box.Y += by.Y
		boxes[child] = box
	}
}

func sizeOf(node flow.Node, containers map[ids.ULID]image.Point, sized sizing) image.Point {
	if size, ok := containers[node.ID]; ok {
		return size
	}
	made := shapeOf(node, flow.Box{X: 0, Y: 0, W: 0, H: 0}, sized.noteHeight, sized.incoming[node.ID])

	return made.rect.Size()
}

// boxAt is the box that puts a node of a size at a point; containers and
// notes keep their size in the box.
func boxAt(node flow.Node, at, size image.Point) flow.Box {
	box := flow.Box{X: at.X, Y: at.Y, W: 0, H: 0}
	switch node.Kind {
	case flow.KindContainer:
		box.W, box.H = size.X, size.Y
	case flow.KindNote:
		box.W = size.X
	}

	return box
}
