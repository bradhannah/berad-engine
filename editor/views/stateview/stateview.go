// Package stateview is the World state windows, Values and Signals, each
// opened from the World state section of the Project window.
//
// Values shows a project's values as a tree. The world's values come first, in their groups; then for zones and
// for NPCs their templates, and each zone and NPC with its own values, an
// NPC's name, and the templates applied to it with what they give it. The
// inspector edits the selected value, group or template, applies templates
// to the selected NPC or zone, and says where a value is used. Every change
// is saved as it is made and undone with Cmd+Z; typed text waits for Enter
// or Save. Nothing a workflow uses can be deleted.
//
// Signals shows the signals workflows send and answer, in their groups,
// each with how many send and answer it.
//
// A workflow's local values are declared on the workflow itself, in its
// Workflow window.
package stateview

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/editor/views/valueform"
	"github.com/bradhannah/berad-engine/editor/views/valuepicker"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/editor/views/writesheet"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The World state windows' view names; their target is the project's
// world-state document. Values shows the world's, zones' and NPCs' values,
// Signals the signals workflows send and answer, and Constants the words
// and numbers the designer sets and the game never changes. All are filed
// under World state on the window bar.
const (
	ViewName          = "world-state"
	SignalsViewName   = "world-signals"
	ConstantsViewName = "world-constants"
)

// Part is which part of the world state a window shows.
type Part int

const (
	// PartValues is the world's values, and each zone's and NPC's.
	PartValues Part = iota
	// PartSignals is the signals workflows send and answer.
	PartSignals
	// PartConstants is the world's constants, and each zone's and NPC's.
	PartConstants
)

// Layout, in screen pixels. The list keeps a readable width, listWidth, with
// its details in a column detailAt from its left, and the inspector stands
// beside it.
const (
	listWidth = 720
	detailAt  = 320
	addWidth  = 120
	headerTop = ui.Inset + ui.LineHeight + ui.LineHeight
	listTop   = headerTop + ui.ButtonH + ui.Gap
	footerGap = 10
	usesShown = 6
)

// Locate names for input scripts.
const (
	addValueID    = "add-value"
	addConstantID = "add-constant"
	addGroupID    = "add-group"
	addTemplateID = "add-template"
	pathID        = "owner-path"
	takeOffID     = "take-off"
	templateID    = "template-"
	rowLocate     = "row:"
	toggleLocate  = "toggle:"
	statePrefix   = "state:"
	stateLoaded   = "loaded"
	stateSelected = "selected="
	stateValue    = "value="
	stateApplied  = "applied="
	stateSays     = "says="
	stateAdds     = "adds="
)

// State is a World state window: its Values or its Signals.
type State struct {
	part Part
	// folded is set while the inspector is folded to its strip.
	folded   bool
	window   ids.ULID
	document ids.ULID
	theme    *ui.Theme
	size     image.Point
	list     *ui.List
	shown    tree
	// toggled are the rows whose triangle was clicked: NPCs and zones
	// opened, anything else closed.
	toggled map[string]bool
	subs    map[ids.ULID]bool
	adds    []*ui.Button
	form    *valueform.Form
	focus   ui.Focus
	save    *ui.SaveBar
	guard   frame.Guard
	// named is the row whose declaration the form holds; uses and usesOf
	// are where it is used, as last asked.
	named      string
	uses       []protocol.Use
	usesOf     string
	usesAsking bool
	// usesFor is the row uses last answered for, kept while asking again.
	usesFor string
	// templates are the selected NPC's or zone's templates to apply or take
	// off, and takeOff takes the selected applied template off; path is its
	// path name, for the row pathOf.
	templates []*ui.Button
	takeOff   *ui.Button
	path      *ui.TextField
	pathOf    string
	// dialog asks before a delete, or says why it cannot be; deleting is
	// what it would delete.
	dialog   *ui.Dialog
	deleting *deletion
	moving   *pendingMove
	// menu is the right-click menu on a row. revealPath is the path of a
	// row to select once the tree shows it, for a move or for Find, and
	// revealRename puts its name to type.
	menu         ui.Menu
	menuOf       string
	revealPath   string
	revealRename bool
	// showing is a constant asked for by a search, to show once the tree
	// is loaded; revealID is a row to select once the tree shows it.
	showing   string
	revealID  string
	nameAdded bool
	// find is Find a value while it is open, with findButton opening it;
	// findUses counts each value's uses, once answered; lastGroup is the
	// group last opened in it.
	find       *valuepicker.Picker
	findButton *ui.Button
	findUses   map[string]int
	lastGroup  string
	// signal is the selected signal's name, line and Delete.
	signal signalFields
	// held is an NPC's or zone's own value of a template's constant, as the
	// Constants window edits it, and holders the rows listing who holds a
	// template's constant, by where each is drawn.
	held    heldConstant
	holders map[string]image.Rectangle
	// sheet is the writing sheet while it is open, writing a text constant
	// declared in sheetIn.
	sheet   *writesheet.Sheet
	sheetOf ids.ULID
	sheetIn valuetree.List
}

