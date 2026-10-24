// Package workflowview is the Workflow window: one workflow drawn as typed
// nodes and their connections on a dotted canvas, a palette of node kinds on
// the left, and an inspector on the right built from the selected node's
// declared parameters (D13). Click a palette entry to add a node after the
// selected one, or drag it onto the canvas; drag from an output's port to a
// node to connect them, or pick a line up by its arrow end to move it to
// another node; drag a node to move it; click a line to select it.
//
// The view turns input into requests in Update without needing a screen; Draw
// renders it.
package workflowview

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/picker"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// ViewName is the Workflow window's view name; its target is a workflow.
const ViewName = "workflow"

// Window layout, in screen pixels.
const (
	PaletteWidth  = 200
	toolbarHeight = 40
	paletteItem   = 28
	paletteGap    = 6
	paletteTop    = toolbarHeight + ui.Inset
	groupSpace    = 26
	arrangeWidth  = 110
	dragStart     = 4
	wheelPan      = 24
	shortID       = 6
	// newGap is the space between a node and one added after it.
	newGap = 60
)

type nodeDrag struct {
	id      ids.ULID
	grab    image.Point
	gesture string
	last    image.Point
}

type linkDrag struct {
	from   ids.ULID
	output string
	to     image.Point
}

type paletteDrag struct {
	kind  string
	start image.Point
	at    image.Point
}

type containerResize struct {
	id      ids.ULID
	gesture string
}

// lineDrag moves the middle of a line by hand.
type lineDrag struct {
	key     routeKey
	start   image.Point
	gesture string
	moved   bool
}

// paletteEntry is one node kind offered on the left.
type paletteEntry struct {
	kind  flow.Kind
	rect  image.Rectangle
	group bool
}

// State is one Workflow window.
type State struct {
	window   ids.ULID
	document ids.ULID
	theme    *ui.Theme
	size     image.Point
	subs     map[ids.ULID]bool
	workflow *flow.Workflow
	shapes   map[ids.ULID]shape
	routes   map[routeKey][]image.Point
	loops    map[routeKey]bool
	problems map[ids.ULID]string
	// laidOut is the workflow revision the shapes, routes and loops were
	// worked out for, and problemKey what the problems were found from;
	// layouts counts layouts, for tests.
	laidOut    uint64
	hasLayout  bool
	layouts    int
	problemKey string
	// watchFloors asks for every Go to's destination floor to be loaded
	// after a new layout, so broken destinations can be found.
	watchFloors bool
	offset      image.Point
	selected    ids.ULID
	cursor      image.Point
	gestures    int
	// hoverLink is the line under the pointer, selectedLink the selected
	// line and hoverMark the node whose problem mark is under the pointer.
	hoverLink    routeKey
	selectedLink routeKey
	hoverMark    ids.ULID

	drag    *nodeDrag
	link    *linkDrag
	adding  *paletteDrag
	resize  *containerResize
	bending *lineDrag
	panning bool
	lastPan image.Point

	palette    []paletteEntry
	arrange    *ui.Button
	deleteLine *ui.Button
	resetLine  *ui.Button
	form       *form
	focus      ui.Focus
	picker     *picker.Picker
	// live is set for a read-only view of an NPC's workflow in the Watch
	// window.
	live *live
}

// New returns the Workflow window for a workflow.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, document: document, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		workflow: nil, shapes: map[ids.ULID]shape{}, routes: map[routeKey][]image.Point{}, loops: map[routeKey]bool{},
		problems: map[ids.ULID]string{}, laidOut: 0, hasLayout: false, layouts: 0, problemKey: "", watchFloors: false,
		offset: image.Pt(0, 0), selected: ids.ULID{}, cursor: image.Pt(0, 0), gestures: 0,
		hoverLink: noRoute(), selectedLink: noRoute(), hoverMark: ids.ULID{},
		drag: nil, link: nil, adding: nil, resize: nil, bending: nil, panning: false, lastPan: image.Pt(0, 0),
		palette: nil, arrange: ui.NewButton("arrange", "Auto-arrange"),
		deleteLine: ui.NewButton("delete-line", "Delete line"), resetLine: ui.NewButton("reset-line", "Reset line"),
		form: nil, focus: ui.Focus{}, picker: picker.New(window), live: nil,
	}
	for _, kind := range flow.Kinds() {
		if kind.Group != "" {
			view.palette = append(view.palette, paletteEntry{kind: kind, rect: ui.Zero(), group: false})
		}
	}

	return view
}

