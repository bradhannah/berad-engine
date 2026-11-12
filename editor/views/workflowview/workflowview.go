// Package workflowview is the Workflow window: one workflow drawn as typed
// nodes and their connections on a dotted canvas, a palette of node kinds on
// the left, and an inspector on the right built from the selected node's
// declared parameters (D13). Click a palette entry to add a node after the
// selected one, or drag it onto the canvas; drag from an output's port to a
// node to connect them, or pick a line up by its arrow end to move it to
// another node; drag a node to move it; click a line to select it.
// Right-click a node or a line for a menu that deletes it.
//
// The view turns input into requests in Update without needing a screen; Draw
// renders it.
package workflowview

import (
	"encoding/json"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/picker"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/hajimehoshi/ebiten/v2"
)

// ViewName is the Workflow window's view name; its target is a workflow.
const ViewName = "workflow"

// Window layout, in screen pixels.
const (
	PaletteWidth  = 200
	toolbarHeight = 40
	paletteItem   = 24
	paletteGap    = 4
	paletteTop    = toolbarHeight + ui.Inset
	groupSpace    = 26
	arrangeWidth  = 110
	recentreWidth = 90
	legendWidth   = 80
	dragStart     = 4
	shortID       = 6
	// newGap is the space between a node and one added after it.
	newGap = 60
)

