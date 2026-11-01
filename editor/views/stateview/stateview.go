// Package stateview is the World state window: the values a project
// declares for the world, for each zone and for each NPC. It lists the
// scopes, each scope's values with their type and what they start as, and
// edits the selected value in the inspector: its name, type, choices,
// default and description, and where it is used. Every change is saved as it
// is made and undone with Cmd+Z; typed text waits for Enter or Save.
//
// A workflow's local values are declared on the workflow itself, in its
// Workflow window.
package stateview

import (
	"encoding/json"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valueform"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// ViewName is the World state window's view name; its target is the
// project's world-state document.
const ViewName = "world-state"

// Layout, in screen pixels.
const (
	scopesWidth = 200
	addWidth    = 120
	headerTop   = ui.Inset + ui.LineHeight + ui.LineHeight
	listTop     = headerTop + ui.ButtonH + ui.Gap
	footerGap   = 10
	usesShown   = 6
)

// Locate names for input scripts.
const (
	addID         = "add-value"
	scopePrefix   = "scope:"
	rowPrefix     = "row:"
	statePrefix   = "state:"
	stateLoaded   = "loaded"
	stateSelected = "selected="
	stateValue    = "value="
)

// State is the World state window.
type State struct {
	window   ids.ULID
	document ids.ULID
	theme    *ui.Theme
	size     image.Point
	scopes   *ui.List
	values   *ui.List
	add      *ui.Button
	form     *valueform.Form
	focus    ui.Focus
	save     *ui.SaveBar
	guard    frame.Guard
	// named is the value whose typed changes the form holds; uses and
	// usesOf are where the selected value is used, as last asked.
	named      ids.ULID
	uses       []protocol.Use
	usesOf     string
	usesAsking bool
}

// New returns the World state window for the project's world state.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, document: document, theme: theme, size: image.Pt(0, 0),
		scopes: &ui.List{ID: "scopes", Rect: ui.Zero(), Rows: nil, Selected: string(worldstate.ScopeWorld), CanDrop: nil},
		values: &ui.List{ID: "values", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil},
		add:    ui.NewButton(addID, "Add value"), form: valueform.New(theme), focus: ui.Focus{},
		save: ui.NewSaveBar(), guard: frame.Guard{}, named: ids.ULID{}, uses: nil, usesOf: "", usesAsking: false,
	}
	view.add.Primary = true

	return view
}

// Title is the window title.
func (s *State) Title(frame.Docs) string { return "World state" }

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Changed asks again where the selected value is used, as workflows may
// have changed.
func (s *State) Changed(client.Update, *frame.Context) { s.usesOf = "" }

// state reads the window's copy of the world state.
func (s *State) state(docs frame.Docs) (*worldstate.Document, bool) {
	var found *worldstate.Document
	docs.View(s.document, func(document client.Document) {
		if stateDoc, ok := document.Model.(*stateplugin.Document); ok {
			found = stateDoc.State.Clone()
		}
	})

	return found, found != nil
}

func (s *State) scope() worldstate.Scope { return worldstate.Scope(s.scopes.Selected) }

// inScope lists a scope's values, in the order they were made.
func inScope(state *worldstate.Document, scope worldstate.Scope) []worldstate.Declaration {
	var listed []worldstate.Declaration
	for _, value := range state.Values.Values() {
		if value.Scope == scope {
			listed = append(listed, value)
		}
	}

	return listed
}

// selectedValue returns the selected value.
func (s *State) selectedValue(state *worldstate.Document) (worldstate.Declaration, bool) {
	selected, err := ids.Parse(s.values.Selected)
	if err != nil || state == nil {
		return worldstate.Declaration{
			ID: ids.ULID{}, Name: "", Scope: "", Type: "", Choices: nil, Default: nil, Description: "",
		}, false
	}

	return state.Values.Value(selected)
}