// Title is the window title.
func (s *State) Title(docs frame.Docs) string {
	return "Workflow: " + s.name(docs) + "  (view " + s.window.Short(shortID) + ")"
}

func (s *State) name(docs frame.Docs) string {
	name := "workflow"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(s.document, name)
		}
	})

	return name
}

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Selected returns the selected node's ID.
func (s *State) Selected() ids.ULID { return s.selected }

// Start subscribes to the project index, for names. The window has already
// subscribed to the workflow.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] {
		return
	}
	s.subs[document] = true
	ctx.Requests.Subscribe(document, func(err error) {
		if err != nil {
			delete(s.subs, document)
		}
	})
}

// Changed reacts to picks answered for the inspector.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	s.picker.Changed(update, ctx)
}

// load copies the workflow and lays it out, its nodes, lines and loops, when
// it has changed since the last layout, and finds its problems when it or
// anything they depend on has changed.
func (s *State) load(docs frame.Docs) {
	revision, found := uint64(0), false
	docs.View(s.document, func(document client.Document) {
		workflowDoc, ok := document.Model.(*workflowplugin.Document)
		if !ok {
			return
		}
		found, revision = true, document.Revision
		if !s.hasLayout || revision != s.laidOut {
			s.workflow = workflowDoc.Workflow.Clone()
		}
	})
	if !found {
		s.workflow, s.hasLayout = nil, false
		s.shapes, s.routes, s.loops = map[ids.ULID]shape{}, map[routeKey][]image.Point{}, map[routeKey]bool{}
		s.problems = map[ids.ULID]string{}

		return
	}
	if !s.hasLayout || revision != s.laidOut {
		s.shapes = shapes(s.workflow, s.noteHeight)
		s.routes, s.loops = routes(s.workflow, s.shapes), loops(s.workflow)
		s.laidOut, s.hasLayout, s.watchFloors = revision, true, true
		s.layouts++
	}
	s.refreshProblems(docs)
	if _, ok := s.workflow.Node(s.selected); !ok {
		s.selected = ids.ULID{}
	}
	if _, ok := s.targetOf(s.selectedLink); !ok {
		s.selectedLink = noRoute()
	}
}

// Canvas returns the screen area that shows the workflow.
func (s *State) Canvas() image.Rectangle {
	if s.live != nil {
		return s.live.area
	}
	content := frame.Content(s.size)

	return image.Rect(PaletteWidth, toolbarHeight, content.Max.X, content.Max.Y)
}

func (s *State) toScreen(point image.Point) image.Point {
	return point.Add(s.Canvas().Min).Add(s.offset)
}

func (s *State) toCanvas(point image.Point) image.Point {
	return point.Sub(s.Canvas().Min).Sub(s.offset)
}

