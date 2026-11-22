// Package mapview is the Map window: one zone, shown a floor at a time with
// the floor below faintly underneath and the zone origin, cell 0,0, marked.
// Authors paint ground, draw walls with a family and change families for a
// whole run, room or area, place doors, stairs and pickup stand-ins, add and
// name anchors, resize floors on any side, answer picks from other windows,
// and preview the zone by walking the player through it (D15).
//
// The view turns input into requests in Update without needing a screen; Draw
// renders it.
package mapview

import (
	"encoding/json"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/zonepalettes"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// ViewName is the Map window's view name; its target is a zone.
const ViewName = "map"

// Panel sizes, in screen pixels.
const (
	// PanelWidth is the left panel's width until it is dragged. What a
	// tool paints with is one chip to a line at this width, and two once
	// the panel is dragged wide enough for the longest name (increment 66).
	PanelWidth = 190
	// toolRows is the tools and the headings of their four groups.
	toolRows     = 12
	floorRows    = 5
	panelGap     = 10
	headingSpace = 22
	// floorsHeading heads the list of the zone's floors.
	floorsHeading = "FLOORS (TOP FIRST)"
	// numberedTiles have a number key; anchorRows of details sit between
	// the anchor name and its Delete button.
	numberedTiles = 9
	anchorRows    = 3
)

// Tool is what dragging on the map does.
type Tool string

// Tools.
const (
	ToolGround  Tool = "ground"
	ToolWalls   Tool = "walls"
	ToolObjects Tool = "objects"
	ToolErase   Tool = "erase"
	ToolAnchor  Tool = "anchor"
	ToolNPC     Tool = "npc"
	ToolSelect  Tool = "select"
	ToolResize  Tool = "resize"
)

// toolInfo is a tool: its name in the status bar and the inspector, its key
// and what it does; and, in the tool list, the group it is listed under by
// its job, its shorter name there, and its icon (increment 66).
type toolInfo struct {
	tool  Tool
	label string
	key   ui.Key
	help  string
	group string
	short string
	icon  ui.Icon
}

// The tools' groups, by job.
const (
	groupSelect = "SELECT"
	groupPaint  = "PAINT"
	groupAdd    = "ADD"
	groupFloor  = "FLOOR"
	// toolGroup prefixes a group's heading row in the tool list.
	toolGroup = "tool-group:"
)

func tools() []toolInfo {
	return []toolInfo{
		{
			ToolSelect, "Select", "S", "Click a thing to select it; drag it to move it. A wall selects its run.",
			groupSelect, "Select", ui.IconSelect,
		},
		{ToolGround, "Ground", "G", "Drag to paint the ground.", groupPaint, "Ground", ui.IconGround},
		{ToolWalls, "Walls", "W", "Drag to draw walls in the current family.", groupPaint, "Walls", ui.IconWalls},
		{
			ToolObjects, "Doors, stairs, pickups", "O", "Drag to place a door, stairs or a pickup.",
			groupPaint, "Doors, stairs, pickups", ui.IconDoor,
		},
		{ToolErase, "Erase", "E", "Drag to clear what sits on the ground.", groupPaint, "Erase", ui.IconErase},
		{ToolAnchor, "Anchors", "A", "Click to add an anchor, a place NPCs go to.", groupAdd, "Anchor", ui.IconPlace},
		{ToolNPC, "Place NPC", "N", "Click to place the NPC chosen on the left.", groupAdd, "NPC", ui.IconNPC},
		{ToolResize, "Resize floor", "R", "Drag an edge or corner of the floor.", groupFloor, "Resize floor", ui.IconResize},
	}
}

// toolListRows lists the tools under the heading of each group, each with
// its icon and its key as a cap.
func toolListRows() []ui.Row {
	var rows []ui.Row
	group := ""
	for _, info := range tools() {
		if info.group != group {
			group = info.group
			rows = append(rows, ui.NewHeading(toolGroup+group, group))
		}
		row := ui.NewRow(string(info.tool), info.short)
		row.Glyph, row.Key = info.icon, string(info.key)
		rows = append(rows, row)
	}

	return rows
}

// tilesFor returns the stand-ins a tool paints with.
func tilesFor(tool Tool) []tiles.Definition {
	var offered []tiles.Definition
	for _, definition := range tiles.StandIns() {
		switch {
		case tool == ToolGround && definition.Layer == tiles.LayerGround,
			(tool == ToolWalls || tool == ToolSelect) && definition.IsWall(),
			tool == ToolObjects && definition.Layer == tiles.LayerTop && !definition.IsWall():
			offered = append(offered, definition)
		}
	}
	if tool == ToolGround {
		offered = append(offered, tiles.Definition{
			ID: "", Name: "Unassigned", Layer: tiles.LayerGround, Category: tiles.CategoryGround,
			Color: tiles.ColorOf(""), Passable: true, BlocksSight: false, Gettable: false,
		})
	}

	return offered
}

type stroke struct {
	active bool
	count  int
	last   Cell
	// left is set while the pointer is off the floor with the button
	// held: where it comes back in is painted alone, not joined to last.
	left bool
	// cells counts the cells the stroke has painted, for its sentence.
	cells int
}

type rectDrag struct {
	from, to Cell
}

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDeleteAnchor
	dialogReplace
	dialogResize
	dialogPortalZone
)

// State is one Map window.
type State struct {
	window ids.ULID
	zone   ids.ULID
	// playerStart is the project's player start, kept from its game settings.
	playerStart game.Start
	ambient     ambientSounds
	portals     portalParts
	// inspect is the inspector's buttons for a selected sound or portal.
	inspect inspectParts
	// zonePalettes are the palettes the zone lists, ending at zoneListFoot.
	zonePalettes *zonepalettes.List
	zoneListFoot int
	// body is how far the inspector's parts are scrolled between its heading
	// and the preview's lines, Clock, Watch and Keys at its foot, and lists
	// how far the left panel's lists are, above Show floor below and Preview
	// at its foot, in a window too short for them (increment 61).
	body   ui.Scroll
	lists  ui.Scroll
	theme  *ui.Theme
	floors []ids.ULID
	floor  ids.ULID
	subs   map[ids.ULID]bool
	// playing says Launch's game is running in the Play tab, so its player
	// can be moved from this Map (increment 63).
	playing bool
	// arriving is the documents subscribed to whose answer has not come
	// yet: while any is on its way a preview's world waits (increment 63).
	arriving map[ids.ULID]bool
	tool     Tool
	tile     map[Tool]string
	camera   camera
	// sprites is the art drawn for stand-ins that sprites show.
	sprites *spriteArt
	size    image.Point
	// hover is the cell under the cursor: hovering when it is on the floor,
	// outside when it is beyond the floor bounds.
	hover    Cell
	hovering bool
	outside  bool
	stroke   stroke
	rect     *rectDrag
	// One thing is selected at a time (increment 65): the walls in
	// selected, the anchor, the NPC placement, the ambient sound, the
	// portal, or the player start when startPicked. moving is a press with
	// Select that may become a drag of the selected thing.
	moving      *thingDrag
	selected    []Cell
	anchor      ids.ULID
	sound       ids.ULID
	portal      ids.ULID
	startPicked bool
	// placement is the selected NPC placement. names is this frame's copy
	// of the project index, placedNPC the selected placement's NPC and
	// behaviour its behaviour workflow.
	placement ids.ULID
	names     *project.Index
	// resizeAsked is a resize asked for while the floor was read, finished
	// once it is not (increment 58).
	resizeAsked *resizeAsk
	placedNPC   ids.ULID
	// zoneNPCs counts how often each NPC is placed on the zone's floors,
	// and floorsOf how many floors in the project place the selected
	// placement's NPC, as asked for floorsFor: an NPC is placed once in the
	// whole project.
	zoneNPCs map[ids.ULID]int
	// homes are the zones the project's NPCs live in, zero for none, as far
	// as the window has them loaded.
	homes map[ids.ULID]ids.ULID
	// watchedFloor is the floor the watched NPC was on last frame, to
	// follow it when it takes stairs.
	watchedFloor ids.ULID
	floorsOf     int
	floorsFor    ids.ULID
	floorsAsking bool
	behaviour    ids.ULID
	// menu is the open right-click menu, or the menu that asks which of a
	// cell's things to select.
	menu     *cellMenu
	pick     *protocol.PickStart
	picking  pickParts
	wheel    float64
	lastPan  image.Point
	panning  bool
	framed   bool
	resizing *resizeDrag
	cursor   image.Point
	// choosing waits for the cell where a preview's player starts; starting
	// waits for the orchestrator to agree. preview is this window's own
	// preview, remote another window's preview of the zone.
	choosing bool
	starting bool
	preview  *previewRun
	remote   *remotePreview

	toolList *ui.List
	// fold is the left panel's fold button and its rail (increment 43).
	fold foldState
	// left is the left panel's width as dragged, zero for PanelWidth
	// (increment 53).
	left      int
	tileList  *ui.List
	floorList *ui.List
	below     *ui.Toggle
	nameField *ui.TextField
	// named is the anchor whose name the field holds; save and guard save
	// or ask about its unsaved name.
	named     ids.ULID
	save      *ui.SaveBar
	guard     frame.Guard
	zoom      *zoomControl
	deleteBtn *ui.Button
	replace   *ui.Button
	previewBn *ui.Button
	// keysBn opens keySheet, the list of the window's keys.
	keysBn   *ui.Button
	keySheet ui.KeySheet
	openBn   *ui.Button
	removeBn *ui.Button
	// clockBn opens the Clock window and watchBn the Watch window; start is
	// the time of day the next preview starts at, set in the Clock window.
	clockBn *ui.Button
	watchBn *ui.Button
	start   float64
	sides   []sideButtons
	focus   ui.Focus

	dialog        *ui.Dialog
	dialogKind    dialogKind
	replaceFrom   string
	pendingResize *pendingResize
}