// nodeDrag is a node being dragged: where it was grabbed, and its gesture.
// In a group, it moves the selection, and moved is set once it has.
type nodeDrag struct {
	id      ids.ULID
	grab    image.Point
	gesture string
	last    image.Point
	group   bool
	moved   bool
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

// lineDrag moves one leg of a line by hand; part is the part of the line's
// bend it sets.
type lineDrag struct {
	key     routeKey
	part    string
	start   image.Point
	gesture string
	moved   bool
}

// paletteEntry is one node kind offered on the left; lines are its one
// line, wrapped, while it is a search's result.
type paletteEntry struct {
	kind  flow.Kind
	rect  image.Rectangle
	group bool
	lines []string
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
	// warnings are what deserves a look on a node but is not wrong, such as
	// a receiver nothing sends to.
	warnings map[ids.ULID]string
	// localProblems say why a place local's start is out of the reach of
	// the NPCs that run the workflow, by the local's ID.
	localProblems map[ids.ULID]string
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
	// group is the selection while several nodes are selected; selected is
	// then none. removeGroup is the inspector's Delete N nodes.
	group       []ids.ULID
	removeGroup *ui.Button
	// box is the box being dragged on empty canvas to select what it holds;
	// nudges the run of arrow-key nudges under way.
	box    *selectBox
	nudges nudging
	// selectAdded is the nodes there before a copy was added, while the
	// copies wait to be selected.
	selectAdded map[ids.ULID]bool
	// showing is a node asked for with NodeItem, to select once it arrives.
	showing  ids.ULID
	cursor   image.Point
	gestures int
	// hoverLink is the line under the pointer, selectedLink the selected
	// line and hoverMark the node whose problem mark is under the pointer.
	hoverLink    routeKey
	selectedLink routeKey
	hoverMark    ids.ULID

	drag   *nodeDrag
	link   *linkDrag
	adding *paletteDrag
	// nodeAdd is where the node menu adds a node, and the line that joins
	// it (increment 40).
	nodeAdd *nodeMenu
	// paletteFolded is set while the palette is folded to its strip, and
	// foldBn folds and unfolds it (increment 43).
	paletteFolded bool
	foldBn        *ui.Button
	resize        *containerResize
	bending       *lineDrag
	panning       bool
	lastPan       image.Point
	// zoom is the canvas's zoom level, an index into zoomLevels; panRest is
	// the part of a canvas pixel panned but not yet moved, and wheel the
	// Cmd+scroll not yet zoomed by. drawn is the image a zoomed canvas is
	// drawn into before it is scaled.
	zoom     int
	zoomCard *zoomControl
	panRest  [2]float64
	// handPanned is set when the author pans the canvas by hand, for
	// Watch's Follow (increment 41).
	handPanned bool
	wheel      float64
	drawn      *ebiten.Image

	palette []paletteEntry
	// pal is the palette's search, its tooltip and the help it shows.
	pal        *palettePanel
	arrange    *ui.Button
	recentre   *ui.Button
	deleteLine *ui.Button
	resetLine  *ui.Button
	// legend is the toolbar's Legend toggle; legendBox is where the legend
	// card sits over the canvas while it is on.
	legend    *ui.Button
	legendBox image.Rectangle
	// tints are the workflow's tints as laid out; colours the inspector's
	// colour rows for the selection.
	tints   flow.Tints
	colours []colourRow
	// values is the value picker while it is open; lastGroup the group last
	// opened in one, opened again in the next.
	values    *valuePicker
	lastGroup string
	// signalPick is the signal picker while it is open.
	signalPick *signalPick
	// others is the list of other places typed the same, once a constant
	// is made.
	others *otherPlaces
	// places is the list of places while it is open; lit is the place
	// control whose box glows, its place just set, for litLeft frames more.
	places  *placeList
	lit     string
	litLeft int
	// shown is whether this view's tab is in front, as the window says.
	shown  bool
	form   *form
	save   *ui.SaveBar
	guard  frame.Guard
	focus  ui.Focus
	picker *picker.Picker
	// live is set for a read-only view of an NPC's workflow in the Watch
	// window.
	live *live
	// audio is the project's audio library as last listed, audioRevision
	// counts listings, and audioAsking is set while a question is out.
	// sounds plays cues in this window, as last seen in Update.
	audio         *protocol.AudioLibrary
	audioRevision int
	audioAsking   bool
	sounds        frame.Sounds
	// modules are the workflows its Calls name, as last loaded, callsKey
	// what they were loaded from, and missingWays each Call's ways out its
	// module no longer has.
	modules     map[ids.ULID]module
	callsKey    string
	missingWays map[ids.ULID]map[string]bool
	// callers is who calls this workflow, as last asked, and runs the NPCs
	// that run it.
	callers callers
	runs    runners
	// locals is the inspector's part for the workflow's local values, shown
	// while no node or line is selected.
	locals *localsPanel
	// contextMenu is the canvas's right-click menu, opened once the
	// right-click waiting in context has settled the selection.
	contextMenu ui.Menu
	context     contextClick
	// zones is the zone picker while it is open, asking where the workflow
	// runs; recentZones are the zones chosen in it lately; runsInBox is the
	// Runs in box.
	zones       *zonepicker.Picker
	zonesChose  func(*frame.Context, zonepicker.Choice)
	recentZones []ids.ULID
	runsInBox   image.Rectangle
	// dialog asks before a change that breaks something, and dialogThen
	// carries the change out, as one gesture; joining is a gesture the
	// edits sent meanwhile join.
	dialog     *ui.Dialog
	dialogThen func(*frame.Context, string)
	joining    string
	tied       bool
	// lineSheet is the writing sheet while it writes the selected node's
	// line; scrolled is how far the node inspector is scrolled.
	lineSheet *lineSheet
	scrolled  inspectorScroll
}

// New returns the Workflow window for a workflow.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, document: document, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		workflow: nil, shapes: map[ids.ULID]shape{}, routes: map[routeKey][]image.Point{}, loops: map[routeKey]bool{},
		problems: map[ids.ULID]string{}, warnings: map[ids.ULID]string{}, localProblems: map[ids.ULID]string{},
		laidOut: 0, hasLayout: false,
		layouts: 0, problemKey: "", watchFloors: false,
		offset: image.Pt(0, 0), selected: ids.ULID{}, group: nil, removeGroup: groupDeleteButton(), box: nil,
		nudges: nudging{gesture: "", of: "", idle: 0}, selectAdded: nil,
		showing: ids.ULID{}, cursor: image.Pt(0, 0), gestures: 0,
		hoverLink: noRoute(), selectedLink: noRoute(), hoverMark: ids.ULID{},
		drag: nil, link: nil, adding: nil, resize: nil, bending: nil, panning: false, lastPan: image.Pt(0, 0),
		zoom: defaultZoom, zoomCard: newZoomControl(), panRest: [2]float64{0, 0}, handPanned: false, wheel: 0, drawn: nil,
		palette: nil, pal: newPalettePanel(theme), arrange: ui.NewButton("arrange", "Auto-arrange"),
		recentre:   ui.NewButton("recentre", "Recentre"),
		deleteLine: ui.NewButton("delete-line", "Unlink"), resetLine: ui.NewButton("reset-line", "Reset line"),
		legend: ui.NewButton("legend", "Legend"), legendBox: ui.Zero(),
		tints: flow.Tints{Nodes: nil, Outcomes: nil, Lines: nil}, colours: nil, values: nil, lastGroup: "", signalPick: nil,
		others: nil,
		form:   nil, save: ui.NewSaveBar(), guard: frame.Guard{}, focus: ui.Focus{}, picker: picker.New(window), live: nil,
		places: nil, lit: "", litLeft: 0, shown: true,
		audio: nil, audioRevision: 0, audioAsking: false, sounds: nil, locals: newLocalsPanel(theme),
		modules: map[ids.ULID]module{}, callsKey: "", missingWays: map[ids.ULID]map[string]bool{},
		callers:     callers{names: nil, key: "", asking: false, wait: 0, known: false},
		runs:        runners{uses: nil, key: "", asking: false, wait: 0, known: false},
		contextMenu: ui.Menu{Items: nil, Width: 0}, context: noContextClick(), nodeAdd: nil,
		paletteFolded: false, foldBn: ui.NewButton(foldID, "‹"),
		zones: nil, zonesChose: nil, recentZones: nil, runsInBox: ui.Zero(), dialog: nil, dialogThen: nil, joining: "",
		tied: false, lineSheet: nil, scrolled: inspectorScroll{offset: 0, most: 0, of: ids.ULID{}},
	}
	view.palette = paletteFor(flow.PurposeBehaviour)

	return view
}

