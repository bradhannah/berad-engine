// Package browser is the project browser window: the project's zones with
// their floors, and its text documents. Authors create, rename, delete and
// open documents here, and reorder a zone's floors.
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
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// ViewName is the project browser's view name; its target is the project
// index.
const ViewName = "browser"

// Views the browser opens.
const (
	mapView  = "map"
	textView = "text"
)

// New floors match the zone's top floor, or this size centred on the zone
// origin.
const (
	defaultWidth  = 32
	defaultHeight = 24
	toolbarHeight = 44
	buttonWidth   = 96
	buttonGap     = 6
	nameFieldTop  = ui.Inset + 2*ui.LineHeight
	inspectorRows = 4
)

const (
	headingZones = "heading:zones"
	headingText  = "heading:text"
	headingLoose = "heading:loose"
)

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDelete
)

// State is the project browser.
type State struct {
	window ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	list   *ui.List
	focus  ui.Focus
	name   *ui.TextField
	named  ids.ULID

	newZone, newFloor, newText, rename, remove, up, down, open *ui.Button

	dialog     *ui.Dialog
	dialogKind dialogKind
	deleting   ids.ULID
	gestures   int
}

// New returns the project browser.
func New(window ids.ULID, theme *ui.Theme) *State {
	button := func(id, label string) *ui.Button {
		return &ui.Button{ID: id, Label: label, Rect: ui.Zero(), Disabled: false, Primary: false}
	}
	view := &State{
		window: window, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		list:    &ui.List{ID: "documents", Rect: ui.Zero(), Rows: nil, Selected: ""},
		focus:   ui.Focus{},
		name:    ui.NewTextField("name", textedit.NewField("", false, project.MaxNameLength), false, theme),
		named:   ids.ULID{},
		newZone: button("new-zone", "New zone"), newFloor: button("new-floor", "New floor"),
		newText: button("new-text", "New text"), rename: button("rename", "Rename"),
		remove: button("delete", "Delete"), up: button("move-up", "Move up"),
		down: button("move-down", "Move down"), open: button("open", "Open"),
		dialog: nil, dialogKind: dialogNone, deleting: ids.ULID{}, gestures: 0,
	}
	view.focus.Register(view.name)

	return view
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

// Changed subscribes to every zone, so their floors can be listed.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.Event == nil && update.Snapshot == nil {
		return
	}
	for _, entry := range s.entries(ctx.Docs, zone.FormatName) {
		s.subscribe(entry.ID, ctx)
	}
}

