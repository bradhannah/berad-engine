package workflowview

import (
	"image"
	"maps"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func testID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed

	return id
}

func plainNotes(flow.Node, int) int { return noteMinHeight }

// graph builds a workflow from a start node and nodes of the given kinds,
// keyed by seed, then links them; links name outputs by kind default:
// "next", or a random choice's "o1" and "o2".
func graph(t *testing.T, kinds map[byte]string, links [][3]any) *flow.Workflow {
	t.Helper()
	workflow, err := flow.New(testID(1))
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]flow.Node, 0, len(kinds))
	for seed := byte(2); seed < 100; seed++ {
		kind, ok := kinds[seed]
		if !ok {
			continue
		}
		node, err := flow.NewNode(kind)
		if err != nil {
			t.Fatal(err)
		}
		node.ID = testID(seed)
		nodes = append(nodes, node)
	}
	if err := workflow.Insert(nodes, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		from, _ := workflow.Node(testID(link[0].(byte)))
		if _, err := workflow.Replace(from.WithLink(link[1].(string), testID(link[2].(byte)))); err != nil {
			t.Fatal(err)
		}
	}

	return workflow
}

// arranged lays a workflow out with auto-arrange and works out its shapes
// and routes.
func arranged(t *testing.T, workflow *flow.Workflow) (map[ids.ULID]shape, map[routeKey][]image.Point) {
	t.Helper()
	if _, err := workflow.Move(arrange(workflow, plainNotes)); err != nil {
		t.Fatal(err)
	}
	laid := shapes(workflow, plainNotes)

	return laid, routes(workflow, laid)
}

type segment struct {
	key      routeKey
	from, to image.Point
}

func segments(found map[routeKey][]image.Point) []segment {
	var all []segment
	for key, corners := range found {
		for index := 1; index < len(corners); index++ {
			if corners[index-1] != corners[index] {
				all = append(all, segment{key: key, from: corners[index-1], to: corners[index]})
			}
		}
	}

	return all
}

func span(a, b int) (int, int) { return min(a, b), max(a, b) }

// overlaps counts pairs of stretches of different lines that lie on top of
// each other for some length.
func overlaps(found map[routeKey][]image.Point) int {
	count := 0
	all := segments(found)
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			a, b := all[i], all[j]
			if a.key == b.key {
				continue
			}
			switch {
			case a.from.X == a.to.X && b.from.X == b.to.X && a.from.X == b.from.X:
				aLow, aHigh := span(a.from.Y, a.to.Y)
				bLow, bHigh := span(b.from.Y, b.to.Y)
				if min(aHigh, bHigh) > max(aLow, bLow) {
					count++
				}
			case a.from.Y == a.to.Y && b.from.Y == b.to.Y && a.from.Y == b.from.Y:
				aLow, aHigh := span(a.from.X, a.to.X)
				bLow, bHigh := span(b.from.X, b.to.X)
				if min(aHigh, bHigh) > max(aLow, bLow) {
					count++
				}
			}
		}
	}

	return count
}

// TestLoopsAreFound finds the line back to a node already on the way, and
// no other.
func TestLoopsAreFound(t *testing.T) {
	t.Parallel()
	workflow := graph(t, map[byte]string{2: flow.KindRandom, 3: flow.KindMove, 4: flow.KindWait}, [][3]any{
		{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(3)}, {byte(3), flow.OutputNext, byte(4)},
		{byte(4), flow.OutputNext, byte(2)},
	})
	found := loops(workflow)
	if len(found) != 1 || !found[routeKey{from: testID(4), output: flow.OutputNext}] {
		t.Fatalf("loops %v", found)
	}
}

