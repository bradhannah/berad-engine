// Package browser is the project browser window: the project's zones with
// their floors, its workflows, NPCs and text documents, in folders, the
// audio library's cues, which open in the Audio window, its sprites and
// palettes, and its settings. Authors create, rename, delete and open
// documents here, from the toolbar, the inspector or a right-click menu,
// keep them in folders and tag them, search them, reorder a zone's floors,
// choose each NPC's behaviour and write its notes, and set the tile size.
package browser

import (
	"encoding/json"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/paletteview"
	"github.com/bradhannah/berad-engine/editor/views/zonepalettes"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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
	spriteView   = "sprite"
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
	newZoneLoop = "new-zone-loop"
	newNPC      = "new-npc"
	newText     = "new-text"
)

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDelete
	dialogBehaviour
	dialogSprite
	dialogDeleteCue
	dialogMoveZone
	dialogZoneLoop
)

// useID is the Use button of a dialog choosing from a list.
const useID = "use"

// State is the project browser.
type State struct {
	window ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	list   *ui.List
	// closed holds the sections and zones whose rows are hidden; it is this
	// window's session state, never saved. revealed is the row last
	// scrolled into view on being selected.
	closed   map[string]bool
	revealed string
	focus    ui.Focus
	// search filters the list; name and tags edit what is selected, and
	// named is the row whose name and tags the fields hold. tileSize edits
	// the settings' tile size, and columns and rows the screen's size.
	search     *ui.TextField
	name       *ui.TextField
	tags       *ui.TextField
	named      string
	tileSize   *ui.TextField
	screenCols *ui.TextField
	screenRows *ui.TextField

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
	// contextMenu is the right-click menu, opened at contextAt once the
	// right-click waiting in context has settled the selection.
	contextMenu ui.Menu
	contextAt   image.Point
	context     contextClick

	npc *npcPanel
	// audio is the Audio section's cues.
	audio *audioState

	dialog     *ui.Dialog
	dialogKind dialogKind
	deleting   ids.ULID
	gestures   int
	// zonePalettes are a selected zone's palettes, and spritePalettes the
	// palettes a selected sprite's tile is painted with.
	zonePalettes   *zonepalettes.List
	spritePalettes []spritePalette
	// zones is the zone picker while it is open, choosing for zonesFor;
	// recentZones are the zones chosen in it lately, most recent first.
	zones       *zonepicker.Picker
	zonesFor    ids.ULID
	recentZones []ids.ULID
	// movingNPC and movingTo are the move of an NPC to another zone its
	// dialog asks about.
	movingNPC ids.ULID
	movingTo  ids.ULID
	// zoneNotes are a selected zone's private notes, for zoneNoted.
	zoneNotes *ui.TextField
	// loop is a selected zone's Zone loop box, and loopFor the zone its
	// dialog chooses a loop for.
	loop      *loopPanel
	loopFor   ids.ULID
	zoneNoted ids.ULID
}

