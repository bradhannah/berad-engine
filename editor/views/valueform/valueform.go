// Package valueform is the inspector for one declared value, shared by the
// World state window and a workflow's local values: its name, type,
// choices, default and description. Buttons apply at once; typed fields
// wait for Save or Enter, as everywhere else in the editor.
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
	maxNumberText  = 12
)

// Locate names for input scripts: "ui:value-name", "ui:value-type-<type>",
// "ui:value-default", "ui:value-default-<choice id or yes/no>",
// "ui:value-choice-<n>", "ui:value-choice-<n>-remove", "ui:value-add-choice",
// "ui:value-description" and "ui:value-delete".
const (
	nameID        = "value-name"
	numberID      = "value-default"
	descriptionID = "value-description"
	typePrefix    = "value-type-"
	defaultPrefix = "value-default-"
	choicePrefix  = "value-choice-"
	addChoiceID   = "value-add-choice"
	deleteID      = "value-delete"
	yesID         = "yes"
	noID          = "no"
)

// choiceRow is one choice of a list: its name and Remove.
type choiceRow struct {
	id     string
	name   *ui.TextField
	remove *ui.Button
}

// Form edits one declared value.
type Form struct {
	theme *ui.Theme
	shown worldstate.Declaration
	// name, number and description are typed; number shows only for a
	// number's default.
	name, number, description *ui.TextField
	types                     []*ui.Button
	defaults                  []*ui.Button
	choices                   []choiceRow
	addChoice                 *ui.Button
	// Delete removes the value; the window places it.
	Delete *ui.Button
	// heads are where each label goes, by label.
	heads map[string]image.Point
}

// New returns an empty form.
func New(theme *ui.Theme) *Form {
	field := func(id string, multiline bool, limit int) *ui.TextField {
		return ui.NewTextField(id, textedit.NewField("", multiline, limit), multiline, theme)
	}
	remove := ui.NewButton(deleteID, "Delete value")
	remove.Danger = true
	form := &Form{
		theme: theme, shown: worldstate.Declaration{
			ID: ids.ULID{}, Name: "", Scope: "", Type: "", Choices: nil, Default: nil, Description: "",
		},
		name: field(nameID, false, worldstate.MaxName), number: field(numberID, false, maxNumberText),
		description: field(descriptionID, true, worldstate.MaxDescription), types: nil, defaults: nil, choices: nil,
		addChoice: ui.NewButton(addChoiceID, "Add choice"), Delete: remove, heads: map[string]image.Point{},
	}
	form.description.Placeholder = "What it is for, for whoever reads it next"

	return form
}

// Shown returns the value the form shows.
func (f *Form) Shown() worldstate.Declaration { return f.shown.Clone() }

// Show shows a value as saved. It reports whether the form was rebuilt, for
// a new value or a different number of choices, so the window can register
// its fields again; a rebuild for the same value keeps what was typed.
func (f *Form) Show(value worldstate.Declaration) bool {
	rebuild := value.ID != f.shown.ID || value.Type != f.shown.Type || value.Scope != f.shown.Scope ||
		len(value.Choices) != len(f.choices) || f.types == nil
	drafts := map[string]string{}
	if rebuild && value.ID == f.shown.ID {
		for _, field := range f.Fields() {
			if field.Field.Unsaved() {
				drafts[field.ID] = field.Field.Value()
			}
		}
	}
	if rebuild {
		f.build(value)
	}
	f.shown = value.Clone()
	f.setSaved()
	for _, field := range f.Fields() {
		if draft, kept := drafts[field.ID]; kept {
			field.Field.Restore(draft)
		}
	}

	return rebuild
}

func (f *Form) build(value worldstate.Declaration) {
	if value.ID != f.shown.ID {
		for _, field := range []*ui.TextField{f.name, f.number, f.description} {
			field.Field.Revert()
		}
	}
	f.types = nil
	for _, kind := range worldstate.Types(value.Scope) {
		f.types = append(f.types, ui.NewButton(typePrefix+string(kind), kind.Label()))
	}
	f.choices = nil
	for index := range value.Choices {
		prefix := choicePrefix + strconv.Itoa(index+1)
		f.choices = append(f.choices, choiceRow{
			id:     "",
			name:   ui.NewTextField(prefix, textedit.NewField("", false, worldstate.MaxName), false, f.theme),
			remove: ui.NewButton(prefix+"-remove", "Remove"),
		})
	}
}

