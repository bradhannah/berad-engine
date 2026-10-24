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
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// ViewName is the Map window's view name; its target is a zone.
const ViewName = "map"

// Panel sizes, in screen pixels.
const (
	PanelWidth   = 190
	toolRows     = 8
	floorRows    = 5
	panelGap     = 10
	headingSpace = 22
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

type toolInfo struct {
	tool  Tool
	label string
	key   ui.Key
	help  string
}

func tools() []toolInfo {
	return []toolInfo{
		{ToolGround, "Ground", "G", "Drag to paint the ground."},
		{ToolWalls, "Walls", "W", "Drag to draw walls in the current family."},
		{ToolObjects, "Doors, stairs, pickups", "O", "Drag to place a door, stairs or a pickup."},
		{ToolErase, "Erase", "E", "Drag to clear what sits on the ground."},
		{ToolAnchor, "Anchors", "A", "Click to add an anchor; drag one to move it."},
		{ToolNPC, "Place NPC", "N", "Click to place the NPC chosen on the left; drag one to move it."},
		{ToolSelect, "Select walls", "S", "Click a wall for its run, double-click a room, or drag an area."},
		{ToolResize, "Resize floor", "R", "Drag an edge or corner of the floor."},
	}
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
}

type rectDrag struct {
	from, to Cell
}

type anchorDrag struct {
	id   ids.ULID
	from Cell
}

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDeleteAnchor
	dialogReplace
	dialogResize
)

// State is one Map window.
type State struct {
	window   ids.ULID
	zone     ids.ULID
	theme    *ui.Theme
	floors   []ids.ULID
	floor    ids.ULID
	subs     map[ids.ULID]bool
	tool     Tool
	tile     map[Tool]string
	camera   camera
	size     image.Point
	hover    Cell
	hovering bool
	stroke   stroke
	rect     *rectDrag
	drag     *anchorDrag
	selected []Cell
	anchor   ids.ULID
	// placement is the selected NPC placement; placing drags one. names is
	// this frame's copy of the project index, placedNPC the selected
	// placement's NPC and behaviour its behaviour workflow.
	placement ids.ULID
	placing   *placementDrag
	names     *project.Index
	placedNPC ids.ULID
	behaviour ids.ULID
	pick      *protocol.PickStart
	wheel     float64
	lastPan   image.Point
	panning   bool
	framed    bool
	resizing  *resizeDrag
	cursor    image.Point
	// choosing waits for the cell where a preview's player starts; starting
	// waits for the orchestrator to agree. preview is this window's own
	// preview, remote another window's preview of the zone.
	choosing bool
	starting bool
	preview  *previewRun
	remote   *remotePreview

	toolList  *ui.List
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
	openBn    *ui.Button
	removeBn  *ui.Button
	// The preview's start time, clock controls and Watch.
	startField *ui.TextField
	jumpField  *ui.TextField
	jumpBn     *ui.Button
	rateBns    []*ui.Button
	stopBn     *ui.Button
	watchBn    *ui.Button
	sides      []sideButtons
	focus      ui.Focus

	dialog        *ui.Dialog
	dialogKind    dialogKind
	replaceFrom   string
	pendingResize *pendingResize
}