// New returns the Values window for the project's world state.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	return newState(PartValues, window, document, theme)
}

// NewSignals returns the Signals window for the project's world state.
func NewSignals(window, document ids.ULID, theme *ui.Theme) *State {
	return newState(PartSignals, window, document, theme)
}

// NewConstants returns the Constants window for the project's world state.
func NewConstants(window, document ids.ULID, theme *ui.Theme) *State {
	return newState(PartConstants, window, document, theme)
}

// newState returns a World state window showing one part.
func newState(part Part, window, document ids.ULID, theme *ui.Theme) *State {
	adds := []*ui.Button{
		ui.NewButton(addValueID, "Add value"), ui.NewButton(addGroupID, "Add group"),
		ui.NewButton(addTemplateID, "Add template"),
	}
	switch part {
	case PartSignals:
		adds = []*ui.Button{ui.NewButton(addSignalID, "Add signal"), ui.NewButton(addGroupID, "Add group")}
	case PartConstants:
		adds[0] = ui.NewButton(addConstantID, "Add constant")
	case PartValues:
	}
	adds[0].Primary = true
	view := &State{
		part: part, folded: false, window: window, document: document, theme: theme, size: image.Pt(0, 0),
		list:  listWithDetails(),
		shown: tree{rows: nil, items: nil, catalog: nil, signals: nil}, toggled: map[string]bool{}, subs: map[ids.ULID]bool{},
		adds: adds, form: valueform.New(theme), focus: ui.Focus{}, save: ui.NewSaveBar(), guard: frame.Guard{},
		named: "", uses: nil, usesOf: "", usesAsking: false, usesFor: "", templates: nil,
		takeOff: ui.NewButton(takeOffID, "Take it off"), dialog: nil, deleting: nil, moving: nil,
		menu:   ui.Menu{Items: nil, Width: 0},
		menuOf: "", revealPath: "", revealRename: false, showing: "", revealID: "", nameAdded: false, find: nil,
		findButton: ui.NewButton(findID, "Find a value"),
		findUses:   nil, lastGroup: "", signal: newSignalFields(theme), held: newHeldConstant(theme), holders: nil,
		sheet: nil, sheetOf: ids.ULID{}, sheetIn: valuetree.List{Document: ids.ULID{}, Edit: "", Values: nil},
		path: ui.NewTextField(pathID, textedit.NewField("", false, worldstate.MaxName), false, theme), pathOf: "",
	}
	view.path.Placeholder = "such as george or market"
	view.form.Dotted = true
	if part == PartConstants {
		view.findButton.Label = "Find a constant"
	}
	view.list.CanDrop = func(dragged, target string) bool {
		_, ok := view.dropInto(dragged, target)

		return ok
	}
	view.list.Faded, view.list.DragLabel = view.faded, view.dragLabel

	return view
}

// listWithDetails is the tree's list, its details close to the names.
func listWithDetails() *ui.List {
	list := ui.NewList("values", "")
	list.DetailAt = detailAt

	return list
}