// New returns the Map window for a zone.
func New(window, zone ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, zone: zone, playerStart: game.NoStart(),
		ambient: ambientSounds{library: nil, asking: false, changing: ids.ULID{}}, theme: theme,
		portals: newPortalParts(window), inspect: newInspectParts(),
		floors: nil, floor: ids.ULID{}, subs: map[ids.ULID]bool{}, playing: false, arriving: map[ids.ULID]bool{},
		tool: ToolGround, tile: map[Tool]string{ToolGround: "grass", ToolWalls: tiles.UnassignedWall, ToolObjects: "door"},
		camera: newCamera(), sprites: newSpriteArt(), size: image.Pt(0, 0), hover: image.Pt(0, 0), hovering: false,
		outside: false,
		stroke:  stroke{active: false, count: 0, last: image.Pt(0, 0), left: false, cells: 0},
		rect:    nil, moving: nil, selected: nil, anchor: ids.ULID{}, placement: ids.ULID{},
		sound: ids.ULID{}, portal: ids.ULID{}, startPicked: false,
		names: project.NewIndex(nil), resizeAsked: nil, placedNPC: ids.ULID{}, zoneNPCs: map[ids.ULID]int{},
		homes: map[ids.ULID]ids.ULID{}, watchedFloor: ids.ULID{}, floorsOf: 0,
		floorsFor: ids.ULID{}, floorsAsking: false, behaviour: ids.ULID{},
		menu: nil,
		pick: nil, picking: newPickParts(), wheel: 0, lastPan: image.Pt(0, 0), panning: false, framed: false, resizing: nil,
		cursor:   image.Pt(0, 0),
		choosing: false, starting: false, preview: nil, remote: nil,
		toolList: ui.NewList("tools", string(ToolGround)), fold: newFoldState(), left: 0,
		tileList:  ui.NewList("tiles", "grass"),
		floorList: ui.NewList("floors", ""),
		below:     &ui.Toggle{ID: "below", Label: "Show floor below", Rect: ui.Zero(), On: true},
		nameField: ui.NewTextField("anchor-name", textedit.NewField("", false, floor.MaxNameLength), false, theme),
		named:     ids.ULID{}, save: ui.NewSaveBar(), guard: frame.Guard{}, zoom: newZoomControl(),
		deleteBtn: ui.NewButton("delete-anchor", "Delete anchor"),
		replace:   ui.NewButton("replace-family", "Replace family everywhere…"),
		previewBn: ui.NewButton("preview", "Preview"),
		keysBn:    ui.NewButton("keys", "Keys"), keySheet: ui.KeySheet{Rows: keyRows()},
		zonePalettes: zonepalettes.New(theme), zoneListFoot: 0, body: ui.NewScroll(), lists: ui.NewScroll(),
		openBn:   ui.NewButton("open-behaviour", "Open behaviour"),
		removeBn: ui.NewButton("remove-npc", "Remove"),
		clockBn:  ui.NewButton("clock", "Clock"), watchBn: ui.NewButton("watch", "Watch"),
		start:  protocol.DefaultClockStart,
		sides:  newSideButtons(),
		focus:  ui.Focus{},
		dialog: nil, dialogKind: dialogNone, replaceFrom: "", pendingResize: nil,
	}
	view.deleteBtn.Danger, view.removeBn.Danger = true, true
	// An inspector has one main button; its other actions are quiet
	// (increment 66).
	view.openBn.Primary = true
	for _, quiet := range []*ui.Button{view.deleteBtn, view.removeBn, view.clockBn, view.watchBn, view.keysBn} {
		quiet.Quiet = true
	}
	view.focus.Register(view.nameField)
	view.toolList.Rows, view.toolList.InHand = toolListRows(), true
	view.previewBn.Key = "P"

	return view
}

// Zone returns the zone the window shows.
func (s *State) Zone() ids.ULID { return s.zone }

// Floor returns the floor being edited.
func (s *State) Floor() ids.ULID { return s.floor }

// Tool returns the current tool.
func (s *State) Tool() Tool { return s.tool }

// Selected returns the selected wall cells.
func (s *State) Selected() []Cell { return s.selected }

// SelectedAnchor returns the selected anchor's ID.
func (s *State) SelectedAnchor() ids.ULID { return s.anchor }

// SelectedPlacement returns the selected NPC placement, if any.
func (s *State) SelectedPlacement() ids.ULID { return s.placement }

// Picking reports whether the window is answering a pick.
func (s *State) Picking() bool { return s.pick != nil }

// Title is the window title.
func (s *State) Title(docs frame.Docs) string {
	zoneName, floorName := "zone", ""
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			zoneName = index.Name(s.zone, "zone")
			floorName = index.Name(s.floor, "")
		}
	})
	title := "Map: " + zoneName
	if floorName != "" {
		title += " — " + floorName
	}

	return title + "  (view " + s.window.Short(shortID) + ")"
}

const shortID = 6

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// sessionChanged follows what the windows share that a Map keeps: when the
// next preview's clock starts, and whether Launch's game is running in the
// Play tab. It reports whether the topic was one of those.
func (s *State) sessionChanged(session protocol.Session) bool {
	switch session.Topic {
	case protocol.TopicPlaying:
		var playing protocol.Playing
		if err := json.Unmarshal(session.Value, &playing); err == nil {
			s.playing = playing.Running
		}
	case protocol.TopicClockStart:
		var start protocol.ClockStart
		if err := json.Unmarshal(session.Value, &start); err == nil {
			s.start = start.Seconds
		}
	default:
		return false
	}

	return true
}

// Changed reacts to the orchestrator: the zone's floors, picks, and the art
// that sprites show stand-ins with.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.Event != nil || update.Snapshot != nil {
		s.syncArt(ctx)
		s.floorsFor = ids.ULID{}
		s.syncStart(ctx)
	}
	// A pick of the Map's own, for a portal, may be answered in this very
	// Map; a cancel of it then also ends the Map's picking.
	if s.portals.picker.Changed(update, ctx) && update.PickCancel == nil {
		return
	}
	switch {
	case update.PreviewStart != nil, update.PreviewState != nil, update.PreviewStop != nil:
		s.previewChanged(update, ctx)
	case update.PreviewControl != nil:
		s.control(*update.PreviewControl, ctx)
	case update.Session != nil && s.sessionChanged(*update.Session):
	case update.PickStart != nil, update.PickCancel != nil:
		s.pickChanged(update, ctx)
	case update.Snapshot != nil && *update.Snapshot == s.zone, update.Event != nil && update.Event.Document == s.zone:
		s.syncZone(ctx)
	default:
		s.attend(update, ctx)
	}
}

// pickChanged starts a pick, or ends the one running when it is
// cancelled.
func (s *State) pickChanged(update client.Update, ctx *frame.Context) {
	switch {
	case update.PickStart != nil:
		start := *update.PickStart
		s.pick = &start
		s.stroke.active, s.choosing = false, false
		ctx.Frame.Status("Choose a place for " + labelOr(start.Label, "another tab") +
			": click a ring or a name on the right. Escape cancels.")
	case s.pick != nil && update.PickCancel.Pick == s.pick.Pick:
		s.pick = nil
		ctx.Frame.Status("Pick cancelled: " + labelOr(update.PickCancel.Reason, "no reason given"))
	}
}

