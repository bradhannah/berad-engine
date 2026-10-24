package workflowview

import (
	"cmp"
	"image"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Node sizes on the canvas, in pixels.
const (
	nodeWidth       = 172
	nodeHeight      = 46
	startWidth      = 110
	headHeight      = 40
	switchHead      = 30
	outputRow       = 24
	noteWidth       = 190
	noteLine        = 16
	noteMinHeight   = 46
	containerHead   = 30
	containerWidth  = 380
	containerHeight = 220
	containerMinW   = 240
	containerMinH   = 140
	containerPad    = 18
	portRadius      = 5
	portReach       = 9
	resizeGrab      = 12
	// Entry dots sit on a node's left edge, the first entryTop below its
	// top and each next one entryGap lower, with entryBottom under the last.
	entryTop    = 20
	entryGap    = 16
	entryBottom = 10
	// Auto-arrange spacing.
	columnGap = 64
	rowGap    = 24
	arrangeX  = 40
	arrangeY  = 40
	// stub is how far a connection leaves its port before turning.
	stub = 18
)

// port is one output of a node on the canvas.
type port struct {
	output flow.Output
	// at is the port's centre; row is the output's row, for nodes that list
	// their outputs.
	at  image.Point
	row image.Rectangle
}

// noShape is the shape of no node.
func noShape() shape {
	var none shape

	return none
}

// shape is where a node is drawn on the canvas and where its outputs are.
type shape struct {
	node  flow.Node
	rect  image.Rectangle
	ports []port
	// entries are where links come in, one dot each, top to bottom in the
	// order of the outputs they come from, so links do not cross; spare is
	// the empty dot for the next link. Start and notes have neither.
	entries []entry
	spare   image.Point
	// addOption is a random choice's "+ option" row.
	addOption image.Rectangle
}

// entry is one link coming into a node: the output it comes from and its dot.
type entry struct {
	from   ids.ULID
	output string
	at     image.Point
}

// entryOf returns where a link from an output comes into the shape.
func (s shape) entryOf(from ids.ULID, output string) (image.Point, bool) {
	for _, candidate := range s.entries {
		if candidate.from == from && candidate.output == output {
			return candidate.at, true
		}
	}

	return image.Pt(0, 0), false
}

// takesEntries reports whether links can come into a kind of node.
func takesEntries(kind string) bool {
	return kind != flow.KindStart && kind != flow.KindNote
}

// incomingLinks lists, for each node, the links that come into it.
func incomingLinks(workflow *flow.Workflow) map[ids.ULID][]flow.Link {
	incoming := map[ids.ULID][]flow.Link{}
	for _, node := range workflow.Nodes() {
		for _, output := range flow.Outputs(node) {
			to, linked := node.Next[output.Name]
			if target, exists := workflow.Node(to); linked && exists && takesEntries(target.Kind) {
				incoming[to] = append(incoming[to], flow.Link{From: node.ID, Output: output.Name, To: to})
			}
		}
	}

	return incoming
}

// entriesHeight is how tall a node must be for its entry dots: one per link
// coming in and the spare.
func entriesHeight(incoming int) int {
	return entryTop + incoming*entryGap + entryBottom
}

// portOf returns a shape's port for an output.
func (s shape) portOf(output string) (port, bool) {
	index := slices.IndexFunc(s.ports, func(candidate port) bool { return candidate.output.Name == output })
	if index < 0 {
		return port{output: flow.Output{Name: "", Label: ""}, at: image.Pt(0, 0), row: image.Rect(0, 0, 0, 0)}, false
	}

	return s.ports[index], true
}

// shapes lays out every node: placed nodes where their boxes say, and nodes
// not yet placed where auto-arrange would put them.
func shapes(workflow *flow.Workflow, noteHeight func(flow.Node, int) int) map[ids.ULID]shape {
	boxes := workflow.Boxes()
	arranged := arrange(workflow, noteHeight)
	incoming := incomingLinks(workflow)
	result := map[ids.ULID]shape{}
	for _, node := range workflow.Nodes() {
		box, placed := boxes[node.ID]
		if !placed {
			box = arranged[node.ID]
		}
		result[node.ID] = shapeOf(node, box, noteHeight, len(incoming[node.ID]))
	}
	placeEntries(result, incoming)

	return result
}

// placeEntries gives each link coming into a node its own dot, in the order
// of the ports the links leave from, top to bottom, then the spare dot.
func placeEntries(result map[ids.ULID]shape, incoming map[ids.ULID][]flow.Link) {
	for nodeID, made := range result {
		if !takesEntries(made.node.Kind) {
			continue
		}
		links := slices.Clone(incoming[nodeID])
		from := func(link flow.Link) image.Point {
			out, _ := result[link.From].portOf(link.Output)

			return out.at
		}
		slices.SortStableFunc(links, func(a, b flow.Link) int {
			return cmp.Or(cmp.Compare(from(a).Y, from(b).Y), cmp.Compare(from(a).X, from(b).X),
				a.From.Compare(b.From), cmp.Compare(a.Output, b.Output))
		})
		slot := func(index int) image.Point {
			return image.Pt(made.rect.Min.X, made.rect.Min.Y+entryTop+index*entryGap)
		}
		made.entries = make([]entry, 0, len(links))
		for index, link := range links {
			made.entries = append(made.entries, entry{from: link.From, output: link.Output, at: slot(index)})
		}
		made.spare = slot(len(links))
		result[nodeID] = made
	}
}

// shapeOf works out one node's rectangle and ports from its box; incoming
// counts the links coming in, which each need an entry dot.
func shapeOf(node flow.Node, box flow.Box, noteHeight func(flow.Node, int) int, incoming int) shape {
	outputs := flow.Outputs(node)
	origin := image.Pt(box.X, box.Y)
	made := shape{
		node: node, rect: image.Rect(0, 0, 0, 0), ports: nil, entries: nil, spare: image.Pt(0, 0),
		addOption: image.Rect(0, 0, 0, 0),
	}
	entries := entriesHeight(incoming)
	switch node.Kind {
	case flow.KindStart:
		made.rect = image.Rectangle{Min: origin, Max: origin.Add(image.Pt(startWidth, nodeHeight))}
		made.ports = rightPorts(made.rect, outputs)
	case flow.KindContainer:
		width, height := max(box.W, containerMinW), max(box.H, containerMinH, entries)
		made.rect = image.Rectangle{Min: origin, Max: origin.Add(image.Pt(width, height))}
		for _, output := range outputs {
			at := image.Pt(made.rect.Max.X, made.rect.Min.Y+containerHead/2)
			if output.Name == flow.OutputInside {
				at = image.Pt(made.rect.Min.X+containerPad, made.rect.Min.Y+containerHead)
			}
			made.ports = append(made.ports, port{output: output, at: at, row: image.Rect(0, 0, 0, 0)})
		}
	case flow.KindNote:
		width := max(box.W, noteWidth)
		made.rect = image.Rectangle{Min: origin, Max: origin.Add(image.Pt(width, noteHeight(node, width)))}
	case flow.KindTimeSwitch, flow.KindRandom:
		head, rows := headHeight, len(outputs)
		if node.Kind == flow.KindTimeSwitch {
			head = switchHead
		} else {
			rows++
		}
		made.rect = image.Rectangle{Min: origin, Max: origin.Add(image.Pt(nodeWidth, max(head+rows*outputRow, entries)))}
		for index, output := range outputs {
			top := made.rect.Min.Y + head + index*outputRow
			row := image.Rect(made.rect.Min.X, top, made.rect.Max.X, top+outputRow)
			made.ports = append(made.ports, port{output: output, at: image.Pt(row.Max.X, top+outputRow/2), row: row})
		}
		if node.Kind == flow.KindRandom {
			top := made.rect.Min.Y + head + len(outputs)*outputRow
			made.addOption = image.Rect(made.rect.Min.X, top, made.rect.Max.X, top+outputRow)
		}
	default:
		made.rect = image.Rectangle{Min: origin, Max: origin.Add(image.Pt(nodeWidth, max(nodeHeight, entries)))}
		made.ports = rightPorts(made.rect, outputs)
	}

	return made
}

// rightPorts places a node's outputs on its right edge, evenly spaced.
func rightPorts(rect image.Rectangle, outputs []flow.Output) []port {
	ports := make([]port, 0, len(outputs))
	for index, output := range outputs {
		top := rect.Min.Y + rect.Dy()*(index+1)/(len(outputs)+1)
		ports = append(ports, port{output: output, at: image.Pt(rect.Max.X, top), row: image.Rect(0, 0, 0, 0)})
	}

	return ports
}

// inside returns a container's inner area, where its contents sit.
func inside(container image.Rectangle) image.Rectangle {
	return image.Rect(container.Min.X+containerPad, container.Min.Y+containerHead+containerPad/2,
		container.Max.X-containerPad, container.Max.Y-containerPad/2)
}
