// Package browser is the project browser window: the project's zones with
// their floors, its workflows, NPCs and text documents, in folders. Authors
// create, rename, delete and open documents here, keep them in folders and
// tag them, search them, reorder a zone's floors, and choose each NPC's
// behaviour and write its notes.
package browser

import (
	"encoding/json"
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// ViewName is the project browser's view name; its target is the project
// index.
const ViewName = "browser"

// Views the browser opens.
const (
	mapView      = "map"
	textView     = "text"
	workflowView = "workflow"
)

// New floors match the zone's top floor, or this size centred on the zone
// origin.
const (
	defaultWidth  = 32
	defaultHeight = 24
	toolbarHeight = 44
	searchHeight  = ui.ButtonH + ui.Inset
	newWidth      = 96
	buttonGap     = 6
	searchBytes   = 100
)

// npcLabel names the NPC type wherever the browser shows it.
const npcLabel = "NPC"

// Menu item IDs, which a section's [+] shares.
const (
	newZone     = "new-zone"
	newFloor    = "new-floor"
	newWorkflow = "new-workflow"
	newNPC      = "new-npc"
	newText     = "new-text"
)

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDelete
	dialogBehaviour
)

// State is the project browser.
type State struct {
	window ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	list   *ui.List
	// closed holds the sections and zones whose rows are hidden; it is this
	// window's session state, never saved.
	closed map[string]bool
	focus  ui.Focus
	// search filters the list; name and tags edit what is selected, and
	// named is the row whose name and tags the fields hold.
	search *ui.TextField
	name   *ui.TextField
	tags   *ui.TextField
	named  string

	newMenu                             *ui.Button
	menu                                ui.Menu
	open, floorButton, up, down, remove *ui.Button
	// moveBn opens moveMenu, the folders the selection can move to;
	// newFolderBn makes a folder inside a selected folder, and tagBns
	// search for each of a document's tags.
	moveBn, newFolderBn *ui.Button
	moveMenu            ui.Menu
	tagBns              []*ui.Button
	save                *ui.SaveBar
	guard               frame.Guard

	npc *npcPanel

	dialog     *ui.Dialog
	dialogKind dialogKind
	deleting   ids.ULID
	gestures   int
}

// New returns the project browser.
func New(window ids.ULID, theme *ui.Theme) *State {
	button := ui.NewButton
	view := &State{
		window: window, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		list:    &ui.List{ID: "documents", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil},
		closed:  map[string]bool{},
		focus:   ui.Focus{},
		search:  ui.NewTextField("search", textedit.NewField("", false, searchBytes), false, theme),
		name:    ui.NewTextField("name", textedit.NewField("", false, project.MaxNameLength), false, theme),
		tags:    ui.NewTextField("tags", textedit.NewField("", false, searchBytes*ui.Half), false, theme),
		named:   "",
		newMenu: button("new", "New"), menu: ui.Menu{Items: menuItems(), Width: 0},
		open: button("open", "Open"), floorButton: button(newFloor, "Add floor"), up: button("move-up", "Move up"),
		down: button("move-down", "Move down"), remove: button("delete", "Delete"),
		moveBn: button("move-to", "Move to…"), newFolderBn: button("new-folder", "New folder"),
		moveMenu: ui.Menu{Items: nil, Width: 0}, tagBns: nil, save: ui.NewSaveBar(),
		guard:  frame.Guard{},
		npc:    newNPCPanel(theme),
		dialog: nil, dialogKind: dialogNone, deleting: ids.ULID{}, gestures: 0,
	}
	view.remove.Danger, view.newMenu.Caret, view.moveBn.Caret = true, true, true
	view.search.Placeholder, view.search.Unmarked = "Search names, folders and #tags", true
	view.tags.Placeholder = "town, guards"
	view.menu.Width = ui.MenuWidth(view.menu.Items, theme)
	view.focus.Register(view.search, view.name, view.tags, view.npc.notes)

	return view
}