// attend shows what the window is opened to show: a placement, or an
// anchor a line in History names.
func (s *State) attend(update client.Update, ctx *frame.Context) {
	switch {
	case !update.Attention:
	case strings.HasPrefix(update.Item, PlacementItem):
		s.showPlacement(update.Item, ctx)
	case strings.HasPrefix(update.Item, AnchorItem):
		s.showAnchor(update.Item, ctx)
	}
}

// PlacementItem prefixes the item a map is opened to show a placement by,
// as "placement:<floor>:<placement>".
const PlacementItem = "placement:"

// showPlacement shows the floor a placement is on and selects it, with
// Select in hand.
func (s *State) showPlacement(item string, ctx *frame.Context) {
	floorID, placementID, ok := s.floorAndID(item, PlacementItem)
	if !ok {
		return
	}
	s.switchFloor(floorID, ctx)
	s.setTool(ToolSelect)
	s.selectPlacement(placementID)
}

func labelOr(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// Start subscribes to what the window shows. The window has already
// subscribed to the zone; this adds the project index for names and the art
// sprites show stand-ins with.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
	s.syncArt(ctx)
	s.syncStart(ctx)
	s.syncZone(ctx)
	s.askAudio(ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] {
		return
	}
	s.subs[document], s.arriving[document] = true, true
	ctx.Requests.Subscribe(document, func(err error) {
		delete(s.arriving, document)
		if err != nil {
			delete(s.subs, document)
			ctx.Frame.Status(err.Error())
		}
	})
}

// syncZone refreshes the floor list from the zone, keeping the current floor
// when it is still there.
func (s *State) syncZone(ctx *frame.Context) {
	ctx.Docs.View(s.zone, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			s.floors = zoneDoc.Zone.Floors()
		}
	})
	if len(s.floors) == 0 {
		s.floor = ids.ULID{}

		return
	}
	if s.preview != nil {
		s.subscribeFloors(ctx)
	}
	if indexOf(s.floors, s.floor) < 0 {
		s.switchFloor(s.floors[0], ctx)

		return
	}
	s.subscribeVisible(ctx)
}

func indexOf(floors []ids.ULID, id ids.ULID) int {
	for index, candidate := range floors {
		if candidate == id {
			return index
		}
	}

	return -1
}

func (s *State) switchFloor(id ids.ULID, ctx *frame.Context) {
	if id == s.floor {
		return
	}
	s.floor = id
	s.selected, s.anchor, s.rect, s.moving = nil, ids.ULID{}, nil, nil
	s.sound, s.portal, s.startPicked = ids.ULID{}, ids.ULID{}, false
	s.stroke.active = false
	s.focus.Set(nil)
	s.subscribeVisible(ctx)
}

// subscribeVisible subscribes to the current floor and the one below it.
func (s *State) subscribeVisible(ctx *frame.Context) {
	if s.floor.IsZero() {
		return
	}
	s.subscribe(s.floor, ctx)
	if below, ok := s.floorBelow(); ok {
		s.subscribe(below, ctx)
	}
}

func (s *State) floorBelow() (ids.ULID, bool) {
	index := indexOf(s.floors, s.floor)
	if index <= 0 {
		return ids.ULID{}, false
	}

	return s.floors[index-1], true
}

// Grid returns the screen area that shows the floor.
func (s *State) Grid() image.Rectangle {
	content := frame.Content(s.size)

	return image.Rect(s.panelWidth(), 0, content.Max.X, content.Max.Y)
}

// stage is the grid in the canvas's points, where the camera places cells:
// the author's scale for text and controls does not change a cell's size
// (increment 56).
func (s *State) stage() image.Rectangle { return pen.CanvasRect(s.Grid()) }

// onStage is a window point in the canvas's points.
func onStage(point image.Point) image.Point { return pen.CanvasPoint(point) }

// layoutWidgets places the panel widgets for the window size.
func (s *State) layoutWidgets(docs frame.Docs) {
	s.camera.setTile(projectTileSize(docs))
	s.refreshNPCs(docs)
	rowY := ui.Half*ui.Gap - s.lists.Offset
	s.toolList.Rect = image.Rect(0, rowY, s.panelFull(), rowY+toolRows*ui.RowHeight)
	rowY = s.toolList.Rect.Max.Y + panelGap + headingSpace
	s.tileList.Rows = s.tileRows()
	if s.tool == ToolSelect {
		s.tileList.Selected = ""
	}
	s.tileList.Rect = image.Rect(0, rowY, s.panelFull(), rowY)
	tilesHeight := s.layoutTiles()
	// With no NPCs to list, the list's place holds the hint's lines.
	tilesHeight = max(tilesHeight, ui.Gap+len(s.npcHintLines())*ui.LineHeight)
	s.tileList.Rect = image.Rect(0, rowY, s.panelFull(), rowY+tilesHeight)
	rowY = s.tileList.Rect.Max.Y + panelGap + headingSpace
	s.floorList.Rows = s.floorRows(docs)
	s.floorList.Selected = s.floor.String()
	s.floorList.Rect = image.Rect(0, rowY, s.panelFull(), rowY+floorRows*ui.RowHeight)
	rowY = s.floorList.Rect.Max.Y + panelGap/ui.Half
	s.below.Rect = image.Rect(ui.Inset/ui.Half+ui.Half, rowY, s.panelFull()-ui.Inset, rowY+ui.RowHeight)
	rowY = s.below.Rect.Max.Y + panelGap
	s.previewBn.Rect = image.Rect(ui.Inset, rowY, s.panelFull()-ui.Inset, rowY+ui.ButtonH)
	s.fitLists()
	switch {
	case s.preview != nil:
		s.previewBn.Label = "Stop preview"
	case s.choosing:
		s.previewBn.Label = "Cancel preview"
	default:
		s.previewBn.Label = "Preview"
	}

	s.layoutFold()
	inspector := frame.Inspector(s.size)
	s.body.For(s.inspecting())
	s.body.Body = image.Rect(inspector.Min.X, bodyTop(inspector), inspector.Max.X,
		max(bodyTop(inspector), footTop(inspector)-ui.ButtonH-ui.Inset-s.previewHeight(docs)-ui.Gap))
	s.layoutInspector()
	s.layoutZonePalettes(docs)
}

// layoutTiles chooses how the list of what the tool paints with is laid
// out, and returns its height: a grid of chips, two to a line when every
// name fits and one when not; the NPCs to place stay a list under their
// headings.
func (s *State) layoutTiles() int {
	s.tileList.Grid = 0
	switch {
	case s.tool == ToolNPC:
		return min(max(len(s.tileList.Rows), 1), npcRows) * ui.RowHeight
	case len(s.tileList.Rows) == 0:
		return ui.RowHeight
	}
	s.tileList.Grid = 1
	if s.tileList.FitsGrid(s.theme, ui.Half) {
		s.tileList.Grid = ui.Half
	}

	return s.tileList.Height()
}

// layoutInspector places the inspector's widgets for what is selected.
func (s *State) layoutInspector() {
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	s.nameField.Rect, s.deleteBtn.Rect, s.replace.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	s.openBn.Rect, s.removeBn.Rect = ui.Zero(), ui.Zero()
	if !s.placement.IsZero() {
		half, top := (width-ui.Gap)/ui.Half, s.down(placementButtonsTop)
		s.openBn.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
		s.removeBn.Rect = image.Rect(left+half+ui.Gap, top, left+width, top+ui.ButtonH)
	}
	s.save.Rect = ui.Zero()
	if !s.anchor.IsZero() {
		top := s.down(inspectorFieldTop)
		s.nameField.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		saveTop := top + ui.ButtonH + anchorRows*ui.LineHeight
		s.save.Rect = image.Rect(left, saveTop, left+width, saveTop+ui.SaveBarHeight(width))
		deleteTop := s.save.Rect.Max.Y + ui.Half*ui.Gap
		s.deleteBtn.Rect = image.Rect(left, deleteTop, left+width, deleteTop+ui.ButtonH)
		s.deleteBtn.Rect = ui.FitButton(s.theme, s.deleteBtn)
	}
	if len(s.selected) > 0 {
		s.replace.Rect = image.Rect(left, s.down(replaceTop), left+width, s.down(replaceTop)+ui.ButtonH)
	}
	s.layoutInspect(left, width)
	s.keysBn.Rect = image.Rect(left+width-keysWidth, inspector.Max.Y-ui.Inset-ui.ButtonH, left+width,
		inspector.Max.Y-ui.Inset)
	s.layoutPreview(left, width, footTop(inspector))
	s.layoutSides(left, s.tool == ToolResize && s.pick == nil && !s.anySelected())
}