// layoutWidgets places the palette, the toolbar and the inspector.
func (s *State) layoutWidgets(docs frame.Docs) {
	content := frame.Content(s.size)
	s.arrange.Rect = image.Rect(content.Max.X-ui.Inset-arrangeWidth, (toolbarHeight-ui.ButtonH)/ui.Half,
		content.Max.X-ui.Inset, (toolbarHeight+ui.ButtonH)/ui.Half)
	top, group := paletteTop, ""
	for index := range s.palette {
		entry := &s.palette[index]
		if entry.kind.Group != group {
			group = entry.kind.Group
			top += groupSpace
		}
		entry.rect = image.Rect(ui.Inset-ui.Half, top, PaletteWidth-ui.Inset, top+paletteItem)
		top += paletteItem + paletteGap
	}
	s.layoutForm(docs)
	s.deleteLine.Rect, s.resetLine.Rect = ui.Zero(), ui.Zero()
	if s.lineSelected() {
		panel := frame.Inspector(s.size)
		bottom := panel.Max.Y - ui.Inset
		s.deleteLine.Rect = image.Rect(panel.Min.X+ui.Inset, bottom-ui.ButtonH, panel.Max.X-ui.Inset, bottom)
		if _, bent := s.workflow.Bend(s.selectedLink.from, s.selectedLink.output); bent {
			above := bottom - ui.ButtonH - rowSpace
			s.resetLine.Rect = image.Rect(panel.Min.X+ui.Inset, above-ui.ButtonH, panel.Max.X-ui.Inset, above)
		}
	}
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	if s.live != nil {
		s.updateLive(input, ctx)

		return
	}
	s.size = ctx.Size
	s.cursor = input.Cursor
	s.load(ctx.Docs)
	s.syncForm(ctx)
	s.layoutWidgets(ctx.Docs)
	if dialog := s.picker.Dialog(); dialog != nil {
		s.picker.UpdateDialog(input, ctx, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})

		return
	}
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field != nil && outcome == textedit.OutcomeCommit {
		s.saveField(field, ctx)
	}
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	if s.workflow == nil {
		return
	}
	if s.watchFloors {
		s.watchFloors = false
		for _, floorID := range s.destinationFloors() {
			s.subscribe(floorID, ctx)
		}
	}
	if s.lineSelected() && s.deleteLine.Update(input) {
		s.removeLine(ctx)
	}
	if s.lineSelected() && !s.resetLine.Rect.Empty() && s.resetLine.Update(input) {
		s.bend(ctx, s.selectedLink, flow.Bend{To: ids.ULID{}, Lane: nil, Depth: nil}, "")
		ctx.Frame.Status("Line reset. Undo puts it back where you moved it.")
	}
	s.updateForm(input, ctx)
	if s.arrange.Update(input) {
		s.autoArrange(ctx)
	}
	s.updatePalette(input, ctx)
	s.updateCanvas(input, ctx)
}

// keys handles the window's own keys while no text field has focus.
func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	plain := ui.Plain()
	switch {
	case s.picker.Picking() && input.Consume(ui.KeyEscape, plain):
		s.picker.Cancel(ctx, "cancelled in the workflow window")
	case input.Consume(ui.KeyEscape, plain):
		s.selected, s.selectedLink, s.link, s.adding = ids.ULID{}, noRoute(), nil, nil
	case input.Consume(ui.KeyBackspace, plain), input.Consume(ui.KeyDelete, plain):
		if s.lineSelected() {
			s.removeLine(ctx)
		} else {
			s.removeSelected(ctx)
		}
	}
}

// lineSelected reports whether a line is selected.
func (s *State) lineSelected() bool {
	_, ok := s.targetOf(s.selectedLink)

	return ok
}

// removeLine takes the selected line away; undo puts it back.
func (s *State) removeLine(ctx *frame.Context) {
	from, ok := s.workflow.Node(s.selectedLink.from)
	if !ok {
		return
	}
	output := s.selectedLink.output
	s.selectedLink = noRoute()
	s.replace(ctx, from.WithLink(output, ids.ULID{}), "Line removed. Undo puts it back.")
}

// updatePalette adds a node when a palette entry is clicked, or dragged onto
// the canvas and dropped.
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
			s.addAt(ctx, adding.kind, s.toCanvas(adding.at))
		case !dragged:
			s.addAfterSelected(ctx, adding.kind)
		}

		return
	}
	if !input.Pressed {
		return
	}
	for _, entry := range s.palette {
		if input.Cursor.In(entry.rect) {
			s.adding = &paletteDrag{kind: entry.kind.Name, start: input.Cursor, at: input.Cursor}
			input.ClearPointer()

			return
		}
	}
}

// updateCanvas handles panning and the gestures on nodes and ports.
func (s *State) updateCanvas(input *ui.Input, ctx *frame.Context) {
	canvas := s.Canvas()
	s.pan(input, canvas)
	if s.panning || input.Space {
		return
	}
	point := s.toCanvas(input.Cursor)
	s.updateHover(ctx, input.Cursor.In(canvas), point)
	switch {
	case s.link != nil:
		s.updateLink(input, ctx, point)
	case s.bending != nil:
		s.updateBend(input, ctx, point)
	case s.drag != nil:
		s.updateDrag(input, ctx, point)
	case s.resize != nil:
		s.updateResize(input, ctx, point)
	case input.Pressed && input.Cursor.In(canvas):
		s.press(ctx, point)
	}
}