// TestArrangeKeepsLinesForward puts a node two branches meet at after the
// longer branch, so every line that is not a loop points forward.
func TestArrangeKeepsLinesForward(t *testing.T) {
	t.Parallel()
	workflow := graph(t, map[byte]string{2: flow.KindRandom, 3: flow.KindWait, 4: flow.KindWait, 5: flow.KindWait},
		[][3]any{
			{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(3)}, {byte(3), flow.OutputNext, byte(4)},
			{byte(4), flow.OutputNext, byte(5)}, {byte(2), "o2", byte(5)},
		})
	laid, found := arranged(t, workflow)
	for key, corners := range found {
		if corners[len(corners)-1].X <= corners[0].X {
			t.Fatalf("%v points back: %v", key, corners)
		}
	}
	if laid[testID(5)].rect.Min.X <= laid[testID(4)].rect.Min.X {
		t.Fatal("the meeting node comes after the longer branch")
	}
	if overlaps(found) != 0 {
		t.Fatalf("%d stretches of line overlap", overlaps(found))
	}
}

// TestRandomWalkArranges lays out "move up or down at random, wait, go back
// to the random choice": the Moves line up in order with the options, the
// loop stays out of the columns and runs under the nodes, and no two lines
// lie on top of each other.
func TestRandomWalkArranges(t *testing.T) {
	t.Parallel()
	workflow := graph(t, map[byte]string{2: flow.KindRandom, 3: flow.KindMove, 4: flow.KindMove, 5: flow.KindWait},
		[][3]any{
			{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(3)}, {byte(2), "o2", byte(4)},
			{byte(3), flow.OutputNext, byte(5)}, {byte(4), flow.OutputNext, byte(5)}, {byte(5), flow.OutputNext, byte(2)},
		})
	laid, found := arranged(t, workflow)
	random, up, down, wait := laid[testID(2)], laid[testID(3)], laid[testID(4)], laid[testID(5)]
	if upRoute := found[routeKey{from: testID(2), output: "o1"}]; len(upRoute) != 2 {
		t.Fatalf("Move up is level with the up option, so its line is straight: %v", upRoute)
	}
	if down.rect.Min.Y <= up.rect.Min.Y {
		t.Fatal("Move down sits below Move up")
	}
	if wait.rect.Min.X <= up.rect.Min.X || random.rect.Min.X >= up.rect.Min.X {
		t.Fatal("the loop back to the random choice does not stretch the columns")
	}
	loop := found[routeKey{from: testID(5), output: flow.OutputNext}]
	bottom := 0
	for _, drawn := range laid {
		bottom = max(bottom, drawn.rect.Max.Y)
	}
	if len(loop) != 6 || loop[2].Y <= bottom {
		t.Fatalf("the loop runs under every node: %v (bottom %d)", loop, bottom)
	}
	if overlaps(found) != 0 {
		t.Fatalf("%d stretches of line overlap: %v", overlaps(found), found)
	}
}

// TestLanesSeparateLinesSharingAGap places three nodes so the lines to them
// from one random choice would all turn at the same spot; each gets its own
// lane.
func TestLanesSeparateLinesSharingAGap(t *testing.T) {
	t.Parallel()
	workflow := graph(t, map[byte]string{2: flow.KindRandom, 3: flow.KindWait, 4: flow.KindWait, 5: flow.KindWait},
		[][3]any{{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(5)}, {byte(2), "o2", byte(4)}})
	random, _ := workflow.Node(testID(2))
	random, _ = random.WithParam("options", []flow.Share{
		{Port: "o1", Label: "a", Share: 1}, {Port: "o2", Label: "b", Share: 1}, {Port: "o3", Label: "c", Share: 1},
	})
	random = random.WithLink("o3", testID(3))
	if _, err := workflow.Replace(random); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Move(map[ids.ULID]flow.Box{
		testID(1): {X: 0, Y: 0, W: 0, H: 0}, testID(2): {X: 160, Y: 0, W: 0, H: 0},
		testID(3): {X: 500, Y: -200, W: 0, H: 0}, testID(4): {X: 500, Y: 100, W: 0, H: 0},
		testID(5): {X: 500, Y: 300, W: 0, H: 0},
	}); err != nil {
		t.Fatal(err)
	}
	found := routes(workflow, shapes(workflow, plainNotes))
	lanes := map[int]bool{}
	for _, output := range []string{"o1", "o2", "o3"} {
		lanes[found[routeKey{from: testID(2), output: output}][1].X] = true
	}
	if len(lanes) != 3 || overlaps(found) != 0 {
		t.Fatalf("three lines, three lanes: %v (%d overlaps)", lanes, overlaps(found))
	}
}