// down is where a part of the inspector laid out at a height from its top
// is, as far as its parts are scrolled (increment 61).
func (s *State) down(top int) int { return top - s.body.Offset }

// bodyTop is where the inspector's parts that scroll start, under its
// heading.
func bodyTop(inspector image.Rectangle) int { return inspector.Min.Y + ui.Inset + ui.LineHeight }

// inspecting names what the inspector shows, so another thing starts at its
// top: the anchor or NPC selected, the walls selected, or the tool.
func (s *State) inspecting() string {
	return s.anchor.String() + s.placement.String() + s.sound.String() + s.portal.String() +
		strconv.FormatBool(s.startPicked) + strconv.Itoa(len(s.selected)) + string(s.tool)
}

// footTop is where the inspector's sections at its foot end, above the
// Keys button.
func footTop(inspector image.Rectangle) int {
	return inspector.Max.Y - ui.Inset - ui.ButtonH - ui.Inset
}

// Inspector layout, from the top of the panel.
const (
	keysWidth         = 72
	inspectorFieldTop = 6*ui.LineHeight + ui.Inset
	replaceTop        = inspectorFieldTop + ui.LineHeight*2
	familyListTop     = replaceTop + ui.ButtonH + ui.LineHeight
)

func (s *State) tileRows() []ui.Row {
	if s.tool == ToolNPC {
		return s.npcRows()
	}
	if s.tool == ToolSelect && len(s.selected) == 0 {
		// Select offers the wall families only for walls it has selected.
		return nil
	}
	offered := tilesFor(s.tool)
	rows := make([]ui.Row, 0, len(offered))
	for index, definition := range offered {
		swatch := ui.Hex(definition.Color)
		row := ui.NewRow(definition.ID, chipName(definition))
		if index < numberedTiles {
			row.Key = strconv.Itoa(index + 1)
		}
		row.Swatch = &swatch
		rows = append(rows, row)
	}

	return rows
}

func (s *State) floorRows(docs frame.Docs) []ui.Row {
	rows := make([]ui.Row, 0, len(s.floors))
	for index := len(s.floors) - 1; index >= 0; index-- {
		floorID := s.floors[index]
		name := "Floor " + strconv.Itoa(index+1)
		docs.View(project.IndexID(), func(document client.Document) {
			if projectIndex, ok := document.Model.(*project.Index); ok {
				name = projectIndex.Name(floorID, name)
			}
		})
		row := ui.NewRow(floorID.String(), name)
		row.Key = strconv.Itoa(index + 1)
		rows = append(rows, row)
	}

	return rows
}

// chipName is a stand-in's name on its chip. A wall family drops "Wall",
// which its heading already says, so two fit on a line.
func chipName(definition tiles.Definition) string {
	if !definition.IsWall() {
		return definition.Name
	}
	name := strings.TrimSpace(strings.TrimPrefix(definition.Name, "Wall"))
	name = strings.TrimSuffix(strings.TrimPrefix(name, "("), ")")
	if name == "" {
		return definition.Name
	}

	return strings.ToUpper(name[:1]) + name[1:]
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.cursor = onStage(input.Cursor)
	s.fold.folded = !ctx.Frame.ShowLeft.On
	s.layoutWidgets(ctx.Docs)
	// The status bar names what is in hand once the frame has moved it.
	defer func() { ctx.Frame.SetHand(s.handLine(ctx.Docs)) }()
	if s.updateOver(input, ctx) {
		return
	}
	if s.pick != nil {
		s.updatePicking(input, ctx)

		return
	}
	s.guardAnchor()
	s.updateInspectorFoot(input, ctx)
	s.updateZoom(input, ctx)
	s.updateFields(input, ctx)
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	if input.Consume(ui.KeyN, ui.Cmd()) {
		ctx.Requests.Open(ViewName, s.zone, true)
		ctx.Frame.Status("Opened a new view of this zone")
	}
	frame.CommonKeys(input, ctx)
	if s.zonePalettes.Update(input, ctx) {
		return
	}
	s.updatePanels(input, ctx)
	s.subscribeNPC(ctx)
	s.subscribeNPCs(ctx)
	s.askPlacing(ctx)
	if s.preview != nil {
		s.stepPreview(input, ctx)
	} else {
		s.arrowHint(input, ctx)
	}
	s.followWatched(ctx)
	if s.floor.IsZero() {
		return
	}
	s.syncPortals(ctx)
	loaded := ctx.Docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if !s.framed {
				s.camera.frame(floorDoc.Floor.Bounds())
				s.framed = true
			}
			s.syncAnchorField(floorDoc.Floor)
			s.syncPlacement(floorDoc.Floor)
			s.syncSelection(floorDoc.Floor)
			s.updateGrid(input, ctx, floorDoc.Floor)
			s.updateSides(input, ctx, floorDoc.Floor)
		}
	})
	if !loaded {
		s.hovering, s.outside = false, false
	}
	s.finishResize(ctx)
	s.guardAnchor()
}

// updateOver gives a frame of input to whatever lies over the window: the
// unsaved changes guard, a dialog, the right-click menu or the key sheet. It
// reports whether one took it.
func (s *State) updateOver(input *ui.Input, ctx *frame.Context) bool {
	switch {
	case s.guard.Active():
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discard)
	case s.dialog != nil:
		s.updateDialog(input, ctx)
	case s.menu != nil:
		s.updateMenu(input, ctx)
	case s.keySheet.IsOpen():
		s.keySheet.Update(input)
	case s.zonePalettes.Active():
		s.zonePalettes.Update(input, ctx)
	default:
		return false
	}

	return true
}

// updateFields handles the anchor's name field and save bar: Enter saves
// the name.
func (s *State) updateFields(input *ui.Input, ctx *frame.Context) {
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field == s.nameField &&
		outcome == textedit.OutcomeCommit {
		s.saveName(ctx)
	}
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)
	case ui.RevertID:
		s.discard()
	}
}

// updateZoom handles the floating zoom control; Fit needs the floor's
// bounds.
func (s *State) updateZoom(input *ui.Input, ctx *frame.Context) {
	bounds := image.Rect(0, 0, 1, 1)
	ctx.Docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			bounds = floorDoc.Floor.Bounds()
		}
	})
	s.zoom.layout(s.Grid(), s.camera)
	s.zoom.update(input, &s.camera, s.stage(), bounds)
}

// keys handles the window's own keys while no text field has focus.
func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	plain := ui.Plain()
	command := ui.Cmd()
	s.toolAndTileKeys(input, ctx)
	switch {
	case input.Consume(ui.KeyPageUp, plain):
		s.stepFloor(1, ctx)
	case input.Consume(ui.KeyPageDown, plain):
		s.stepFloor(-1, ctx)
	case input.Consume(ui.KeyEqual, command):
		s.camera.zoomBy(1, onStage(input.Cursor), s.stage())
	case input.Consume(ui.KeyMinus, command):
		s.camera.zoomBy(-1, onStage(input.Cursor), s.stage())
	case input.Consume(ui.Key0, command):
		ctx.Docs.View(s.floor, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				s.camera.frame(floorDoc.Floor.Bounds())
			}
		})
	case input.Consume(ui.KeyF, plain):
		s.below.On = !s.below.On
	case input.Consume(ui.KeyP, plain):
		s.togglePreview(ctx)
	case input.Consume(ui.KeySlash, ui.Mods{Shortcut: false, Shift: true, Alt: false, Control: false}):
		s.keySheet.Open(image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, s.theme)
	case s.choosing && input.Consume(ui.KeyEscape, plain):
		s.togglePreview(ctx)
	case input.Consume(ui.KeyBackspace, plain), input.Consume(ui.KeyDelete, plain):
		s.deleteSelection(ctx)
	case s.pick == nil && input.Consume(ui.KeyEscape, plain):
		s.clearSelection()
		s.rect, s.moving = nil, nil
	}
}

// deleteSelection deletes the selected anchor, or removes the selected NPC,
// ambient sound or portal. The player start and walls are not removed this
// way.
func (s *State) deleteSelection(ctx *frame.Context) {
	switch {
	case !s.anchor.IsZero():
		s.deleteAnchor(ctx)
	case !s.placement.IsZero():
		s.removePlacement(ctx)
	case !s.sound.IsZero():
		s.removeSound(ctx, s.sound)
	case !s.portal.IsZero():
		s.removePortal(ctx, s.portal)
	}
}