// paletteFor lists the node kinds a kind of workflow offers, in palette
// order.
func paletteFor(purpose flow.Purpose) []paletteEntry {
	var entries []paletteEntry
	for _, kind := range flow.KindsFor(purpose) {
		if kind.Group != "" {
			entries = append(entries, paletteEntry{kind: kind, rect: ui.Zero(), group: false, lines: nil})
		}
	}

	return entries
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
// While the value picker is open, it is the picker's.
func (s *State) Focused() *ui.TextField {
	switch {
	case s.values != nil:
		return s.values.picker.Focused()
	case s.signalPick != nil:
		return s.signalPick.picker.Focused()
	case s.places != nil:
		return s.places.list.Focused()
	case s.pal.focus.Current() != nil:
		return s.pal.focus.Current()
	}

	return s.focus.Current()
}

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
	if update.Attention && strings.HasPrefix(update.Item, NodeItem) {
		if node, err := ids.Parse(strings.TrimPrefix(update.Item, NodeItem)); err == nil {
			s.showing = node
		}
	}
	s.picker.Changed(update, ctx)
	if update.AudioChanged != nil && s.audio != nil {
		s.askAudio(ctx)
	}
}

// load copies the workflow and lays it out, its nodes, lines and loops, when
// it has changed since the last layout, and finds its problems when it or
// anything they depend on has changed.
func (s *State) load(docs frame.Docs) {
	revision, found := uint64(0), false
	var called []ids.ULID
	docs.View(s.document, func(document client.Document) {
		workflowDoc, ok := document.Model.(*workflowplugin.Document)
		if !ok {
			return
		}
		found, revision, called = true, document.Revision, callees(workflowDoc.Workflow)
	})
	calls := s.loadModules(docs, called)
	changed := !s.hasLayout || revision != s.laidOut || calls != s.callsKey
	if found && changed {
		docs.View(s.document, func(document client.Document) {
			if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
				s.workflow = workflowDoc.Workflow.Clone()
				s.palette = paletteFor(s.workflow.Purpose())
			}
		})
		s.callsKey = calls
		s.showWays()
	}
	if !found {
		s.workflow, s.hasLayout = nil, false
		s.shapes, s.routes, s.loops = map[ids.ULID]shape{}, map[routeKey][]image.Point{}, map[routeKey]bool{}
		s.problems, s.tints = map[ids.ULID]string{}, flow.Tints{Nodes: nil, Outcomes: nil, Lines: nil}

		return
	}
	if changed {
		s.shapes = shapes(s.workflow, s.noteHeight)
		s.routes, s.loops = routes(s.workflow, s.shapes), loops(s.workflow)
		s.tints = s.workflow.Tints()
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
	s.selectAddedNodes()
	s.settleSelection()
}

// Canvas returns the screen area that shows the workflow.
func (s *State) Canvas() image.Rectangle {
	if s.live != nil {
		return s.live.area
	}
	content := frame.Content(s.size)

	return image.Rect(s.paletteWidth(), toolbarHeight, content.Max.X, content.Max.Y)
}

func (s *State) toScreen(point image.Point) image.Point {
	return point.Add(s.Canvas().Min).Add(s.offset)
}

// toCanvas returns the canvas point under a screen point.
func (s *State) toCanvas(point image.Point) image.Point {
	from, scale := point.Sub(s.Canvas().Min), s.scale()
	at := image.Pt(int(math.Floor(float64(from.X)/scale)), int(math.Floor(float64(from.Y)/scale)))

	return at.Sub(s.offset)
}