func (s *State) pan(input *ui.Input, canvas image.Rectangle) {
	if input.Cursor.In(canvas) && (input.WheelX != 0 || input.WheelY != 0) {
		s.offset = s.offset.Sub(image.Pt(int(input.WheelX*wheelPan), int(input.WheelY*wheelPan)))
	}
	dragPan := input.PanDown || (input.Space && input.Down)
	if dragPan && s.panning {
		s.offset = s.offset.Add(input.Cursor.Sub(s.lastPan))
	}
	s.panning = dragPan && (s.panning || input.Cursor.In(canvas))
	s.lastPan = input.Cursor
}

// press starts what a press on the canvas begins: a connection from a port,
// a line picked up by its arrow end, resizing a container, or selecting and
// moving a node.
func (s *State) press(ctx *frame.Context, point image.Point) {
	if from, output, ok := s.portAt(point); ok {
		s.selected = from
		s.link = &linkDrag{from: from, output: output, to: point}

		return
	}
	if in, ok := s.entryAt(point); ok {
		s.selected, s.selectedLink = ids.ULID{}, routeKey{from: in.from, output: in.output}
		s.link = &linkDrag{from: in.from, output: in.output, to: point}

		return
	}
	for _, nodeID := range s.drawOrder() {
		if drawn := s.shapes[nodeID]; point.In(drawn.addOption) {
			s.selected = nodeID
			s.addOption(ctx, drawn.node)

			return
		}
	}
	if container, ok := s.resizeHandleAt(point); ok {
		s.selected = container
		s.resize = &containerResize{id: container, gesture: s.gesture()}

		return
	}
	if key, onLine := s.linkAt(point); onLine {
		s.selected, s.selectedLink = ids.ULID{}, key
		if s.onMiddle(key, point) {
			s.bending = &lineDrag{key: key, start: point, gesture: s.gesture(), moved: false}
		}

		return
	}
	hit, ok := s.nodeAt(point)
	if !ok {
		s.selected, s.selectedLink = ids.ULID{}, noRoute()

		return
	}
	s.selected, s.selectedLink = hit, noRoute()
	s.drag = &nodeDrag{id: hit, grab: point.Sub(s.shapes[hit].rect.Min), gesture: s.gesture(), last: point}
}

// updateDrag moves the dragged node, and a container's contents with it,
// keeping a node inside its container.
func (s *State) updateDrag(input *ui.Input, ctx *frame.Context, point image.Point) {
	drag := s.drag
	if !input.Down {
		s.drag = nil

		return
	}
	if point == drag.last {
		return
	}
	drag.last = point
	moving, ok := s.shapes[drag.id]
	if !ok {
		s.drag = nil

		return
	}
	target := point.Sub(drag.grab)
	if container, inContainer := s.shapes[moving.node.Parent]; inContainer {
		target = clampInto(target, moving.rect.Size(), inside(container.rect))
	}
	delta := target.Sub(moving.rect.Min)
	if delta == image.Pt(0, 0) {
		return
	}
	boxes := map[ids.ULID]flow.Box{drag.id: s.movedBox(moving, delta)}
	for _, child := range s.workflow.Children(drag.id) {
		boxes[child] = s.movedBox(s.shapes[child], delta)
	}
	s.command(ctx, workflowplugin.MoveEdit, drag.gesture, workflowplugin.MoveArgs{Boxes: boxes, Lines: nil}, nil)
}

// movedBox is a node's box shifted by a distance, keeping a container's or
// note's size.
func (s *State) movedBox(moving shape, delta image.Point) flow.Box {
	box, placed := s.workflow.Box(moving.node.ID)
	if !placed {
		box = flow.Box{X: moving.rect.Min.X, Y: moving.rect.Min.Y, W: 0, H: 0}
		if moving.node.Kind == flow.KindContainer {
			box.W, box.H = moving.rect.Dx(), moving.rect.Dy()
		}
	}
	box.X, box.Y = moving.rect.Min.X+delta.X, moving.rect.Min.Y+delta.Y

	return box
}