// Title is the window title: World state, then the part it shows.
func (s *State) Title(frame.Docs) string { return "World state: " + s.heading() }

// heading names the part the window shows.
func (s *State) heading() string {
	switch s.part {
	case PartSignals:
		return signalsTopLabel
	case PartConstants:
		return "Constants"
	case PartValues:
	}

	return "Values"
}

// about says what the window holds, under its heading.
func (s *State) about() string {
	switch s.part {
	case PartSignals:
		return "Signals workflows send and answer. Only the zone running hears one; tell another with a value."
	case PartConstants:
		return "Words and numbers the game never changes: the world's, and each zone's and NPC's."
	case PartValues:
	}

	return "The world's values, and each zone's and NPC's. Templates give many the same values."
}

// Focused returns the focused text field.
// While Find is open, it is Find's.
func (s *State) Focused() *ui.TextField {
	if s.sheet != nil {
		return s.sheet.Focused()
	}
	if s.find != nil {
		return s.find.Focused()
	}

	return s.focus.Current()
}

// Start keeps the project index loaded, for the NPCs and zones.
func (s *State) Start(ctx *frame.Context) { s.subscribe(ctx, project.IndexID()) }

// Changed asks again where the selected value is used, as workflows may
// have changed.
func (s *State) Changed(update client.Update, _ *frame.Context) {
	s.usesOf = ""
	if update.Attention && s.part == PartConstants && strings.HasPrefix(update.Item, ConstantItem) {
		s.showing = update.Item
	}
}

func (s *State) subscribe(ctx *frame.Context, document ids.ULID) {
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

// refresh rebuilds the tree from the documents held, and keeps the
// inspector on the selected row.
func (s *State) refresh(docs frame.Docs) bool {
	built, loaded := s.build(docs)
	if !loaded {
		return false
	}
	s.shown, s.list.Rows = built, built.rows
	selected, found := s.selected()
	if !found {
		s.list.Selected, s.named = "", ""

		return true
	}
	s.inspect(selected)

	return true
}

// selected returns what the selected row means.
func (s *State) selected() (item, bool) {
	meaning, found := s.shown.items[s.list.Selected]

	return meaning, found && s.list.Selected != ""
}

// panel is the inspector's area: beside the list when the window has room
// for both, at the window's right edge when it has not or while the
// inspector is folded.
func (s *State) panel() image.Rectangle {
	edge := frame.Inspector(s.size)
	if s.folded || edge.Min.X <= listWidth {
		return edge
	}

	return image.Rect(listWidth, edge.Min.Y, listWidth+frame.InspectorWidth, edge.Max.Y)
}

// listRight is where the list and the toolbar above it end.
func (s *State) listRight() int {
	return min(frame.Inspector(s.size).Min.X, listWidth) - ui.Inset
}

func (s *State) layout() {
	content := frame.Content(s.size)
	panel := s.panel()
	right := s.listRight()
	for index, button := range s.adds {
		left := ui.Inset + index*(addWidth+ui.Gap)
		button.Rect = image.Rect(left, headerTop, left+addWidth, headerTop+ui.ButtonH)
	}
	s.list.Rect = image.Rect(0, listTop, right, content.Max.Y)
	s.layoutFind(right)
	s.layoutInspector(panel)
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size, s.folded = ctx.Size, ctx.InspectorFolded
	for _, document := range slices.Concat(valuetree.Documents(ctx.Docs), signals.Documents(ctx.Docs)) {
		s.subscribe(ctx, document)
	}
	loaded := s.refresh(ctx.Docs)
	s.layout()
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	s.askUses(ctx)
	if s.updateOverlay(input, ctx) {
		return
	}
	if s.guard.Active() {
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.form.Revert)

		return
	}
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field != nil && outcome == textedit.OutcomeCommit {
		s.Save(ctx)
	}
	frame.CommonKeys(input, ctx)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)

		return
	case ui.RevertID:
		s.focus.Set(nil)
		s.form.Revert()
		s.path.Field.Revert()
		s.held.revert()

		return
	}
	if !loaded {
		return
	}
	s.showFound(ctx)
	s.revealRow()
	s.nameNewRow()
	if selected, found := s.selected(); found && isSignal(selected.kind) {
		s.showSignal(ctx.Docs, selected)
	} else {
		s.signal.of = ids.ULID{}
	}
	if s.part != PartSignals && (s.findButton.Update(input) || input.Consume(ui.KeyF, ui.Cmd())) {
		s.openFind(ctx, "")

		return
	}
	s.updateTree(input, ctx)
	s.updateAdds(input, ctx)
	s.updateInspector(input, ctx)
}