// toolAndTileKeys chooses a tool by its letter and a tile by its number.
func (s *State) toolAndTileKeys(input *ui.Input, ctx *frame.Context) {
	for _, info := range tools() {
		if input.Consume(info.key, ui.Plain()) {
			s.setTool(info.tool)
		}
	}
	for number := 1; number <= numberedTiles; number++ {
		if input.Consume(ui.Key(strconv.Itoa(number)), ui.Plain()) {
			offered := slices.DeleteFunc(s.tileRows(), func(row ui.Row) bool { return row.Heading })
			if number <= len(offered) {
				s.chooseTile(offered[number-1].ID, ctx)
			}
		}
	}
}

func (s *State) setTool(tool Tool) {
	s.tool = tool
	s.toolList.Selected = string(tool)
	s.tileList.Selected = s.tile[tool]
	s.stroke.active, s.rect, s.moving, s.resizing = false, nil, nil, nil
}

func (s *State) stepFloor(delta int, ctx *frame.Context) {
	index := indexOf(s.floors, s.floor) + delta
	if index >= 0 && index < len(s.floors) {
		s.switchFloor(s.floors[index], ctx)
	}
}

// chooseTile selects a tile for the tool, or applies a family to the selected
// walls with the Select tool.
func (s *State) chooseTile(tileID string, ctx *frame.Context) {
	if s.tool == ToolSelect {
		s.applyFamily(tileID, ctx)

		return
	}
	s.tile[s.tool] = tileID
	s.tileList.Selected = tileID
}

func (s *State) updatePanels(input *ui.Input, ctx *frame.Context) {
	if s.updateFold(input, ctx) {
		return
	}
	if s.lists.Wheel(input, s.lists.Body) {
		s.layoutWidgets(ctx.Docs)
	}
	s.updateLists(s.listsInput(input), ctx)
	s.below.Update(input)
	if s.previewBn.Update(input) {
		s.togglePreview(ctx)
	}
	s.updateInspectorButtons(input, ctx)
}

// listsInput is the input the left panel's lists take: none of the pointer
// while it is beside them, on Show floor below or Preview, which take it
// alone (increment 61).
func (s *State) listsInput(input *ui.Input) *ui.Input {
	if !s.lists.Hidden(input.Cursor) {
		return input
	}
	beside := *input
	beside.ClearPointer()

	return &beside
}

// updateLists handles the tools, tiles and floors.
func (s *State) updateLists(input *ui.Input, ctx *frame.Context) {
	if s.toolList.Update(input).Changed {
		s.setTool(Tool(s.toolList.Selected))
	}
	if result := s.tileList.Update(input); result.Changed || (s.tool == ToolSelect && s.tileList.Selected != "") {
		s.chooseTile(s.tileList.Selected, ctx)
		if s.tool == ToolSelect {
			s.tileList.Selected = ""
		}
	}
	if s.floorList.Update(input).Changed {
		if id, err := ids.Parse(s.floorList.Selected); err == nil {
			s.switchFloor(id, ctx)
		}
	}
}

// updateInspectorFoot scrolls the inspector's parts with the wheel over it,
// and gives a press beside them, on Clock, Watch, Keys or the heading, to
// those alone, never to a part scrolled behind them (increment 61).
func (s *State) updateInspectorFoot(input *ui.Input, ctx *frame.Context) {
	if s.body.Wheel(input, frame.Inspector(s.size)) {
		s.layoutWidgets(ctx.Docs)
	}
	if !s.body.Hidden(input.Cursor) {
		return
	}
	defer input.ClearPointer()
	s.updatePreviewControls(input, ctx)
	if s.keysBn.Update(input) {
		s.keySheet.Open(image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, s.theme)
	}
}

// updateInspectorButtons handles the buttons for what is selected.
func (s *State) updateInspectorButtons(input *ui.Input, ctx *frame.Context) {
	s.updatePreviewControls(input, ctx)
	if s.keysBn.Update(input) {
		s.keySheet.Open(image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, s.theme)
	}
	if s.deleteBtn.Update(input) && !s.anchor.IsZero() {
		s.deleteAnchor(ctx)
	}
	if s.replace.Update(input) && len(s.selected) > 0 {
		s.openReplace(ctx)
	}
	if s.openBn.Update(input) && !s.placement.IsZero() {
		s.openBehaviour(ctx)
	}
	if s.removeBn.Update(input) && !s.placement.IsZero() {
		s.removePlacement(ctx)
	}
	s.updateInspect(input, ctx)
}

// updateGrid handles panning, zooming and the current tool on the floor.
func (s *State) updateGrid(input *ui.Input, ctx *frame.Context, floorData *floor.Floor) {
	s.panAndZoom(input, s.Grid())
	cell, inGrid := s.camera.cellAt(onStage(input.Cursor), s.stage())
	onFloor := inGrid && floorData.In(cell.X, cell.Y)
	s.hover, s.hovering, s.outside = cell, onFloor, inGrid && !onFloor && !s.zoom.contains(input.Cursor)
	if s.panning || input.Space || s.updateGridMode(input, ctx, floorData, cell, inGrid) {
		return
	}
	switch s.tool {
	case ToolGround, ToolWalls, ToolObjects, ToolErase:
		s.updatePaint(input, ctx, floorData, cell, onFloor)
	case ToolAnchor:
		s.updateAnchor(input, ctx, floorData, cell, onFloor)
	case ToolNPC:
		s.updateNPCTool(input, ctx, floorData, cell, onFloor)
	case ToolSelect:
		s.updateSelect(input, ctx, floorData, cell, onFloor)
	case ToolResize:
		s.updateResize(input, ctx, floorData)
	}
}

// updateGridMode handles what takes a click before the current tool: picking
// for another window, choosing where a preview starts, the right-click menu,
// and watching an NPC in a preview. It reports whether one took it.
func (s *State) updateGridMode(
	input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, inGrid bool,
) bool {
	onFloor := inGrid && floorData.In(cell.X, cell.Y)
	switch {
	case s.pick != nil:
		s.updatePick(input, ctx, floorData, cell, onFloor)
	case s.choosing:
		s.updateChoosing(input, ctx, floorData, cell, onFloor)
	case input.Secondary && inGrid && !s.zoom.contains(input.Cursor):
		s.openMenu(ctx, floorData, cell, onFloor, input.Cursor)
	case s.closeBox(input):
	default:
		return s.watchClicked(input, ctx, cell, onFloor)
	}

	return true
}

// panAndZoom pans and zooms the camera, which works in the canvas's points.
func (s *State) panAndZoom(input *ui.Input, grid image.Rectangle) {
	cursor, stage := onStage(input.Cursor), s.stage()
	if input.Cursor.In(grid) && (input.WheelX != 0 || input.WheelY != 0) {
		if input.Held.Shortcut {
			s.wheel += input.WheelY
			for s.wheel >= 1 {
				s.camera.zoomBy(1, cursor, stage)
				s.wheel--
			}
			for s.wheel <= -1 {
				s.camera.zoomBy(-1, cursor, stage)
				s.wheel++
			}
		} else {
			s.camera.pan(pen.CanvasDelta(input.WheelPan()))
		}
	}
	dragPan := input.PanDown || (input.Space && input.Down)
	if dragPan && s.panning {
		s.camera.pan(cursor.Sub(s.lastPan))
	}
	s.panning = dragPan && (s.panning || input.Cursor.In(grid))
	s.lastPan = cursor
}

// paintValue returns the layer and tile the current tool paints.
func (s *State) paintValue() (string, string) {
	switch s.tool {
	case ToolGround:
		return tilefloor.LayerGround, s.tile[ToolGround]
	case ToolWalls:
		return tilefloor.LayerTop, s.tile[ToolWalls]
	case ToolObjects:
		return tilefloor.LayerTop, s.tile[ToolObjects]
	case ToolErase, ToolAnchor, ToolNPC, ToolSelect, ToolResize:
		return tilefloor.LayerTop, ""
	}

	return tilefloor.LayerTop, ""
}

