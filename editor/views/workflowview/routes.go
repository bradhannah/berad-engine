package workflowview

import (
	"cmp"
	"image"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Line routing, in canvas pixels.
const (
	// laneGap is the space between lanes; lanes squeeze to minLaneGap when a
	// gap between columns is narrow.
	laneGap    = 8
	minLaneGap = 4
	// laneMargin keeps the first lane clear of the port it leaves from and
	// the last clear of the node it goes to.
	laneMargin = 10
	// lineReach is how close the pointer must be to a line to be over it.
	lineReach = 4
)

// The order lines leaving one column take their lanes in.
const (
	rankBack = iota
	rankDown
	rankUp
)

// routeKey names a line by the output it leaves from; each output leads to
// at most one node.
type routeKey struct {
	from   ids.ULID
	output string
}

// noRoute is the key of no line.
func noRoute() routeKey { return routeKey{from: ids.ULID{}, output: ""} }

// line is one connection to route: where it leaves, where it enters and the
// shapes at either end.
type line struct {
	key    routeKey
	to     ids.ULID
	from   image.Point
	end    image.Point
	source image.Rectangle
	target image.Rectangle
	inside bool
}

// loops lists the lines that lead back to a node already on the way to them:
// a depth-first walk from the start node, then from any node it does not
// reach, in creation order, finds them. Without them the workflow has no
// cycles, so they are left out when columns are worked out, and drawn
// dashed.
func loops(workflow *flow.Workflow) map[routeKey]bool {
	found := map[routeKey]bool{}
	const (
		unseen = iota
		walking
		done
	)
	state := map[ids.ULID]int{}
	var walk func(nodeID ids.ULID)
	walk = func(nodeID ids.ULID) {
		state[nodeID] = walking
		node, _ := workflow.Node(nodeID)
		for _, output := range flow.Outputs(node) {
			next, linked := node.Next[output.Name]
			if _, exists := workflow.Node(next); !linked || !exists {
				continue
			}
			switch state[next] {
			case walking:
				found[routeKey{from: nodeID, output: output.Name}] = true
			case unseen:
				walk(next)
			}
		}
		state[nodeID] = done
	}
	walk(workflow.Start())
	for _, node := range workflow.Nodes() {
		if state[node.ID] == unseen {
			walk(node.ID)
		}
	}

	return found
}

// lines lists every connection to draw, in creation order of the nodes they
// leave and the order of their outputs.
func lines(workflow *flow.Workflow, shapes map[ids.ULID]shape) []line {
	var result []line
	for _, node := range workflow.Nodes() {
		from := shapes[node.ID]
		for _, out := range from.ports {
			targetID, linked := node.Next[out.output.Name]
			target, ok := shapes[targetID]
			if !linked || !ok {
				continue
			}
			end, entered := target.entryOf(node.ID, out.output.Name)
			if !entered {
				end = image.Pt(target.rect.Min.X, target.rect.Min.Y+entryTop)
			}
			end.X -= portRadius
			result = append(result, line{
				key: routeKey{from: node.ID, output: out.output.Name}, to: targetID, from: out.at, end: end,
				source: from.rect, target: target.rect, inside: out.output.Name == flow.OutputInside,
			})
		}
	}

	return result
}

// routes works out every line's corners. A line that goes forward turns in
// its own lane in the gap after the node it leaves, so no two lines run on
// top of each other; a line that goes back drops under the nodes between its
// ends, each at its own depth, and rises into its entry dot in its own lane.
// A line moved by hand turns, or runs, where its bend puts it instead. Notes
// play no part.
func routes(workflow *flow.Workflow, shapes map[ids.ULID]shape) map[routeKey][]image.Point {
	result := map[routeKey][]image.Point{}
	gaps := map[int][]line{}
	var back []line
	for _, link := range lines(workflow, shapes) {
		bend, bent := workflow.Bend(link.key.from, link.key.output)
		switch {
		case link.inside:
			result[link.key] = []image.Point{link.from, {X: link.from.X, Y: link.end.Y}, link.end}
		case link.end.X < link.from.X+2*stub:
			back = append(back, link)
		case link.from.Y == link.end.Y:
			result[link.key] = []image.Point{link.from, link.end}
		case bent && bend.Lane != nil:
			lane := min(max(link.from.X+*bend.Lane, link.from.X+minLaneGap), link.end.X-minLaneGap)
			result[link.key] = []image.Point{link.from, {X: lane, Y: link.from.Y}, {X: lane, Y: link.end.Y}, link.end}
		default:
			gaps[link.from.X] = append(gaps[link.from.X], link)
		}
	}
	under := backRoutes(back, shapes, workflow)
	for _, link := range back {
		gaps[link.from.X] = append(gaps[link.from.X], link)
	}
	for _, x := range sortedKeys(gaps) {
		routeGap(gaps[x], under, result)
	}

	return result
}

// backRoute is where a line that goes back runs: the depth under the nodes,
// the lane it comes back up in to its entry dot, and, when its drop leg was
// moved by hand, the lane it drops in.
type backRoute struct {
	depth int
	rise  int
	lane  int
	laned bool
}

// routeGap gives each line leaving one column its own lane: lines that go
// back nearest the column, shallowest first, then lines that go down, those
// starting lower first, then lines that go up, those starting higher first.
// In that order, lines cross only where they must.
func routeGap(gap []line, back map[routeKey]backRoute, result map[routeKey][]image.Point) {
	rank := func(link line) int {
		_, goesBack := back[link.key]
		switch {
		case goesBack:
			return rankBack
		case link.end.Y > link.from.Y:
			return rankDown
		default:
			return rankUp
		}
	}
	slices.SortStableFunc(gap, func(first, second line) int {
		order := cmp.Compare(rank(first), rank(second))
		switch {
		case order != 0:
			return order
		case rank(first) == rankBack:
			order = cmp.Compare(back[first.key].depth, back[second.key].depth)
		case rank(first) == rankDown:
			order = cmp.Compare(second.from.Y, first.from.Y)
		default:
			order = cmp.Compare(first.from.Y, second.from.Y)
		}

		return cmp.Or(order, first.key.from.Compare(second.key.from), cmp.Compare(first.key.output, second.key.output))
	})
	left, right := gap[0].from.X+laneMargin, gap[0].from.X+laneMargin+len(gap)*laneGap
	for _, link := range gap {
		if _, goesBack := back[link.key]; !goesBack {
			right = min(right, link.end.X-laneMargin)
		}
	}
	spacing := max(min(laneGap, (right-left)/len(gap)), minLaneGap)
	for index, link := range gap {
		lane := left + index*spacing + spacing/ui.Half
		if under, goesBack := back[link.key]; goesBack {
			if under.laned {
				lane = under.lane
			}
			result[link.key] = []image.Point{
				link.from, {X: lane, Y: link.from.Y}, {X: lane, Y: under.depth}, {X: under.rise, Y: under.depth},
				{X: under.rise, Y: link.end.Y}, link.end,
			}

			continue
		}
		result[link.key] = []image.Point{link.from, {X: lane, Y: link.from.Y}, {X: lane, Y: link.end.Y}, link.end}
	}
}

// backRoutes works out where each line that goes back runs: below every node
// between its ends, and below any shorter line that goes back over the same
// stretch, so shorter lines sit inside longer ones. Lines rising into one
// column's entry dots each get their own lane, the deepest furthest out. A
// leg moved by hand runs where its bend puts it instead: the drop leg Lane
// right of the output, the run Depth below it, the end leg End left of the
// entry.
func backRoutes(back []line, shapes map[ids.ULID]shape, workflow *flow.Workflow) map[routeKey]backRoute {
	slices.SortStableFunc(back, func(a, b line) int {
		return cmp.Or(cmp.Compare(a.from.X-a.end.X, b.from.X-b.end.X), a.key.from.Compare(b.key.from),
			cmp.Compare(a.key.output, b.key.output))
	})
	result := map[routeKey]backRoute{}
	placed := make([]line, 0, len(back))
	for _, link := range back {
		bend, bent := workflow.Bend(link.key.from, link.key.output)
		under := backRoute{depth: 0, rise: link.end.X - stub, lane: 0, laned: false}
		if bent && bend.Lane != nil {
			under.lane, under.laned = link.from.X+max(*bend.Lane, minLaneGap), true
		}
		if bent && bend.Depth != nil {
			under.depth = link.from.Y + *bend.Depth
		} else {
			under.depth = clearDepth(link, shapes, placed, result)
			placed = append(placed, link)
		}
		result[link.key] = under
	}
	riseLanes(back, result, workflow)

	return result
}

// clearDepth is how far down a line that goes back runs to clear every node
// between its ends and every back line already placed over the same stretch.
func clearDepth(link line, shapes map[ids.ULID]shape, placed []line, result map[routeKey]backRoute) int {
	low, high := link.end.X-stub, link.from.X+stub
	depth := max(link.source.Max.Y, link.target.Max.Y) + stub
	for _, drawn := range shapes {
		if drawn.rect.Max.X >= low && drawn.rect.Min.X <= high && runsBehindLines(drawn.node.Kind) {
			depth = max(depth, drawn.rect.Max.Y+stub)
		}
	}
	for _, other := range placed {
		if other.end.X-stub <= high && other.from.X+stub >= low {
			depth = max(depth, result[other.key].depth+laneGap)
		}
	}

	return depth
}

// riseLanes gives the lines rising into one column's entry dots their own
// lanes, the deepest furthest out; a line whose end leg was moved by hand
// rises where its bend puts it.
func riseLanes(back []line, result map[routeKey]backRoute, workflow *flow.Workflow) {
	rising := map[int][]line{}
	for _, link := range back {
		rising[link.end.X] = append(rising[link.end.X], link)
	}
	for _, column := range rising {
		slices.SortStableFunc(column, func(a, b line) int {
			return cmp.Or(cmp.Compare(result[a.key].depth, result[b.key].depth), a.key.from.Compare(b.key.from),
				cmp.Compare(a.key.output, b.key.output))
		})
		for index, link := range column {
			under := result[link.key]
			under.rise = link.end.X - stub - index*laneGap
			if bend, bent := workflow.Bend(link.key.from, link.key.output); bent && bend.End != nil {
				under.rise = link.end.X + min(*bend.End, -minLaneGap)
			}
			result[link.key] = under
		}
	}
}

func sortedKeys(gaps map[int][]line) []int {
	keys := make([]int, 0, len(gaps))
	for x := range gaps {
		keys = append(keys, x)
	}
	slices.Sort(keys)

	return keys
}

// runsBehindLines reports whether lines are drawn under a kind of node:
// every running node but a container, which, like a note, sits behind them.
func runsBehindLines(kind string) bool {
	return kind != flow.KindContainer && kind != flow.KindNote
}

// linkAt returns the line the pointer is over, at a canvas point. Lines are
// drawn under running nodes, so a point on one is over no line; containers
// and notes sit behind lines.
func (s *State) linkAt(point image.Point) (routeKey, bool) {
	if s.workflow == nil {
		return noRoute(), false
	}
	if hit, onNode := s.nodeAt(point); onNode && runsBehindLines(s.shapes[hit].node.Kind) {
		return noRoute(), false
	}
	for _, link := range lines(s.workflow, s.shapes) {
		corners := s.routes[link.key]
		for index := 1; index < len(corners); index++ {
			if nearSegment(point, corners[index-1], corners[index], lineReach) {
				return link.key, true
			}
		}
	}

	return noRoute(), false
}

// nearSegment reports whether a point is within reach of a straight,
// level or upright, segment.
func nearSegment(point, from, to image.Point, reach int) bool {
	low := image.Pt(min(from.X, to.X)-reach, min(from.Y, to.Y)-reach)
	high := image.Pt(max(from.X, to.X)+reach, max(from.Y, to.Y)+reach)

	return point.In(image.Rectangle{Min: low, Max: high.Add(image.Pt(1, 1))})
}

// locateBend finds the middle of a draggable part of the line from
// "<kind>:<n>:<output>": a forward line's upright part or a back line's run,
// or with ":drop" or ":end" after it, a back line's drop or end leg.
func (s *State) locateBend(value string) (image.Point, bool) {
	part := partDepth
	if name, which, found := strings.Cut(value, ":"+legDrop); found && which == "" {
		value, part = name, partLane
	} else if name, which, found := strings.Cut(value, ":"+legEnd); found && which == "" {
		value, part = name, partEnd
	}
	key, ok := s.lineNamed(value)
	if !ok {
		return image.Pt(0, 0), false
	}
	legs := legsOf(s.routes[key])
	if part == partDepth && len(legs) == 1 {
		part = partLane
	}
	for _, moved := range legs {
		if moved.part == part {
			return image.Pt((moved.from.X+moved.to.X)/ui.Half, (moved.from.Y+moved.to.Y)/ui.Half), true
		}
	}

	return image.Pt(0, 0), false
}

// Names for a back line's drop and end legs in "bend:..." locations.
const (
	legDrop = "drop"
	legEnd  = "end"
)

// lineNamed reads "<kind>:<n>:<output>" as a line.
func (s *State) lineNamed(value string) (routeKey, bool) {
	where := strings.Split(value, ":")
	if len(where) != portParts {
		return noRoute(), false
	}
	drawn, ok := s.nthNode(where[0] + ":" + where[1])

	return routeKey{from: drawn.node.ID, output: where[2]}, ok
}

// locateLine finds a point on the line from "<kind>:<n>:<output>": the middle
// of its longest stretch.
func (s *State) locateLine(value string) (image.Point, bool) {
	where := strings.Split(value, ":")
	if len(where) != portParts {
		return image.Pt(0, 0), false
	}
	drawn, ok := s.nthNode(where[0] + ":" + where[1])
	corners := s.routes[routeKey{from: drawn.node.ID, output: where[2]}]
	if !ok || len(corners) < ui.Half {
		return image.Pt(0, 0), false
	}
	best, longest := image.Pt(0, 0), -1
	for index := 1; index < len(corners); index++ {
		from, to := corners[index-1], corners[index]
		if length := max(abs(to.X-from.X), abs(to.Y-from.Y)); length > longest {
			best, longest = image.Pt((from.X+to.X)/ui.Half, (from.Y+to.Y)/ui.Half), length
		}
	}

	return best, true
}

// leg is a part of a line that can be dragged, and the part of its bend
// that dragging it sets.
type leg struct {
	from image.Point
	to   image.Point
	part string
}

// The parts of a bend: the lane a forward line turns in or a back line
// drops in, the depth a back line runs at, and where its end leg comes back
// up.
const (
	partLane  = "lane"
	partDepth = "depth"
	partEnd   = "end"
)

// legsOf returns the parts of a line that can be dragged: a forward line's
// upright part, or a back line's drop leg, level run and end leg. Straight
// lines and a container's line to its first step have none.
func legsOf(corners []image.Point) []leg {
	switch len(corners) {
	case forwardCorners:
		return []leg{{from: corners[1], to: corners[2], part: partLane}}
	case backCorners:
		return []leg{
			{from: corners[1], to: corners[2], part: partLane},
			{from: corners[2], to: corners[3], part: partDepth},
			{from: corners[3], to: corners[4], part: partEnd},
		}
	default:
		return nil
	}
}

// Corners of a forward line that turns, and of a back line.
const (
	forwardCorners = 4
	backCorners    = 6
)

// legAt returns the part of a line's bend that a canvas point is on a leg
// of, if it is on one.
func (s *State) legAt(key routeKey, point image.Point) (string, bool) {
	for _, moved := range legsOf(s.routes[key]) {
		if nearSegment(point, moved.from, moved.to, lineReach) {
			return moved.part, true
		}
	}

	return "", false
}

// targetOf returns the node a line leads to.
func (s *State) targetOf(key routeKey) (ids.ULID, bool) {
	if s.workflow == nil {
		return ids.ULID{}, false
	}
	from, ok := s.workflow.Node(key.from)
	if !ok {
		return ids.ULID{}, false
	}
	to, linked := from.Next[key.output]

	return to, linked
}