// updateOverlay gives a frame's input to the dialog, the right-click menu
// or Find while one is open, and reports whether one was.
func (s *State) updateOverlay(input *ui.Input, ctx *frame.Context) bool {
	switch {
	case s.sheet != nil:
		s.updateSheet(input, ctx)
	case s.dialog != nil:
		s.updateDialog(input, ctx)
	case s.menu.IsOpen():
		s.chooseFromMenu(ctx, s.menu.Update(input))
	case s.find != nil:
		if !s.find.Update(input, ctx, s.screen()) {
			s.find = nil
		}
	default:
		return false
	}

	return true
}

// updateTree opens and closes rows and moves the selection, asking first
// about typed changes to the declaration being left.
func (s *State) updateTree(input *ui.Input, ctx *frame.Context) {
	selected := s.list.Selected
	result := s.list.Update(input)
	if result.Toggled != "" {
		s.toggled[result.Toggled] = !s.toggled[result.Toggled]
	}
	if result.Context {
		s.rightClick(input, ctx, result.ContextRow)
	}
	s.dropped(ctx, result)
	if s.list.Selected == selected {
		return
	}
	if unsaved := s.Unsaved(ctx.Docs); unsaved > 0 {
		wanted := s.list.Selected
		s.list.Selected = selected
		s.focus.Set(nil)
		s.guard.Ask(unsaved, s.UnsavedName(ctx.Docs), func() { s.list.Selected = wanted })
	}
}

// Unsaved counts the selected declaration's, or NPC's or zone's path
// name's, typed changes not saved yet.
func (s *State) Unsaved(frame.Docs) int {
	switch {
	case s.named != "":
		return s.form.Unsaved()
	case s.held.of != "":
		return frame.UnsavedFields(s.held.field())
	case s.pathOf != "":
		return frame.UnsavedFields(s.path)
	case !s.signal.of.IsZero():
		return frame.UnsavedFields(s.signal.name, s.signal.description)
	}

	return 0
}

// UnsavedName names the declaration with unsaved changes.
func (s *State) UnsavedName(frame.Docs) string {
	if meaning, found := s.shown.items[s.named]; found {
		return meaning.path
	}
	if meaning, found := s.shown.items[s.held.of]; found {
		return meaning.owner.Name + "'s " + meaning.declared.Name
	}
	if meaning, found := s.shown.items[s.pathOf]; found {
		return meaning.owner.Name + "'s path name"
	}
	if !s.signal.of.IsZero() {
		return "this signal"
	}

	return "this value"
}