// updateResize sizes a container by its bottom-right corner, never smaller
// than its contents.
func (s *State) updateResize(input *ui.Input, ctx *frame.Context, point image.Point) {
	if !input.Down {
		s.resize = nil

		return
	}
	container, ok := s.shapes[s.resize.id]
	if !ok {
		s.resize = nil

		return
	}
	right, bottom := max(point.X, container.rect.Min.X+containerMinW), max(point.Y, container.rect.Min.Y+containerMinH)
	for _, child := range s.workflow.Children(s.resize.id) {
		right = max(right, s.shapes[child].rect.Max.X+containerPad)
		bottom = max(bottom, s.shapes[child].rect.Max.Y+containerPad/ui.Half)
	}
	if right == container.rect.Max.X && bottom == container.rect.Max.Y {
		return
	}
	box := flow.Box{
		X: container.rect.Min.X, Y: container.rect.Min.Y, W: right - container.rect.Min.X, H: bottom - container.rect.Min.Y,
	}
	s.command(ctx, workflowplugin.MoveEdit, s.resize.gesture,
		workflowplugin.MoveArgs{Boxes: map[ids.ULID]flow.Box{s.resize.id: box}, Lines: nil}, nil)
}

// updateBend moves the middle of a line as it is dragged: a forward line's
// upright part sideways, or a back line's run up or down, above the nodes if
// wanted. Each change is a move edit in the drag's gesture.
func (s *State) updateBend(input *ui.Input, ctx *frame.Context, point image.Point) {
	drag := s.bending
	if !input.Down {
		s.bending = nil

		return
	}
	moved := point.Sub(drag.start)
	if !drag.moved && max(abs(moved.X), abs(moved.Y)) <= dragStart {
		return
	}
	drag.moved = true
	corners := s.routes[drag.key]
	target, linked := s.targetOf(drag.key)
	if !linked || len(corners) < forwardCorners {
		s.bending = nil

		return
	}
	output := corners[0]
	bend := flow.Bend{To: target, Lane: nil, Depth: nil}
	if len(corners) == backCorners {
		depth := point.Y - output.Y
		bend.Depth = &depth
	} else {
		lane := point.X - output.X
		bend.Lane = &lane
	}
	if current, bent := s.workflow.Bend(drag.key.from, drag.key.output); bent && sameBend(current, bend) {
		return
	}
	s.bend(ctx, drag.key, bend, drag.gesture)
}

// bend sends the move edit that sets, or with no To removes, a line's bend.
func (s *State) bend(ctx *frame.Context, key routeKey, bend flow.Bend, gesture string) {
	args := workflowplugin.MoveArgs{
		Boxes: map[ids.ULID]flow.Box{}, Lines: map[string]flow.Bend{flow.LineKey(key.from, key.output): bend},
	}
	s.command(ctx, workflowplugin.MoveEdit, gesture, args, nil)
}

// sameBend reports whether two bends put a line in the same place.
func sameBend(a, b flow.Bend) bool {
	same := func(x, y *int) bool { return (x == nil && y == nil) || (x != nil && y != nil && *x == *y) }

	return a.To == b.To && same(a.Lane, b.Lane) && same(a.Depth, b.Depth)
}

// updateLink follows a connection being drawn from a port, or a line picked
// up by its arrow end, then connects it to the node it is dropped on, or
// clears it when dropped on empty canvas. Dropped back on the node it
// already leads to, it changes nothing.
func (s *State) updateLink(input *ui.Input, ctx *frame.Context, point image.Point) {
	s.link.to = point
	if input.Down {
		return
	}
	link := s.link
	s.link = nil
	from, ok := s.workflow.Node(link.from)
	if !ok {
		return
	}
	target, dropped := s.dropTarget(point)
	if !dropped {
		if _, linked := from.Next[link.output]; linked {
			s.replace(ctx, from.WithLink(link.output, ids.ULID{}), "Disconnected")
		}

		return
	}
	if current, linked := from.Next[link.output]; linked && current == target {
		return
	}
	if reason := linkProblem(s.workflow, from, link.output, target); reason != "" {
		ctx.Frame.Status(reason)

		return
	}
	s.replace(ctx, from.WithLink(link.output, target), "Connected")
}

