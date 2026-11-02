package workflowview

import (
	"encoding/json"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// chooseValueLabel is the picker's title, and what an empty box says.
const chooseValueLabel = "Choose a value"

// valuePicker is the open value picker and what it chooses for: a node's
// parameter, or, with operand set, another value for an operand to compare
// with, saved as a reference.
type valuePicker struct {
	picker  *valuepicker.Picker
	node    ids.ULID
	param   string
	role    valueRole
	operand bool
}

// valueSource is a node's side of the picker: the values its parameter may
// choose, where a name typed could be declared, and what a pick does.
type valueSource struct {
	state   *State
	node    ids.ULID
	param   string
	role    valueRole
	operand bool
	// targets are the declare targets last offered, by their IDs.
	targets map[string]declareTarget
}

// openValuePicker opens the picker for a parameter of a node, with the
// current choice selected and the search focused.
func (s *State) openValuePicker(node flow.Node, param, title string, only worldstate.Type, operand bool) {
	role := roleOf(node.Kind, param)
	source := &valueSource{state: s, node: node.ID, param: param, role: role, operand: operand, targets: nil}
	placeholder, empty := "Find a value, or type a new name", "No value matches. Type a name to declare one."
	if operand {
		placeholder = "Find " + only.Label() + " values"
		empty = "No other " + only.Label() + " to compare with. Type a name to declare one."
	}
	selected := ""
	if operand {
		if ref, refers := flow.OperandRef(savedOperand(node, param)); refers {
			selected = encodeRef(ref)
		}
	} else if ref, err := node.Ref(param); err == nil && !ref.IsZero() {
		selected = encodeRef(ref)
	}
	config := valuepicker.Config{
		Title: chooseValueLabel, For: title, Placeholder: placeholder, Empty: empty, Only: only,
		Rail: pickerRail(source.optionRole()), Selected: selected, Query: "", LastGroup: &s.lastGroup, PickLabel: "Choose",
		PickHint: "picks", DeclareAs: "Declare and choose", DeclareLead: "It is chosen as soon as it is declared.",
	}
	s.values = &valuePicker{
		picker: valuepicker.New(config, source, s.theme), node: node.ID, param: param, role: role, operand: operand,
	}
}

// optionRole is whose values the picker offers: a Check value's, for an
// operand.
func (v *valueSource) optionRole() valueRole {
	if v.operand {
		return roleCheck
	}

	return v.role
}

// Options lists the values the parameter may choose.
func (v *valueSource) Options(docs frame.Docs) []valuepicker.Option {
	found := v.state.valueOptions(docs, v.optionRole())
	listed := make([]valuepicker.Option, 0, len(found))
	for _, option := range found {
		listed = append(listed, valuepicker.Option{
			ID: encodeRef(option.ref), Path: option.path, Declared: option.declared, Heading: option.heading,
			Rail: option.rail, Root: option.root, Detail: "",
		})
	}

	return listed
}

// Targets lists where a name typed could be declared.
func (v *valueSource) Targets(docs frame.Docs, query string) []valuepicker.Target {
	v.targets = map[string]declareTarget{}
	found := v.state.declareTargets(docs, v.optionRole(), query)
	targets := make([]valuepicker.Target, 0, len(found))
	for _, target := range found {
		heading := target.Path
		if target.For != "" {
			heading += " for " + v.state.ownerName(docs, target.Document)
		}
		v.targets[target.located()] = target
		targets = append(targets, valuepicker.Target{
			ID: target.located(), Path: target.Path, Label: v.state.whereLabel(docs, target), Heading: heading,
			SetIn: declaredWhere(target.Scope),
		})
	}

	return targets
}

// Pick chooses the value picked: as a reference for an operand, or as the
// parameter's value.
func (v *valueSource) Pick(ctx *frame.Context, option valuepicker.Option) {
	node, found := v.state.currentNode(ctx.Docs, v.node)
	ref, refers := decodeRef(option.ID)
	if !found || !refers {
		return
	}
	if v.state.chooseForCall(ctx, node, v.param, ref) {
		return
	}
	if v.operand {
		v.state.saveParam(ctx, node, v.param, flow.RefOperand(ref))

		return
	}
	v.state.chooseValue(ctx, ctx.Docs, node, v.param, ref)
}

// Declare declares the value the panel describes and chooses it.
func (v *valueSource) Declare(ctx *frame.Context, target valuepicker.Target, kind worldstate.Type,
	start json.RawMessage,
) {
	declared, found := v.targets[target.ID]
	if !found {
		return
	}
	v.state.declareValue(ctx, v.node, v.param, declaring{target: declared, kind: kind, start: start, operand: v.operand})
}

// whereLabel names where a value can be declared: its scope, one NPC's
// own, or a template.
func (s *State) whereLabel(docs frame.Docs, target declareTarget) string {
	switch {
	case target.For != "":
		return s.ownerName(docs, target.Document) + "'s own"
	case target.Parent != (ids.ULID{}):
		names := strings.Split(target.Path, ".")

		return target.Scope.Label() + " template " + names[1]
	default:
		return target.Scope.Label()
	}
}

// pickerRail lists the scopes a role's values come from, after Everywhere.
func pickerRail(role valueRole) []valuepicker.RailEntry {
	everywhere := valuepicker.RailEntry{ID: valuepicker.Everywhere, Label: "Everywhere"}
	local := valuepicker.RailEntry{ID: string(worldstate.ScopeLocal), Label: "Local"}
	switch role {
	case roleKeep:
		return []valuepicker.RailEntry{everywhere, local}
	case roleWhere:
		return []valuepicker.RailEntry{everywhere, {ID: string(worldstate.ScopeNPC), Label: "The NPC it finds"}}
	case roleSet, roleCheck, roleGive, roleKept:
	}
	rail := []valuepicker.RailEntry{
		everywhere, local, {ID: string(worldstate.ScopeNPC), Label: "This NPC"},
		{ID: string(worldstate.ScopeZone), Label: "This zone"}, {ID: string(worldstate.ScopeWorld), Label: "World"},
	}
	if role == roleCheck || role == roleGive {
		rail = append(rail, valuepicker.RailEntry{ID: railOthers, Label: "Other NPCs"},
			valuepicker.RailEntry{ID: railZones, Label: "Zones"})
	}

	return rail
}

// screen is the whole window, where the picker is centred.
func (s *State) screen() image.Rectangle { return image.Rectangle{Min: image.Pt(0, 0), Max: s.size} }

// updateValuePicker runs the open picker for a frame, closing it once it
// has picked, declared or been closed.
func (s *State) updateValuePicker(input *ui.Input, ctx *frame.Context) {
	if !s.values.picker.Update(input, ctx, s.screen()) {
		s.values = nil
	}
}

// drawValuePicker draws the open picker over the window.
func (s *State) drawValuePicker(screen *ebiten.Image, docs frame.Docs) {
	s.values.picker.Draw(screen, docs, s.screen())
}

// locateInPicker finds a part of the open picker for input scripts.
func (s *State) locateInPicker(docs frame.Docs, kind, value string) (image.Point, bool) {
	return s.values.picker.Locate(docs, s.screen(), kind, value)
}

// hasPickerState answers "picker", whether the picker is open, and the
// picker's own states.
func (s *State) hasPickerState(state string, docs frame.Docs) (bool, bool) {
	if state == valuepicker.LocatePart {
		return s.values != nil, true
	}
	if s.values == nil {
		return false, strings.HasPrefix(state, valuepicker.LocatePart+"-")
	}

	return s.values.picker.HasState(docs, s.screen(), state)
}

// pickerTitle says what a picker chooses for: a parameter by its label, an
// operand by the type it must match, or a value to keep an NPC in.
func pickerTitle(param flow.Param, only worldstate.Type, operand bool) string {
	switch {
	case operand:
		return "for " + param.Label + " · " + strings.ToLower(only.Label()) + "s only, to match its value"
	case only != "":
		return "for " + param.Label + " · " + only.Label() + " values only"
	default:
		return "for " + param.Label
	}
}