// layoutWidgets places the palette, the toolbar and the inspector.
func (s *State) layoutWidgets(docs frame.Docs) {
	content := frame.Content(s.size)
	s.arrange.Rect = image.Rect(content.Max.X-ui.Inset-arrangeWidth, (toolbarHeight-ui.ButtonH)/ui.Half,
		content.Max.X-ui.Inset, (toolbarHeight+ui.ButtonH)/ui.Half)
	s.zoomCard.layout(s.Canvas(), s.zoom, s.lowestZoom())
	s.recentre.Rect = image.Rect(s.arrange.Rect.Min.X-ui.Inset-recentreWidth, s.arrange.Rect.Min.Y,
		s.arrange.Rect.Min.X-ui.Inset, s.arrange.Rect.Max.Y)
	s.legend.Rect = image.Rect(s.recentre.Rect.Min.X-ui.Inset-legendWidth, s.arrange.Rect.Min.Y,
		s.recentre.Rect.Min.X-ui.Inset, s.arrange.Rect.Max.Y)
	s.legendBox = s.legendLayout()
	s.layoutPalette()
	s.layoutHelp()
	s.layoutForm(docs)
	s.layoutLocals()
	s.deleteLine.Rect, s.resetLine.Rect = ui.Zero(), ui.Zero()
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	s.colours = nil
	s.layoutGroup(left, width, panel.Min.Y+ui.Inset+ui.LineHeight)
	if s.lineSelected() {
		bottom := panel.Max.Y - ui.Inset
		s.deleteLine.Rect = image.Rect(left, bottom-ui.ButtonH, panel.Max.X-ui.Inset, bottom)
		if _, bent := s.workflow.Bend(s.selectedLink.from, s.selectedLink.output); bent {
			above := bottom - ui.ButtonH - rowSpace
			s.resetLine.Rect = image.Rect(left, above-ui.ButtonH, panel.Max.X-ui.Inset, above)
		}
		top := panelTextTop(panel.Min.Y+ui.Inset) + (forwardCorners+1)*ui.LineHeight + fieldSpace
		s.colours = s.layoutColours(s.colourRows(), left, width, top)
	} else if s.form != nil {
		s.colours = s.layoutColours(s.colourRows(), left, width, s.form.bottom)
		if s.fitScroll(max(s.form.bottom, coloursBottom(s.colours))) {
			s.layoutForm(docs)
			s.colours = s.layoutColours(s.colourRows(), left, width, s.form.bottom)
		}
	}
}

// coloursBottom is where the last colour row ends, or nothing with none.
func coloursBottom(rows []colourRow) int {
	bottom := 0
	for _, row := range rows {
		for _, choice := range row.swatches {
			bottom = max(bottom, choice.rect.Max.Y)
		}
	}

	return bottom
}

// ask asks the orchestrator, when what it knew may have changed, who calls
// the workflow, which NPCs run it and the project's audio.
func (s *State) ask(ctx *frame.Context) {
	s.askCallers(ctx)
	s.askRunners(ctx)
	s.subscribeHomes(ctx)
	s.listAudio(ctx)
	s.subscribeSignals(ctx)
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	if s.live != nil {
		s.updateLive(input, ctx)

		return
	}
	s.startFrame(input, ctx)
	// The status bar names what is in hand once the frame has moved it.
	defer func() { ctx.Frame.SetHand(s.handLine(ctx.Docs)) }()
	s.showNode()
	s.ask(ctx)
	if s.guard.Active() {
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discardForm)

		return
	}
	s.guardSelection(ctx)
	s.syncForm(ctx)
	s.syncLocals(ctx)
	s.layoutWidgets(ctx.Docs)
	if s.updateOverlay(input, ctx) {
		return
	}
	s.scrollForm(input, ctx)
	s.updateTyping(input, ctx)
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
	if !s.updateHelp(input, ctx) {
		s.updateLineButtons(input, ctx)
		s.updateGroup(input, ctx)
		s.updateColours(input, ctx)
		s.updateForm(input, ctx)
		s.updateLocals(input, ctx)
	}
	s.updateToolbar(input, ctx)
	s.updateZoom(input)
	if !s.updateFold(input, ctx) && !s.paletteFolded {
		s.updatePalette(input, ctx)
	}
	s.updateCanvas(input, ctx)
	s.settleSelection()
	s.guardSelection(ctx)
	s.openContextMenu()
}