// linkProblem explains why an output cannot lead to a node, or returns "".
func linkProblem(workflow *flow.Workflow, from flow.Node, output string, target ids.ULID) string {
	targetNode, ok := workflow.Node(target)
	switch {
	case !ok:
		return ""
	case targetNode.Kind == flow.KindStart:
		return "Nothing links back to the start: a branch that ends goes back there by itself."
	case !flow.Runs(targetNode.Kind):
		return "Notes are not part of the flow, so nothing can link to them."
	case from.Kind == flow.KindContainer && output == flow.OutputInside:
		if targetNode.Parent != from.ID {
			return "A container's first step must be inside it."
		}
	case targetNode.Parent != from.Parent:
		return "Links stay inside one container; leave a container by its exit."
	}

	return ""
}

// addAfterSelected adds a node after the selected one, connected from its
// first free output, or inside a selected container; with nothing selected
// it goes in the middle of the view.
func (s *State) addAfterSelected(ctx *frame.Context, kind string) {
	selected, ok := s.shapes[s.selected]
	if !ok {
		canvas := s.Canvas()
		s.add(ctx, kind, s.toCanvas(ui.Center(canvas)).Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half)),
			ids.ULID{}, nil)

		return
	}
	if kind == flow.KindNote {
		place := image.Pt(selected.rect.Min.X, selected.rect.Min.Y-noteMinHeight-rowGap)
		s.add(ctx, kind, place, selected.node.Parent, nil)

		return
	}
	if selected.node.Kind == flow.KindContainer && kind != flow.KindContainer {
		place := inside(selected.rect).Min
		var link *flow.Link
		if _, used := selected.node.Next[flow.OutputInside]; !used {
			link = &flow.Link{From: selected.node.ID, Output: flow.OutputInside, To: ids.ULID{}}
		} else {
			place = s.freeSpot(selected.node.ID, place)
		}
		s.add(ctx, kind, place, selected.node.ID, link)

		return
	}
	place := image.Pt(selected.rect.Max.X+newGap, selected.rect.Min.Y)
	var link *flow.Link
	for _, candidate := range selected.ports {
		if _, used := selected.node.Next[candidate.output.Name]; used || candidate.output.Name == flow.OutputInside {
			continue
		}
		link = &flow.Link{From: selected.node.ID, Output: candidate.output.Name, To: ids.ULID{}}
		place.Y = candidate.at.Y - nodeHeight/ui.Half

		break
	}
	s.add(ctx, kind, s.clearOf(place, selected.node.Parent), selected.node.Parent, link)
}

// clearOf moves a new node's place down until it overlaps none of the nodes
// beside it, so nodes added after outputs that are close together do not
// pile up.
func (s *State) clearOf(place image.Point, parent ids.ULID) image.Point {
	for moved := true; moved; {
		moved = false
		added := image.Rectangle{Min: place, Max: place.Add(image.Pt(nodeWidth, nodeHeight))}
		for _, other := range s.shapes {
			if other.node.Parent == parent && other.node.Kind != flow.KindNote &&
				added.Overlaps(other.rect.Inset(-rowGap/ui.Half)) {
				place.Y = other.rect.Max.Y + rowGap
				moved = true
			}
		}
	}

	return place
}

// freeSpot returns a place inside a container below its contents.
func (s *State) freeSpot(container ids.ULID, place image.Point) image.Point {
	for _, child := range s.workflow.Children(container) {
		place.Y = max(place.Y, s.shapes[child].rect.Max.Y+rowGap)
	}

	return place
}

// addAt adds a node where it is dropped, inside the container there, if any.
func (s *State) addAt(ctx *frame.Context, kind string, point image.Point) {
	parent := ids.ULID{}
	if kind != flow.KindContainer {
		for nodeID, candidate := range s.shapes {
			if candidate.node.Kind == flow.KindContainer && point.In(inside(candidate.rect)) {
				parent = nodeID
			}
		}
	}
	s.add(ctx, kind, point.Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half)), parent, nil)
}

// newBox is a new node's box at a place; containers and notes start at
// their usual size.
func newBox(kind string, place image.Point) flow.Box {
	box := flow.Box{X: place.X, Y: place.Y, W: 0, H: 0}
	switch kind {
	case flow.KindContainer:
		box.W, box.H = containerWidth, containerHeight
	case flow.KindNote:
		box.W = noteWidth
	}

	return box
}

