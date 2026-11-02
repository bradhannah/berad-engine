// Package valueform is the inspector for one declaration, shared by the
// World state window and a workflow's local values: a value's name, type,
// choices, limits, default and description, or a group's or template's name
// and description. The name can be dotted, as quests.dragon_slain, where the
// window allows groups. Buttons apply at once; typed fields wait for Save or
// Enter, as everywhere else in the editor.
package valueform

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Layout, in screen pixels.
const (
	fieldSpace     = 14
	rowSpace       = 6
	removeWidth    = 76
	descHeight     = 72
	choicesPerLine = 3
	maxNumberText  = 20
	// dottedLength bounds a dotted name: a name and its groups.
	dottedLength = (worldstate.MaxName + 1) * (worldstate.MaxDepth + 1)
)

// Locate names for input scripts: "ui:value-name", "ui:value-type-<type>",
// "ui:value-default" for an integer's or text's default,
// "ui:value-default-<choice id, true or false>", "ui:value-min",
// "ui:value-max", "ui:value-choice-<n>", "ui:value-choice-<n>-remove",
// "ui:value-add-choice", "ui:value-description", "ui:value-declare-new",
// a local's "ui:value-role-local", "ui:value-role-input" and
// "ui:value-role-output", a place's "ui:value-place" box,
// "ui:value-pick-place" and "ui:value-clear-place", and "ui:value-delete".
const (
	nameID        = "value-name"
	defaultID     = "value-default"
	minID         = "value-min"
	maxID         = "value-max"
	descriptionID = "value-description"
	typePrefix    = "value-type-"
	defaultPrefix = "value-default-"
	choicePrefix  = "value-choice-"
	addChoiceID   = "value-add-choice"
	deleteID      = "value-delete"
	declareNewID  = "value-declare-new"
	rolePrefix    = "value-role-"
	pickPlaceID   = "value-pick-place"
	placeBoxID    = "value-place"
	clearPlaceID  = "value-clear-place"
)

// What each role means, under its buttons.
const (
	inputMeans = "An input is filled by whatever calls this workflow, read once as the call starts. Where " +
		"a caller leaves it unfilled, or it runs as a behaviour of its own, it starts as below."
	outputMeans = "An output is handed back when the call returns, by any way out, to the value the Call " +
		"keeps it in."
)

// Headings.
const (
	headName        = "NAME"
	headDescription = "DESCRIPTION"
)

// choiceRow is one choice of a list: its name and Remove.
type choiceRow struct {
	id     string
	name   *ui.TextField
	remove *ui.Button
}

// Form edits one declaration.
type Form struct {
	theme *ui.Theme
	shown worldstate.Declaration
	// dotted is the shown name from its root, and Dotted whether a name may
	// have dots, moving it into groups.
	dotted string
	Dotted bool
	// name, number, text, low, high and description are typed; number
	// shows only for an integer's default, low and high for its limits, and
	// text for a text value's default.
	name, number, text, low, high, description *ui.TextField
	types                                      []*ui.Button
	roles                                      []*ui.Button
	defaults                                   []*ui.Button
	choices                                    []choiceRow
	addChoice                                  *ui.Button
	// Delete removes the declaration; the window places it.
	Delete *ui.Button
	// heads are where each label goes, by label; hint is where the line
	// under the name goes, when it has one.
	heads map[string]image.Point
	hint  image.Point
	// Locked, when set, says why a value's type cannot change, and its other
	// types are greyed; lock is where that goes, in lines, with DeclareNew
	// under it.
	Locked     string
	lock       image.Point
	lockLines  []string
	DeclareNew *ui.Button
	// PickPlace picks a place value's default on a map and ClearPlace
	// clears it; PlaceBox shows it and opens the window's list of places.
	// The window sets PlaceName, PlaceWhere, its zone and floor, PlaceLit,
	// just picked, and PlaceOpen, while the list is open. means is where what
	// a role means goes, in lines.
	PickPlace  *ui.Button
	ClearPlace *ui.Button
	PlaceBox   image.Rectangle
	PlaceName  string
	PlaceWhere string
	PlaceLit   bool
	PlaceOpen  bool
	means      image.Point
	meansLines []string
	// placeAt is where what a place is goes, in placeLines.
	placeAt    image.Point
	placeLines []string
}