// refresh rebuilds the lists from the world state and keeps the form on the
// selected value.
func (s *State) refresh(state *worldstate.Document) {
	s.scopes.Rows = nil
	for _, scope := range worldstate.Scopes() {
		row := ui.NewRow(string(scope), scope.Label())
		row.Detail = strconv.Itoa(len(inScope(state, scope)))
		s.scopes.Rows = append(s.scopes.Rows, row)
	}
	s.values.Rows = nil
	for _, value := range inScope(state, s.scope()) {
		row := ui.NewRow(value.ID.String(), value.Path())
		row.Detail = value.Type.Label() + " · starts as " + value.Label(value.Zero())
		s.values.Rows = append(s.values.Rows, row)
	}
	value, ok := s.selectedValue(state)
	if !ok || value.Scope != s.scope() {
		s.values.Selected, s.named = "", ids.ULID{}

		return
	}
	if s.form.Show(value) {
		s.focus = ui.Focus{}
		s.focus.Register(s.form.Fields()...)
	}
	s.named = value.ID
}

func (s *State) layout() {
	content := frame.Content(s.size)
	s.scopes.Rect = image.Rect(0, ui.Inset+ui.LineHeight+ui.Gap, scopesWidth, content.Max.Y)
	left := scopesWidth + ui.Inset
	s.add.Rect = image.Rect(content.Max.X-ui.Inset-addWidth, headerTop, content.Max.X-ui.Inset, headerTop+ui.ButtonH)
	s.values.Rect = image.Rect(left, listTop, content.Max.X-ui.Inset, content.Max.Y-ui.Inset)
	panel := frame.Inspector(s.size)
	formLeft, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	bottom := panel.Max.Y - ui.Inset
	s.form.Delete.Rect, s.save.Rect = ui.Zero(), ui.Zero()
	if s.named.IsZero() {
		return
	}
	s.form.Delete.Rect = image.Rect(formLeft, bottom-ui.ButtonH, formLeft+width, bottom)
	saveTop := s.form.Delete.Rect.Min.Y - footerGap - ui.SaveBarHeight(width)
	s.save.Rect = image.Rect(formLeft, saveTop, formLeft+width, saveTop+ui.SaveBarHeight(width))
	s.form.Layout(formLeft, formTop(panel), width)
}

// formTop is where the form starts, under the selected value's path.
func formTop(panel image.Rectangle) int {
	return panel.Min.Y + ui.Inset + ui.Half*ui.LineHeight + ui.Gap
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	state, loaded := s.state(ctx.Docs)
	if loaded {
		s.refresh(state)
	}
	s.layout()
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	s.askUses(ctx)
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

		return
	}
	if !loaded {
		return
	}
	s.updateLists(input, ctx)
	if s.add.Update(input) {
		s.addValue(ctx, state)

		return
	}
	if s.named.IsZero() {
		return
	}
	switch changed, change := s.form.Update(input); change {
	case valueform.ChangeValue:
		s.send(ctx, stateplugin.SetValueEdit, stateplugin.ValueArgs{Value: changed}, "", nil)
	case valueform.ChangeDelete:
		s.send(ctx, stateplugin.RemoveValueEdit, stateplugin.RemoveValueArgs{ID: changed.ID},
			changed.Path()+" deleted. Undo brings it back.", nil)
	case valueform.ChangeNone:
	}
}

// updateLists moves between scopes and values, asking first about typed
// changes to the value being left.
func (s *State) updateLists(input *ui.Input, ctx *frame.Context) {
	scope, value := s.scopes.Selected, s.values.Selected
	s.scopes.Update(input)
	if s.scopes.Selected != scope {
		s.values.Selected = ""
	}
	s.values.Update(input)
	if s.values.Selected == value && s.scopes.Selected == scope {
		return
	}
	if unsaved := s.Unsaved(ctx.Docs); unsaved > 0 {
		wantedScope, wantedValue := s.scopes.Selected, s.values.Selected
		s.scopes.Selected, s.values.Selected = scope, value
		s.focus.Set(nil)
		s.guard.Ask(unsaved, s.UnsavedName(ctx.Docs), func() {
			s.scopes.Selected, s.values.Selected = wantedScope, wantedValue
		})
	}
}