// New returns the project browser.
func New(window ids.ULID, theme *ui.Theme) *State {
	button := ui.NewButton
	view := &State{
		window: window, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		list:   ui.NewList("documents", ""),
		closed: map[string]bool{}, revealed: "",
		focus:      ui.Focus{},
		search:     ui.NewTextField("search", textedit.NewField("", false, searchBytes), false, theme),
		name:       ui.NewTextField("name", textedit.NewField("", false, project.MaxNameLength), false, theme),
		tags:       ui.NewTextField("tags", textedit.NewField("", false, searchBytes*ui.Half), false, theme),
		named:      "",
		tileSize:   ui.NewTextField("tile-size", textedit.NewField("", false, tileSizeBytes), false, theme),
		screenCols: ui.NewTextField("screen-columns", textedit.NewField("", false, tileSizeBytes), false, theme),
		screenRows: ui.NewTextField("screen-rows", textedit.NewField("", false, tileSizeBytes), false, theme),
		newMenu:    button("new", "New"), menu: ui.Menu{Items: menuItems(), Width: 0},
		open: button("open", "Open"), floorButton: button(newFloor, "Add floor"), up: button("move-up", "Move up"),
		down: button("move-down", "Move down"), remove: button("delete", "Delete"),
		moveBn: button("move-to", "Move to…"), newFolderBn: button("new-folder", "New folder"),
		moveMenu: ui.Menu{Items: nil, Width: 0}, tagBns: nil, save: ui.NewSaveBar(),
		guard:       frame.Guard{},
		contextMenu: ui.Menu{Items: nil, Width: 0}, contextAt: image.Pt(0, 0), context: noContextClick(),
		npc: newNPCPanel(theme), audio: newAudioState(),
		dialog: nil, dialogKind: dialogNone, deleting: ids.ULID{}, gestures: 0,
		zonePalettes: zonepalettes.New(theme), spritePalettes: nil,
		zones: nil, zonesFor: ids.ULID{}, recentZones: nil, zoneNotes: newZoneNotes(theme), zoneNoted: ids.ULID{},
		loop: newLoopPanel(), loopFor: ids.ULID{},
		movingNPC: ids.ULID{}, movingTo: ids.ULID{},
	}
	view.remove.Danger, view.newMenu.Caret, view.moveBn.Caret = true, true, true
	view.search.Placeholder, view.search.Unmarked = "Search names, folders and #tags", true
	view.tags.Placeholder = "town, guards"
	view.menu.Width = ui.MenuWidth(view.menu.Items, theme)
	view.focus.Register(view.search, view.name, view.tags, view.npc.path, view.npc.notes, view.zoneNotes, view.tileSize,
		view.screenCols, view.screenRows)

	return view
}

// menuItems lists what New ▾ creates.
func menuItems() []ui.MenuItem {
	return []ui.MenuItem{
		{ID: newZone, Label: "Zone", Icon: ui.IconZone, Disabled: false, Danger: false, Rule: false},
		{ID: newFloor, Label: "Floor in the selected zone", Icon: ui.IconFloor, Disabled: false, Danger: false, Rule: false},
		{ID: newWorkflow, Label: flow.PurposeBehaviour.Label(), Icon: ui.IconWorkflow, Disabled: false, Danger: false,
			Rule: false},
		{ID: newZoneLoop, Label: flow.PurposeZoneLoop.Label(), Icon: ui.IconZoneLoop, Disabled: false, Danger: false,
			Rule: false},
		{ID: newNPC, Label: npcLabel, Icon: ui.IconNPC, Disabled: false, Danger: false, Rule: false},
		{ID: newText, Label: "Text document", Icon: ui.IconText, Disabled: false, Danger: false, Rule: false},
		{ID: newSound, Label: "Sound…", Icon: ui.IconSound, Disabled: false, Danger: false, Rule: false},
		{ID: newSprite, Label: "Sprite", Icon: ui.IconSprite, Disabled: false, Danger: false, Rule: false},
		{ID: newPalette, Label: "Palette", Icon: ui.IconPalette, Disabled: false, Danger: false, Rule: false},
		{ID: newTemplate, Label: "Template", Icon: ui.IconTemplate, Disabled: false, Danger: false, Rule: false},
		{ID: newFolderPrefix + zone.FormatName, Label: "Zone folder", Icon: ui.IconFolder, Disabled: false, Danger: false,
			Rule: true},
		{ID: newFolderPrefix + flow.FormatName, Label: "Workflow folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + npc.FormatName, Label: npcLabel + " folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + textdoc.FormatName, Label: "Text folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + art.SpriteFormatName, Label: "Sprite folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + art.PaletteFormatName, Label: "Palette folder", Icon: ui.IconFolder, Disabled: false,
			Danger: false, Rule: false},
		{ID: newFolderPrefix + artplugin.TemplateFormatName, Label: "Template folder", Icon: ui.IconFolder,
			Disabled: false, Danger: false, Rule: false},
	}
}

// Title is the window title.
func (s *State) Title(frame.Docs) string { return "Project" }

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField {
	if s.zones != nil {
		return s.zones.Focused()
	}

	return s.focus.Current()
}

// Start subscribes to the project index, the art and game settings and the
// world state, and lists the audio library.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
	s.subscribe(art.SettingsID(), ctx)
	s.subscribe(game.SettingsID(), ctx)
	s.subscribe(worldstate.ID(), ctx)
	s.askAudio(ctx)
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