// New returns an empty form.
func New(theme *ui.Theme) *Form {
	field := func(id string, multiline bool, limit int) *ui.TextField {
		return ui.NewTextField(id, textedit.NewField("", multiline, limit), multiline, theme)
	}
	remove := ui.NewButton(deleteID, "Delete value")
	remove.Danger = true
	form := &Form{
		theme: theme, shown: worldstate.New(ids.ULID{}, "", "", ""), dotted: "", Dotted: false,
		name: field(nameID, false, dottedLength), number: field(defaultID, false, maxNumberText),
		text: field(defaultID, false, worldstate.MaxText), low: field(minID, false, maxNumberText),
		high: field(maxID, false, maxNumberText), description: field(descriptionID, true, worldstate.MaxDescription),
		types: nil, defaults: nil, choices: nil, addChoice: ui.NewButton(addChoiceID, "Add choice"), Delete: remove,
		heads: map[string]image.Point{}, hint: image.Pt(0, 0), Locked: "", lock: image.Pt(0, 0), lockLines: nil,
		DeclareNew: ui.NewButton(declareNewID, "Declare a new value…"), roles: nil,
		PickPlace: ui.NewButton(pickPlaceID, "Pick on map"), ClearPlace: ui.NewButton(clearPlaceID, "Clear"),
		PlaceBox: ui.Zero(), PlaceName: "", PlaceWhere: "", PlaceLit: false, PlaceOpen: false, means: image.Pt(0, 0),
		meansLines: nil, placeAt: image.Pt(0, 0), placeLines: nil,
	}
	form.PickPlace.Primary = true
	form.description.Placeholder = "What it is for, for whoever reads it next"
	form.low.Placeholder, form.high.Placeholder = "No minimum", "No maximum"

	return form
}

// Shown returns the declaration the form shows.
func (f *Form) Shown() worldstate.Declaration { return f.shown.Clone() }

// Show shows a declaration as saved, with its name from its root. It
// reports whether the form was rebuilt, for a new declaration, type or
// number of choices, so the window can register its fields again; a rebuild
// for the same declaration keeps what was typed.
func (f *Form) Show(declared worldstate.Declaration, dotted string) bool {
	rebuild := declared.ID != f.shown.ID || declared.Type != f.shown.Type || declared.Scope != f.shown.Scope ||
		len(declared.Choices) != len(f.choices) || (f.types == nil && declared.Type.IsValue())
	drafts := map[string]string{}
	if rebuild && declared.ID == f.shown.ID {
		for _, field := range f.Fields() {
			if field.Field.Unsaved() {
				drafts[field.ID] = field.Field.Value()
			}
		}
	}
	if rebuild {
		f.build(declared)
	}
	f.shown, f.dotted = declared.Clone(), dotted
	f.setSaved()
	for _, field := range f.Fields() {
		if draft, kept := drafts[field.ID]; kept {
			field.Field.Restore(draft)
		}
	}
	f.Delete.Label = "Delete " + kindName(declared.Type)

	return rebuild
}

// kindName names what a declaration is: a value, a group or a template.
func kindName(kind worldstate.Type) string {
	if kind.IsValue() {
		return "value"
	}

	return kind.Label()
}

func (f *Form) build(declared worldstate.Declaration) {
	if declared.ID != f.shown.ID {
		for _, field := range []*ui.TextField{f.name, f.number, f.text, f.low, f.high, f.description} {
			field.Field.Revert()
		}
	}
	f.types, f.roles = nil, nil
	if declared.Type.IsValue() {
		for _, kind := range worldstate.Types(declared.Scope) {
			button := ui.NewButton(typePrefix+string(kind), kind.Label())
			if kind == worldstate.TypePlace {
				button.Icon = ui.IconPlace
			}
			f.types = append(f.types, button)
		}
	}
	if declared.Type.IsValue() && declared.Scope == worldstate.ScopeLocal {
		for _, role := range worldstate.Roles() {
			f.roles = append(f.roles, ui.NewButton(rolePrefix+strings.ToLower(role.Label()), role.Label()))
		}
	}
	f.choices = nil
	for index := range declared.Choices {
		prefix := choicePrefix + strconv.Itoa(index+1)
		f.choices = append(f.choices, choiceRow{
			id:     "",
			name:   ui.NewTextField(prefix, textedit.NewField("", false, worldstate.MaxName), false, f.theme),
			remove: ui.NewButton(prefix+"-remove", "Remove"),
		})
	}
}