// Save saves the selected declaration's typed changes as one change: a new
// dotted name renames it, or moves it into the groups the name gives,
// declaring any it needs. If a field cannot be read nothing is saved, that
// field takes focus and the status says why.
func (s *State) Save(ctx *frame.Context) bool {
	if s.Unsaved(ctx.Docs) == 0 {
		s.focus.Set(nil)

		return true
	}
	if !s.signal.of.IsZero() {
		return s.saveSignal(ctx)
	}
	if s.held.of != "" {
		return s.saveHeld(ctx)
	}
	if s.named == "" {
		return s.savePath(ctx)
	}
	meaning := s.shown.items[s.named]
	typed, dotted, field, problem := s.form.Typed()
	if problem != "" {
		s.focus.Set(field)
		ctx.Frame.Status(problem)

		return false
	}
	change, err := valuetree.Move(meaning.list.Values, typed, valuetree.Root(meaning.list.Values, typed.ID), dotted)
	if first, _, _ := strings.Cut(dotted, "."); err == nil && meaning.list.Document != worldstate.ID() {
		if clash := valuetree.Clash(s.shown.catalog, meaning.list.Document, first); clash != "" {
			err = fmt.Errorf("%w: %s", valuetree.ErrName, clash)
		}
	}
	if err != nil {
		s.focus.Set(s.form.Fields()[0])
		ctx.Frame.Status(strings.TrimPrefix(err.Error(), valuetree.ErrName.Error()+": "))

		return false
	}
	saving := []*ui.TextField{}
	for _, each := range s.form.Fields() {
		if each.Field.Unsaved() {
			each.Field.Commit()
			saving = append(saving, each)
		}
	}
	s.send(ctx, meaning.list.Document, meaning.list.Edit, change, func(result protocol.Result, err error) {
		if err != nil {
			for _, each := range saving {
				each.Field.Failed()
			}
			ctx.Frame.Report("")(result, err)

			return
		}
		s.focus.Set(nil)
		ctx.Frame.Status("Saved " + dotted)
	})

	return true
}

// Leave asks about unsaved typed changes before then, such as closing the
// window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

// Locate finds a named part of the window for input scripts: "ui:<id>" for
// Add value, Add group, Add template, the inspector's fields and buttons
// and "ui:template-<name>" for a template to apply; "row:<path>" for a row,
// "toggle:<path>" for its triangle and "section:<scope>" for a scope;
// "dialog:<button>"; "menu:<id>" for the right-click menu; and "state:loaded", "state:selected=<path>",
// "state:value=<path>:<type>:<default>", "state:applied=<owner path>:<template>"
// "state:says=<the open dialog's title or one of its lines>" or
// "state:adds=<where Add value adds>".
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	loaded := s.refresh(docs)
	s.layout()
	kind, value, _ := strings.Cut(name, ":")
	switch {
	case kind+":" == statePrefix:
		return image.Pt(0, 0), loaded && s.hasState(docs, value)
	case s.sheet != nil:
		return s.sheet.Locate(name)
	case s.dialog != nil:
		return s.dialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	case s.find != nil:
		return s.find.Locate(docs, s.screen(), kind, value)
	case kind == menuLocate:
		return s.menu.Locate(value)
	case s.guard.Active():
		return s.guard.Locate(value, s.size)
	case kind+":" == rowLocate, kind+":" == toggleLocate:
		for _, row := range s.list.Rows {
			if meaning, found := s.shown.items[row.ID]; found && meaning.path == value {
				if kind+":" == toggleLocate {
					return s.list.LocateToggle(row.ID)
				}

				return s.list.Locate(row.ID)
			}
		}

		return image.Pt(0, 0), false
	case kind+":" == sectionPrefix:
		return s.list.Locate(sectionPrefix + value)
	case kind == signalLocate:
		return s.locateSignal(value)
	}

	return s.locateWidget(strings.TrimPrefix(name, "ui:"))
}