// menuItems lists what New ▾ creates.
func menuItems() []ui.MenuItem {
	return []ui.MenuItem{
		{ID: newZone, Label: "Zone", Icon: ui.IconZone, Disabled: false, Danger: false, Rule: false},
		{ID: newFloor, Label: "Floor in the selected zone", Icon: ui.IconFloor, Disabled: false, Danger: false, Rule: false},
		{ID: newWorkflow, Label: "Workflow", Icon: ui.IconWorkflow, Disabled: false, Danger: false, Rule: false},
		{ID: newNPC, Label: npcLabel, Icon: ui.IconNPC, Disabled: false, Danger: false, Rule: false},
		{ID: newText, Label: "Text document", Icon: ui.IconText, Disabled: false, Danger: false, Rule: false},
		{ID: newFolderPrefix + zone.FormatName, Label: "Zone folder", Icon: ui.IconFolder, Disabled: false, Danger: false,
			Rule: true},
		{ID: newFolderPrefix + flow.FormatName, Label: "Workflow folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + npc.FormatName, Label: npcLabel + " folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + textdoc.FormatName, Label: "Text folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
	}
}

// Title is the window title.
func (s *State) Title(frame.Docs) string { return "Project" }

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Start subscribes to the project index.
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

// Changed subscribes to every zone, so their floors can be listed, and to
// every NPC, so the list can say which workflow each follows.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.Event == nil && update.Snapshot == nil {
		return
	}
	for _, entry := range append(s.entries(ctx.Docs, zone.FormatName), s.entries(ctx.Docs, npc.FormatName)...) {
		s.subscribe(entry.ID, ctx)
	}
}

func (s *State) index(docs frame.Docs) *project.Index {
	found := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			found = project.NewIndexWithFolders(index.Entries(), index.AllFolders())
		}
	})

	return found
}

func (s *State) entries(docs frame.Docs, format string) []project.Entry {
	return s.index(docs).OfFormat(format)
}

func (s *State) floorsOf(docs frame.Docs, zoneID ids.ULID) []ids.ULID {
	var floors []ids.ULID
	docs.View(zoneID, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			floors = zoneDoc.Zone.Floors()
		}
	})

	return floors
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

// layout places the toolbar, the search box, the list and the inspector.
func (s *State) layout(docs frame.Docs) {
	content := frame.Content(s.size)
	top := (toolbarHeight - ui.ButtonH) / ui.Half
	s.newMenu.Rect = image.Rect(content.Max.X-ui.Inset-newWidth, top, content.Max.X-ui.Inset, top+ui.ButtonH)
	searchTop := toolbarHeight + ui.Inset/ui.Half
	s.search.Rect = image.Rect(ui.Inset, searchTop, content.Max.X-ui.Inset, searchTop+ui.ButtonH)
	s.list.Rect = image.Rect(0, toolbarHeight+searchHeight, content.Max.X, content.Max.Y)
	s.list.Rows = s.rows(docs)
	index := s.index(docs)
	s.list.CanDrop = func(dragged, target string) bool { return s.canDrop(index, dragged, target) }
	s.layoutInspector(docs)
}

// selected returns the selected document's entry.
func (s *State) selected(docs frame.Docs) (project.Entry, bool) {
	selected, err := ids.Parse(s.list.Selected)
	if err != nil {
		var none project.Entry

		return none, false
	}

	return s.index(docs).Entry(selected)
}

// zoneOf returns the zone listing a floor.
func (s *State) zoneOf(docs frame.Docs, floorID ids.ULID) (ids.ULID, bool) {
	for _, entry := range s.entries(docs, zone.FormatName) {
		for _, candidate := range s.floorsOf(docs, entry.ID) {
			if candidate == floorID {
				return entry.ID, true
			}
		}
	}

	return ids.ULID{}, false
}

// Update handles one frame of input. The guard, a dialog or the New menu,
// while showing, takes all of it.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.layout(ctx.Docs)
	switch {
	case s.guard.Active():
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.discard)

		return
	case s.dialog != nil:
		s.updateDialog(input, ctx)

		return
	case s.menu.IsOpen():
		s.create(ctx, s.menu.Update(input))

		return
	case s.moveMenu.IsOpen():
		if chosen, ok := strings.CutPrefix(s.moveMenu.Update(input), movePrefix); ok {
			s.moveSelected(ctx, chosen)
		}

		return
	}
	s.guardSelection(ctx)
	s.syncName(ctx.Docs)
	s.syncNPC(ctx)
	s.updateFields(input, ctx)
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	s.updateList(input, ctx)
	s.buttons(input, ctx)
	s.guardSelection(ctx)
}