// startFrame takes what a frame starts from: the window's size, the
// pointer, the sounds and whether the palette is folded; then fades what
// was lit, counts a frame since the last nudge, and loads the workflow.
func (s *State) startFrame(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.cursor = input.Cursor
	s.sounds = ctx.Sounds
	s.paletteFolded = s.live == nil && !ctx.Frame.ShowLeft.On
	s.fade()
	s.nudges.idle++
	s.load(ctx.Docs)
}

// updateTyping runs the palette's search, then the inspector's fields while
// no help page covers them, then the window's own keys while no field has
// focus.
func (s *State) updateTyping(input *ui.Input, ctx *frame.Context) {
	s.updateSearch(input, ctx)
	if s.pal.help == "" {
		if field, outcome := s.focus.Update(input, ctx.Clipboard); field != nil && outcome == textedit.OutcomeCommit {
			s.Save(ctx)
		}
	}
	if s.focus.Current() == nil && s.pal.focus.Current() == nil {
		s.keys(input, ctx)
	}
}

// updateToolbar handles Auto-arrange, Recentre and the Legend toggle, and
// keeps a press on the legend's card from reaching the canvas under it.
func (s *State) updateToolbar(input *ui.Input, ctx *frame.Context) {
	if s.arrange.Update(input) {
		if len(s.group) > 0 {
			s.arrangeSelection(ctx)
		} else {
			s.autoArrange(ctx)
		}
	}
	if s.recentre.Update(input) {
		s.recentreView(ctx)
	}
	if s.legend.Update(input) {
		s.legend.On = !s.legend.On
	}
	if input.Pressed && input.Cursor.In(s.legendBox) {
		input.ClearPointer()
	}
}

// updateLineButtons handles Delete line and Reset line for the selected
// line.
func (s *State) updateLineButtons(input *ui.Input, ctx *frame.Context) {
	if !s.lineSelected() {
		return
	}
	if s.deleteLine.Update(input) {
		s.removeLine(ctx)
	}
	if !s.resetLine.Rect.Empty() && s.resetLine.Update(input) {
		s.bend(ctx, s.selectedLink, flow.Bend{To: ids.ULID{}, Lane: nil, Depth: nil, End: nil}, "")
		ctx.Frame.Status("Line reset. Undo puts it back where you moved it.")
	}
}

// listAudio asks for the audio library once a workflow with a Play sound
// is shown, for its cues' names and problems, and keeps the world state and
// the NPCs loaded, for its values' paths and problems.
func (s *State) listAudio(ctx *frame.Context) {
	if s.workflow != nil && s.audio == nil && s.playsSounds() {
		s.askAudio(ctx)
	}
	s.watchValues(ctx)
	s.watchCalls(ctx)
}