// TestBackLinesGetTheirOwnDepth gives two loops over the same stretch their
// own depths and rising lanes.
func TestBackLinesGetTheirOwnDepth(t *testing.T) {
	t.Parallel()
	workflow := graph(t, map[byte]string{2: flow.KindWait, 3: flow.KindWait, 4: flow.KindRandom}, [][3]any{
		{byte(1), flow.OutputNext, byte(2)}, {byte(2), flow.OutputNext, byte(3)}, {byte(3), flow.OutputNext, byte(4)},
		{byte(4), "o1", byte(2)}, {byte(4), "o2", byte(3)},
	})
	_, found := arranged(t, workflow)
	first, second := found[routeKey{from: testID(4), output: "o1"}], found[routeKey{from: testID(4), output: "o2"}]
	if len(first) != 6 || len(second) != 6 || first[2].Y == second[2].Y {
		t.Fatalf("two loops, two depths: %v %v", first, second)
	}
	if overlaps(found) != 0 {
		t.Fatalf("%d stretches of line overlap", overlaps(found))
	}
}

// TestArrangeIsTheSameEveryTime arranges one workflow several times.
func TestArrangeIsTheSameEveryTime(t *testing.T) {
	t.Parallel()
	workflow := graph(t, map[byte]string{2: flow.KindRandom, 3: flow.KindMove, 4: flow.KindMove, 5: flow.KindWait},
		[][3]any{
			{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(3)}, {byte(2), "o2", byte(4)},
			{byte(3), flow.OutputNext, byte(5)}, {byte(4), flow.OutputNext, byte(5)}, {byte(5), flow.OutputNext, byte(2)},
		})
	first := arrange(workflow, plainNotes)
	for range 5 {
		if again := arrange(workflow, plainNotes); !maps.Equal(first, again) {
			t.Fatalf("arranged differently: %v then %v", first, again)
		}
	}
}

// TestOverlapsCountsLinesOnTopOfEachOther checks the test helper itself:
// two lines turning at the same spot overlap; lines that only cross do not.
func TestOverlapsCountsLinesOnTopOfEachOther(t *testing.T) {
	t.Parallel()
	a, b := routeKey{from: testID(1), output: "a"}, routeKey{from: testID(2), output: "b"}
	shared := map[routeKey][]image.Point{
		a: {{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 100}, {X: 60, Y: 100}},
		b: {{X: 0, Y: 40}, {X: 20, Y: 40}, {X: 20, Y: 140}, {X: 60, Y: 140}},
	}
	crossing := map[routeKey][]image.Point{
		a: {{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 100}, {X: 60, Y: 100}},
		b: {{X: 0, Y: 40}, {X: 28, Y: 40}, {X: 28, Y: 140}, {X: 60, Y: 140}},
	}
	if overlaps(shared) != 1 || overlaps(crossing) != 0 {
		t.Fatalf("overlaps %d and %d", overlaps(shared), overlaps(crossing))
	}
}

// randomWalk is "random choice up or down, wait, back to the random choice",
// with a note (seed 6) not yet linked to anything.
func randomWalk(t *testing.T) *flow.Workflow {
	t.Helper()

	return graph(t, map[byte]string{
		2: flow.KindRandom, 3: flow.KindMove, 4: flow.KindMove, 5: flow.KindWait, 6: flow.KindNote,
	}, [][3]any{
		{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(3)}, {byte(2), "o2", byte(4)},
		{byte(3), flow.OutputNext, byte(5)}, {byte(4), flow.OutputNext, byte(5)}, {byte(5), flow.OutputNext, byte(2)},
	})
}