// updateFields handles the text fields: Enter saves a name or tags, and the
// search box filters the list as it is typed. Escape clears the search.
func (s *State) updateFields(input *ui.Input, ctx *frame.Context) {
	field, outcome := s.focus.Update(input, ctx.Clipboard)
	switch {
	case field == s.search && outcome == textedit.OutcomeCommit:
		value, _ := s.search.Field.Commit()
		s.search.Field.SetSaved(value)
	case field == s.search && outcome == textedit.OutcomeBlur:
		s.search.Field.SetSaved("")
	case field == s.name && outcome == textedit.OutcomeCommit:
		s.commitName(ctx)
	case field == s.tags && outcome == textedit.OutcomeCommit:
		s.commitTags(ctx)
	}
}

// updateList handles the list: selecting, opening, showing and hiding rows,
// a section's [+], and dropping a row on a folder or a section.
func (s *State) updateList(input *ui.Input, ctx *frame.Context) {
	result := s.list.Update(input)
	if result.Changed {
		s.focus.Set(nil)
	}
	if result.Activated {
		s.openSelected(ctx)
	}
	if result.Toggled != "" {
		s.closed[result.Toggled] = !s.closed[result.Toggled]
	}
	if result.Added != "" {
		s.create(ctx, sectionNew(result.Added))
	}
	if result.Dropped != "" {
		s.drop(ctx, result.Dragged, result.Dropped)
	}
}

func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	plain := ui.Plain()
	switch {
	case input.Consume(ui.KeyEnter, plain):
		s.openSelected(ctx)
	case input.Consume(ui.KeyF, ui.Cmd()):
		s.focus.Set(s.search)
	case input.Consume(ui.KeyBackspace, ui.Cmd()):
		s.deleteSelected(ctx)
	case input.Consume(ui.KeyUp, plain):
		s.step(-1)
	case input.Consume(ui.KeyDown, plain):
		s.step(1)
	case input.Consume(ui.KeyLeft, plain):
		s.fold(ctx.Docs, true)
	case input.Consume(ui.KeyRight, plain):
		s.fold(ctx.Docs, false)
	}
}

// step moves the selection to the previous or next document row.
func (s *State) step(delta int) {
	selectable := []string{}
	for _, row := range s.list.Rows {
		if !row.Heading {
			selectable = append(selectable, row.ID)
		}
	}
	current := -1
	for position, id := range selectable {
		if id == s.list.Selected {
			current = position
		}
	}
	next := current + delta
	if current < 0 {
		next = 0
	}
	if next >= 0 && next < len(selectable) {
		s.list.Selected = selectable[next]
		s.focus.Set(nil)
	}
}

// fold hides or shows what a selected zone or folder holds, as Left and
// Right do in an outline; Left on a closed folder, a floor, or a document in
// a folder selects what holds it.
func (s *State) fold(docs frame.Docs, hide bool) {
	if folder, ok := s.selectedFolder(); ok {
		rowID := folderRowID(folder.Format, folder.Path)
		if hide && s.closed[rowID] && project.FolderParent(folder.Path) != "" {
			s.list.Selected = folderRowID(folder.Format, project.FolderParent(folder.Path))

			return
		}
		s.closed[rowID] = hide

		return
	}
	entry, ok := s.selected(docs)
	if !ok {
		return
	}
	switch {
	case entry.Format == zone.FormatName && (!hide || !s.closed[entry.ID.String()]):
		s.closed[entry.ID.String()] = hide
	case entry.Format == floor.FormatName:
		if zoneID, inZone := s.zoneOf(docs, entry.ID); inZone && hide {
			s.list.Selected = zoneID.String()
		}
	case hide && entry.Folder != "":
		s.list.Selected = folderRowID(entry.Format, entry.Folder)
	}
}