// keys handles the window's own keys while no text field has focus. Escape
// closes an open cue chooser before anything else; Enter opens the picker
// for a selected value node's first box.
func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	plain := ui.Plain()
	if made, open := s.openChooser(); open && input.Consume(ui.KeyEscape, plain) {
		s.closeChooser(made)

		return
	}
	if s.selectionKeys(input, ctx) {
		return
	}
	switch {
	case s.picker.Picking() && input.Consume(ui.KeyEscape, plain):
		s.picker.Cancel(ctx, cancelledHere)
	case s.pal.help != "" && input.Consume(ui.KeyEscape, plain):
		s.pal.help = ""
	case input.Consume(ui.KeyEscape, plain):
		s.selected, s.selectedLink, s.link, s.adding, s.group, s.box = ids.ULID{}, noRoute(), nil, nil, nil, nil
	case s.zoomKeys(input):
	case input.Consume(ui.KeyEnter, plain):
		s.openFirstBox()
	case input.ConsumeExact(ui.KeyTab, plain):
		s.openNodeMenuAt(input.Cursor)
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

// removeLine unlinks the selected line, saying what from what; undo joins
// it again.
func (s *State) removeLine(ctx *frame.Context) {
	from, ok := s.workflow.Node(s.selectedLink.from)
	if !ok {
		return
	}
	output := s.selectedLink.output
	said := "Unlinked " + labelOf(from.Kind) + " · " + output
	if target, found := s.workflow.Node(from.Next[output]); found {
		said += " from " + labelOf(target.Kind)
	}
	s.selectedLink = noRoute()
	s.replace(ctx, from.WithLink(output, ids.ULID{}), said+". Undo joins it again.")
}

// updateCanvas handles panning and the gestures on nodes and ports.
func (s *State) updateCanvas(input *ui.Input, ctx *frame.Context) {
	canvas := s.Canvas()
	s.pan(input, canvas)
	if s.panning || input.Space {
		return
	}
	point := s.toCanvas(input.Cursor)
	onCanvas := input.Cursor.In(canvas) && !s.zoomCard.contains(input.Cursor) && !input.Cursor.In(s.legendBox)
	s.updateHover(ctx, onCanvas, point)
	if s.updateGesture(input, ctx, point) {
		return
	}
	switch {
	case input.Secondary && input.Cursor.In(canvas) && !s.zoomCard.contains(input.Cursor):
		s.rightClick(point, input.Cursor)
	case input.Pressed && input.Cursor.In(canvas):
		s.pressCanvas(input, ctx, point)
	}
}

// updateGesture carries on a gesture begun by a press, and reports whether
// one was under way: drawing a line, bending one, dragging a node, sizing a
// container or dragging a box.
func (s *State) updateGesture(input *ui.Input, ctx *frame.Context, point image.Point) bool {
	switch {
	case s.link != nil:
		s.updateLink(input, ctx, point)
	case s.bending != nil:
		s.updateBend(input, ctx, point)
	case s.drag != nil && s.drag.group:
		s.updateGroupDrag(input, ctx, point)
	case s.drag != nil:
		s.updateDrag(input, ctx, point)
	case s.resize != nil:
		s.updateResize(input, ctx, point)
	case s.box != nil:
		s.updateBox(input, point)
	default:
		return false
	}

	return true
}

func (s *State) pan(input *ui.Input, canvas image.Rectangle) {
	if input.Cursor.In(canvas) && (input.WheelX != 0 || input.WheelY != 0) {
		if input.Held.Shortcut && s.live == nil {
			s.wheelZoom(input)
		} else {
			s.panBy(input.WheelPan())
			s.handPanned = true
		}
	}
	dragPan := input.PanDown || (input.Space && input.Down)
	if dragPan && s.panning {
		s.panBy(input.Cursor.Sub(s.lastPan))
		s.handPanned = true
	}
	s.panning = dragPan && (s.panning || input.Cursor.In(canvas))
	s.lastPan = input.Cursor
}

// press starts what a press on the canvas begins: a connection from a port,
// a line picked up by its arrow end, resizing a container, or selecting and
// moving a node.
func (s *State) press(ctx *frame.Context, point image.Point) {
	group := s.group
	s.group = nil
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
			if drawn.node.Kind == flow.KindTogether {
				s.addLane(ctx, drawn.node)
			} else {
				s.addOption(ctx, drawn.node)
			}

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
		if part, onLeg := s.legAt(key, point); onLeg {
			s.bending = &lineDrag{key: key, part: part, start: point, gesture: s.gesture(), moved: false}
		}

		return
	}
	hit, ok := s.nodeAt(point)
	if !ok {
		s.selected, s.selectedLink = ids.ULID{}, noRoute()
		s.startBox(point, false)

		return
	}
	drag := &nodeDrag{
		id: hit, grab: point.Sub(s.shapes[hit].rect.Min), gesture: s.gesture(), last: point, group: false, moved: false,
	}
	if slices.Contains(group, hit) {
		s.group, drag.group = group, true
	} else {
		s.selected, s.selectedLink = hit, noRoute()
	}
	s.drag = drag
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
		target = clampInto(target, moving.rect.Size(), innerOf(container))
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
		if flow.IsContainer(moving.node.Kind) {
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
	least := container.rect.Min.Add(containerMin(container.node))
	right, bottom := max(point.X, least.X), max(point.Y, least.Y)
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

// updateBend moves one leg of a line as it is dragged: a forward line's
// upright part or a back line's drop and end legs sideways, a back line's
// run up or down, above the nodes if wanted. The line's other legs keep
// their offsets. Each change is a move edit in the drag's gesture.
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
	output, entry := corners[0], corners[len(corners)-1]
	current, bent := s.workflow.Bend(drag.key.from, drag.key.output)
	bend := current
	bend.To = target
	switch drag.part {
	case partDepth:
		depth := point.Y - output.Y
		bend.Depth = &depth
	case partEnd:
		end := point.X - entry.X
		bend.End = &end
	default:
		lane := point.X - output.X
		bend.Lane = &lane
	}
	if bent && sameBend(current, bend) {
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

	return a.To == b.To && same(a.Lane, b.Lane) && same(a.Depth, b.Depth) && same(a.End, b.End)
}

// updateLink follows a connection being drawn from a port, or a line picked
// up by its arrow end, then connects it to the node it is dropped on, or
// offers a node to join it to when dropped on empty canvas. Dropped back on
// the node it already leads to, it changes nothing.
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
		// A line dropped on empty canvas offers a node to join it to; the
		// line stays as it was unless one is chosen (increment 40).
		s.openNodeMenu(input.Cursor, point, &flow.Link{From: from.ID, Output: link.output, To: ids.ULID{}})

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
	case targetNode.Kind == flow.KindOnSignal:
		return "Nothing links into a receiver: it runs when its signal is heard."
	case !flow.Runs(targetNode.Kind):
		return "Notes are not part of the flow, so nothing can link to them."
	case flow.IsInner(from, output):
		if targetNode.Parent != from.ID {
			return "A container's first step must be inside it."
		}
	case targetNode.Parent != from.Parent:
		return "Links stay inside one container; leave a container by its exit."
	}

	return ""
}