// setSaved shows the saved value in the fields and buttons.
func (f *Form) setSaved() {
	value := f.shown
	f.name.Field.SetSaved(value.Name)
	f.description.Field.SetSaved(value.Description)
	for index, kind := range worldstate.Types(value.Scope) {
		if index < len(f.types) {
			f.types[index].On = kind == value.Type
		}
	}
	for index, choice := range value.Choices {
		if index < len(f.choices) {
			f.choices[index].id = choice.ID
			f.choices[index].name.Field.SetSaved(choice.Name)
			f.choices[index].remove.Disabled = len(value.Choices) == 1
		}
	}
	f.addChoice.Disabled = len(value.Choices) >= worldstate.MaxChoices
	current := value.Zero()
	f.defaults = nil
	switch value.Type {
	case worldstate.TypeNumber:
		f.number.Field.SetSaved(strconv.Itoa(current.Number))
	case worldstate.TypeYesNo:
		no, yes := ui.NewButton(defaultPrefix+noID, "no"), ui.NewButton(defaultPrefix+yesID, "yes")
		no.On, yes.On = !current.Yes, current.Yes
		f.defaults = []*ui.Button{no, yes}
	case worldstate.TypeChoice:
		for _, choice := range value.Choices {
			button := ui.NewButton(defaultPrefix+choice.ID, choice.Name)
			button.On = choice.ID == current.Choice
			f.defaults = append(f.defaults, button)
		}
	case worldstate.TypeNPC:
	}
}