// New returns the Map window for a zone.
func New(window, zone ids.ULID, theme *ui.Theme) *State {
	rateBns, stopBn, watchBn := newPreviewButtons()
	view := &State{
		window: window, zone: zone, theme: theme, floors: nil, floor: ids.ULID{}, subs: map[ids.ULID]bool{},
		tool: ToolGround, tile: map[Tool]string{ToolGround: "grass", ToolWalls: tiles.UnassignedWall, ToolObjects: "door"},
		camera: newCamera(), size: image.Pt(0, 0), hover: image.Pt(0, 0), hovering: false,
		stroke: stroke{active: false, count: 0, last: image.Pt(0, 0)},
		rect:   nil, drag: nil, selected: nil, anchor: ids.ULID{}, placement: ids.ULID{}, placing: nil,
		names: project.NewIndex(nil), placedNPC: ids.ULID{}, behaviour: ids.ULID{},
		pick: nil, wheel: 0, lastPan: image.Pt(0, 0), panning: false, framed: false, resizing: nil,
		cursor:   image.Pt(0, 0),
		choosing: false, starting: false, preview: nil, remote: nil,
		toolList:  &ui.List{ID: "tools", Rect: ui.Zero(), Rows: nil, Selected: string(ToolGround)},
		tileList:  &ui.List{ID: "tiles", Rect: ui.Zero(), Rows: nil, Selected: "grass"},
		floorList: &ui.List{ID: "floors", Rect: ui.Zero(), Rows: nil, Selected: ""},
		below:     &ui.Toggle{ID: "below", Label: "Show floor below", Rect: ui.Zero(), On: true},
		nameField: ui.NewTextField("anchor-name", textedit.NewField("", false, floor.MaxNameLength), false, theme),
		named:     ids.ULID{}, save: ui.NewSaveBar(), guard: frame.Guard{}, zoom: newZoomControl(),
		deleteBtn:  ui.NewButton("delete-anchor", "Delete anchor"),
		replace:    ui.NewButton("replace-family", "Replace family everywhere…"),
		previewBn:  ui.NewButton("preview", "Preview"),
		openBn:     ui.NewButton("open-behaviour", "Open behaviour"),
		removeBn:   ui.NewButton("remove-npc", "Remove"),
		startField: ui.NewTextField("start-time", textedit.NewField("", false, clockFieldBytes), false, theme),
		jumpField:  ui.NewTextField("jump-time", textedit.NewField("", false, clockFieldBytes), false, theme),
		jumpBn:     ui.NewButton("jump", "Jump"),
		rateBns:    rateBns, stopBn: stopBn, watchBn: watchBn,
		sides:  newSideButtons(),
		focus:  ui.Focus{},
		dialog: nil, dialogKind: dialogNone, replaceFrom: "", pendingResize: nil,
	}
	view.startField.Field.SetSaved(defaultStart)
	view.deleteBtn.Danger = true
	view.jumpField.Placeholder = "Jump to…"
	view.focus.Register(view.nameField, view.startField, view.jumpField)
	for _, info := range tools() {
		row := ui.NewRow(string(info.tool), info.label)
		row.Detail = string(info.key)
		view.toolList.Rows = append(view.toolList.Rows, row)
	}

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

// Changed reacts to the orchestrator: the zone's floors, picks.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	switch {
	case update.PreviewStart != nil, update.PreviewState != nil, update.PreviewStop != nil:
		s.previewChanged(update, ctx)
	case update.PreviewControl != nil:
		s.control(*update.PreviewControl, ctx)
	case update.PickStart != nil:
		start := *update.PickStart
		s.pick = &start
		s.stroke.active, s.choosing = false, false
		ctx.Frame.Status("Pick an anchor for " + labelOr(start.Label, "another window") + ". Escape cancels.")
	case update.PickCancel != nil && s.pick != nil && update.PickCancel.Pick == s.pick.Pick:
		s.pick = nil
		ctx.Frame.Status("Pick cancelled: " + labelOr(update.PickCancel.Reason, "no reason given"))
	case update.Snapshot != nil && *update.Snapshot == s.zone, update.Event != nil && update.Event.Document == s.zone:
		s.syncZone(ctx)
	}
}