// addValue declares a number in the scope shown, named so no value of the
// scope clashes, and selects it.
func (s *State) addValue(ctx *frame.Context, state *worldstate.Document) {
	taken := map[string]bool{}
	for _, value := range inScope(state, s.scope()) {
		taken[value.Name] = true
	}
	name := "value"
	for number := 2; taken[name]; number++ {
		name = "value_" + strconv.Itoa(number)
	}
	made := worldstate.New(ids.ULID{}, name, s.scope(), worldstate.TypeNumber)
	success := "Added " + made.Path() + ". Name it in the inspector."
	s.send(ctx, stateplugin.AddValueEdit, stateplugin.ValueArgs{Value: made}, success,
		func(result protocol.Result) { s.values.Selected = result.Created.String() })
}

// send sends an edit to the world state.
func (s *State) send(ctx *frame.Context, edit string, args any, success string, done func(protocol.Result)) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: s.document, Edit: edit, Gesture: "", Args: data}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		ctx.Frame.Report(success)(result, err)
		if err == nil && done != nil {
			done(result)
		}
	})
}

// askUses asks where the selected value is used, once per value and change.
func (s *State) askUses(ctx *frame.Context) {
	if s.named.IsZero() || s.usesAsking || s.usesOf == s.named.String() {
		return
	}
	asked := s.named.String()
	s.usesAsking = true
	ctx.Requests.WhereUsed(s.document, asked, func(uses []protocol.Use, err error) {
		s.usesAsking = false
		if err == nil {
			s.uses, s.usesOf = uses, asked
		}
	})
}

// Unsaved counts the selected value's typed changes not saved yet.
func (s *State) Unsaved(frame.Docs) int {
	if s.named.IsZero() {
		return 0
	}

	return s.form.Unsaved()
}

// UnsavedName names the value with unsaved changes.
func (s *State) UnsavedName(frame.Docs) string {
	if s.named.IsZero() {
		return "this value"
	}

	return s.form.Shown().Path()
}

// Save saves the selected value's typed changes as one change. If a field
// cannot be read nothing is saved, that field takes focus and the status
// says why.
func (s *State) Save(ctx *frame.Context) bool {
	if s.Unsaved(ctx.Docs) == 0 {
		s.focus.Set(nil)

		return true
	}
	typed, field, problem := s.form.Typed()
	if problem != "" {
		s.focus.Set(field)
		ctx.Frame.Status(problem)

		return false
	}
	saving := []*ui.TextField{}
	for _, each := range s.form.Fields() {
		if each.Field.Unsaved() {
			each.Field.Commit()
			saving = append(saving, each)
		}
	}
	data, err := json.Marshal(stateplugin.ValueArgs{Value: typed})
	if err != nil {
		ctx.Frame.Status(err.Error())

		return false
	}
	command := protocol.Command{Document: s.document, Edit: stateplugin.SetValueEdit, Gesture: "", Args: data}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			for _, each := range saving {
				each.Field.Failed()
			}
			ctx.Frame.Report("")(result, err)

			return
		}
		s.focus.Set(nil)
		ctx.Frame.Status("Saved " + typed.Path())
	})

	return true
}

// Leave asks about unsaved typed changes before then, such as closing the
// window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

// Locate finds a named part of the window for input scripts: "ui:add-value"
// and the inspector's fields and buttons, "scope:<world|zone|npc>",
// "row:<path>" for a value, and "state:loaded", "state:selected=<path>" or
// "state:value=<path>:<type>:<starts as>".
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	state, loaded := s.state(docs)
	if loaded {
		s.refresh(state)
	}
	s.layout()
	if s.guard.Active() {
		_, value, _ := strings.Cut(name, ":")

		return s.guard.Locate(value, s.size)
	}
	switch {
	case strings.HasPrefix(name, statePrefix):
		return image.Pt(0, 0), loaded && s.hasState(strings.TrimPrefix(name, statePrefix), state)
	case strings.HasPrefix(name, scopePrefix):
		return s.scopes.Locate(strings.TrimPrefix(name, scopePrefix))
	case strings.HasPrefix(name, rowPrefix) && loaded:
		for _, value := range state.Values.Values() {
			if value.Path() == strings.TrimPrefix(name, rowPrefix) {
				return s.values.Locate(value.ID.String())
			}
		}

		return image.Pt(0, 0), false
	}
	widget := strings.TrimPrefix(name, "ui:")
	if point, found := s.save.Locate(widget); found {
		return point, true
	}
	if widget == addID {
		return ui.Center(s.add.Rect), true
	}

	return s.form.Locate(widget)
}