// updatePaint paints a stroke: the cell pressed, then the cells between
// each frame's cell and the last, so a fast drag leaves no gaps. A stroke
// that leaves the floor and comes back paints the cell it comes back in at,
// and none between, which the pointer never touched.
func (s *State) updatePaint(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	switch {
	case input.Pressed && onFloor:
		s.stroke = stroke{active: true, count: s.stroke.count + 1, last: cell, left: false, cells: 0}
		s.paint(ctx, floorData, []Cell{cell})
	case s.stroke.active && input.Down && !onFloor:
		s.stroke.left = true
	case s.stroke.active && input.Down && s.stroke.left:
		s.paint(ctx, floorData, []Cell{cell})
		s.stroke.last, s.stroke.left = cell, false
	case s.stroke.active && input.Down && cell != s.stroke.last:
		s.paint(ctx, floorData, LineCells(s.stroke.last, cell)[1:])
		s.stroke.last = cell
	}
	if !input.Down {
		s.stroke.active = false
	}
}

// paint sends one command for the cells whose value would change; every
// command in a stroke shares a gesture, so the stroke undoes as one step.
func (s *State) paint(ctx *frame.Context, floorData *floor.Floor, cells []Cell) {
	layer, tile := s.paintValue()
	changed := make([]tilefloor.Cell, 0, len(cells))
	for _, cell := range cells {
		current, err := floorData.Ground(cell.X, cell.Y)
		if layer == tilefloor.LayerTop {
			current, err = floorData.Top(cell.X, cell.Y)
		}
		if err == nil && current != tile {
			changed = append(changed, tilefloor.Cell{Col: cell.X, Row: cell.Y, Layer: layer, Tile: tile})
		}
	}
	if len(changed) == 0 {
		return
	}
	s.stroke.cells += len(changed)
	gesture := s.window.String() + "-" + strconv.Itoa(s.stroke.count)
	says := paintSentence(tile, s.stroke.cells, s.names.Name(s.floor, "this floor"))
	s.commandSaying(ctx, s.floor, tilefloor.PaintEdit, gesture, says, tilefloor.PaintArgs{Cells: changed}, nil)
}

// paintSentence says a stroke: how many cells it painted, with what, on
// which floor (increment 51).
func paintSentence(tile string, cells int, floorName string) string {
	counted := strconv.Itoa(cells) + " cells"
	if cells == 1 {
		counted = "1 cell"
	}
	if tile == "" {
		return "Cleared " + counted + " on " + floorName
	}

	return "Painted " + counted + " of " + tileName(tile) + " on " + floorName
}

// command sends an edit, reporting a refusal in the status bar.
func (s *State) command(
	ctx *frame.Context, document ids.ULID, edit, gesture string, args any, done func(protocol.Result),
) {
	s.commandSaying(ctx, document, edit, gesture, "", args, done)
}

// commandSaying sends an edit said in the Map's own words (increment 51),
// as "Painted 6 cells of Road on Ground floor".
func (s *State) commandSaying(
	ctx *frame.Context, document ids.ULID, edit, gesture, says string, args any, done func(protocol.Result),
) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: document, Edit: edit, Gesture: gesture, Args: data, Says: says}
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

// updateAnchor handles the Anchors tool, which only adds (increment 65): a
// click on a cell with no anchor adds one; one that has an anchor is left
// as it is, and the status bar says how to change it.
func (s *State) updateAnchor(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if !input.Pressed || !onFloor {
		return
	}
	if _, found := floorData.AnchorAt(cell.X, cell.Y); found {
		ctx.Frame.Refuse("An anchor is already here. Select (S) selects it, to rename, move or delete it.")

		return
	}
	s.addAnchor(ctx, floorData, cell)
}

// addAnchor adds an anchor on a cell with a name not yet used, selects it
// and focuses its name, ready to type over.
func (s *State) addAnchor(ctx *frame.Context, floorData *floor.Floor, cell Cell) {
	name := uniqueAnchorName(floorData)
	args := tilefloor.AddAnchorArgs{
		Anchor: floor.Anchor{ID: ids.ULID{}, Name: name, Col: cell.X, Row: cell.Y}, Index: len(floorData.Anchors()),
	}
	says := fmt.Sprintf("Added %s at %d, %d on %s", anchorName(name), cell.X, cell.Y, s.names.Name(s.floor, "this floor"))
	s.commandSaying(ctx, s.floor, tilefloor.AddAnchorEdit, "", says, args, func(result protocol.Result) {
		s.anchor = result.Created
		s.placement, s.selected = ids.ULID{}, nil
		if s.nameField.Field.Unsaved() {
			// The guard asks about the other anchor's unsaved name first.
			return
		}
		s.named = result.Created
		s.nameField.Field.SetSaved(name)
		s.focus.Set(s.nameField)
		if buffer := s.nameField.Field.Buffer(); buffer != nil {
			buffer.SelectAll()
		}
		ctx.Frame.Status("Added “" + name + "”. Type a name, then press Enter or Save.")
	})
}

// uniqueAnchorName returns "Anchor N" not yet used on the floor.
func uniqueAnchorName(floorData *floor.Floor) string {
	used := map[string]bool{}
	for _, anchor := range floorData.Anchors() {
		used[strings.ToLower(anchor.Name)] = true
	}
	for number := 1; ; number++ {
		name := "Anchor " + strconv.Itoa(number)
		if !used[strings.ToLower(name)] {
			return name
		}
	}
}

func (s *State) selectAnchor(anchorID ids.ULID) {
	if s.anchor != anchorID {
		s.focus.Set(nil)
	}
	s.clearSelection()
	s.anchor = anchorID
}

// syncAnchorField keeps the name field showing the selected anchor, and drops
// the selection if the anchor is gone. The guard has already asked about an
// unsaved name before the selection moved.
func (s *State) syncAnchorField(floorData *floor.Floor) {
	if s.anchor != s.named {
		s.nameField.Field.Revert()
		s.named = s.anchor
	}
	if s.anchor.IsZero() {
		return
	}
	anchor, _, ok := floorData.Anchor(s.anchor)
	if !ok {
		s.anchor, s.named = ids.ULID{}, ids.ULID{}
		s.focus.Set(nil)
		s.nameField.Field.Revert()

		return
	}
	s.nameField.Field.SetSaved(anchor.Name)
}

// guardAnchor asks about an unsaved anchor name before the inspector moves
// to another anchor or none: the selection goes back until the author
// decides.
func (s *State) guardAnchor() {
	if s.named.IsZero() || s.anchor == s.named || !s.nameField.Field.Unsaved() {
		return
	}
	wanted := s.anchor
	s.anchor = s.named
	s.selected, s.moving = nil, nil
	s.focus.Set(nil)
	s.guard.Ask(1, s.nameField.Field.Saved(), func() { s.anchor = wanted })
}

// Unsaved counts the anchor name's unsaved change.
func (s *State) Unsaved(frame.Docs) int {
	if s.named.IsZero() {
		return 0
	}

	return frame.UnsavedFields(s.nameField)
}

// Save saves the anchor's name, from the save bar or Cmd+S.
func (s *State) Save(ctx *frame.Context) bool {
	s.saveName(ctx)

	return true
}

// UnsavedName names the anchor whose name is unsaved.
func (s *State) UnsavedName(frame.Docs) string {
	return "the anchor " + s.nameField.Field.Saved()
}

// Leave asks about an unsaved anchor name before then, such as closing the
// window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.nameField.Field.Saved(), then)
}

// discard drops the anchor name's unsaved change.
func (s *State) discard() {
	s.focus.Set(nil)
	s.nameField.Field.Revert()
}

// saveName saves the anchor's new name.
func (s *State) saveName(ctx *frame.Context) {
	if s.named.IsZero() {
		return
	}
	name, changed := s.nameField.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	var renamed floor.Anchor
	found := false
	ctx.Docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			renamed, _, found = floorDoc.Floor.Anchor(s.named)
		}
	})
	if !found {
		s.nameField.Field.Failed()

		return
	}
	renamed.Name = strings.TrimSpace(name)
	data, err := json.Marshal(tilefloor.UpdateAnchorArgs{Anchor: renamed})
	if err != nil {
		s.nameField.Field.Failed()

		return
	}
	command := protocol.Command{Document: s.floor, Edit: tilefloor.UpdateAnchorEdit, Gesture: "", Args: data, Says: ""}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			s.nameField.Field.Failed()
			ctx.Frame.Report("")(result, err)

			return
		}
		s.focus.Set(nil)
	})
}