// buttons handles the toolbar and the inspector's buttons.
func (s *State) buttons(input *ui.Input, ctx *frame.Context) {
	if s.newMenu.Update(input) {
		s.openMenu(ctx.Docs)

		return
	}
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)

		return
	case ui.RevertID:
		s.discard()

		return
	}
	switch {
	case s.open.Update(input):
		s.openSelected(ctx)
	case s.floorButton.Update(input):
		s.createFloor(ctx)
	case s.up.Update(input):
		s.moveFloor(ctx, 1)
	case s.down.Update(input):
		s.moveFloor(ctx, -1)
	case s.remove.Update(input):
		s.deleteSelected(ctx)
	case s.moveBn.Update(input):
		s.openMoveMenu(ctx.Docs)
	case s.newFolderBn.Update(input):
		if folder, ok := s.selectedFolder(); ok {
			s.createFolder(ctx, folder.Format)
		}
	default:
		s.updateNPC(input, ctx)
		for _, chip := range s.tagBns {
			if chip.Update(input) {
				s.searchTag(strings.TrimPrefix(chip.ID, "tag:"))
			}
		}
	}
}

// openMenu shows New ▾; a floor needs a zone, or a floor of one, selected.
func (s *State) openMenu(docs frame.Docs) {
	_, hasZone := s.selectedZone(docs)
	for index := range s.menu.Items {
		if s.menu.Items[index].ID == newFloor {
			s.menu.Items[index].Disabled = !hasZone
		}
	}
	s.menu.Open(s.newMenu.Rect, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
}

// create makes what a New ▾ item or a section's [+] names.
func (s *State) create(ctx *frame.Context, item string) {
	if format, isFolder := strings.CutPrefix(item, newFolderPrefix); isFolder {
		s.createFolder(ctx, format)

		return
	}
	switch item {
	case newZone:
		s.createZone(ctx)
	case newFloor:
		s.createFloor(ctx)
	case newWorkflow:
		s.createWorkflow(ctx)
	case newNPC:
		s.createNPC(ctx)
	case newText:
		s.createText(ctx)
	}
}

// command sends an edit; done runs on success, failed on refusal.
func (s *State) command(ctx *frame.Context, document ids.ULID, edit, gesture string, args any,
	done func(protocol.Result, error), failed func(),
) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	ctx.Requests.Command(protocol.Command{Document: document, Edit: edit, Gesture: gesture, Args: data},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)
				if failed != nil {
					failed()
				}

				return
			}
			if done != nil {
				done(result, nil)
			}
		})
}

func (s *State) gesture() string {
	s.gestures++

	return s.window.String() + "-" + strconv.Itoa(s.gestures)
}

// uniqueName returns "<base> N" unused among names.
func uniqueName(base string, names []string) string {
	used := map[string]bool{}
	for _, name := range names {
		used[strings.ToLower(name)] = true
	}
	for number := 1; ; number++ {
		candidate := base + " " + strconv.Itoa(number)
		if !used[strings.ToLower(candidate)] {
			return candidate
		}
	}
}

func names(entries []project.Entry) []string {
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Name)
	}

	return result
}

// newDocument returns the arguments that create a document; the orchestrator
// assigns its ID and version.
func newDocument(format, name string, scope ids.ULID, data json.RawMessage) documentArgs {
	return documentArgs{project.DocumentArgs{
		ID: ids.ULID{}, Format: format, Name: name, Version: 0, Revision: 0, Scope: scope, Folder: "", Tags: nil,
		Data: data, Layout: nil,
	}}
}

// documentArgs builds project.DocumentArgs.
type documentArgs struct {
	project.DocumentArgs
}

// withID names an existing document, as a deletion does.
func (a documentArgs) withID(id ids.ULID) project.DocumentArgs {
	a.ID = id

	return a.DocumentArgs
}

// createZone creates a zone with one floor, as one undo step.
func (s *State) createZone(ctx *frame.Context) {
	name := uniqueName("Zone", names(s.entries(ctx.Docs, zone.FormatName)))
	data, err := zoneplugin.NewDocument().Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	gesture := s.gesture()
	args := newDocument(zone.FormatName, name, ids.ULID{}, data)
	args.Folder = s.targetFolder(ctx.Docs, zone.FormatName)
	s.command(ctx, project.IndexID(), project.CreateEdit, gesture, args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
		s.subscribe(result.Created, ctx)
		s.addFloor(ctx, result.Created, "Floor 1", floor.Centered(defaultWidth, defaultHeight), "grass", 0, gesture)
		ctx.Frame.Status("Created “" + name + "” with one floor")
	}, nil)
}