func labelOr(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// Start subscribes to what the window shows. The window has already
// subscribed to the zone; this adds the project index for names.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
	s.syncZone(ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] {
		return
	}
	s.subs[document] = true
	ctx.Requests.Subscribe(document, func(err error) {
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
	s.selected, s.anchor, s.rect, s.drag = nil, ids.ULID{}, nil, nil
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

	return image.Rect(PanelWidth, 0, content.Max.X, content.Max.Y)
}

// layoutWidgets places the panel widgets for the window size.
func (s *State) layoutWidgets(docs frame.Docs) {
	s.refreshNPCs(docs)
	rowY := ui.Inset + headingSpace
	s.toolList.Rect = image.Rect(0, rowY, PanelWidth, rowY+toolRows*ui.RowHeight)
	rowY = s.toolList.Rect.Max.Y + panelGap + headingSpace
	s.tileList.Rows = s.tileRows()
	if s.tool == ToolSelect {
		s.tileList.Selected = ""
	}
	tilesHeight := max(len(s.tileList.Rows), 1) * ui.RowHeight
	if s.tool == ToolNPC {
		tilesHeight = min(max(len(s.tileList.Rows), 1), npcRows) * ui.RowHeight
	}
	s.tileList.Rect = image.Rect(0, rowY, PanelWidth, rowY+tilesHeight)
	rowY = s.tileList.Rect.Max.Y + panelGap + headingSpace
	s.floorList.Rows = s.floorRows(docs)
	s.floorList.Selected = s.floor.String()
	s.floorList.Rect = image.Rect(0, rowY, PanelWidth, rowY+floorRows*ui.RowHeight)
	rowY = s.floorList.Rect.Max.Y + panelGap/ui.Half
	s.below.Rect = image.Rect(ui.Inset/ui.Half+ui.Half, rowY, PanelWidth-ui.Inset, rowY+ui.RowHeight)
	rowY = s.below.Rect.Max.Y + panelGap
	s.previewBn.Rect = image.Rect(ui.Inset, rowY, PanelWidth-ui.Inset, rowY+ui.ButtonH)
	switch {
	case s.preview != nil:
		s.previewBn.Label = "Stop preview (P)"
	case s.choosing:
		s.previewBn.Label = "Cancel preview (P)"
	default:
		s.previewBn.Label = "Preview (P)"
	}

	s.layoutInspector()
}

// layoutInspector places the inspector's widgets for what is selected.
func (s *State) layoutInspector() {
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	s.nameField.Rect, s.deleteBtn.Rect, s.replace.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	s.openBn.Rect, s.removeBn.Rect = ui.Zero(), ui.Zero()
	if !s.placement.IsZero() {
		half := (width - ui.Gap) / ui.Half
		s.openBn.Rect = image.Rect(left, placementButtonsTop, left+half, placementButtonsTop+ui.ButtonH)
		s.removeBn.Rect = image.Rect(left+half+ui.Gap, placementButtonsTop, left+width, placementButtonsTop+ui.ButtonH)
	}
	s.save.Rect = ui.Zero()
	if !s.anchor.IsZero() {
		s.nameField.Rect = image.Rect(left, inspectorFieldTop, left+width, inspectorFieldTop+ui.ButtonH)
		saveTop := inspectorFieldTop + ui.ButtonH + anchorRows*ui.LineHeight
		s.save.Rect = image.Rect(left, saveTop, left+width, saveTop+ui.SaveBarHeight(width))
		deleteTop := s.save.Rect.Max.Y + ui.Half*ui.Gap
		s.deleteBtn.Rect = image.Rect(left, deleteTop, left+width, deleteTop+ui.ButtonH)
	}
	if len(s.selected) > 0 {
		s.replace.Rect = image.Rect(left, replaceTop, left+width, replaceTop+ui.ButtonH)
	}
	s.layoutPreview(left, width, inspector.Max.Y-len(helpLines())*ui.LineHeight)
	s.layoutSides(left, s.tool == ToolResize && s.pick == nil && len(s.selected) == 0 && s.anchor.IsZero() &&
		s.placement.IsZero())
}

// Inspector layout, from the top of the panel.
const (
	inspectorFieldTop = 6*ui.LineHeight + ui.Inset
	replaceTop        = inspectorFieldTop + ui.LineHeight*2
	familyListTop     = replaceTop + ui.ButtonH + ui.LineHeight
)

func (s *State) tileRows() []ui.Row {
	if s.tool == ToolNPC {
		return s.npcRows()
	}
	offered := tilesFor(s.tool)
	rows := make([]ui.Row, 0, len(offered))
	for index, definition := range offered {
		swatch := ui.Hex(definition.Color)
		detail := ""
		if index < numberedTiles {
			detail = strconv.Itoa(index + 1)
		}
		row := ui.NewRow(definition.ID, definition.Name)
		row.Detail, row.Swatch = detail, &swatch
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
		rows = append(rows, ui.NewRow(floorID.String(), name))
	}

	return rows
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.cursor = input.Cursor
	s.layoutWidgets(ctx.Docs)
	if s.guard.Active() {
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discard)

		return
	}
	if s.dialog != nil {
		s.updateDialog(input, ctx)

		return
	}
	s.guardAnchor()
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
	s.updatePanels(input, ctx)
	s.subscribeNPC(ctx)
	if s.preview != nil {
		s.stepPreview(input, ctx)
	} else {
		s.arrowHint(input, ctx)
	}
	if s.floor.IsZero() {
		return
	}
	loaded := ctx.Docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if !s.framed {
				s.camera.frame(floorDoc.Floor.Bounds())
				s.framed = true
			}
			s.syncAnchorField(floorDoc.Floor)
			s.syncPlacement(floorDoc.Floor)
			s.updateGrid(input, ctx, floorDoc.Floor)
			s.updateSides(input, ctx, floorDoc.Floor)
		}
	})
	if !loaded {
		s.hovering = false
	}
	s.guardAnchor()
}