// deleteAnchor removes the selected anchor, first showing where it is used
// (D4); with no uses it is removed at once and can be undone.
func (s *State) deleteAnchor(ctx *frame.Context) {
	anchorID, floorID := s.anchor, s.floor
	ctx.Requests.WhereUsed(floorID, anchorID.String(), func(uses []protocol.Use, err error) {
		if err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
		// The project's answer leaves out the anchor's own floor, whose
		// portals may lead to it.
		var here []string
		ctx.Docs.View(floorID, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				here = leadingHere(floorDoc.Floor, floorID, anchorID)
			}
		})
		if len(uses)+len(here) == 0 {
			s.removeAnchor(ctx, floorID, anchorID)

			return
		}
		count := len(uses) + len(here)
		lines := []string{fmt.Sprintf("This anchor is used in %d %s:", count, plural(count, "place", "places"))}
		for _, use := range uses {
			lines = append(lines, "  "+use.Name+" ("+use.Label+")")
		}
		for _, portal := range here {
			lines = append(lines, "  "+s.names.Name(floorID, "This floor")+" ("+portal+")")
		}
		lines = append(lines, "Deleting leaves those references broken. You can undo it.")
		s.dialog = ui.NewDialog("Delete this anchor?", lines,
			ui.DialogButton("delete", "Delete anyway"), ui.DialogButton(ui.DialogCancel, "Cancel"))
		s.dialogKind = dialogDeleteAnchor
	})
}

func (s *State) removeAnchor(ctx *frame.Context, floorID, anchorID ids.ULID) {
	removal := tilefloor.RemoveAnchorArgs{ID: anchorID}
	s.command(ctx, floorID, tilefloor.RemoveAnchorEdit, "", removal, func(protocol.Result) {
		ctx.Frame.Status("Anchor deleted. Undo brings it back.")
	})
	s.anchor, s.named = ids.ULID{}, ids.ULID{}
	s.focus.Set(nil)
	s.nameField.Field.Revert()
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

// applyFamily changes the family of every selected wall in one undo step.
func (s *State) applyFamily(family string, ctx *frame.Context) {
	if len(s.selected) == 0 {
		ctx.Frame.Refuse("Select walls first: click a wall, double-click a room or drag an area")

		return
	}
	ctx.Docs.View(s.floor, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		cells := []tilefloor.Cell{}
		for _, cell := range s.selected {
			if top, err := floorDoc.Floor.Top(cell.X, cell.Y); err == nil && tiles.IsWall(top) && top != family {
				cells = append(cells, tilefloor.Cell{Col: cell.X, Row: cell.Y, Layer: tilefloor.LayerTop, Tile: family})
			}
		}
		if len(cells) == 0 {
			return
		}
		name := family
		if definition, found := tiles.Lookup(family); found {
			name = definition.Name
		}
		s.paintTogether(ctx, cells, func() {
			ctx.Frame.Status(fmt.Sprintf("Changed %d walls to %s", len(cells), name))
		})
	})
	s.tile[ToolWalls] = family
}

// paintTogether paints any number of cells as one undo step: in as many
// commands as the cells need, none holding more than a paint command may,
// all sharing a gesture. It runs done once the last is carried out.
func (s *State) paintTogether(ctx *frame.Context, cells []tilefloor.Cell, done func()) {
	s.stroke.count++
	gesture := s.window.String() + "-" + strconv.Itoa(s.stroke.count)
	sent := 0
	for part := range slices.Chunk(cells, tilefloor.MaxCells) {
		sent += len(part)
		var then func(protocol.Result)
		if sent == len(cells) {
			then = func(protocol.Result) { done() }
		}
		s.command(ctx, s.floor, tilefloor.PaintEdit, gesture, tilefloor.PaintArgs{Cells: part}, then)
	}
}

// selectionFamily returns the family of the selected walls when they share
// one.
func (s *State) selectionFamily(docs frame.Docs) (string, bool) {
	family, uniform := "", true
	docs.View(s.floor, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			uniform = false

			return
		}
		for _, cell := range s.selected {
			top, _ := floorDoc.Floor.Top(cell.X, cell.Y) //nolint:errcheck // Selected cells lie on the floor.
			if family == "" {
				family = top
			}
			uniform = uniform && top == family
		}
	})

	return family, uniform && family != ""
}

// openReplace asks which family should replace the selected walls' family
// everywhere, on this floor or in the whole zone.
func (s *State) openReplace(ctx *frame.Context) {
	from, ok := s.selectionFamily(ctx.Docs)
	if !ok {
		ctx.Frame.Status("Select walls of one family to replace that family everywhere")

		return
	}
	for _, id := range s.floors {
		s.subscribe(id, ctx)
	}
	list := ui.NewList("families", "")
	for _, family := range tiles.WallFamilies() {
		if family.ID != from {
			swatch := ui.Hex(family.Color)
			row := ui.NewRow(family.ID, family.Name)
			row.Swatch = &swatch
			list.Rows = append(list.Rows, row)
		}
	}
	if len(list.Rows) > 0 {
		list.Selected = list.Rows[0].ID
	}
	fromName := from
	if definition, found := tiles.Lookup(from); found {
		fromName = definition.Name
	}
	s.dialog = ui.NewDialog("Replace “"+fromName+"” everywhere with:", nil,
		ui.DialogButton("floor", "This floor"), ui.DialogButton("zone", "Whole zone"),
		ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogKind = dialogReplace
	s.replaceFrom = from
}

func (s *State) updateDialog(input *ui.Input, ctx *frame.Context) {
	choice := s.dialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	if choice == "" {
		return
	}
	dialog, kind, resize := s.dialog, s.dialogKind, s.pendingResize
	s.dialog, s.dialogKind, s.pendingResize = nil, dialogNone, nil
	switch {
	case choice == ui.DialogCancel:
	case kind == dialogDeleteAnchor:
		s.removeAnchor(ctx, s.floor, s.anchor)
	case kind == dialogResize && resize != nil:
		s.sendResize(ctx, *resize)
	case kind == dialogPortalZone && dialog.List != nil:
		s.pickLeads(ctx, dialog.List.Selected)
	case kind == dialogReplace && dialog.List != nil:
		floors := []ids.ULID{s.floor}
		if choice == "zone" {
			floors = s.floors
		}
		s.replaceFamily(ctx, s.replaceFrom, dialog.List.Selected, floors)
	}
}

// replaceFamily replaces one wall family with another on several floors in
// one undo step: one command per floor, sharing a gesture.
func (s *State) replaceFamily(ctx *frame.Context, fromFamily, toFamily string, floors []ids.ULID) {
	if toFamily == "" {
		return
	}
	s.stroke.count++
	gesture := s.window.String() + "-replace-" + strconv.Itoa(s.stroke.count)
	total, missing := 0, 0
	for _, floorID := range floors {
		cells := []tilefloor.Cell{}
		loaded := ctx.Docs.View(floorID, func(document client.Document) {
			floorDoc, ok := document.Model.(*tilefloor.Document)
			if !ok {
				return
			}
			floorData := floorDoc.Floor
			bounds := floorData.Bounds()
			for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
				for col := bounds.Min.X; col < bounds.Max.X; col++ {
					if top, _ := floorData.Top(col, row); top == fromFamily { //nolint:errcheck // Inside the floor.
						cells = append(cells, tilefloor.Cell{Col: col, Row: row, Layer: tilefloor.LayerTop, Tile: toFamily})
					}
				}
			}
		})
		if !loaded {
			missing++

			continue
		}
		for start := 0; start < len(cells); start += tilefloor.MaxCells {
			batch := cells[start:min(start+tilefloor.MaxCells, len(cells))]
			s.command(ctx, floorID, tilefloor.PaintEdit, gesture, tilefloor.PaintArgs{Cells: batch}, nil)
		}
		total += len(cells)
	}
	message := fmt.Sprintf("Replaced %d walls", total)
	if missing > 0 {
		message += fmt.Sprintf("; %d floors were still loading, try again for those", missing)
	}
	ctx.Frame.Status(message)
}

func (s *State) updatePick(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if input.Consume(ui.KeyEscape, ui.Plain()) {
		s.cancelPick(ctx)

		return
	}
	if !input.Pressed || !onFloor {
		return
	}
	anchor, found := floorData.AnchorAt(cell.X, cell.Y)
	if !found {
		ctx.Frame.Status("Click one of the rings, or a name on the right, or press Escape")

		return
	}
	s.answerPick(ctx, anchor)
}

// Live reports whether the Map runs a preview, which its tab says in orange
// (increment 38).
func (s *State) Live() bool { return s.preview != nil }