// Changed subscribes to every zone, so their floors can be listed, to every
// NPC, so the list can say which workflow each follows, and to every sprite,
// so its tile goes with it. It lists the audio library again when it
// changes.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.AudioChanged != nil {
		s.askAudio(ctx)
	}
	if update.Event == nil && update.Snapshot == nil {
		return
	}
	for _, format := range []string{zone.FormatName, npc.FormatName, art.SpriteFormatName} {
		for _, entry := range s.entries(ctx.Docs, format) {
			s.subscribe(entry.ID, ctx)
		}
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
	s.revealSelected()
	index := s.index(docs)
	s.list.CanDrop = func(dragged, target string) bool { return s.canDrop(index, dragged, target) }
	s.layoutInspector(docs)
}

// revealSelected scrolls a newly selected row into view once it is listed,
// as a document just made, which can land below the rows shown.
func (s *State) revealSelected() {
	if s.list.Selected == s.revealed {
		return
	}
	for _, row := range s.list.Rows {
		if row.ID == s.list.Selected {
			s.list.Reveal(row.ID)
			s.revealed = row.ID
		}
	}
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

// Update handles one frame of input. The guard, a dialog or a menu, while
// showing, takes all of it. A right-click opens its menu once the selection
// it makes has settled.
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
	case s.zones != nil:
		s.updateZonePicker(input, ctx)

		return
	case s.menu.IsOpen():
		s.create(ctx, s.menu.Update(input))

		return
	case s.contextMenu.IsOpen():
		s.chooseFromContext(ctx, s.contextMenu.Update(input))

		return
	case s.moveMenu.IsOpen():
		if chosen, ok := strings.CutPrefix(s.moveMenu.Update(input), movePrefix); ok {
			s.moveSelected(ctx, chosen)
		}

		return
	case s.zonePalettes.Active():
		s.zonePalettes.Update(input, ctx)

		return
	}
	s.guardSelection(ctx)
	s.sync(ctx)
	s.updateFields(input, ctx)
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	s.updateList(input, ctx)
	s.buttons(input, ctx)
	s.guardSelection(ctx)
	s.openContextMenu(ctx.Docs)
}

// sync keeps the inspector showing what is selected.
func (s *State) sync(ctx *frame.Context) {
	s.syncArt(ctx)
	s.syncName(ctx.Docs)
	s.syncSettings(ctx.Docs)
	s.syncNPC(ctx)
	s.syncZoneNotes(ctx)
	s.syncCueUses(ctx)
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
	case field == s.tileSize && outcome == textedit.OutcomeCommit:
		s.commitTileSize(ctx)
	case (field == s.screenCols || field == s.screenRows) && outcome == textedit.OutcomeCommit:
		s.commitScreen(ctx)
	}
}

// updateList handles the list: selecting, opening, showing and hiding rows,
// a section's [+], dropping a row on a folder or a section, and a
// right-click.
func (s *State) updateList(input *ui.Input, ctx *frame.Context) {
	result := s.list.Update(input)
	if result.Changed {
		s.focus.Set(nil)
	}
	if result.Context {
		s.rightClicked(result.ContextRow, input.Cursor)
	}
	if result.Activated {
		s.openRow(ctx)
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
// updateTagChips searches for a tag when its chip is clicked.
func (s *State) updateTagChips(input *ui.Input) {
	for _, chip := range s.tagBns {
		if chip.Update(input) {
			s.searchTag(strings.TrimPrefix(chip.ID, "tag:"))
		}
	}
}

func (s *State) buttons(input *ui.Input, ctx *frame.Context) {
	if s.newMenu.Update(input) {
		s.openMenu(ctx.Docs)

		return
	}
	if s.saveBar(input, ctx) || s.updateArt(input, ctx) || s.updateZoneLoop(input, ctx) {
		return
	}
	switch {
	case s.open.Update(input):
		s.openRow(ctx)
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
		s.updateTagChips(input)
	}
}

// saveBar handles the save bar and the Audio section's button, and reports
// whether one was used.
func (s *State) saveBar(input *ui.Input, ctx *frame.Context) bool {
	if s.audio.open.Update(input) {
		s.openCue(ctx)

		return true
	}
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)
	case ui.RevertID:
		s.discard()
	default:
		return false
	}

	return true
}