// updateFields handles the text fields and the anchor's save bar: Enter
// saves a name, jumps the clock or sets the next preview's start.
func (s *State) updateFields(input *ui.Input, ctx *frame.Context) {
	switch field, outcome := s.focus.Update(input, ctx.Clipboard); {
	case field == s.nameField && outcome == textedit.OutcomeCommit:
		s.saveName(ctx)
	case field == s.jumpField && outcome == textedit.OutcomeCommit:
		s.jumpClock(ctx)
	case field == s.startField && outcome == textedit.OutcomeCommit:
		s.setStart(ctx)
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
	s.zoom.update(input, &s.camera, s.Grid(), bounds)
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
		s.camera.zoomBy(1, input.Cursor, s.Grid())
	case input.Consume(ui.KeyMinus, command):
		s.camera.zoomBy(-1, input.Cursor, s.Grid())
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
	case s.choosing && input.Consume(ui.KeyEscape, plain):
		s.togglePreview(ctx)
	case input.Consume(ui.KeyBackspace, plain), input.Consume(ui.KeyDelete, plain):
		s.deleteSelection(ctx)
	case s.pick == nil && input.Consume(ui.KeyEscape, plain):
		s.selected, s.anchor, s.placement, s.rect = nil, ids.ULID{}, ids.ULID{}, nil
	}
}