// Locate finds a named part of the window for input scripts: "cell:C,R",
// "edge:left" (or top, right, bottom) of the floor, "tool:<tool>",
// "tile:<id>", "floor:<n>" (1 is the bottom floor), "player" for a
// preview's player, "ui:anchor-name", "ui:delete-anchor",
// "ui:replace-family", "ui:below", "ui:preview", "ui:grow-left" and the other
// side buttons, or a dialog button or list row by ID.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layoutWidgets(docs)
	kind, value, _ := strings.Cut(name, ":")
	if point, found := s.locatePicking(docs, kind, value); found {
		return point, true
	}
	if point, found, over := s.locateOver(kind, value); over {
		return point, found
	}
	switch kind {
	case "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	case "cell":
		var col, row int
		if _, err := fmt.Sscanf(value, "%d,%d", &col, &row); err != nil {
			return image.Pt(0, 0), false
		}
		rect := pen.WindowRect(s.camera.cellRect(Cell{X: col, Y: row}, s.stage()))

		return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2), true
	case "edge":
		return s.locateEdge(value, docs)
	case "tool", "tile", "npc", "floor":
		return s.locateInLists(kind, value, docs)
	case "ui":
		return s.locateInspector(name, value, docs)
	case "live-npc":
		return s.locateLiveNPC(value)
	case "player":
		return s.locatePlayer()
	}

	return image.Pt(0, 0), false
}

// locateInLists finds a tool, a tile, an NPC to place or a floor for an
// input script, scrolling the left panel's lists to one hidden below them,
// as an author would (increment 61).
func (s *State) locateInLists(kind, value string, docs frame.Docs) (image.Point, bool) {
	find := func() (image.Point, bool) {
		switch kind {
		case "tool":
			return s.toolList.Locate(value)
		case "tile":
			return s.tileList.Locate(value)
		case "npc":
			return s.locateNPC(value)
		}

		return s.locateFloor(value)
	}
	point, found := find()
	if !found || !s.lists.Reveal(point) {
		return point, found
	}
	s.layoutWidgets(docs)

	return find()
}

// locateInspector finds a widget for an input script, scrolling the
// inspector's parts to one hidden above or below them, as an author would;
// Clock, Watch and Keys stay where they are (increment 61).
func (s *State) locateInspector(name, value string, docs frame.Docs) (image.Point, bool) {
	point, found := s.locateLaidOut(name, value)
	foot := value == s.keysBn.ID || value == s.clockBn.ID || value == s.watchBn.ID
	if !found || foot || !s.body.Reveal(point) {
		return point, found
	}
	s.layoutWidgets(docs)

	return s.locateLaidOut(name, value)
}

// locateLaidOut finds a widget, a side's buttons among them, as laid out.
func (s *State) locateLaidOut(name, value string) (image.Point, bool) {
	if at, found := s.locateSides(name); found {
		return at, true
	}

	return s.locateWidget(value)
}

// hasState answers input scripts waiting for the window: "loaded" once the
// current floor has arrived and is in view, "picking" while answering a pick,
// "pick-now=Name" while the pick would replace the named place,
// "floor:N" when the Nth floor from the bottom is shown,
// "bounds:C,R,W,H" when the current floor has those bounds, "zoom:N" when
// the zoom control shows N%, "sprite:…" and "art:…" for the art sprites
// show stand-ins with (hasArt), "launched" while Launch's game runs in the
// Play tab, "previewing"
// while this window runs a preview, "player:C,R" or "remote-player:C,R"
// when this window's or another window's player stands on that cell, and
// "placement:C,R" or "placement:C,R=Name" when an NPC, or the named NPC,
// is placed on that cell.
func (s *State) hasState(state string, docs frame.Docs) bool {
	_, offView := s.floorOffView(docs)
	simple := map[string]bool{
		"picking": s.pick != nil, "previewing": s.preview != nil, "launched": s.playing, "keys": s.keySheet.IsOpen(),
		"floor-off-view": offView,
	}
	if s.pick != nil {
		simple["pick-now="+s.pick.Now.Name] = true
	}
	if value, isSimple := simple[state]; isSimple {
		return value
	}
	switch {
	case state == "loaded":
		return !s.floor.IsZero() && s.framed && docs.View(s.floor, func(client.Document) {})
	case strings.HasPrefix(state, "bounds:"):
		return s.hasBounds(strings.TrimPrefix(state, "bounds:"), docs)
	case strings.HasPrefix(state, "sprite:"), strings.HasPrefix(state, "art:"), strings.HasPrefix(state, "npc-sprite:"):
		return s.hasArt(state, docs)
	case strings.HasPrefix(state, "zoom:"):
		s.zoom.layout(s.Grid(), s.camera)

		return s.zoom.reset.Label == strings.TrimPrefix(state, "zoom:")+"%"
	case state == "watching", state == "npcs-moved", strings.HasPrefix(state, "npc:"), strings.HasPrefix(state, "heard:"),
		strings.HasPrefix(state, "speaking:"), strings.HasPrefix(state, "box="), strings.HasPrefix(state, "boxes="),
		state == "shaking":
		return s.hasNPCState(state, docs)
	case strings.HasPrefix(state, "placement:"):
		return s.hasPlacement(strings.TrimPrefix(state, "placement:"), docs)
	case strings.HasPrefix(state, "ambient:"), strings.HasPrefix(state, "portal:"),
		strings.HasPrefix(state, "selected:"):
		return s.hasMark(state, docs)
	case strings.HasPrefix(state, "clock:"):
		return s.clockShows(strings.TrimPrefix(state, "clock:"))
	case strings.HasPrefix(state, "player:"), strings.HasPrefix(state, "remote-player:"),
		strings.HasPrefix(state, "player-start:"):
		return s.hasPlayerState(state)
	case strings.HasPrefix(state, "floor:"):
		return s.showsFloor(strings.TrimPrefix(state, "floor:"))
	}

	return false
}

// hasPlayerState answers "player:C,R", "remote-player:C,R" and
// "player-start:C,R".
func (s *State) hasPlayerState(state string) bool {
	which, cell, _ := strings.Cut(state, ":")
	if which == "player-start" {
		return s.hasStart(cell)
	}

	return s.hasPlayer(cell, which == "remote-player")
}

// showsFloor reports whether the Nth floor from the bottom is shown.
func (s *State) showsFloor(value string) bool {
	number, err := strconv.Atoi(value)

	return err == nil && number >= 1 && number <= len(s.floors) && s.floors[number-1] == s.floor
}

// locateFloor finds the Nth floor from the bottom in the floor list.
// locateOver finds a part of whatever lies over the window: the unsaved
// changes guard, a dialog, or the right-click menu. over reports whether
// one does.
func (s *State) locateOver(kind, value string) (image.Point, bool, bool) {
	if kind == speechPart || kind == boxPart {
		point, found := s.locateShown(kind, value)

		return point, found, true
	}
	switch {
	case s.guard.Active():
		point, found := s.guard.Locate(value, s.size)

		return point, found, true
	case s.dialog != nil:
		point, found := s.locateDialog(value)

		return point, found, true
	case s.menu != nil && kind == "menu":
		point, found := s.menu.Locate(value)

		return point, found, true
	case s.zonePalettes.Active():
		point, found := s.zonePalettes.Locate(kind + ":" + value)

		return point, found, true
	}

	return image.Pt(0, 0), false, false
}

func (s *State) locateFloor(value string) (image.Point, bool) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 || number > len(s.floors) {
		return image.Pt(0, 0), false
	}

	return s.floorList.Locate(s.floors[number-1].String())
}

func (s *State) locateWidget(widget string) (image.Point, bool) {
	center := func(rect image.Rectangle) image.Point {
		return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
	}
	switch widget {
	case s.nameField.ID:
		return s.nameField.Center(), true
	case s.deleteBtn.ID:
		return center(s.deleteBtn.Rect), true
	case s.replace.ID:
		return center(s.replace.Rect), true
	case s.below.ID:
		return center(s.below.Rect), true
	case s.previewBn.ID:
		return center(s.previewBn.Rect), true
	case s.openBn.ID:
		return center(s.openBn.Rect), true
	case s.removeBn.ID:
		return center(s.removeBn.Rect), true
	case s.keysBn.ID:
		return center(s.keysBn.Rect), true
	}
	if at, found := s.locatePreviewWidget(widget); found {
		return at, true
	}
	s.zoom.layout(s.Grid(), s.camera)
	if at, found := s.zoom.locate(widget); found {
		return at, true
	}
	if at, found := s.save.Locate(widget); found {
		return at, true
	}

	return s.locateButton(widget)
}

// locateButton finds a button of the inspector's section for a selected
// sound or portal, or one that grows or shrinks a side of the floor.
func (s *State) locateButton(widget string) (image.Point, bool) {
	shown := s.inspect.buttons()
	for _, side := range s.sides {
		shown = append(shown, side.grow, side.shrink)
	}
	for _, button := range shown {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}