// openMenu shows New ▾, with a sprite drawn to each template; a floor needs
// a zone, or a floor of one, selected.
func (s *State) openMenu(docs frame.Docs) {
	s.menu.Items = s.newItems(docs)
	_, hasZone := s.selectedZone(docs)
	for index := range s.menu.Items {
		if s.menu.Items[index].ID == newFloor {
			s.menu.Items[index].Disabled = !hasZone
		}
	}
	s.menu.Open(s.newMenu.Rect, frame.AboveStatus(s.size))
}

// create makes what a New ▾ item or a section's [+] names.
func (s *State) create(ctx *frame.Context, item string) {
	if format, isFolder := strings.CutPrefix(item, newFolderPrefix); isFolder {
		s.createFolder(ctx, format)

		return
	}
	if template, drawn := drawnItem(item); drawn {
		s.createDrawn(ctx, template)

		return
	}
	switch item {
	case newZone:
		s.createZone(ctx)
	case newFloor:
		s.createFloor(ctx)
	case newWorkflow:
		s.createWorkflow(ctx, flow.PurposeBehaviour)
	case newZoneLoop:
		s.createWorkflow(ctx, flow.PurposeZoneLoop)
	case newNPC:
		s.createNPC(ctx)
	case newText:
		s.createText(ctx)
	case newSound:
		s.addSound(ctx)
	case newSprite:
		s.createSprite(ctx)
	case newPalette:
		s.createPalette(ctx)
	case newTemplate:
		s.createTemplate(ctx)
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
		Kind: "", Data: data, Layout: nil, Changed: time.Time{},
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
	made := zoneplugin.NewDocument()
	made.Zone.Path = worldstate.UniqueName(zone.SuggestPath(name), s.zonePaths(ctx.Docs))
	data, err := made.Encode()
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

// zonePaths lists the path names the window's zones use.
func (s *State) zonePaths(docs frame.Docs) map[string]bool {
	taken := map[string]bool{}
	for _, entry := range s.entries(docs, zone.FormatName) {
		docs.View(entry.ID, func(document client.Document) {
			if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok && zoneDoc.Zone.Path != "" {
				taken[zoneDoc.Zone.Path] = true
			}
		})
	}

	return taken
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

// createWorkflow creates a workflow of a kind, holding only its start node.
func (s *State) createWorkflow(ctx *frame.Context, purpose flow.Purpose) {
	base := "Workflow"
	if purpose == flow.PurposeZoneLoop {
		base = purpose.Label()
	}
	name := uniqueName(base, names(s.entries(ctx.Docs, flow.FormatName)))
	args := newDocument(flow.FormatName, name, ids.ULID{}, workflowplugin.NewDataFor(purpose))
	args.Folder = s.targetFolder(ctx.Docs, flow.FormatName)
	s.command(ctx, project.IndexID(), project.CreateEdit, s.gesture(), args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
	}, nil)
}

// documentView names the view a document of a format opens in, for the
// formats that open in a view of their own.
func documentView(format string) (string, bool) {
	view, ok := map[string]string{
		textdoc.FormatName: textView, flow.FormatName: workflowView, art.SpriteFormatName: spriteView,
		art.PaletteFormatName: paletteView, artplugin.TemplateFormatName: templateView,
	}[format]

	return view, ok
}

// openRow opens what the selected row shows: World state's Values or
// Signals window for its rows, or what openSelected opens.
func (s *State) openRow(ctx *frame.Context) {
	switch s.list.Selected {
	case worldStateRow:
		ctx.Requests.Open(worldStateView, worldstate.ID(), false)

		return
	case worldSignalsRow:
		ctx.Requests.Open(worldSignalsView, worldstate.ID(), false)

		return
	}
	s.openSelected(ctx)
}

// openSelected opens a zone, or the zone of a floor, in a Map window, a text
// document in a Text window, a workflow, or an NPC's behaviour workflow, in a
// Workflow window, a sprite in a Pixel editor, a palette in the Palette
// Editor and a template in the Template editor. A folder opens or closes.
func (s *State) openSelected(ctx *frame.Context) {
	if s.openCue(ctx) {
		return
	}
	if strings.HasPrefix(s.list.Selected, audioCategoryPrefix) {
		s.closed[s.list.Selected] = !s.closed[s.list.Selected]

		return
	}
	if folder, isFolder := s.selectedFolder(); isFolder {
		id := folderRowID(folder.Format, folder.Path)
		s.closed[id] = !s.closed[id]

		return
	}
	entry, ok := s.selected(ctx.Docs)
	if !ok {
		return
	}
	if view, own := documentView(entry.Format); own {
		ctx.Requests.Open(view, entry.ID, false)

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
	if _, isCue := s.selectedCue(); isCue {
		s.deleteCue(ctx)

		return
	}
	if folder, isFolder := s.selectedFolder(); isFolder {
		s.deleteFolder(ctx, folder)

		return
	}
	entry, ok := s.selected(ctx.Docs)
	if !ok {
		return
	}
	if everyProjectHolds(entry.ID) {
		ctx.Frame.Status("Every project holds “" + entry.Name + "”, so it cannot be deleted")

		return
	}
	floors := s.floorsOf(ctx.Docs, entry.ID)
	targets := append([]ids.ULID{entry.ID}, floors...)
	if tile, isSprite := s.tileOf(ctx.Docs, entry.ID); isSprite {
		targets = append(targets, tile)
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
				s.askDelete(ctx, entry, len(floors), uses)
			}
		})
	}
}