// createFloor adds a floor on top of the selected zone (or the zone of the
// selected floor), with the same bounds as the floor below it.
func (s *State) createFloor(ctx *frame.Context) {
	zoneID, found := s.selectedZone(ctx.Docs)
	if !found {
		return
	}
	floors := s.floorsOf(ctx.Docs, zoneID)
	floorNames := make([]string, 0, len(floors))
	index := s.index(ctx.Docs)
	for _, id := range floors {
		floorNames = append(floorNames, index.Name(id, ""))
	}
	bounds, ground := floor.Centered(defaultWidth, defaultHeight), "grass"
	if len(floors) > 0 {
		ground = ""
		s.subscribe(floors[len(floors)-1], ctx)
		ctx.Docs.View(floors[len(floors)-1], func(document client.Document) {
			if floorDoc, isFloor := document.Model.(*tilefloor.Document); isFloor {
				bounds = floorDoc.Floor.Bounds()
			}
		})
	}
	name := uniqueName("Floor", floorNames)
	s.addFloor(ctx, zoneID, name, bounds, ground, len(floors), s.gesture())
}

// selectedZone returns the selected zone, or the zone of the selected floor.
func (s *State) selectedZone(docs frame.Docs) (ids.ULID, bool) {
	entry, found := s.selected(docs)
	switch {
	case !found:
		return ids.ULID{}, false
	case entry.Format == zone.FormatName:
		return entry.ID, true
	default:
		return s.zoneOf(docs, entry.ID)
	}
}

// addFloor creates a floor document and inserts it into a zone, in one
// gesture.
func (s *State) addFloor(
	ctx *frame.Context, zoneID ids.ULID, name string, bounds image.Rectangle, ground string, position int, gesture string,
) {
	document, err := tilefloor.NewDocumentAt(bounds, ground)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	data, err := document.Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	args := newDocument(floor.FormatName, name, zoneID, data)
	s.command(ctx, project.IndexID(), project.CreateEdit, gesture, args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
		s.command(ctx, zoneID, zoneplugin.InsertFloorEdit, gesture,
			zoneplugin.FloorArgs{Floor: result.Created, Index: position}, nil, nil)
	}, nil)
}

func (s *State) createText(ctx *frame.Context) {
	name := uniqueName("Text", names(s.entries(ctx.Docs, textdoc.FormatName)))
	data, err := textdoc.NewDocument().Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	args := newDocument(textdoc.FormatName, name, ids.ULID{}, data)
	args.Folder = s.targetFolder(ctx.Docs, textdoc.FormatName)
	s.command(ctx, project.IndexID(), project.CreateEdit, s.gesture(), args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
	}, nil)
}

// createWorkflow creates a workflow holding only its start node.
func (s *State) createWorkflow(ctx *frame.Context) {
	name := uniqueName("Workflow", names(s.entries(ctx.Docs, flow.FormatName)))
	args := newDocument(flow.FormatName, name, ids.ULID{}, workflowplugin.NewData())
	args.Folder = s.targetFolder(ctx.Docs, flow.FormatName)
	s.command(ctx, project.IndexID(), project.CreateEdit, s.gesture(), args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
	}, nil)
}

// openSelected opens a zone, or the zone of a floor, in a Map window, a text
// document in a Text window, and a workflow, or an NPC's behaviour workflow,
// in a Workflow window. A folder opens or closes.
func (s *State) openSelected(ctx *frame.Context) {
	if folder, isFolder := s.selectedFolder(); isFolder {
		id := folderRowID(folder.Format, folder.Path)
		s.closed[id] = !s.closed[id]

		return
	}
	entry, ok := s.selected(ctx.Docs)
	if !ok {
		return
	}
	switch entry.Format {
	case zone.FormatName:
		ctx.Requests.Open(mapView, entry.ID, false)
	case floor.FormatName:
		zoneID, inZone := s.zoneOf(ctx.Docs, entry.ID)
		if !inZone {
			ctx.Frame.Status("This floor is in no zone, so no map shows it")

			return
		}
		ctx.Requests.Open(mapView, zoneID, false)
	case textdoc.FormatName:
		ctx.Requests.Open(textView, entry.ID, false)
	case flow.FormatName:
		ctx.Requests.Open(workflowView, entry.ID, false)
	case npc.FormatName:
		if found, loaded := s.npcOf(ctx.Docs, entry.ID); loaded && !found.Behaviour.IsZero() {
			ctx.Requests.Open(workflowView, found.Behaviour, false)

			return
		}
		ctx.Frame.Status("This NPC has no behaviour yet. Choose one in the inspector.")
	}
}