// setSaved shows the saved declaration in the fields and buttons.
func (f *Form) setSaved() {
	declared := f.shown
	f.name.Field.SetSaved(f.dotted)
	f.description.Field.SetSaved(declared.Description)
	for index, button := range f.types {
		button.On = typePrefix+string(declared.Type) == button.ID && index >= 0
	}
	for _, button := range f.roles {
		button.On = button.ID == rolePrefix+strings.ToLower(declared.Role.Label())
	}
	for index, choice := range declared.Choices {
		if index < len(f.choices) {
			f.choices[index].id = choice.ID
			f.choices[index].name.Field.SetSaved(choice.Name)
			f.choices[index].remove.Disabled = len(declared.Choices) == 1
		}
	}
	f.addChoice.Disabled = len(declared.Choices) >= worldstate.MaxChoices
	f.low.Field.SetSaved(limitText(declared.Min))
	f.high.Field.SetSaved(limitText(declared.Max))
	f.defaults = nil
	current := declared.Zero()
	switch declared.Type {
	case worldstate.TypeInteger:
		f.number.Field.SetSaved(strconv.Itoa(current.Integer))
	case worldstate.TypeText:
		f.text.Field.SetSaved(current.Text)
	case worldstate.TypeBool:
		for _, truth := range []bool{false, true} {
			button := ui.NewButton(defaultPrefix+strconv.FormatBool(truth), strconv.FormatBool(truth))
			button.On = current.Bool == truth
			f.defaults = append(f.defaults, button)
		}
	case worldstate.TypeChoice:
		for _, choice := range declared.Choices {
			button := ui.NewButton(defaultPrefix+choice.ID, choice.Name)
			button.On = choice.ID == current.Choice
			f.defaults = append(f.defaults, button)
		}
	case worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup, worldstate.TypeTemplate:
	}
}

func limitText(limit *int) string {
	if limit == nil {
		return ""
	}

	return strconv.Itoa(*limit)
}

// Fields lists the form's typed fields, in order.
func (f *Form) Fields() []*ui.TextField {
	fields := []*ui.TextField{f.name}
	for _, row := range f.choices {
		fields = append(fields, row.name)
	}
	switch f.shown.Type {
	case worldstate.TypeInteger:
		fields = append(fields, f.number, f.low, f.high)
	case worldstate.TypeText:
		fields = append(fields, f.text)
	case worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}

	return append(fields, f.description)
}

// Unsaved counts the typed changes not saved yet.
func (f *Form) Unsaved() int {
	count := 0
	for _, field := range f.Fields() {
		if field.Field.Unsaved() {
			count++
		}
	}

	return count
}

// Revert drops what was typed.
func (f *Form) Revert() {
	for _, field := range f.Fields() {
		field.Field.Revert()
	}
}

// Hide takes the form off the screen, for while the window shows something
// else, so its fields and buttons take no clicks.
func (f *Form) Hide() {
	for _, field := range f.Fields() {
		field.Rect = ui.Zero()
	}
	for _, button := range f.Buttons() {
		button.Rect = ui.Zero()
	}
	f.heads = map[string]image.Point{}
}

// Layout places the form from top down, left to left+width, and returns
// where the next part goes.
func (f *Form) Layout(left, top, width int) int {
	f.heads = map[string]image.Point{}
	head := func(label string) {
		f.heads[label] = image.Pt(left, top)
		top += ui.LineHeight
	}
	place := func(field *ui.TextField, height int) {
		field.Rect = image.Rect(left, top, left+width, top+height)
		top += height + fieldSpace
	}
	for _, field := range f.Fields() {
		field.Rect = ui.Zero()
	}
	f.addChoice.Rect = ui.Zero()
	for _, row := range f.choices {
		row.remove.Rect = ui.Zero()
	}
	head(headName)
	place(f.name, ui.ButtonH)
	f.hint = image.Pt(0, 0)
	if f.Dotted && f.shown.Type != worldstate.TypeTemplate {
		f.hint = image.Pt(left, top-rowSpace)
		top += ui.LineHeight
	}
	f.DeclareNew.Rect, f.lockLines, f.meansLines, f.PickPlace.Rect = ui.Zero(), nil, nil, ui.Zero()
	f.PlaceBox, f.ClearPlace.Rect = ui.Zero(), ui.Zero()
	if len(f.roles) > 0 {
		head("ROLE")
		top = buttonRows(f.roles, left, top, width, len(f.roles))
		top = f.layoutMeans(left, top, width)
	}
	if f.shown.Type.IsValue() {
		head("TYPE")
		top = f.fitRows(f.types, left, top, width)
		top = f.layoutLock(left, top, width)
		top = f.layoutPlaceLine(left, top, width)
		top = f.layoutValue(left, top, width)
	}
	head(headDescription)
	place(f.description, descHeight)

	return top
}

