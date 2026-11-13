package workflowview

import (
	"encoding/json"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/editor/views/valueform"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// While no node or line is selected, the inspector shows the workflow's own
// local values: a list of them, Add local, and the selected one's form.
const (
	localsShown   = 5
	addLocalWidth = 120
	// localPrefix names a local's row for input scripts, "local:<name>",
	// and localState its state, "state:local=<name>:<type>:<starts as>".
	localPrefix = "local"
	localState  = "local="
)

// localsPanel is the inspector's part for the workflow's local values.
type localsPanel struct {
	list *ui.List
	add  *ui.Button
	form *valueform.Form
	// top is where the panel's heading goes; registered is whether the
	// form's fields take part in focus. A form taller than the room above
	// the save bar scrolls by scroll pixels inside shown.
	top        int
	registered bool
	scroll     int
	shown      image.Rectangle
}

func newLocalsPanel(theme *ui.Theme) *localsPanel {
	return &localsPanel{
		list: ui.NewList("locals", ""),
		add:  ui.NewButton("add-local", "Add local"), form: valueform.New(theme), top: 0, registered: false,
		scroll: 0, shown: ui.Zero(),
	}
}

// localsActive reports whether the inspector shows the local values.
func (s *State) localsActive() bool {
	return s.workflow != nil && s.form == nil && !s.lineSelected() && len(s.group) == 0
}

// selectedLocal returns the selected local value.
func (s *State) selectedLocal() (worldstate.Declaration, bool) {
	selected, err := ids.Parse(s.locals.list.Selected)
	if err != nil || s.workflow == nil {
		return worldstate.New(ids.ULID{}, "", "", ""), false
	}

	return s.workflow.Locals().Value(selected)
}

// syncLocals lists the local values and keeps the form on the selected one,
// its fields taking part in focus.
func (s *State) syncLocals(ctx *frame.Context) {
	panel := s.locals
	if !s.localsActive() {
		panel.registered = false

		return
	}
	panel.list.Rows = nil
	for _, value := range s.workflow.Locals().Values() {
		row := ui.NewRow(value.ID.String(), localPath(value))
		row.Detail = value.Type.Label()
		if value.Type == worldstate.TypePlace {
			row.Icon = ui.IconPlace
		}
		if _, out := s.localProblems[value.ID]; out {
			row.Detail += " · out of reach"
		}
		panel.list.Rows = append(panel.list.Rows, row)
	}
	value, ok := s.selectedLocal()
	if !ok {
		panel.list.Selected, panel.registered = "", false

		return
	}
	panel.form.PlaceName, panel.form.Locked = s.placeName(ctx, value), s.lockLocal(value)
	panel.form.PlaceWhere, panel.form.PlaceLit, panel.form.PlaceOpen = "", s.isLit(localOwner(value.ID)),
		s.placesOpen(localOwner(value.ID))
	if start := value.Zero().Place; value.Type == worldstate.TypePlace && !start.IsZero() {
		panel.form.PlaceWhere = places.Where(ctx.Docs, start.Floor)
	}
	problem, out := s.localProblems[value.ID]
	if out {
		panel.form.PlaceWhere = problem
	}
	panel.form.PlaceWarn = out
	if panel.form.Show(value, value.Name) || !panel.registered {
		s.focus = ui.Focus{}
		s.focus.Register(panel.form.Fields()...)
		panel.registered = true
	}
}

// layoutLocals places the local values' list, Add local and the form, with
// Delete value and the save bar at the inspector's foot.
func (s *State) layoutLocals() {
	panel := s.locals
	panel.list.Rect, panel.add.Rect, panel.form.Delete.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	s.runsInBox = ui.Zero()
	if !s.localsActive() {
		return
	}
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	panel.top = s.layoutRunsIn(inspector.Min.Y + ui.Inset)
	top := panel.top + ui.Half*ui.LineHeight
	rows := min(max(len(panel.list.Rows), 1), localsShown)
	panel.list.Rect = image.Rect(left, top, left+width, top+rows*ui.RowHeight)
	top = panel.list.Rect.Max.Y + rowSpace
	panel.add.Rect = image.Rect(left, top, left+addLocalWidth, top+ui.ButtonH)
	top += ui.ButtonH + fieldSpace
	s.save.Rect = ui.Zero()
	if _, ok := s.selectedLocal(); !ok {
		return
	}
	bottom := inspector.Max.Y - ui.Inset
	panel.form.Delete.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
	bottom = panel.form.Delete.Rect.Min.Y - ui.Half*ui.Gap
	s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
	panel.shown = image.Rect(inspector.Min.X, top-ui.Gap, inspector.Max.X, s.save.Rect.Min.Y-ui.Gap)
	tall := panel.form.Layout(left, top, width) - top
	panel.scroll = max(0, min(panel.scroll, tall-panel.shown.Dy()+ui.Gap))
	if panel.scroll > 0 {
		panel.form.Layout(left, top-panel.scroll, width)
	}
}

// scrollLocal scrolls a form taller than its room under the pointer, and
// reports whether a press falls outside the room, where the form is hidden,
// other than on Delete value below it.
func (s *State) scrollLocal(input *ui.Input) bool {
	panel := s.locals
	if input.Cursor.In(panel.shown) && input.WheelY != 0 {
		panel.scroll -= int(input.WheelY * ui.RowHeight)
		s.layoutLocals()
	}

	return input.Pressed && !input.Cursor.In(panel.shown) && !input.Cursor.In(panel.form.Delete.Rect)
}

// updateLocals handles the Runs in box, the local values' list, Add local
// and the form's buttons.
func (s *State) updateLocals(input *ui.Input, ctx *frame.Context) {
	if !s.localsActive() || s.updateRunsIn(input, ctx) {
		return
	}
	panel := s.locals
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)

		return
	case ui.RevertID:
		s.discardForm()

		return
	}
	before := panel.list.Selected
	panel.list.Update(input)
	if panel.list.Selected != before {
		if unsaved := panel.form.Unsaved(); unsaved > 0 && before != "" {
			wanted := panel.list.Selected
			panel.list.Selected = before
			s.focus.Set(nil)
			s.guard.Ask(unsaved, s.formName(), func() { panel.list.Selected = wanted })
		}

		return
	}
	if panel.add.Update(input) {
		s.addLocalAsking(ctx)

		return
	}
	s.updateLocalForm(input, ctx)
}