// moveFloor moves the selected floor up or down in its zone.
func (s *State) moveFloor(ctx *frame.Context, delta int) {
	entry, ok := s.selected(ctx.Docs)
	if !ok {
		return
	}
	zoneID, inZone := s.zoneOf(ctx.Docs, entry.ID)
	if !inZone {
		return
	}
	floors := s.floorsOf(ctx.Docs, zoneID)
	position := -1
	for index, id := range floors {
		if id == entry.ID {
			position = index
		}
	}
	target := position + delta
	if position < 0 || target < 0 || target >= len(floors) {
		return
	}
	s.command(ctx, zoneID, zoneplugin.MoveFloorEdit, "", zoneplugin.FloorArgs{Floor: entry.ID, Index: target}, nil, nil)
}

// deleteSelected asks where the selected document is used, then confirms
// (D4). Deleting a zone deletes its floors with it, as one undo step. A
// folder is deleted when it is empty.
func (s *State) deleteSelected(ctx *frame.Context) {
	if folder, isFolder := s.selectedFolder(); isFolder {
		s.deleteFolder(ctx, folder)

		return
	}
	entry, ok := s.selected(ctx.Docs)
	if !ok {
		return
	}
	targets := []ids.ULID{entry.ID}
	if entry.Format == zone.FormatName {
		targets = append(targets, s.floorsOf(ctx.Docs, entry.ID)...)
	}
	containing, _ := s.zoneOf(ctx.Docs, entry.ID)
	var uses []protocol.Use
	remaining := len(targets)
	for _, target := range targets {
		ctx.Requests.WhereUsed(target, "", func(found []protocol.Use, err error) {
			remaining--
			if err == nil {
				for _, use := range found {
					if use.Document != containing && use.Document != entry.ID && !contains(targets, use.Document) {
						uses = append(uses, use)
					}
				}
			}
			if remaining == 0 {
				s.confirmDelete(entry, len(targets)-1, uses)
			}
		})
	}
}

func contains(list []ids.ULID, id ids.ULID) bool {
	for _, candidate := range list {
		if candidate == id {
			return true
		}
	}

	return false
}