// layoutMeans places what an input or an output means, under the roles.
func (f *Form) layoutMeans(left, top, width int) int {
	means := map[worldstate.Role]string{worldstate.RoleInput: inputMeans, worldstate.RoleOutput: outputMeans}[f.shown.Role]
	if means == "" {
		return top
	}
	f.means, f.meansLines = image.Pt(left, top-rowSpace), f.theme.Wrap(means, width)

	return top + len(f.meansLines)*ui.LineHeight + fieldSpace - rowSpace
}

// placeLine says what a place is, under the type buttons.
const placeLine = "A place is an anchor on a map: one of the blue rings put down with the Anchors tool."

// layoutPlaceLine places what a place is under the types, for a place
// whose type can still change; one in use shows its ring and place below.
func (f *Form) layoutPlaceLine(left, top, width int) int {
	f.placeLines = nil
	if f.shown.Type != worldstate.TypePlace || f.Locked != "" {
		return top
	}
	f.placeAt, f.placeLines = image.Pt(left, top-rowSpace), f.theme.Wrap(placeLine, width)

	return top + len(f.placeLines)*ui.LineHeight + fieldSpace - rowSpace
}

// layoutLock greys the types a locked value cannot change to, and places
// why under them, with Declare a new value….
func (f *Form) layoutLock(left, top, width int) int {
	for _, button := range f.types {
		button.Disabled = f.Locked != "" && !button.On
	}
	if f.Locked == "" {
		return top
	}
	f.lock, f.lockLines = image.Pt(left, top-rowSpace), f.theme.Wrap(f.Locked, width)
	top += len(f.lockLines)*ui.LineHeight - rowSpace
	f.DeclareNew.Rect = image.Rect(left, top, left+min(width, f.theme.TextWidth(f.DeclareNew.Label)+ui.Inset*ui.Half),
		top+ui.ButtonH)

	return top + ui.ButtonH + fieldSpace
}