// askDelete asks before deleting, once where it is used is known. A palette
// that is used opens in its Palette Editor instead, which lists each use to
// remove first.
func (s *State) askDelete(ctx *frame.Context, entry project.Entry, floors int, uses []protocol.Use) {
	if entry.Format == art.PaletteFormatName && len(uses) > 0 {
		ctx.Requests.OpenItem(paletteView, entry.ID, paletteview.ItemDelete)
		ctx.Frame.Status("“" + entry.Name + "” is used. Its Palette Editor lists each use to remove first.")

		return
	}
	s.confirmDelete(entry, floors, uses)
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
	if choice == useID && list != nil {
		s.useChosen(ctx, kind, list.Selected)

		return
	}
	switch {
	case kind == dialogDelete && choice == "delete":
		s.deleteDocument(ctx, s.deleting)
	case kind == dialogDeleteCue && choice == "delete":
		s.removeCue(ctx, s.audio.deleting)
	case kind == dialogMoveZone && choice == zoneMoveID:
		s.applyMove(ctx, s.movingNPC, s.movingTo)
	}
}

// useChosen sets what a dialog's list chose, by Use: an NPC's behaviour or
// sprite, or a zone's loop.
func (s *State) useChosen(ctx *frame.Context, kind dialogKind, chosen string) {
	switch kind {
	case dialogBehaviour:
		s.setBehaviour(ctx, chosen)
	case dialogSprite:
		s.setSprite(ctx, chosen)
	case dialogZoneLoop:
		s.setZoneLoop(ctx, chosen)
	case dialogNone, dialogDelete, dialogDeleteCue, dialogMoveZone:
	}
}

// deleteDocument deletes a document in one gesture: a floor is first taken out
// of its zone; a zone takes its floors with it, and a sprite its tile.
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
	tile, isSprite := s.tileOf(ctx.Docs, documentID)
	remove(documentID)
	if isSprite {
		remove(tile)
	}
	s.list.Selected = ""
	ctx.Frame.Status("Deleted “" + entry.Name + "”. Undo brings it back.")
}