// deleteSelection deletes the selected anchor or removes the selected NPC.
func (s *State) deleteSelection(ctx *frame.Context) {
	switch {
	case !s.anchor.IsZero():
		s.deleteAnchor(ctx)
	case !s.placement.IsZero():
		s.removePlacement(ctx)
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
			offered := s.tileRows()
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
	s.stroke.active, s.rect, s.drag, s.resizing, s.placing = false, nil, nil, nil, nil
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
	s.below.Update(input)
	if s.previewBn.Update(input) {
		s.togglePreview(ctx)
	}
	s.updateInspectorButtons(input, ctx)
}

// updateInspectorButtons handles the buttons for what is selected.
func (s *State) updateInspectorButtons(input *ui.Input, ctx *frame.Context) {
	s.updatePreviewControls(input, ctx)
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
}

// updateGrid handles panning, zooming and the current tool on the floor.
func (s *State) updateGrid(input *ui.Input, ctx *frame.Context, floorData *floor.Floor) {
	grid := s.Grid()
	s.panAndZoom(input, grid)
	cell, onFloor := s.camera.cellAt(input.Cursor, grid)
	onFloor = onFloor && floorData.In(cell.X, cell.Y)
	s.hover, s.hovering = cell, onFloor
	if s.panning || input.Space {
		return
	}
	if s.pick != nil {
		s.updatePick(input, ctx, floorData, cell, onFloor)

		return
	}
	if s.choosing {
		s.updateChoosing(input, ctx, floorData, cell, onFloor)

		return
	}
	if s.watchClicked(input, ctx, cell, onFloor) {
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

func (s *State) panAndZoom(input *ui.Input, grid image.Rectangle) {
	if input.Cursor.In(grid) && (input.WheelX != 0 || input.WheelY != 0) {
		if input.Held.Shortcut {
			s.wheel += input.WheelY
			for s.wheel >= 1 {
				s.camera.zoomBy(1, input.Cursor, grid)
				s.wheel--
			}
			for s.wheel <= -1 {
				s.camera.zoomBy(-1, input.Cursor, grid)
				s.wheel++
			}
		} else {
			s.camera.pan(input.WheelPan())
		}
	}
	dragPan := input.PanDown || (input.Space && input.Down)
	if dragPan && s.panning {
		s.camera.pan(input.Cursor.Sub(s.lastPan))
	}
	s.panning = dragPan && (s.panning || input.Cursor.In(grid))
	s.lastPan = input.Cursor
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

func (s *State) updatePaint(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	switch {
	case input.Pressed && onFloor:
		s.stroke = stroke{active: true, count: s.stroke.count + 1, last: cell}
		s.paint(ctx, floorData, []Cell{cell})
	case s.stroke.active && input.Down && onFloor && cell != s.stroke.last:
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
	gesture := s.window.String() + "-" + strconv.Itoa(s.stroke.count)
	s.command(ctx, s.floor, tilefloor.PaintEdit, gesture, tilefloor.PaintArgs{Cells: changed}, nil)
}

// command sends an edit, reporting a refusal in the status bar.
func (s *State) command(
	ctx *frame.Context, document ids.ULID, edit, gesture string, args any, done func(protocol.Result),
) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: document, Edit: edit, Gesture: gesture, Args: data}
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

func (s *State) updateAnchor(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if input.Pressed && onFloor {
		if existing, found := floorData.AnchorAt(cell.X, cell.Y); found {
			s.selectAnchor(existing.ID)
			s.drag = &anchorDrag{id: existing.ID, from: cell}

			return
		}
		name := uniqueAnchorName(floorData)
		args := tilefloor.AddAnchorArgs{
			Anchor: floor.Anchor{ID: ids.ULID{}, Name: name, Col: cell.X, Row: cell.Y}, Index: len(floorData.Anchors()),
		}
		s.command(ctx, s.floor, tilefloor.AddAnchorEdit, "", args, func(result protocol.Result) {
			s.anchor = result.Created
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
	if s.drag != nil && !input.Down {
		drag := s.drag
		s.drag = nil
		if !onFloor || cell == drag.from {
			return
		}
		if _, taken := floorData.AnchorAt(cell.X, cell.Y); taken {
			ctx.Frame.Status("Another anchor is already there")

			return
		}
		if moved, _, found := floorData.Anchor(drag.id); found {
			moved.Col, moved.Row = cell.X, cell.Y
			s.command(ctx, s.floor, tilefloor.UpdateAnchorEdit, "", tilefloor.UpdateAnchorArgs{Anchor: moved}, nil)
		}
	}
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
	s.anchor = anchorID
	s.selected, s.placement = nil, ids.ULID{}
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
	s.selected, s.drag = nil, nil
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
	command := protocol.Command{Document: s.floor, Edit: tilefloor.UpdateAnchorEdit, Gesture: "", Args: data}
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
		if len(uses) == 0 {
			s.removeAnchor(ctx, floorID, anchorID)

			return
		}
		lines := []string{fmt.Sprintf("This anchor is used in %d %s:", len(uses), plural(len(uses), "place", "places"))}
		for _, use := range uses {
			lines = append(lines, "  "+use.Name+" ("+use.Label+")")
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

func (s *State) updateSelect(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if input.Pressed && onFloor {
		s.anchor = ids.ULID{}
		switch {
		case input.DoubleClick && !isWallAt(floorData, cell):
			walls, closed := RoomWalls(floorData, cell)
			if !closed {
				ctx.Frame.Status("That space is not closed in by walls and doors")

				return
			}
			s.selected = walls
			ctx.Frame.Status(fmt.Sprintf("Selected the %d walls around this room", len(walls)))
		case isWallAt(floorData, cell):
			s.selected = WallRun(floorData, cell)
			ctx.Frame.Status(fmt.Sprintf("Selected a run of %d walls", len(s.selected)))
		default:
			s.rect = &rectDrag{from: cell, to: cell}
		}
	}
	if s.rect != nil && input.Down && onFloor {
		s.rect.to = cell
	}
	if s.rect != nil && !input.Down {
		s.selected = WallsIn(floorData, s.rect.from, s.rect.to)
		if s.rect.from != s.rect.to || len(s.selected) > 0 {
			ctx.Frame.Status(fmt.Sprintf("Selected %d walls", len(s.selected)))
		}
		s.rect = nil
	}
}

// applyFamily changes the family of every selected wall in one undo step.
func (s *State) applyFamily(family string, ctx *frame.Context) {
	if len(s.selected) == 0 {
		ctx.Frame.Status("Select walls first: click a wall, double-click a room or drag an area")

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
		s.command(ctx, s.floor, tilefloor.PaintEdit, "", tilefloor.PaintArgs{Cells: cells}, func(protocol.Result) {
			ctx.Frame.Status(fmt.Sprintf("Changed %d walls to %s", len(cells), name))
		})
	})
	s.tile[ToolWalls] = family
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
	list := &ui.List{ID: "families", Rect: ui.Zero(), Rows: nil, Selected: ""}
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
		ctx.Requests.PickCancel(protocol.PickCancel{Pick: s.pick.Pick, Reason: "cancelled in the map"})
		s.pick = nil
		ctx.Frame.Status("Pick cancelled")

		return
	}
	if !input.Pressed || !onFloor {
		return
	}
	anchor, found := floorData.AnchorAt(cell.X, cell.Y)
	if !found {
		ctx.Frame.Status("Click one of the marked anchors, or press Escape")

		return
	}
	ctx.Requests.PickAnswer(protocol.PickAnswer{Pick: s.pick.Pick, Floor: s.floor, Item: anchor.ID.String()})
	s.pick = nil
	ctx.Frame.Status("Picked “" + anchor.Name + "”")
}

// Locate finds a named part of the window for input scripts: "cell:C,R",
// "edge:left" (or top, right, bottom) of the floor, "tool:<tool>",
// "tile:<id>", "floor:<n>" (1 is the bottom floor), "player" for a
// preview's player, "ui:anchor-name", "ui:delete-anchor",
// "ui:replace-family", "ui:below", "ui:preview", "ui:grow-left" and the other
// side buttons, or a dialog button or list row by ID.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layoutWidgets(docs)
	kind, value, _ := strings.Cut(name, ":")
	if s.guard.Active() {
		return s.guard.Locate(value, s.size)
	}
	if s.dialog != nil {
		return s.dialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	}
	switch kind {
	case "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	case "cell":
		var col, row int
		if _, err := fmt.Sscanf(value, "%d,%d", &col, &row); err != nil {
			return image.Pt(0, 0), false
		}
		rect := s.camera.cellRect(Cell{X: col, Y: row}, s.Grid())

		return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2), true
	case "edge":
		return s.locateEdge(value, docs)
	case "tool":
		return s.toolList.Locate(value)
	case "tile":
		return s.tileList.Locate(value)
	case "npc":
		return s.locateNPC(value)
	case "floor":
		return s.locateFloor(value)
	case "ui":
		return s.locateWidget(value)
	case "live-npc":
		return s.locateLiveNPC(value)
	case "player":
		return s.locatePlayer()
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts waiting for the window: "loaded" once the
// current floor has arrived and is in view, "picking" while answering a pick,
// "floor:N" when the Nth floor from the bottom is shown,
// "bounds:C,R,W,H" when the current floor has those bounds, "zoom:N" when
// the zoom control shows N%, "previewing"
// while this window runs a preview, "player:C,R" or "remote-player:C,R"
// when this window's or another window's player stands on that cell, and
// "placement:C,R" or "placement:C,R=Name" when an NPC, or the named NPC,
// is placed on that cell.
func (s *State) hasState(state string, docs frame.Docs) bool {
	switch {
	case state == "loaded":
		return !s.floor.IsZero() && s.framed && docs.View(s.floor, func(client.Document) {})
	case strings.HasPrefix(state, "bounds:"):
		return s.hasBounds(strings.TrimPrefix(state, "bounds:"), docs)
	case state == "picking":
		return s.pick != nil
	case strings.HasPrefix(state, "zoom:"):
		s.zoom.layout(s.Grid(), s.camera)

		return s.zoom.reset.Label == strings.TrimPrefix(state, "zoom:")+"%"
	case state == "previewing":
		return s.preview != nil
	case state == "watching", state == "npcs-moved", strings.HasPrefix(state, "npc:"):
		return s.hasNPCState(state, docs)
	case strings.HasPrefix(state, "placement:"):
		return s.hasPlacement(strings.TrimPrefix(state, "placement:"), docs)
	case strings.HasPrefix(state, "player:"), strings.HasPrefix(state, "remote-player:"):
		which, cell, _ := strings.Cut(state, ":")

		return s.hasPlayer(cell, which == "remote-player")
	case strings.HasPrefix(state, "floor:"):
		number, err := strconv.Atoi(strings.TrimPrefix(state, "floor:"))

		return err == nil && number >= 1 && number <= len(s.floors) && s.floors[number-1] == s.floor
	}

	return false
}

// locateFloor finds the Nth floor from the bottom in the floor list.
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
	for _, side := range s.sides {
		for _, button := range []*ui.Button{side.grow, side.shrink} {
			if button.ID == widget {
				return center(button.Rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}