// layoutValue places what a value's type has: its choices, its limits and
// its default.
func (f *Form) layoutValue(left, top, width int) int {
	headAt := func(label string) {
		f.heads[label] = image.Pt(left, top)
		top += ui.LineHeight
	}
	switch f.shown.Type {
	case worldstate.TypeChoice:
		headAt("CHOICES")
		for _, row := range f.choices {
			row.name.Rect = image.Rect(left, top, left+width-removeWidth-rowSpace, top+ui.ButtonH)
			row.remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
			top += ui.ButtonH + rowSpace
		}
		f.addChoice.Rect = image.Rect(left, top, left+width/ui.Half, top+ui.ButtonH)
		top += ui.ButtonH + fieldSpace
	case worldstate.TypeInteger:
		f.heads["MINIMUM"], f.heads["MAXIMUM"] = image.Pt(left, top), image.Pt(left+width/ui.Half+rowSpace/ui.Half, top)
		top += ui.LineHeight
		f.low.Rect = image.Rect(left, top, left+width/ui.Half-rowSpace/ui.Half, top+ui.ButtonH)
		f.high.Rect = image.Rect(left+width/ui.Half+rowSpace/ui.Half, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + fieldSpace
	case worldstate.TypeBool, worldstate.TypeText, worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}
	if f.shown.Type == worldstate.TypeNPC {
		return top
	}
	headAt("DEFAULT")
	switch f.shown.Type {
	case worldstate.TypePlace:
		top = f.layoutPlace(left, top, width)
	case worldstate.TypeInteger:
		f.number.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + fieldSpace
	case worldstate.TypeText:
		f.text.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + fieldSpace
	case worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypeGroup, worldstate.TypeTemplate:
		top = buttonRows(f.defaults, left, top, width, choicesPerLine)
	}

	return top
}

// buttonRows places buttons in rows of at most perRow, and returns where the
// next part goes.
func buttonRows(buttons []*ui.Button, left, top, width, perRow int) int {
	for start := 0; start < len(buttons); start += perRow {
		row := buttons[start:min(start+perRow, len(buttons))]
		gaps := (perRow - 1) * rowSpace
		for index, button := range row {
			from := left + index*(width-gaps)/perRow + index*rowSpace
			to := left + (index+1)*(width-gaps)/perRow + index*rowSpace
			button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
		}
		top += ui.ButtonH + rowSpace
	}

	return top + fieldSpace - rowSpace
}

// fitRows places buttons at the width their labels and icons need, as many
// to a row as fit, and returns where the next part goes.
func (f *Form) fitRows(buttons []*ui.Button, left, top, width int) int {
	across := left
	for _, button := range buttons {
		need := f.theme.TextWidth(button.Label) + ui.Half*ui.Inset
		if button.Icon != ui.IconNone {
			need += ui.IconSize + ui.Gap
		}
		if across > left && across+need > left+width {
			across, top = left, top+ui.ButtonH+rowSpace
		}
		button.Rect = image.Rect(across, top, across+need, top+ui.ButtonH)
		across += need + rowSpace
	}

	return top + ui.ButtonH + fieldSpace
}

// Change is what a click on the form asks for.
type Change int

// Changes: nothing, a changed declaration to save, deleting it, or
// declaring a new value instead of changing a locked one's type; for a
// place, picking its default on a map or choosing it from a list.
const (
	ChangeNone Change = iota
	ChangeValue
	ChangeDelete
	ChangeDeclareNew
	// ChangePickPlace asks the window to pick a place value's default on a
	// map, and ChangeChoosePlace to open its list of places.
	ChangePickPlace
	ChangeChoosePlace
)

// Update handles the form's buttons. A changed declaration is returned for
// the window to save; buttons never wait for Save.
func (f *Form) Update(input *ui.Input) (worldstate.Declaration, Change) {
	declared := f.shown.Clone()
	if f.Delete.Update(input) {
		return declared, ChangeDelete
	}
	if f.DeclareNew.Update(input) {
		return declared, ChangeDeclareNew
	}
	if change := f.updatePlace(input); change != ChangeNone {
		if change == ChangeValue {
			declared.Default = declared.Empty().Encode()
		}

		return declared, change
	}
	for index, button := range f.roles {
		if button.Update(input) && !button.On && index < len(worldstate.Roles()) {
			declared.Role = worldstate.Roles()[index]

			return declared, ChangeValue
		}
	}
	for _, button := range f.types {
		kind := worldstate.Type(strings.TrimPrefix(button.ID, typePrefix))
		if button.Update(input) && kind != declared.Type && f.Locked == "" {
			return retype(declared, kind), ChangeValue
		}
	}
	if changed, ok := f.updateDefaults(input, declared); ok {
		return changed, ChangeValue
	}
	if changed, ok := f.updateChoices(input, declared); ok {
		return changed, ChangeValue
	}

	return declared, ChangeNone
}

// updateDefaults handles a click on a value's default.
func (f *Form) updateDefaults(input *ui.Input, declared worldstate.Declaration) (worldstate.Declaration, bool) {
	for index, button := range f.defaults {
		if !button.Update(input) || button.On {
			continue
		}
		chosen := worldstate.NoValue()
		chosen.Type, chosen.Bool = declared.Type, index == 1
		if declared.Type == worldstate.TypeChoice && index < len(declared.Choices) {
			chosen.Choice = declared.Choices[index].ID
		}
		declared.Default = chosen.Encode()

		return declared, true
	}

	return declared, false
}

// updateChoices adds a choice, or removes one, starting the value at its
// first choice when its default goes.
func (f *Form) updateChoices(input *ui.Input, declared worldstate.Declaration) (worldstate.Declaration, bool) {
	if f.addChoice.Update(input) {
		name := uniqueChoiceName(declared)
		declared.Choices = append(declared.Choices, worldstate.Choice{ID: declared.NewChoiceID(), Name: name})

		return declared, true
	}
	for index, row := range f.choices {
		if !row.remove.Update(input) || index >= len(declared.Choices) || len(declared.Choices) == 1 {
			continue
		}
		removed := declared.Choices[index].ID
		declared.Choices = slices.Delete(declared.Choices, index, index+1)
		if value, err := declared.Parse(declared.Default); err != nil || value.Choice == removed {
			declared.Default = declared.Empty().Encode()
		}

		return declared, true
	}

	return declared, false
}

// retype changes a value's type, starting it at the new type's own default.
func retype(declared worldstate.Declaration, kind worldstate.Type) worldstate.Declaration {
	made := worldstate.New(declared.ID, declared.Name, declared.Scope, kind)
	made.Parent, made.Description, made.Role = declared.Parent, declared.Description, declared.Role

	return made
}

func uniqueChoiceName(declared worldstate.Declaration) string {
	for number := len(declared.Choices) + 1; ; number++ {
		name := "choice " + strconv.Itoa(number)
		if !slices.ContainsFunc(declared.Choices, func(choice worldstate.Choice) bool { return choice.Name == name }) {
			return name
		}
	}
}

// Typed returns the declaration with everything typed applied and the
// dotted name typed, for the window to rename or move it, or the field that
// cannot be read and why.
func (f *Form) Typed() (worldstate.Declaration, string, *ui.TextField, string) {
	declared := f.shown.Clone()
	dotted := strings.TrimSpace(f.name.Field.Value())
	if problem := f.checkName(dotted); problem != "" {
		return declared, dotted, f.name, problem
	}
	declared.Description = strings.TrimSpace(f.description.Field.Value())
	for index, row := range f.choices {
		if index >= len(declared.Choices) {
			break
		}
		name := strings.TrimSpace(row.name.Field.Value())
		if name == "" {
			return declared, dotted, row.name, "A choice needs a name"
		}
		declared.Choices[index].Name = name
	}
	if problem := checkChoiceNames(declared); problem != "" {
		return declared, dotted, f.choices[0].name, problem
	}
	switch declared.Type {
	case worldstate.TypeInteger:
		if field, problem := f.typedInteger(&declared); problem != "" {
			return declared, dotted, field, problem
		}
	case worldstate.TypeText:
		text := worldstate.NoValue()
		text.Type, text.Text = worldstate.TypeText, f.text.Field.Value()
		declared.Default = text.Encode()
	case worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}

	return declared, dotted, nil, ""
}

// checkName says what is wrong with a typed name, or "".
func (f *Form) checkName(dotted string) string {
	if !f.Dotted {
		if worldstate.CheckName(dotted) != nil {
			return "A name is lower-case letters, digits and underscores, starting with a letter"
		}

		return ""
	}
	if _, err := worldstate.SplitName(dotted); err != nil {
		return "A name is lower-case letters, digits and underscores, starting with a letter; " +
			"dots put it in groups, such as quests.dragon_slain"
	}

	return ""
}

// typedInteger reads an integer's typed limits and default.
func (f *Form) typedInteger(declared *worldstate.Declaration) (*ui.TextField, string) {
	bounds := fmt.Sprintf("Enter a whole number from %d to %d", -worldstate.MaxInteger, worldstate.MaxInteger)
	limits := []**int{&declared.Min, &declared.Max}
	for index, field := range []*ui.TextField{f.low, f.high} {
		typed := strings.TrimSpace(field.Field.Value())
		if typed == "" {
			*limits[index] = nil

			continue
		}
		number, err := strconv.Atoi(typed)
		if err != nil || number > worldstate.MaxInteger || number < -worldstate.MaxInteger {
			return field, bounds + ", or leave it empty for no limit"
		}
		*limits[index] = worldstate.Limit(number)
	}
	if declared.Lowest() > declared.Highest() {
		return f.low, "The minimum is more than the maximum"
	}
	number, err := strconv.Atoi(strings.TrimSpace(f.number.Field.Value()))
	if err != nil || number > worldstate.MaxInteger || number < -worldstate.MaxInteger {
		return f.number, bounds
	}
	if number < declared.Lowest() || number > declared.Highest() {
		return f.number, fmt.Sprintf("The default is from %d to %d, its limits", declared.Lowest(), declared.Highest())
	}
	value := worldstate.NoValue()
	value.Type, value.Integer = worldstate.TypeInteger, number
	declared.Default = value.Encode()

	return nil, ""
}

func checkChoiceNames(declared worldstate.Declaration) string {
	seen := map[string]bool{}
	for _, choice := range declared.Choices {
		if seen[choice.Name] {
			return "Each choice needs its own name"
		}
		seen[choice.Name] = true
	}

	return ""
}

// Buttons lists the form's buttons, for input scripts.
func (f *Form) Buttons() []*ui.Button {
	buttons := slices.Concat(f.roles, f.types, f.defaults)
	for _, row := range f.choices {
		buttons = append(buttons, row.remove)
	}

	return append(buttons, f.addChoice, f.DeclareNew, f.PickPlace, f.ClearPlace, f.Delete)
}

// Locate finds a field or button by its ID.
func (f *Form) Locate(widget string) (image.Point, bool) {
	for _, field := range f.Fields() {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	for _, button := range f.Buttons() {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}
	if widget == placeBoxID && !f.PlaceBox.Empty() {
		return ui.Center(f.PlaceBox), true
	}

	return image.Pt(0, 0), false
}

// Draw draws the form.
func (f *Form) Draw(screen *ebiten.Image) {
	for label, at := range f.heads {
		f.theme.Heading(screen, label, at.X, at.Y)
	}
	for _, field := range f.Fields() {
		if !field.Rect.Empty() {
			field.Draw(screen)
		}
	}
	for _, button := range f.Buttons() {
		if !button.Rect.Empty() && button != f.Delete {
			button.Draw(screen, f.theme)
		}
	}
	if f.hint != image.Pt(0, 0) {
		f.theme.DrawText(screen, "Dots move it into a group, as group."+f.shown.Name, f.hint.X,
			f.hint.Y, ui.ColorMuted())
	}
	for index, line := range f.lockLines {
		f.theme.DrawText(screen, line, f.lock.X, f.lock.Y+index*ui.LineHeight, ui.ColorMuted())
	}
	for index, line := range f.meansLines {
		f.theme.DrawText(screen, line, f.means.X, f.means.Y+index*ui.LineHeight, ui.ColorMuted())
	}
	for index, line := range f.placeLines {
		f.theme.DrawText(screen, line, f.placeAt.X, f.placeAt.Y+index*ui.LineHeight, ui.ColorMuted())
	}
	if !f.PlaceBox.Empty() {
		f.drawPlace(screen)
	}
	if f.shown.Type == worldstate.TypeNPC {
		at := f.heads[headDescription]
		f.theme.DrawText(screen, "Starts as no NPC.", at.X, at.Y-ui.LineHeight-rowSpace, ui.ColorMuted())
	}
}

// updatePlace handles a place's default: Pick on map, its box, which opens
// the list of places, and Clear, which changes the value.
func (f *Form) updatePlace(input *ui.Input) Change {
	switch {
	case f.PickPlace.Update(input):
		return ChangePickPlace
	case input.Pressed && input.Cursor.In(f.PlaceBox):
		return ChangeChoosePlace
	case f.ClearPlace.Update(input):
		return ChangeValue
	}

	return ChangeNone
}

// layoutPlace places a place's default: its box, the line naming its zone
// and floor, then Pick on map and Clear.
func (f *Form) layoutPlace(left, top, width int) int {
	f.PlaceBox = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + ui.LineHeight
	f.PickPlace.Rect = image.Rect(left, top, left+width/ui.Half-rowSpace/ui.Half, top+ui.ButtonH)
	if f.PlaceName != "" {
		f.ClearPlace.Rect = image.Rect(left+width/ui.Half+rowSpace/ui.Half, top, left+width, top+ui.ButtonH)
	}

	return top + ui.ButtonH + fieldSpace
}

// drawPlace draws a place's default in its box, and its zone and floor.
func (f *Form) drawPlace(screen *ebiten.Image) {
	label, look := f.PlaceName, places.Chosen
	if label == "" {
		label, look = "No place yet", places.Empty
	}
	places.DrawBox(screen, f.theme, f.PlaceBox, label, look, f.PlaceLit, f.PlaceOpen)
	f.theme.DrawText(screen, f.PlaceWhere, f.PlaceBox.Min.X, f.PlaceBox.Max.Y+ui.Gap, ui.ColorMuted())
}