func (s *State) confirmDelete(entry project.Entry, floors int, uses []protocol.Use) {
	title := "Delete “" + entry.Name + "”?"
	lines := []string{}
	if floors > 0 {
		lines = append(lines, fmt.Sprintf("Its %d %s will be deleted too.", floors, plural(floors, "floor", "floors")))
	}
	if len(uses) > 0 {
		lines = append(lines, fmt.Sprintf("It is used in %d %s:", len(uses), plural(len(uses), "place", "places")))
		for _, use := range uses {
			lines = append(lines, "  "+use.Name+" ("+use.Label+")")
		}
		lines = append(lines, "Those references will be broken.")
	}
	lines = append(lines, "You can undo this.")
	s.dialog = ui.NewDialog(title, lines, ui.DialogButton("delete", "Delete"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialogKind = dialogDelete
	s.deleting = entry.ID
}

func (s *State) updateDialog(input *ui.Input, ctx *frame.Context) {
	choice := s.dialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	if choice == "" {
		return
	}
	kind, list := s.dialogKind, s.dialog.List
	s.dialog, s.dialogKind = nil, dialogNone
	switch {
	case kind == dialogDelete && choice == "delete":
		s.deleteDocument(ctx, s.deleting)
	case kind == dialogBehaviour && choice == "use" && list != nil:
		s.setBehaviour(ctx, list.Selected)
	}
}

// deleteDocument deletes a document in one gesture: a floor is first taken out
// of its zone; a zone takes its floors with it.
func (s *State) deleteDocument(ctx *frame.Context, documentID ids.ULID) {
	entry, ok := s.index(ctx.Docs).Entry(documentID)
	if !ok {
		return
	}
	gesture := s.gesture()
	remove := func(target ids.ULID) {
		deletion := newDocument("", "", ids.ULID{}, nil).withID(target)
		s.command(ctx, project.IndexID(), project.DeleteEdit, gesture, deletion, nil, nil)
	}
	switch entry.Format {
	case zone.FormatName:
		for _, floorID := range s.floorsOf(ctx.Docs, documentID) {
			remove(floorID)
		}
	case floor.FormatName:
		if zoneID, inZone := s.zoneOf(ctx.Docs, documentID); inZone {
			out := zoneplugin.FloorArgs{Floor: documentID, Index: 0}
			s.command(ctx, zoneID, zoneplugin.RemoveFloorEdit, gesture, out, nil, nil)
		}
	}
	remove(documentID)
	s.list.Selected = ""
	ctx.Frame.Status("Deleted “" + entry.Name + "”. Undo brings it back.")
}

// Locate finds a named part of the window for input scripts: "row:<name>"
// for a document by name, "toggle:<name>" for its disclosure triangle,
// "section:<zones|workflows|npcs|text>" for a section heading,
// "ui:<button id>", "ui:name", "ui:notes", a New ▾ item while the menu is
// open, or a dialog button or list row. "ui:new-zone", "ui:new-workflow",
// "ui:new-npc" and "ui:new-text" are their section's [+].
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout(docs)
	kind, value, _ := strings.Cut(name, ":")
	switch {
	case s.guard.Active():
		return s.guard.Locate(value, s.size)
	case s.dialog != nil:
		return s.locateInDialog(kind, value)
	case s.menu.IsOpen():
		return s.menu.Locate(value)
	case s.moveMenu.IsOpen():
		return s.moveMenu.Locate(movePrefix + value)
	}
	switch kind {
	case "row", "toggle":
		return s.locateRow(kind, value)
	case "section":
		return s.list.Locate(sectionPrefix + value)
	case "ui":
		return s.locateWidget(value)
	}

	return image.Pt(0, 0), false
}

// locateRow finds a document row by its name, or its disclosure triangle.
func (s *State) locateRow(kind, label string) (image.Point, bool) {
	for _, row := range s.list.Rows {
		if row.Label != label || row.Heading {
			continue
		}
		if kind == "toggle" {
			return s.list.LocateToggle(row.ID)
		}

		return s.list.Locate(row.ID)
	}

	return image.Pt(0, 0), false
}

// locateWidget finds a field, a button, a section's [+] or the save bar's
// buttons.
func (s *State) locateWidget(widget string) (image.Point, bool) {
	if section := newSection(widget); section != "" {
		return s.list.LocateAdd(section)
	}
	if point, found := s.save.Locate(widget); found {
		return point, true
	}
	for _, field := range []*ui.TextField{s.search, s.name, s.tags, s.npc.notes} {
		if widget == field.ID && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	for _, button := range s.allButtons() {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// allButtons lists the window's buttons outside dialogs.
func (s *State) allButtons() []*ui.Button {
	return append([]*ui.Button{
		s.newMenu, s.open, s.floorButton, s.up, s.down, s.remove, s.moveBn, s.newFolderBn, s.npc.choose, s.npc.open,
	}, s.tagBns...)
}

// locateInDialog finds a dialog button, or a dialog list row by its name.
func (s *State) locateInDialog(kind, value string) (image.Point, bool) {
	if kind == "row" && s.dialog.List != nil {
		for _, row := range s.dialog.List.Rows {
			if row.Label == value {
				value = row.ID
			}
		}
	}

	return s.dialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
}

// Draw renders the window: the toolbar, the list, the inspector and anything
// showing over them.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout(docs)
	screen.Fill(ui.ColorBackground())
	width := frame.Content(s.size).Max.X
	s.theme.DrawTitle(screen, "Project", ui.Inset, (toolbarHeight-ui.TitleSize)/ui.Half-1, ui.ColorInk())
	ui.Divider(screen, image.Pt(0, toolbarHeight-ui.Outline), image.Pt(width, toolbarHeight-ui.Outline))
	s.newMenu.Draw(screen, s.theme)
	s.search.Draw(screen)
	s.list.Draw(screen, s.theme)
	s.drawInspector(screen, docs)
	s.menu.Draw(screen, s.theme)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
	s.guard.Draw(screen, s.theme)
}
