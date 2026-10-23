// Package workflowview is the Workflow window: one workflow drawn as typed
// nodes and their connections on a dotted canvas, a palette of node kinds on
// the left, and an inspector on the right built from the selected node's
// declared parameters (D13). Click a palette entry to add a node after the
// selected one, or drag it onto the canvas; drag from an output's port to a
// node to connect them; drag a node to move it.
//
// The view turns input into requests in Update without needing a screen; Draw
// renders it.
package workflowview

import (
	"encoding/json"
	"image"
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
	offset   image.Point
	selected ids.ULID
	cursor   image.Point
	gestures int

	drag    *nodeDrag
	link    *linkDrag
	adding  *paletteDrag
	resize  *containerResize
	panning bool
	lastPan image.Point

	palette []paletteEntry
	arrange *ui.Button
	form    *form
	focus   ui.Focus
	picker  *picker.Picker
}

// New returns the Workflow window for a workflow.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, document: document, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		workflow: nil, shapes: map[ids.ULID]shape{}, offset: image.Pt(0, 0), selected: ids.ULID{},
		cursor: image.Pt(0, 0), gestures: 0,
		drag: nil, link: nil, adding: nil, resize: nil, panning: false, lastPan: image.Pt(0, 0),
		palette: nil, arrange: ui.NewButton("arrange", "Auto-arrange"), form: nil, focus: ui.Focus{},
		picker: picker.New(window),
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

// load copies this frame's workflow and lays it out.
func (s *State) load(docs frame.Docs) {
	s.workflow = nil
	docs.View(s.document, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			s.workflow = workflowDoc.Workflow.Clone()
		}
	})
	if s.workflow == nil {
		s.shapes = map[ids.ULID]shape{}

		return
	}
	s.shapes = shapes(s.workflow, s.noteHeight)
	if _, ok := s.workflow.Node(s.selected); !ok {
		s.selected = ids.ULID{}
	}
}

// Canvas returns the screen area that shows the workflow.
func (s *State) Canvas() image.Rectangle {
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
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
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
		s.selected, s.link, s.adding = ids.ULID{}, nil, nil
	case input.Consume(ui.KeyBackspace, plain), input.Consume(ui.KeyDelete, plain):
		s.removeSelected(ctx)
	}
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
	switch {
	case s.link != nil:
		s.updateLink(input, ctx, point)
	case s.drag != nil:
		s.updateDrag(input, ctx, point)
	case s.resize != nil:
		s.updateResize(input, ctx, point)
	case input.Pressed && input.Cursor.In(canvas):
		s.press(point)
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
// resizing a container, or selecting and moving a node.
func (s *State) press(point image.Point) {
	if from, output, ok := s.portAt(point); ok {
		s.selected = from
		s.link = &linkDrag{from: from, output: output, to: point}

		return
	}
	if container, ok := s.resizeHandleAt(point); ok {
		s.selected = container
		s.resize = &containerResize{id: container, gesture: s.gesture()}

		return
	}
	hit, ok := s.nodeAt(point)
	if !ok {
		s.selected = ids.ULID{}

		return
	}
	s.selected = hit
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
	s.command(ctx, workflowplugin.MoveEdit, drag.gesture, workflowplugin.MoveArgs{Boxes: boxes}, nil)
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
		workflowplugin.MoveArgs{Boxes: map[ids.ULID]flow.Box{s.resize.id: box}}, nil)
}

// updateLink follows a connection being drawn from a port, then connects it
// to the node it is dropped on, or clears it when dropped on empty canvas.
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
	target, dropped := s.nodeAt(point)
	if !dropped {
		if _, linked := from.Next[link.output]; linked {
			s.replace(ctx, from.WithLink(link.output, ids.ULID{}), "Disconnected")
		}

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
	s.add(ctx, kind, place, selected.node.Parent, link)
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

// add sends the edit that adds a node, then selects it and, given a link,
// connects it, all as one undo step.
func (s *State) add(ctx *frame.Context, kind string, place image.Point, parent ids.ULID, link *flow.Link) {
	node, err := flow.NewNode(kind)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	node.Parent = parent
	box := flow.Box{X: place.X, Y: place.Y, W: 0, H: 0}
	switch kind {
	case flow.KindContainer:
		box.W, box.H = containerWidth, containerHeight
	case flow.KindNote:
		box.W = noteWidth
	}
	gesture := s.gesture()
	args := workflowplugin.AddArgs{Nodes: []flow.Node{node}, Boxes: map[ids.ULID]flow.Box{{}: box}, Links: nil}
	s.command(ctx, workflowplugin.AddEdit, gesture, args, func(result protocol.Result) {
		s.selected = result.Created
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

// autoArrange lays the whole workflow out again, as one undo step. Notes
// already placed keep their place, beside whatever they explain.
func (s *State) autoArrange(ctx *frame.Context) {
	boxes := arrange(s.workflow, s.noteHeight)
	for _, node := range s.workflow.Nodes() {
		if _, placed := s.workflow.Box(node.ID); placed && node.Kind == flow.KindNote {
			delete(boxes, node.ID)
		}
	}
	s.command(ctx, workflowplugin.MoveEdit, "", workflowplugin.MoveArgs{Boxes: boxes}, func(protocol.Result) {
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

// drawOrder lists nodes back to front: containers first, then the rest in
// creation order.
func (s *State) drawOrder() []ids.ULID {
	if s.workflow == nil {
		return nil
	}
	var containers, others []ids.ULID
	for _, node := range s.workflow.Nodes() {
		if node.Kind == flow.KindContainer {
			containers = append(containers, node.ID)
		} else {
			others = append(others, node.ID)
		}
	}

	return append(containers, others...)
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