func (s *State) index(docs frame.Docs) *project.Index {
	found := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			found = project.NewIndex(index.Entries())
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

// rows lists zones with their floors (top floor first), text documents, and
// floors not in any zone.
func (s *State) rows(docs frame.Docs) []ui.Row {
	index := s.index(docs)
	row := func(id ids.ULID, label, detail string, indent int) ui.Row {
		return ui.Row{
			ID: id.String(), Label: label, Detail: detail, Indent: indent, Swatch: nil, Heading: false, Marked: false,
		}
	}
	heading := func(id, label string) ui.Row {
		return ui.Row{ID: id, Label: label, Detail: "", Indent: 0, Swatch: nil, Heading: true, Marked: false}
	}
	rows := []ui.Row{heading(headingZones, "ZONES")}
	listed := map[ids.ULID]bool{}
	for _, entry := range index.OfFormat(zone.FormatName) {
		floors := s.floorsOf(docs, entry.ID)
		count := fmt.Sprintf("%d %s", len(floors), plural(len(floors), "floor", "floors"))
		rows = append(rows, row(entry.ID, entry.Name, count, 0))
		for position := len(floors) - 1; position >= 0; position-- {
			listed[floors[position]] = true
			rows = append(rows, row(floors[position], index.Name(floors[position], "missing floor"), "", 1))
		}
	}
	rows = append(rows, heading(headingText, "TEXT DOCUMENTS"))
	for _, entry := range index.OfFormat(textdoc.FormatName) {
		rows = append(rows, row(entry.ID, entry.Name, "", 0))
	}
	var loose []ui.Row
	for _, entry := range index.OfFormat(floor.FormatName) {
		if !listed[entry.ID] {
			loose = append(loose, row(entry.ID, entry.Name, "", 0))
		}
	}
	if len(loose) > 0 {
		rows = append(rows, heading(headingLoose, "FLOORS IN NO ZONE"))
		rows = append(rows, loose...)
	}

	return rows
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

func (s *State) layout(docs frame.Docs) {
	content := frame.Content(s.size)
	x := ui.Inset
	for _, button := range []*ui.Button{s.newZone, s.newFloor, s.newText, s.open, s.up, s.down} {
		button.Rect = image.Rect(x, ui.Inset/ui.Half+ui.Gap, x+buttonWidth, ui.Inset/ui.Half+ui.Gap+ui.ButtonH)
		x += buttonWidth + buttonGap
	}
	s.list.Rect = image.Rect(0, toolbarHeight, content.Max.X, content.Max.Y)
	s.list.Rows = s.rows(docs)
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	selected, hasSelection := s.selected(docs)
	s.name.Rect, s.rename.Rect, s.remove.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	if hasSelection {
		s.name.Rect = image.Rect(left, nameFieldTop, left+width, nameFieldTop+ui.ButtonH)
		buttons := nameFieldTop + ui.ButtonH + inspectorRows*ui.LineHeight
		half := (width - buttonGap) / ui.Half
		s.rename.Rect = image.Rect(left, buttons, left+half, buttons+ui.ButtonH)
		s.remove.Rect = image.Rect(left+half+buttonGap, buttons, left+width, buttons+ui.ButtonH)
	}
	_, inZone := s.zoneOf(docs, selected.ID)
	s.up.Disabled = !hasSelection || selected.Format != floor.FormatName || !inZone
	s.down.Disabled = s.up.Disabled
	s.open.Disabled = !hasSelection
	s.newFloor.Disabled = !hasSelection || (selected.Format != zone.FormatName && !inZone)
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

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.layout(ctx.Docs)
	if s.dialog != nil {
		s.updateDialog(input, ctx)

		return
	}
	s.syncName(ctx.Docs)
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field == s.name && outcome == textedit.OutcomeCommit {
		s.commitName(ctx)
	}
	if s.focus.Current() == nil {
		s.keys(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	result := s.list.Update(input)
	if result.Changed {
		s.focus.Set(nil)
	}
	if result.Activated {
		s.openSelected(ctx)
	}
	s.buttons(input, ctx)
}

func (s *State) keys(input *ui.Input, ctx *frame.Context) {
	plain := ui.Plain()
	switch {
	case input.Consume(ui.KeyEnter, plain):
		s.openSelected(ctx)
	case input.Consume(ui.KeyBackspace, ui.Cmd()):
		s.deleteSelected(ctx)
	case input.Consume(ui.KeyUp, plain):
		s.step(-1)
	case input.Consume(ui.KeyDown, plain):
		s.step(1)
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

func (s *State) buttons(input *ui.Input, ctx *frame.Context) {
	switch {
	case s.newZone.Update(input):
		s.createZone(ctx)
	case s.newFloor.Update(input):
		s.createFloor(ctx)
	case s.newText.Update(input):
		s.createText(ctx)
	case s.open.Update(input):
		s.openSelected(ctx)
	case s.up.Update(input):
		s.moveFloor(ctx, 1)
	case s.down.Update(input):
		s.moveFloor(ctx, -1)
	case s.rename.Update(input):
		s.renameSelected(ctx)
	case s.remove.Update(input):
		s.deleteSelected(ctx)
	}
}

// syncName keeps the name field showing the selected document's name.
func (s *State) syncName(docs frame.Docs) {
	entry, ok := s.selected(docs)
	if !ok {
		if !s.named.IsZero() {
			s.focus.Set(nil)
			s.name.Field.Revert()
			s.named = ids.ULID{}
		}

		return
	}
	if entry.ID != s.named {
		s.focus.Set(nil)
		s.name.Field.Revert()
		s.named = entry.ID
	}
	s.name.Field.SetSaved(entry.Name)
}

// renameSelected saves a name typed into the name field, as Enter does. With
// nothing typed yet, it selects the name so typing replaces it.
func (s *State) renameSelected(ctx *frame.Context) {
	if s.name.Field.Dirty() {
		s.commitName(ctx)

		return
	}
	s.focus.Set(s.name)
	if buffer := s.name.Field.Buffer(); buffer != nil {
		buffer.SelectAll()
	}
	ctx.Frame.Status("Type a new name, then press Enter or click Rename")
}

func (s *State) commitName(ctx *frame.Context) {
	name, changed := s.name.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	name = strings.TrimSpace(name)
	rename := project.RenameArgs{ID: s.named, Name: name}
	s.command(ctx, project.IndexID(), project.RenameEdit, "", rename,
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Renamed to “" + name + "”")
		},
		func() { s.name.Field.Failed() })
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
		ID: ids.ULID{}, Format: format, Name: name, Version: 0, Revision: 0, Scope: scope, Data: data,
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
	entry, found := s.selected(ctx.Docs)
	if !found {
		return
	}
	zoneID := entry.ID
	if entry.Format != zone.FormatName {
		if zoneID, found = s.zoneOf(ctx.Docs, entry.ID); !found {
			return
		}
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
	s.command(ctx, project.IndexID(), project.CreateEdit, s.gesture(), args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
	}, nil)
}

// openSelected opens a zone, or the zone of a floor, in a Map window, and a
// text document in a Text window.
func (s *State) openSelected(ctx *frame.Context) {
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
// (D4). Deleting a zone deletes its floors with it, as one undo step.
func (s *State) deleteSelected(ctx *frame.Context) {
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
	s.dialog, s.dialogKind = nil, dialogNone
	if choice == "delete" {
		s.deleteDocument(ctx, s.deleting)
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
// for a document by name, "ui:<button id>", "ui:name", or a dialog button.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout(docs)
	kind, value, _ := strings.Cut(name, ":")
	if s.dialog != nil {
		return s.dialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	}
	switch kind {
	case "row":
		for _, row := range s.list.Rows {
			if row.Label == value && !row.Heading {
				return s.list.Locate(row.ID)
			}
		}
	case "ui":
		if value == s.name.ID {
			return s.name.Center(), true
		}
		for _, button := range []*ui.Button{s.newZone, s.newFloor, s.newText, s.rename, s.remove, s.up, s.down, s.open} {
			if button.ID == value {
				return image.Pt(button.Rect.Min.X+button.Rect.Dx()/2, button.Rect.Min.Y+button.Rect.Dy()/2), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout(docs)
	screen.Fill(ui.ColorBackground())
	ui.FillRect(screen, image.Rect(0, 0, frame.Content(s.size).Max.X, toolbarHeight), ui.ColorPanel())
	width := frame.Content(s.size).Max.X
	ui.FillRect(screen, image.Rect(0, toolbarHeight-ui.Hairline, width, toolbarHeight), ui.ColorRule())
	for _, button := range []*ui.Button{s.newZone, s.newFloor, s.newText, s.open, s.up, s.down} {
		button.Draw(screen, s.theme)
	}
	s.list.Draw(screen, s.theme)
	s.drawInspector(screen, docs)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
}

func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	textX, textY := panel.Min.X+ui.Inset, panel.Min.Y+ui.Inset
	s.theme.Heading(screen, "INSPECTOR", textX, textY)
	entry, ok := s.selected(docs)
	if !ok {
		s.theme.DrawText(screen, "Select a document.", textX, textY+ui.LineHeight, ui.ColorMuted())
		s.theme.DrawText(screen, "Double-click or Enter opens it.", textX, textY+2*ui.LineHeight, ui.ColorMuted())

		return
	}
	s.theme.DrawText(screen, "NAME", textX, nameFieldTop-ui.LineHeight, ui.ColorMuted())
	s.name.Draw(screen)
	textY = nameFieldTop + ui.ButtonH + ui.Gap
	hint := "Enter saves, Escape reverts"
	if s.name.Field.Dirty() {
		hint = "Unsaved. " + hint
	}
	s.theme.DrawText(screen, hint, textX, textY, ui.ColorMuted())
	kinds := map[string]string{zone.FormatName: "Zone", floor.FormatName: "Floor", textdoc.FormatName: "Text document"}
	kind := kinds[entry.Format]
	s.theme.DrawText(screen, kind, textX, textY+ui.LineHeight, ui.ColorInk())
	s.theme.DrawText(screen, "ID "+entry.ID.String(), textX, textY+2*ui.LineHeight, ui.ColorSubdued())
	s.rename.Draw(screen, s.theme)
	s.remove.Draw(screen, s.theme)
}