// Locate finds a named part of the window for input scripts: "row:<name>"
// for a document by name or the Settings row, "toggle:<name>" for its
// disclosure triangle, "section:<zones|workflows|npcs|text|audio|sprites|
// palettes>" for a section heading, "ui:<button id>", "ui:name",
// "ui:notes", "ui:tile-size", an item of New ▾ or the right-click menu while
// it is open, or a dialog button or list row. "ui:new-zone",
// "ui:new-workflow", "ui:new-npc", "ui:new-text", "ui:new-sound" and
// "ui:new-sprite" are their section's [+]. A row or section located is
// scrolled into view first, as an author scrolls to it.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout(docs)
	kind, value, _ := strings.Cut(name, ":")
	switch {
	case s.guard.Active():
		return s.guard.Locate(value, s.size)
	case s.dialog != nil:
		return s.locateInDialog(kind, value)
	case s.zones != nil:
		return s.locateZonePicker(name, docs)
	case s.menu.IsOpen():
		return s.menu.Locate(value)
	case s.contextMenu.IsOpen():
		return s.contextMenu.Locate(value)
	case s.moveMenu.IsOpen():
		return s.moveMenu.Locate(movePrefix + value)
	case s.zonePalettes.Active():
		return s.zonePalettes.Locate(name)
	}
	switch kind {
	case "row", "toggle":
		return s.locateRow(kind, value)
	case "section":
		s.list.Reveal(sectionPrefix + value)

		return s.list.Locate(sectionPrefix + value)
	case "ui":
		return s.locateWidget(value)
	case "state":
		found, answered := s.hasZoneState(docs, value)
		if !answered {
			found = s.hasRowState(value)
		}

		return image.Pt(0, 0), found
	}

	return image.Pt(0, 0), false
}

// hasRowState answers a row's states: "row-detail:<name>=<detail>", the
// detail of the row named, and "row-loop:<name>", a row showing a zone
// loop; and "flag=<text>", a flag in the inspector holding the text, under
// an NPC's behaviour or a zone's loop.
func (s *State) hasRowState(state string) bool {
	if text, isFlag := strings.CutPrefix(state, "flag="); isFlag {
		return strings.Contains(s.npc.flag+" "+s.loop.flag, text)
	}
	if rest, isDetail := strings.CutPrefix(state, "row-detail:"); isDetail {
		label, detail, _ := strings.Cut(rest, "=")

		return slices.ContainsFunc(s.list.Rows, func(row ui.Row) bool { return row.Label == label && row.Detail == detail })
	}
	if label, isLoop := strings.CutPrefix(state, "row-loop:"); isLoop {
		return slices.ContainsFunc(s.list.Rows, func(row ui.Row) bool {
			return row.Label == label && row.Icon == ui.IconZoneLoop
		})
	}

	return false
}

// locateRow finds a document row by its name, or its disclosure triangle.
func (s *State) locateRow(kind, label string) (image.Point, bool) {
	for _, row := range s.list.Rows {
		if row.Label != label || row.Heading {
			continue
		}
		s.list.Reveal(row.ID)
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
		s.list.Reveal(section)

		return s.list.LocateAdd(section)
	}
	if point, found := s.save.Locate(widget); found {
		return point, true
	}
	if widget == zoneBoxID && !s.npc.zone.Empty() {
		return ui.Center(s.npc.zone), true
	}
	fields := []*ui.TextField{
		s.search, s.name, s.tags, s.npc.path, s.npc.notes, s.zoneNotes, s.tileSize, s.screenCols, s.screenRows,
	}
	for _, field := range fields {
		if widget == field.ID && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	for _, button := range s.allButtons() {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return s.locateArt(widget)
}

// allButtons lists the window's buttons outside dialogs.
func (s *State) allButtons() []*ui.Button {
	return append([]*ui.Button{
		s.newMenu, s.open, s.floorButton, s.up, s.down, s.remove, s.moveBn, s.newFolderBn, s.npc.choose, s.npc.open,
		s.npc.chooseSprite, s.npc.openSprite, s.npc.showOnMap, s.loop.choose, s.loop.open,
		s.audio.open,
	}, s.tagBns...)
}

// locateInDialog finds a dialog button, or a dialog list row by its name,
// or answers "state:dialog-line=<text>", a line of the dialog holding the
// text, and "state:dialog-row=<name>", a row of its list.
func (s *State) locateInDialog(kind, value string) (image.Point, bool) {
	if kind == "state" {
		line, isLine := strings.CutPrefix(value, "dialog-line=")
		row, isRow := strings.CutPrefix(value, "dialog-row=")
		found := isLine && strings.Contains(strings.Join(s.dialog.Lines, " "), line)
		if isRow && s.dialog.List != nil {
			found = slices.ContainsFunc(s.dialog.List.Rows, func(listed ui.Row) bool { return listed.Label == row })
		}

		return image.Pt(0, 0), found || value == "dialog"
	}
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
	s.contextMenu.Draw(screen, s.theme)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
	s.drawZonePicker(screen, docs)
	s.guard.Draw(screen, s.theme)
}