func (s *State) hasState(state string, document *worldstate.Document) bool {
	switch {
	case state == stateLoaded:
		return true
	case strings.HasPrefix(state, stateSelected):
		value, ok := s.selectedValue(document)

		return ok && value.Path() == strings.TrimPrefix(state, stateSelected)
	case strings.HasPrefix(state, stateValue):
		for _, value := range document.Values.Values() {
			shown := value.Path() + ":" + string(value.Type) + ":" + value.Label(value.Zero())
			if shown == strings.TrimPrefix(state, stateValue) {
				return true
			}
		}
	}

	return false
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	state, loaded := s.state(docs)
	if loaded {
		s.refresh(state)
	}
	s.layout()
	screen.Fill(ui.ColorBackground())
	s.theme.Heading(screen, "SCOPES", ui.Inset, ui.Inset)
	s.scopes.Draw(screen, s.theme)
	left := scopesWidth + ui.Inset
	s.theme.DrawTitle(screen, s.scope().Label()+" values", left, ui.Inset, ui.ColorInk())
	s.theme.DrawText(screen, scopeHint(s.scope()), left, headerTop+ui.LineHeight/ui.Half, ui.ColorMuted())
	s.add.Draw(screen, s.theme)
	if loaded && len(s.values.Rows) == 0 {
		s.theme.DrawText(screen, "No "+strings.ToLower(s.scope().Label())+" values yet. Add value declares one.",
			left, listTop, ui.ColorMuted())
	}
	s.values.Draw(screen, s.theme)
	s.drawInspector(screen)
	s.guard.Draw(screen, s.theme)
}

// scopeHint says whose a scope's values are.
func scopeHint(scope worldstate.Scope) string {
	switch scope {
	case worldstate.ScopeWorld:
		return "One for the whole game."
	case worldstate.ScopeZone:
		return "One in each zone. A workflow reads and sets its own zone's."
	case worldstate.ScopeNPC:
		return "One on each NPC, wherever it is placed. A workflow sets its own NPC's."
	case worldstate.ScopeLocal:
	}

	return ""
}

func (s *State) drawInspector(screen *ebiten.Image) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	left := panel.Min.X + ui.Inset
	if s.named.IsZero() {
		s.theme.Heading(screen, "INSPECTOR", left, panel.Min.Y+ui.Inset)
		s.theme.DrawText(screen, "Select a value to change it.", left, panel.Min.Y+ui.Inset+ui.LineHeight,
			ui.ColorMuted())

		return
	}
	s.theme.Heading(screen, "SELECTED VALUE", left, panel.Min.Y+ui.Inset)
	s.theme.DrawMonoStrong(screen, s.form.Shown().Path(), left, panel.Min.Y+ui.Inset+ui.LineHeight, ui.ColorInk())
	s.form.Draw(screen)
	top := s.form.Layout(left, formTop(panel), panel.Dx()-ui.Half*ui.Inset)
	s.theme.Heading(screen, "USED IN", left, top)
	top += ui.LineHeight
	switch {
	case s.usesOf != s.named.String():
		s.theme.DrawText(screen, "…", left, top, ui.ColorMuted())
	case len(s.uses) == 0:
		s.theme.DrawText(screen, "Nothing uses it yet.", left, top, ui.ColorMuted())
	default:
		for index, use := range s.uses {
			if index == usesShown {
				s.theme.DrawText(screen, "and "+strconv.Itoa(len(s.uses)-usesShown)+" more", left, top, ui.ColorMuted())

				break
			}
			s.theme.DrawText(screen, s.theme.FitText(use.Name+" · "+use.Label, panel.Dx()-ui.Half*ui.Inset), left, top,
				ui.ColorInk())
			top += ui.LineHeight
		}
	}
	s.save.Draw(screen, s.theme)
	s.form.Delete.Draw(screen, s.theme)
}