// Fields lists the form's typed fields, in order.
func (f *Form) Fields() []*ui.TextField {
	fields := []*ui.TextField{f.name}
	for _, row := range f.choices {
		fields = append(fields, row.name)
	}
	if f.shown.Type == worldstate.TypeNumber {
		fields = append(fields, f.number)
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
	head("NAME")
	place(f.name, ui.ButtonH)
	head("TYPE")
	top = buttonRows(f.types, left, top, width, len(f.types))
	f.number.Rect, f.addChoice.Rect = ui.Zero(), ui.Zero()
	for _, row := range f.choices {
		row.name.Rect, row.remove.Rect = ui.Zero(), ui.Zero()
	}
	if f.shown.Type == worldstate.TypeChoice {
		head("CHOICES")
		for _, row := range f.choices {
			row.name.Rect = image.Rect(left, top, left+width-removeWidth-rowSpace, top+ui.ButtonH)
			row.remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
			top += ui.ButtonH + rowSpace
		}
		f.addChoice.Rect = image.Rect(left, top, left+width/ui.Half, top+ui.ButtonH)
		top += ui.ButtonH + fieldSpace
	}
	if f.shown.Type != worldstate.TypeNPC {
		head("STARTS AS")
		if f.shown.Type == worldstate.TypeNumber {
			place(f.number, ui.ButtonH)
		} else {
			top = buttonRows(f.defaults, left, top, width, choicesPerLine)
		}
	}
	head("DESCRIPTION")
	place(f.description, descHeight)

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

// Change is what a click on the form asks for.
type Change int

// Changes: nothing, a changed value to save, or deleting the value.
const (
	ChangeNone Change = iota
	ChangeValue
	ChangeDelete
)

// Update handles the form's buttons. A changed value is returned for the
// window to save; buttons never wait for Save.
func (f *Form) Update(input *ui.Input) (worldstate.Declaration, Change) {
	value := f.shown.Clone()
	if f.Delete.Update(input) {
		return value, ChangeDelete
	}
	for index, kind := range worldstate.Types(value.Scope) {
		if index < len(f.types) && f.types[index].Update(input) && kind != value.Type {
			return retype(value, kind), ChangeValue
		}
	}
	if changed, ok := f.updateDefaults(input, value); ok {
		return changed, ChangeValue
	}
	if changed, ok := f.updateChoices(input, value); ok {
		return changed, ChangeValue
	}

	return value, ChangeNone
}

// updateDefaults handles a click on what the value starts as.
func (f *Form) updateDefaults(input *ui.Input, value worldstate.Declaration) (worldstate.Declaration, bool) {
	for index, button := range f.defaults {
		if !button.Update(input) || button.On {
			continue
		}
		chosen := worldstate.Value{Type: value.Type, Number: 0, Yes: index == 1, Choice: "", NPC: ids.ULID{}}
		if value.Type == worldstate.TypeChoice && index < len(value.Choices) {
			chosen.Choice = value.Choices[index].ID
		}
		value.Default = chosen.Encode()

		return value, true
	}

	return value, false
}

// updateChoices adds a choice, or removes one, starting the value at its
// first choice when the one it started at goes.
func (f *Form) updateChoices(input *ui.Input, value worldstate.Declaration) (worldstate.Declaration, bool) {
	if f.addChoice.Update(input) {
		name := uniqueChoiceName(value)
		value.Choices = append(value.Choices, worldstate.Choice{ID: value.NewChoiceID(), Name: name})

		return value, true
	}
	for index, row := range f.choices {
		if !row.remove.Update(input) || index >= len(value.Choices) || len(value.Choices) == 1 {
			continue
		}
		removed := value.Choices[index].ID
		value.Choices = slices.Delete(value.Choices, index, index+1)
		if value.Zero().Choice == removed || value.Zero().Choice == "" {
			value.Default = value.Empty().Encode()
		}
		if _, err := value.Parse(value.Default); err != nil {
			value.Default = value.Empty().Encode()
		}

		return value, true
	}

	return value, false
}

// retype changes a value's type, starting it at the new type's own default.
func retype(value worldstate.Declaration, kind worldstate.Type) worldstate.Declaration {
	made := worldstate.New(value.ID, value.Name, value.Scope, kind)
	made.Description = value.Description

	return made
}

func uniqueChoiceName(value worldstate.Declaration) string {
	for number := len(value.Choices) + 1; ; number++ {
		name := "choice " + strconv.Itoa(number)
		if !slices.ContainsFunc(value.Choices, func(choice worldstate.Choice) bool { return choice.Name == name }) {
			return name
		}
	}
}

// Typed returns the value with everything typed applied, or the field that
// cannot be read and why.
func (f *Form) Typed() (worldstate.Declaration, *ui.TextField, string) {
	value := f.shown.Clone()
	value.Name = strings.TrimSpace(f.name.Field.Value())
	if err := worldstate.CheckName(value.Name); err != nil {
		return value, f.name, "A name is lower-case letters, digits and underscores, starting with a letter"
	}
	value.Description = strings.TrimSpace(f.description.Field.Value())
	for index, row := range f.choices {
		if index >= len(value.Choices) {
			break
		}
		name := strings.TrimSpace(row.name.Field.Value())
		if name == "" {
			return value, row.name, "A choice needs a name"
		}
		value.Choices[index].Name = name
	}
	if err := checkChoiceNames(value); err != "" {
		return value, f.choices[0].name, err
	}
	if value.Type == worldstate.TypeNumber {
		number, err := strconv.Atoi(strings.TrimSpace(f.number.Field.Value()))
		if err != nil || number > worldstate.MaxNumber || number < -worldstate.MaxNumber {
			return value, f.number, fmt.Sprintf("Enter a whole number from %d to %d", -worldstate.MaxNumber,
				worldstate.MaxNumber)
		}
		value.Default = worldstate.Value{Type: worldstate.TypeNumber, Number: number, Yes: false, Choice: "",
			NPC: ids.ULID{}}.Encode()
	}

	return value, nil, ""
}

func checkChoiceNames(value worldstate.Declaration) string {
	seen := map[string]bool{}
	for _, choice := range value.Choices {
		if seen[choice.Name] {
			return "Each choice needs its own name"
		}
		seen[choice.Name] = true
	}

	return ""
}

// Buttons lists the form's buttons, for input scripts.
func (f *Form) Buttons() []*ui.Button {
	buttons := append(append([]*ui.Button{}, f.types...), f.defaults...)
	for _, row := range f.choices {
		buttons = append(buttons, row.remove)
	}

	return append(buttons, f.addChoice, f.Delete)
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

	return image.Pt(0, 0), false
}

// Draw draws the form.
func (f *Form) Draw(screen *ebiten.Image) {
	for label, at := range f.heads {
		f.theme.Heading(screen, label, at.X, at.Y)
	}
	for _, field := range f.Fields() {
		field.Draw(screen)
	}
	for _, button := range f.Buttons() {
		if !button.Rect.Empty() && button != f.Delete {
			button.Draw(screen, f.theme)
		}
	}
	if f.shown.Type == worldstate.TypeNPC {
		at := f.heads["DESCRIPTION"]
		f.theme.DrawText(screen, "Starts as no NPC.", at.X, at.Y-ui.LineHeight-rowSpace, ui.ColorMuted())
	}
}