// addAfterSelected adds a node after the selected one, connected from its
// first free output, or beside it, connected to nothing, when it has none
// free; or inside a selected container; with nothing selected it goes in the
// middle of the view.
func (s *State) addAfterSelected(ctx *frame.Context, kind string) {
	selected, ok := s.shapes[s.selected]
	if !ok {
		canvas := s.Canvas()
		s.add(ctx, kind, s.toCanvas(ui.Center(canvas)).Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half)),
			ids.ULID{}, nil)

		return
	}
	if kind == flow.KindNote || flow.IsStart(kind) {
		place, parent := image.Pt(selected.rect.Min.X, selected.rect.Min.Y-noteMinHeight-rowGap), selected.node.Parent
		if flow.IsStart(kind) {
			place, parent = s.clearOf(image.Pt(selected.rect.Min.X, selected.rect.Max.Y+rowGap), ids.ULID{}), ids.ULID{}
		}
		s.add(ctx, kind, place, parent, nil)

		return
	}
	if flow.IsContainer(selected.node.Kind) && !flow.IsContainer(kind) {
		s.addInside(ctx, kind, selected)

		return
	}
	place := image.Pt(selected.rect.Max.X+newGap, selected.rect.Min.Y)
	var link *flow.Link
	for _, candidate := range selected.ports {
		if _, used := selected.node.Next[candidate.output.Name]; used || flow.IsInner(selected.node, candidate.output.Name) {
			continue
		}
		link = &flow.Link{From: selected.node.ID, Output: candidate.output.Name, To: ids.ULID{}}
		place.Y = candidate.at.Y - nodeHeight/ui.Half

		break
	}
	s.add(ctx, kind, s.clearOf(place, selected.node.Parent), selected.node.Parent, link)
}

// addInside adds a node inside a selected container: as its first step, or
// a Together's first empty lane's, when it has none; otherwise free, below
// what is inside.
func (s *State) addInside(ctx *frame.Context, kind string, selected shape) {
	place := innerOf(selected).Min
	first := port{output: flow.Output{Name: flow.OutputInside, Label: ""}, at: place, row: image.Rect(0, 0, 0, 0)}
	var free bool
	if selected.node.Kind == flow.KindTogether {
		first, free = firstEmptyLane(selected)
		place = image.Pt(place.X, first.at.Y-entryTop)
	} else {
		_, used := selected.node.Next[flow.OutputInside]
		free = !used
	}
	var link *flow.Link
	if free {
		link = &flow.Link{From: selected.node.ID, Output: first.output.Name, To: ids.ULID{}}
	} else {
		place = s.freeSpot(selected.node.ID, innerOf(selected).Min)
	}
	s.add(ctx, kind, place, selected.node.ID, link)
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

// addAt adds a node where it is dropped, inside the container there, if
// any, unless it is a receiver, which sits at the top level; dropped in an
// empty lane of a Together, it is the lane's first step.
func (s *State) addAt(ctx *frame.Context, kind string, point image.Point) {
	parent := ids.ULID{}
	var link *flow.Link
	if !flow.IsContainer(kind) && !flow.IsStart(kind) {
		for nodeID, candidate := range s.shapes {
			if flow.IsContainer(candidate.node.Kind) && point.In(innerOf(candidate)) {
				parent = nodeID
			}
		}
	}
	if lane, ok := laneAt(s.shapes[parent], point); ok && kind != flow.KindNote {
		if _, used := s.shapes[parent].node.Next[lane.Port]; !used {
			link = &flow.Link{From: parent, Output: lane.Port, To: ids.ULID{}}
		}
	}
	s.add(ctx, kind, point.Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half)), parent, link)
}

// newBox is a new node's box at a place; containers and notes start at
// their usual size.
func newBox(kind string, place image.Point) flow.Box {
	box := flow.Box{X: place.X, Y: place.Y, W: 0, H: 0}
	switch {
	case flow.IsContainer(kind):
		box.W, box.H = containerWidth, containerHeight
	case kind == flow.KindNote:
		box.W = noteWidth
	}

	return box
}