// addLocalAsking adds a local, which the form then shows, once the author
// has decided about the selected local's unsaved typed changes.
func (s *State) addLocalAsking(ctx *frame.Context) {
	s.focus.Set(nil)
	s.guard.Ask(s.locals.form.Unsaved(), s.formName(), func() { s.addLocal(ctx) })
}

// updateLocalForm handles the selected local's form.
func (s *State) updateLocalForm(input *ui.Input, ctx *frame.Context) {
	panel := s.locals
	declared, ok := s.selectedLocal()
	if !ok {
		return
	}
	panel.form.PickPlace.Label = s.picker.Label(localOwner(declared.ID))
	if s.scrollLocal(input) {
		return
	}
	switch changed, change := panel.form.Update(input); change {
	case valueform.ChangeValue:
		s.command(ctx, workflowplugin.ChangeLocalsEdit, "", localChange(nil, changed, ids.ULID{}), nil)
	case valueform.ChangeDelete:
		removal := localChange(nil, worldstate.New(ids.ULID{}, "", "", ""), changed.ID)
		s.command(ctx, workflowplugin.ChangeLocalsEdit, "", removal,
			func(protocol.Result) { ctx.Frame.Status(localPath(changed) + " deleted. Undo brings it back.") })
	case valueform.ChangePickPlace:
		s.pickOrCancel(ctx, s.localTarget(ctx, changed))
	case valueform.ChangeChoosePlace:
		s.openPlaces(ctx.Docs, panel.form.PlaceBox, s.localTarget(ctx, changed))
	case valueform.ChangeDeclareNew:
		s.addLocalAsking(ctx)
	case valueform.ChangeNone, valueform.ChangeWrite:
	}
}

// placeName names a place local's start, its anchor's name, loading its
// floor; "" is no place yet.
func (s *State) placeName(ctx *frame.Context, declared worldstate.Declaration) string {
	start := declared.Zero()
	if declared.Type != worldstate.TypePlace || start.Place.IsZero() {
		return ""
	}
	s.subscribe(start.Place.Floor, ctx)
	name, _ := s.anchorLabel(ctx.Docs, floor.AnchorRef{Floor: start.Place.Floor, Anchor: start.Place.Anchor})

	return name
}

// localTarget is a place local's start, as a place control.
func (s *State) localTarget(ctx *frame.Context, declared worldstate.Declaration) placeTarget {
	start := declared.Zero().Place
	current := floor.AnchorRef{Floor: start.Floor, Anchor: start.Anchor}
	label := localPath(declared) + " in “" + s.name(ctx.Docs) + "”"
	localID := declared.ID

	return s.newPlaceTarget(localOwner(localID), current, localPath(declared), label,
		func(ctx *frame.Context, ref floor.AnchorRef) {
			now, ok := s.workflow.Locals().Value(localID)
			if !ok {
				return
			}
			value := worldstate.NoValue()
			value.Type, value.Place = worldstate.TypePlace, worldstate.Place{Floor: ref.Floor, Anchor: ref.Anchor}
			now.Default = value.Encode()
			s.command(ctx, workflowplugin.ChangeLocalsEdit, "", localChange(nil, now, ids.ULID{}), nil)
		})
}

// localPath is how a workflow names a local value, such as local.clerk.
func localPath(declared worldstate.Declaration) string {
	return string(worldstate.ScopeLocal) + "." + declared.Name
}

// localChange is a change to a workflow's local values: one to add, one to
// set, or one to remove.
func localChange(added *worldstate.Declaration, set worldstate.Declaration, removed ids.ULID) stateplugin.ChangeArgs {
	change := stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: nil}
	switch {
	case added != nil:
		change.Add = []worldstate.Declaration{*added}
	case !removed.IsZero():
		change.Remove = []ids.ULID{removed}
	default:
		change.Set = []worldstate.Declaration{set}
	}

	return change
}