// updateHover follows the line and the problem mark under the pointer, and
// says what the problem is when the pointer comes onto a mark.
func (s *State) updateHover(ctx *frame.Context, onCanvas bool, point image.Point) {
	s.hoverLink = noRoute()
	mark := ids.ULID{}
	if onCanvas && s.drag == nil && s.resize == nil {
		s.hoverLink, _ = s.linkAt(point)
		mark = s.markAt(point)
	}
	if mark != s.hoverMark && !mark.IsZero() {
		ctx.Frame.Status(s.problems[mark])
	}
	s.hoverMark = mark
}

// destinationFloors lists the floors the workflow's Go to nodes lead to.
func (s *State) destinationFloors() []ids.ULID {
	var floors []ids.ULID
	for _, node := range s.workflow.Nodes() {
		if destination, err := node.Anchor("destination"); err == nil && !destination.Floor.IsZero() &&
			!slices.Contains(floors, destination.Floor) {
			floors = append(floors, destination.Floor)
		}
	}

	return floors
}

// add sends the edit that adds a node, then selects it and, given a link,
// connects it, all as one undo step.
func (s *State) add(ctx *frame.Context, kind string, place image.Point, parent ids.ULID, link *flow.Link) {
	node, err := flow.NewNode(kind)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	node.Parent = parent
	box := newBox(kind, place)
	gesture := s.gesture()
	args := workflowplugin.AddArgs{Nodes: []flow.Node{node}, Boxes: map[ids.ULID]flow.Box{{}: box}, Links: nil}
	s.command(ctx, workflowplugin.AddEdit, gesture, args, func(result protocol.Result) {
		s.selected, s.selectedLink = result.Created, noRoute()
		if link == nil {
			return
		}
		if from, found := s.currentNode(ctx.Docs, link.From); found {
			s.command(ctx, workflowplugin.ReplaceEdit, gesture,
				workflowplugin.ReplaceArgs{Node: from.WithLink(link.Output, result.Created)}, nil)
		}
	})
}

// removeSelected removes the selected node; the start node stays.
func (s *State) removeSelected(ctx *frame.Context) {
	if s.selected.IsZero() || s.workflow == nil || s.selected == s.workflow.Start() {
		return
	}
	removed := s.selected
	s.selected = ids.ULID{}
	args := workflowplugin.RemoveArgs{Nodes: []ids.ULID{removed}}
	s.command(ctx, workflowplugin.RemoveEdit, "", args, func(protocol.Result) {
		ctx.Frame.Status("Node removed. Undo puts it back.")
	})
}

// autoArrange lays the whole workflow out again and puts every line moved by
// hand back where routing puts it, as one undo step. Notes already placed
// keep their place, beside whatever they explain.
func (s *State) autoArrange(ctx *frame.Context) {
	boxes := arrange(s.workflow, s.noteHeight)
	for _, node := range s.workflow.Nodes() {
		if _, placed := s.workflow.Box(node.ID); placed && node.Kind == flow.KindNote {
			delete(boxes, node.ID)
		}
	}
	args := workflowplugin.MoveArgs{Boxes: boxes, Lines: nil}
	for key := range s.workflow.Bends() {
		if args.Lines == nil {
			args.Lines = map[string]flow.Bend{}
		}
		args.Lines[key] = flow.Bend{To: ids.ULID{}, Lane: nil, Depth: nil}
	}
	s.command(ctx, workflowplugin.MoveEdit, "", args, func(protocol.Result) {
		ctx.Frame.Status("Arranged. Undo puts the nodes back where they were.")
	})
}

// replace sends the edit that changes one node.
func (s *State) replace(ctx *frame.Context, node flow.Node, success string) {
	s.command(ctx, workflowplugin.ReplaceEdit, "", workflowplugin.ReplaceArgs{Node: node}, func(protocol.Result) {
		if success != "" {
			ctx.Frame.Status(success)
		}
	})
}

// command sends an edit to the workflow; done runs when it succeeds.
func (s *State) command(ctx *frame.Context, edit, gesture string, args any, done func(protocol.Result)) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: s.document, Edit: edit, Gesture: gesture, Args: data}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			ctx.Frame.Report("")(result, err)

			return
		}
		if done != nil {
			done(result)
		}
	})
}