// updateHover follows the line and the problem mark under the pointer, and
// says what the problem is when the pointer comes onto a mark.
func (s *State) updateHover(ctx *frame.Context, onCanvas bool, point image.Point) {
	was := s.hoverLink
	s.hoverLink = noRoute()
	mark := ids.ULID{}
	if onCanvas && s.drag == nil && s.resize == nil {
		s.hoverLink, _ = s.linkAt(point)
		mark = s.markAt(point)
	}
	if s.hoverLink != was && s.hoverLink != noRoute() {
		ctx.Frame.Status(s.describeLine(s.hoverLink, ctx.Docs))
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
	s.addNode(ctx, node, place, link)
}

// addNode sends the edit that adds a node already made, then selects it
// and, given a link, connects it, all as one undo step.
func (s *State) addNode(ctx *frame.Context, node flow.Node, place image.Point, link *flow.Link) {
	if node.Kind == flow.KindReturn {
		node = s.freshReturn(node)
	}
	box := newBox(node.Kind, place)
	gesture := s.gesture()
	args := workflowplugin.AddArgs{
		Nodes: []flow.Node{node}, Boxes: map[ids.ULID]flow.Box{{}: box}, Links: nil, Tints: nil, Fresh: false,
	}
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
	if len(s.group) > 0 {
		s.removeGroupNodes(ctx)

		return
	}
	if s.selected.IsZero() || s.workflow == nil || s.selected == s.workflow.Start() {
		return
	}
	removed := s.selected
	// Typed changes to a node being removed go with it.
	s.discardForm()
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
		args.Lines[key] = flow.Bend{To: ids.ULID{}, Lane: nil, Depth: nil, End: nil}
	}
	s.command(ctx, workflowplugin.MoveEdit, "", args, func(protocol.Result) {
		ctx.Frame.Status("Arranged. Undo puts the nodes back where they were.")
	})
}

// recentreView pans the canvas so the whole workflow is in the middle of it,
// at the current zoom, for an author who has panned away from it.
func (s *State) recentreView(ctx *frame.Context) {
	if bounds, ok := s.workflowBounds(); ok {
		s.centreOn(bounds)
		ctx.Frame.Status("Centred on the workflow.")
	}
}

// workflowBounds is the canvas area every node takes, if there are any.
func (s *State) workflowBounds() (image.Rectangle, bool) {
	var bounds image.Rectangle
	first := true
	for _, made := range s.shapes {
		if first {
			bounds, first = made.rect, false
		} else {
			bounds = bounds.Union(made.rect)
		}
	}

	return bounds, !first
}

// NodeItem opens a workflow with one of its nodes selected and in the
// middle of the canvas: NodeItem then the node's ID.
const NodeItem = "node:"

// Selection is the node selected, as the item that shows it, for Back and
// Forward (increment 38); "" while no node is.
func (s *State) Selection() string {
	if s.selected.IsZero() {
		return ""
	}

	return NodeItem + s.selected.String()
}

// showNode selects the node asked for with NodeItem, and centres on it, once
// the workflow holding it has arrived.
func (s *State) showNode() {
	if s.showing.IsZero() || s.workflow == nil {
		return
	}
	if drawn, ok := s.shapes[s.showing]; ok {
		s.selected, s.selectedLink = s.showing, noRoute()
		s.centreOn(drawn.rect)
	}
	s.showing = ids.ULID{}
}

// centreOn pans the canvas so an area is in the middle of it. An area wider
// or taller than the canvas shows is shown from its top-left corner instead,
// where the start is.
func (s *State) centreOn(bounds image.Rectangle) {
	room := s.drawnArea().Size()
	s.offset = image.Pt(centred(bounds.Min.X, bounds.Dx(), room.X), centred(bounds.Min.Y, bounds.Dy(), room.Y))
	s.panRest = [2]float64{0, 0}
}

// centred returns the offset that puts a span of the given size, starting at
// from, in the middle of room, or at an inset from its start when it does not
// fit.
func centred(from, size, room int) int {
	if size > room {
		return ui.Inset - from
	}

	return (room-size)/ui.Half - from
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
	if gesture == "" {
		gesture = s.joining
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
		if flow.IsContainer(candidate.node.Kind) && point.In(image.Rectangle{
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
		switch {
		case flow.IsContainer(node.Kind):
			containers = append(containers, node.ID)
		case node.Kind == flow.KindNote:
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