// TestNotesDoNotMoveLines lays a note right across the loop's path: every
// line routes exactly as it would without it.
func TestNotesDoNotMoveLines(t *testing.T) {
	t.Parallel()
	workflow := randomWalk(t)
	laid, without := arranged(t, workflow)
	bottom := 0
	for _, drawn := range laid {
		if drawn.node.Kind != flow.KindNote {
			bottom = max(bottom, drawn.rect.Max.Y)
		}
	}
	if _, err := workflow.Move(map[ids.ULID]flow.Box{testID(6): {X: 0, Y: bottom, W: 600, H: 0}}); err != nil {
		t.Fatal(err)
	}
	with := routes(workflow, shapes(workflow, plainNotes))
	if !maps.EqualFunc(without, with, slices.Equal[[]image.Point]) {
		t.Fatalf("a note moved the lines:\n%v\n%v", without, with)
	}
}

// TestArrangeLeavesNotesOut arranges the same with or without a note, and
// puts a note with no place yet under the flow.
func TestArrangeLeavesNotesOut(t *testing.T) {
	t.Parallel()
	withNote := randomWalk(t)
	withoutNote := graph(t, map[byte]string{2: flow.KindRandom, 3: flow.KindMove, 4: flow.KindMove, 5: flow.KindWait},
		[][3]any{
			{byte(1), flow.OutputNext, byte(2)}, {byte(2), "o1", byte(3)}, {byte(2), "o2", byte(4)},
			{byte(3), flow.OutputNext, byte(5)}, {byte(4), flow.OutputNext, byte(5)}, {byte(5), flow.OutputNext, byte(2)},
		})
	boxes, plain := arrange(withNote, plainNotes), arrange(withoutNote, plainNotes)
	note := boxes[testID(6)]
	delete(boxes, testID(6))
	if !maps.Equal(boxes, plain) {
		t.Fatalf("a note changed the layout:\n%v\n%v", boxes, plain)
	}
	for nodeID, box := range plain {
		size := sizeOf(func() flow.Node { node, _ := withoutNote.Node(nodeID); return node }(), nil, sizing{
			noteHeight: plainNotes, incoming: map[ids.ULID]int{},
		})
		if note.Y < box.Y+size.Y {
			t.Fatalf("the note goes under the flow: note at %d, %v ends at %d", note.Y, nodeID, box.Y+size.Y)
		}
	}
}

// TestBendsMoveLines moves the loop's run above the nodes and a forward
// line's upright part sideways, and the lines follow.
func TestBendsMoveLines(t *testing.T) {
	t.Parallel()
	workflow := randomWalk(t)
	laid, found := arranged(t, workflow)
	loop := found[routeKey{from: testID(5), output: flow.OutputNext}]
	down := found[routeKey{from: testID(2), output: "o2"}]
	if len(loop) != backCorners || len(down) != forwardCorners {
		t.Fatalf("the loop goes back and the down line turns: %v %v", loop, down)
	}
	top := laid[testID(2)].rect.Min.Y
	for _, drawn := range laid {
		top = min(top, drawn.rect.Min.Y)
	}
	depth, lane := top-loop[0].Y-30, down[1].X-down[0].X+20
	if _, err := workflow.SetBends(map[string]flow.Bend{
		flow.LineKey(testID(5), flow.OutputNext): {To: testID(2), Lane: nil, Depth: &depth},
		flow.LineKey(testID(2), "o2"):            {To: testID(4), Lane: &lane, Depth: nil},
	}); err != nil {
		t.Fatal(err)
	}
	bent := routes(workflow, shapes(workflow, plainNotes))
	if run := bent[routeKey{from: testID(5), output: flow.OutputNext}][2].Y; run != top-30 {
		t.Fatalf("the loop runs above the nodes: %d, top %d", run, top)
	}
	if upright := bent[routeKey{from: testID(2), output: "o2"}][1].X; upright != down[1].X+20 {
		t.Fatalf("the down line turns where it was moved: %d, was %d", upright, down[1].X)
	}
}