// addLocal declares a local integer, named so no local clashes, and selects
// it.
func (s *State) addLocal(ctx *frame.Context) {
	taken := map[string]bool{}
	for _, value := range s.workflow.Locals().Values() {
		taken[value.Name] = true
	}
	name := "value"
	for number := 2; taken[name]; number++ {
		name = "value_" + strconv.Itoa(number)
	}
	made := worldstate.New(stateplugin.TempID(1), name, worldstate.ScopeLocal, worldstate.TypeInteger)
	added := func(result protocol.Result) {
		s.locals.list.Selected = result.Created.String()
		ctx.Frame.Status("Added " + localPath(made) + ". Name it below.")
	}
	s.command(ctx, workflowplugin.ChangeLocalsEdit, "", localChange(&made, made, ids.ULID{}), added)
}

// saveLocal saves the selected local's typed changes as one change, or says
// what is wrong with them.
func (s *State) saveLocal(ctx *frame.Context) bool {
	panel := s.locals
	if panel.form.Unsaved() == 0 {
		s.focus.Set(nil)

		return true
	}
	typed, name, field, problem := panel.form.Typed()
	if problem != "" {
		s.focus.Set(field)
		ctx.Frame.Status(problem)

		return false
	}
	typed.Name = name
	saving := []*ui.TextField{}
	for _, each := range panel.form.Fields() {
		if each.Field.Unsaved() {
			each.Field.Commit()
			saving = append(saving, each)
		}
	}
	data, err := json.Marshal(localChange(nil, typed, ids.ULID{}))
	if err != nil {
		ctx.Frame.Status(err.Error())

		return false
	}
	command := protocol.Command{
		Document: s.document, Edit: workflowplugin.ChangeLocalsEdit, Gesture: "", Args: data, Says: "",
	}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			for _, each := range saving {
				each.Field.Failed()
			}
			ctx.Frame.Report("")(result, err)

			return
		}
		s.focus.Set(nil)
		ctx.Frame.Status("Saved " + localPath(typed))
	})

	return true
}

// drawLocals draws where the workflow runs and the local values' part of
// the inspector.
func (s *State) drawLocals(screen *ebiten.Image, docs frame.Docs) {
	s.drawRunsIn(screen, docs)
	panel := s.locals
	left := frame.Inspector(s.size).Min.X + ui.Inset
	s.theme.Heading(screen, "Local values", left, panel.top)
	s.theme.DrawText(screen, "Only this workflow reads and sets them.", left, panel.top+ui.LineHeight, ui.ColorMuted())
	if len(panel.list.Rows) == 0 {
		s.theme.DrawText(screen, "None yet.", left+ui.Gap, panel.list.Rect.Min.Y+ui.Gap, ui.ColorMuted())
	}
	panel.list.Draw(screen, s.theme)
	panel.add.Draw(screen, s.theme)
	if _, ok := s.selectedLocal(); !ok {
		s.drawHowToSelect(screen, left, panel.add.Rect.Max.Y+ui.Gap+ui.LineHeight)

		return
	}
	if shown, ok := screen.SubImage(panel.shown).(*ebiten.Image); ok && !panel.shown.Empty() {
		panel.form.Draw(shown)
	}
	s.save.Draw(screen, s.theme)
	panel.form.Delete.Draw(screen, s.theme)
}

// drawHowToSelect says how to select or add a node, from top down.
func (s *State) drawHowToSelect(screen *ebiten.Image, left, top int) {
	width := frame.Inspector(s.size).Dx() - ui.Half*ui.Inset
	for index, line := range s.wrap("Select a node to change it, or several with Cmd+click or a box dragged on empty "+
		"canvas. To add one, drag it from the palette onto the canvas; click it there to read what it does first.",
		width) {
		s.theme.DrawText(screen, line, left, top+index*ui.LineHeight, ui.ColorMuted())
	}
}

// locateLocal finds a local's row by its name, "local:<name>".
func (s *State) locateLocal(name string) (image.Point, bool) {
	if s.workflow == nil {
		return image.Pt(0, 0), false
	}
	for _, value := range s.workflow.Locals().Values() {
		if value.Name == name {
			return s.locals.list.Locate(value.ID.String())
		}
	}

	return image.Pt(0, 0), false
}

// locateLocalsWidget finds Add local or a field or button of the local's
// form.
func (s *State) locateLocalsWidget(widget string) (image.Point, bool) {
	if !s.localsActive() {
		return image.Pt(0, 0), false
	}
	if widget == s.locals.add.ID {
		return ui.Center(s.locals.add.Rect), true
	}
	if point, found := s.save.Locate(widget); found {
		return point, true
	}

	return s.locals.form.Locate(widget)
}

// hasLocal answers "local=<name>:<type>:<starts as>".
func (s *State) hasLocal(state string) bool {
	if s.workflow == nil {
		return false
	}
	for _, value := range s.workflow.Locals().Values() {
		if value.Name+":"+string(value.Type)+":"+value.Label(value.Zero()) == strings.TrimPrefix(state, localState) {
			return true
		}
	}

	return false
}