func (s *State) gesture() string {
	s.gestures++

	return s.window.String() + "-" + strconv.Itoa(s.gestures)
}

// portAt returns the output whose port is at a canvas point.
func (s *State) portAt(point image.Point) (ids.ULID, string, bool) {
	for _, nodeID := range s.drawOrder() {
		for _, candidate := range s.shapes[nodeID].ports {
			if distance := point.Sub(candidate.at); abs(distance.X) <= portReach && abs(distance.Y) <= portReach {
				return nodeID, candidate.output.Name, true
			}
		}
	}

	return ids.ULID{}, "", false
}

// resizeHandleAt returns the container whose bottom-right corner is at a
// canvas point.
func (s *State) resizeHandleAt(point image.Point) (ids.ULID, bool) {
	for nodeID, candidate := range s.shapes {
		corner := candidate.rect.Max
		if candidate.node.Kind == flow.KindContainer && point.In(image.Rectangle{
			Min: corner.Sub(image.Pt(resizeGrab, resizeGrab)), Max: corner,
		}) {
			return nodeID, true
		}
	}

	return ids.ULID{}, false
}

// entryAt returns the filled entry dot at a canvas point: the end of a line
// that can be picked up and moved to another node.
func (s *State) entryAt(point image.Point) (entry, bool) {
	order := s.drawOrder()
	for index := len(order) - 1; index >= 0; index-- {
		for _, in := range s.shapes[order[index]].entries {
			if distance := point.Sub(in.at); abs(distance.X) <= portReach && abs(distance.Y) <= portReach {
				return in, true
			}
		}
	}

	return entry{from: ids.ULID{}, output: "", at: image.Pt(0, 0)}, false
}

// dropTarget returns the node a link dropped at a canvas point goes to: the
// node whose entry dot is there, or the topmost node there.
func (s *State) dropTarget(point image.Point) (ids.ULID, bool) {
	for _, nodeID := range s.drawOrder() {
		drawn := s.shapes[nodeID]
		if !takesEntries(drawn.node.Kind) {
			continue
		}
		dots := make([]image.Point, 0, 1+len(drawn.entries))
		dots = append(dots, drawn.spare)
		for _, in := range drawn.entries {
			dots = append(dots, in.at)
		}
		for _, dot := range dots {
			if distance := point.Sub(dot); abs(distance.X) <= portReach && abs(distance.Y) <= portReach {
				return nodeID, true
			}
		}
	}

	return s.nodeAt(point)
}

// linkTarget returns the node the link being drawn would connect to now, if
// it can take it.
func (s *State) linkTarget() ids.ULID {
	target, ok := s.dropTarget(s.link.to)
	from, exists := s.workflow.Node(s.link.from)
	if !ok || !exists || linkProblem(s.workflow, from, s.link.output, target) != "" {
		return ids.ULID{}
	}

	return target
}

// nodeAt returns the topmost node at a canvas point.
func (s *State) nodeAt(point image.Point) (ids.ULID, bool) {
	order := s.drawOrder()
	for index := len(order) - 1; index >= 0; index-- {
		if point.In(s.shapes[order[index]].rect) {
			return order[index], true
		}
	}

	return ids.ULID{}, false
}

// drawOrder lists nodes back to front: containers, then notes, which lines
// run over, then the rest, in creation order.
func (s *State) drawOrder() []ids.ULID {
	if s.workflow == nil {
		return nil
	}
	var containers, notes, others []ids.ULID
	for _, node := range s.workflow.Nodes() {
		switch node.Kind {
		case flow.KindContainer:
			containers = append(containers, node.ID)
		case flow.KindNote:
			notes = append(notes, node.ID)
		default:
			others = append(others, node.ID)
		}
	}

	return slices.Concat(containers, notes, others)
}

// clampInto keeps a rectangle of a size at a point inside an area.
func clampInto(at, size image.Point, area image.Rectangle) image.Point {
	return image.Pt(
		min(max(at.X, area.Min.X), max(area.Max.X-size.X, area.Min.X)),
		min(max(at.Y, area.Min.Y), max(area.Max.Y-size.Y, area.Min.Y)),
	)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