func (s *State) locateWidget(widget string) (image.Point, bool) {
	if point, found := s.save.Locate(widget); found {
		return point, true
	}
	if widget == pathID && !s.path.Rect.Empty() {
		return s.path.Center(), true
	}
	for _, field := range []*ui.TextField{s.signal.name, s.signal.description, s.held.text, s.held.number} {
		if widget == field.ID && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	if widget == deleteSignalID && !s.signal.remove.Rect.Empty() {
		return ui.Center(s.signal.remove.Rect), true
	}
	buttons := append(append([]*ui.Button{s.findButton, s.held.useDefault, s.held.write}, s.adds...), s.templates...)
	for _, button := range append(buttons, s.takeOff) {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return s.form.Locate(widget)
}

func (s *State) hasState(docs frame.Docs, state string) bool {
	if found, handled := s.sheetState(state); handled {
		return found
	}
	if s.find != nil {
		if found, handled := s.find.HasState(docs, s.screen(), state); handled {
			return found
		}
	}
	if found, handled := hasSignalState(docs, state); handled {
		return found
	}
	switch {
	case state == stateLoaded:
		return true
	case state == findState:
		return s.find != nil
	case strings.HasPrefix(state, stateSelected):
		meaning, found := s.selected()

		return found && meaning.path == strings.TrimPrefix(state, stateSelected)
	case strings.HasPrefix(state, stateValue):
		return s.hasValue(strings.TrimPrefix(state, stateValue))
	case strings.HasPrefix(state, stateApplied):
		return s.applied(strings.TrimPrefix(state, stateApplied))
	case strings.HasPrefix(state, stateAdds):
		return s.addsNote(docs) == strings.TrimPrefix(state, stateAdds)
	case strings.HasPrefix(state, stateSays) && s.dialog != nil:
		return slices.ContainsFunc(append([]string{s.dialog.Title}, s.dialog.Lines...), func(line string) bool {
			return strings.TrimSpace(line) == strings.TrimPrefix(state, stateSays)
		})
	}

	return false
}

// hasValue answers "<path>:<type>:<default>": whether a value shown in the
// tree is one, or for an NPC's or zone's constant its own value or the
// default.
func (s *State) hasValue(asked string) bool {
	for _, meaning := range s.shown.items {
		declared := meaning.declared
		value := declared.Zero()
		if meaning.kind == itemHeld && declared.IsConstant() {
			value, _ = ownValue(declared, meaning.owner)
		}
		if declared.Type.IsValue() && !declared.ID.IsZero() &&
			meaning.path+":"+string(declared.Type)+":"+declared.Label(value) == asked {
			return true
		}
	}

	return false
}

// applied answers "<owner path>:<template>": whether the NPC or zone has
// the template, shown or not.
func (s *State) applied(asked string) bool {
	catalog := s.shown.catalog
	for _, scope := range []worldstate.Scope{worldstate.ScopeNPC, worldstate.ScopeZone} {
		for _, ownerID := range catalog.Owners(scope) {
			owner, _ := catalog.Owner(ownerID)
			for _, template := range owner.Owned.Templates {
				declared, found := catalog.World().Value(template)
				if found && ownerPath(owner, ownerID)+":"+declared.Name == asked {
					return true
				}
			}
		}
	}

	return false
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	loaded := s.refresh(docs)
	s.layout()
	screen.Fill(ui.ColorBackground())
	s.theme.DrawTitle(screen, "World state › "+s.heading(), ui.Inset, ui.Inset, ui.ColorInk())
	s.theme.DrawText(screen, s.theme.FitText(s.about(), s.listRight()-ui.Inset), ui.Inset, ui.Inset+ui.LineHeight,
		ui.ColorMuted())
	for _, button := range s.adds {
		button.Draw(screen, s.theme)
	}
	right := s.listRight()
	if s.part != PartSignals {
		s.findButton.Draw(screen, s.theme)
		right = s.findButton.Rect.Min.X
	}
	if where := s.addsNote(docs); loaded && where != "" {
		last := s.adds[len(s.adds)-1].Rect
		room := right - last.Max.X - ui.Half*ui.Gap*ui.Half
		s.theme.DrawText(screen, s.theme.FitText(where, room), last.Max.X+ui.Half*ui.Gap,
			last.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
	}
	if loaded {
		s.list.Draw(screen, s.theme)
	}
	s.drawInspector(screen, docs)
	s.menu.Draw(screen, s.theme)
	s.guard.Draw(screen, s.theme)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
	if s.find != nil {
		s.find.Draw(screen, docs, s.screen())
	}
	if s.sheet != nil {
		s.sheet.Draw(screen, s.screen())
	}
}

// screen is the whole window, where dialogs are centred.
func (s *State) screen() image.Rectangle { return image.Rectangle{Min: image.Pt(0, 0), Max: s.size} }
