package stateview

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Dialog limits.
const (
	usesListed = 8
	deleteID   = "delete"
	moveID     = "move"
)

// deletion is a delete waiting for its dialog: the list, what goes, and
// what to call it.
type deletion struct {
	list     valuetree.List
	removing []ids.ULID
	path     string
}

// setOnly is a change to one declaration.
func setOnly(declared worldstate.Declaration) stateplugin.ChangeArgs {
	return stateplugin.ChangeArgs{Add: nil, Set: []worldstate.Declaration{declared}, Remove: nil}
}

// change sends a change to a list.
func (s *State) change(ctx *frame.Context, list valuetree.List, change stateplugin.ChangeArgs, success string) {
	s.send(ctx, list.Document, list.Edit, change, ctx.Frame.Report(success))
}

// send sends an edit to a document.
func (s *State) send(ctx *frame.Context, document ids.ULID, edit string, args any, done func(protocol.Result, error)) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	ctx.Requests.Command(protocol.Command{Document: document, Edit: edit, Gesture: "", Args: data}, done)
}

// target is where Add value and Add group declare: a list, a scope and a
// parent in it.
type target struct {
	list   valuetree.List
	scope  worldstate.Scope
	parent ids.ULID
}

// addTarget returns where Add value and Add group declare: inside the
// selected group or template, beside the selected value, among the selected
// NPC's or zone's own values, or at the top of the world.
func (s *State) addTarget(docs frame.Docs) (target, bool) {
	selected, found := s.selected()
	world, loaded := valuetree.World(docs)
	if !found {
		return target{list: world, scope: worldstate.ScopeWorld, parent: ids.ULID{}}, loaded
	}
	switch selected.kind {
	case itemDeclared:
		parent := selected.declared.Parent
		if !selected.declared.Type.IsValue() {
			parent = selected.declared.ID
		}

		return target{list: selected.list, scope: selected.scope, parent: parent}, true
	case itemOwner, itemName, itemApplied, itemHeld:
		own, ok := valuetree.Owned(docs, selected.ownerID)

		return target{list: own, scope: selected.scope, parent: ids.ULID{}}, ok
	case itemTemplates:
	}

	return target{list: world, scope: selected.scope, parent: ids.ULID{}}, false
}

// templateScope returns the scope Add template makes a template in: the
// selected row's, when it is an NPC's or zone's.
func (s *State) templateScope() (worldstate.Scope, bool) {
	selected, found := s.selected()
	if !found || !selected.scope.Owned() {
		return "", false
	}

	return selected.scope, true
}

// updateAdds handles Add value, Add group and Add template.
func (s *State) updateAdds(input *ui.Input, ctx *frame.Context) {
	into, canAdd := s.addTarget(ctx.Docs)
	scope, canTemplate := s.templateScope()
	s.adds[0].Disabled, s.adds[1].Disabled, s.adds[2].Disabled = !canAdd, !canAdd, !canTemplate
	switch {
	case s.adds[0].Update(input):
		s.declare(ctx, into, "value", worldstate.TypeInteger)
	case s.adds[1].Update(input):
		s.declare(ctx, into, "group", worldstate.TypeGroup)
	case s.adds[2].Update(input):
		world, _ := valuetree.World(ctx.Docs)
		s.declare(ctx, target{list: world, scope: scope, parent: ids.ULID{}}, "template", worldstate.TypeTemplate)
	}
}

// declare declares a value, group or template with a name nothing beside
// it has, and selects it to be named in the inspector.
func (s *State) declare(ctx *frame.Context, into target, base string, kind worldstate.Type) {
	owned := into.list.Document != worldstate.ID() && into.parent.IsZero()
	name := base
	for number := 2; ; number++ {
		_, taken := into.list.Values.Find(into.scope, into.parent, name)
		if !taken && (!owned || valuetree.Clash(s.shown.catalog, into.list.Document, name) == "") {
			break
		}
		name = base + "_" + strconv.Itoa(number)
	}
	change, err := valuetree.Declare(into.list.Values, into.scope, into.parent, name, kind)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	switch {
	case !into.parent.IsZero():
		delete(s.toggled, declaredPrefix+into.parent.String())
	case into.list.Document == worldstate.ID():
		delete(s.toggled, sectionPrefix+string(into.scope))
	default:
		s.toggled[ownerPrefix+into.list.Document.String()] = true
	}
	s.send(ctx, into.list.Document, into.list.Edit, change, func(result protocol.Result, err error) {
		ctx.Frame.Report("Added "+name+". Name it in the inspector.")(result, err)
		if err == nil {
			s.list.Selected = declaredPrefix + result.Created.String()
		}
	})
}

// toggleTemplate applies a template to the selected NPC or zone, or takes
// it off.
func (s *State) toggleTemplate(ctx *frame.Context, owner item, template worldstate.Declaration) {
	apply, unapply := npcplugin.ApplyTemplateEdit, npcplugin.UnapplyTemplateEdit
	if owner.scope == worldstate.ScopeZone {
		apply, unapply = zoneplugin.ApplyTemplateEdit, zoneplugin.UnapplyTemplateEdit
	}
	args := stateplugin.TemplateArgs{Template: template.ID, Index: len(owner.owner.Owned.Templates)}
	if slices.Contains(owner.owner.Owned.Templates, template.ID) {
		s.send(ctx, owner.ownerID, unapply, args,
			ctx.Frame.Report("Took "+template.Name+" off "+owner.owner.Name+". Undo puts it back."))

		return
	}
	if clash := valuetree.Owns(s.shown.catalog, owner.ownerID, template); clash != "" {
		ctx.Frame.Status(clash + ".")

		return
	}
	s.send(ctx, owner.ownerID, apply, args, ctx.Frame.Report("Applied "+template.Name+" to "+owner.owner.Name+"."))
}

// startDelete asks where everything a delete would take is used, then
// refuses, asks or deletes.
func (s *State) startDelete(ctx *frame.Context, selected item) {
	removing, values := valuetree.Removal(selected.list.Values, selected.declared.ID)
	going := deletion{list: selected.list, removing: removing, path: selected.path}
	whereUsed(ctx, selected, func(uses []protocol.Use) { s.decideDelete(ctx, going, selected.declared, values, uses) })
}

// whereUsed asks where a declaration and everything in it is used, then
// hands every use found on, at once when there is nothing to ask about.
func whereUsed(ctx *frame.Context, selected item, then func([]protocol.Use)) {
	removing, _ := valuetree.Removal(selected.list.Values, selected.declared.ID)
	var asking []string
	for _, id := range removing {
		if held, ok := selected.list.Values.Value(id); ok && held.Type != worldstate.TypeGroup {
			asking = append(asking, id.String())
		}
	}
	if len(asking) == 0 {
		then(nil)

		return
	}
	var uses []protocol.Use
	remaining := len(asking)
	for _, asked := range asking {
		ctx.Requests.WhereUsed(worldstate.ID(), asked, func(found []protocol.Use, err error) {
			remaining--
			if err == nil {
				uses = append(uses, found...)
			}
			if remaining == 0 {
				then(uses)
			}
		})
	}
}

// decideDelete refuses a delete while anything in it is used, saying
// where; asks before deleting a group or template with what it holds; and
// deletes a value at once, as Undo brings it back.
func (s *State) decideDelete(
	ctx *frame.Context, going deletion, declared worldstate.Declaration, values int, uses []protocol.Use,
) {
	if len(uses) > 0 {
		lines := []string{"It cannot be deleted while it is used:"}
		for index, use := range uses {
			if index == usesListed {
				lines = append(lines, "  and "+strconv.Itoa(len(uses)-usesListed)+" more")

				break
			}
			lines = append(lines, "  "+use.Name+" ("+use.Label+")")
		}
		lines = append(lines, "Remove those uses first.")
		s.dialog = ui.NewDialog("“"+going.path+"” is in use", lines, ui.DialogButton(ui.DialogCancel, "OK"))

		return
	}
	if declared.Type.IsValue() {
		s.change(ctx, going.list, stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: going.removing},
			going.path+" deleted. Undo brings it back.")

		return
	}
	lines := []string{"Nothing uses it or anything in it."}
	if values > 0 {
		lines = append(lines, "The "+strconv.Itoa(values)+" "+valuesWord(values)+" in it will be deleted with it.")
	}
	lines = append(lines, "You can undo this.")
	s.dialog = ui.NewDialog("Delete the "+declared.Type.Label()+" “"+going.path+"”?", lines,
		ui.DialogButton(deleteID, "Delete"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.deleting = &going
}

func valuesWord(count int) string {
	if count == 1 {
		return "value"
	}

	return "values"
}

// updateDialog handles the delete dialog.
func (s *State) updateDialog(input *ui.Input, ctx *frame.Context) {
	choice := s.dialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	if choice == "" {
		return
	}
	going, moving := s.deleting, s.moving
	s.dialog, s.deleting, s.moving = nil, nil, nil
	if choice == moveID && moving != nil {
		s.change(ctx, moving.list, moving.change, moving.success)

		return
	}
	s.revealPath = ""
	if choice == deleteID && going != nil {
		s.change(ctx, going.list, stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: going.removing},
			going.path+" deleted. Undo brings it back.")
	}
}

// savePath saves the selected NPC's or zone's path name, or says what is
// wrong with it.
func (s *State) savePath(ctx *frame.Context) bool {
	typed := strings.TrimSpace(s.path.Field.Value())
	if typed != "" && worldstate.CheckName(typed) != nil {
		s.focus.Set(s.path)
		ctx.Frame.Status("A path name is lower-case letters, digits and underscores, starting with a letter")

		return false
	}
	if _, changed := s.path.Field.Commit(); !changed {
		s.focus.Set(nil)

		return true
	}
	meaning := s.shown.items[s.pathOf]
	edit, args := npcplugin.SetPathEdit, any(npcplugin.SetPathArgs{Path: typed})
	if meaning.scope == worldstate.ScopeZone {
		edit, args = zoneplugin.SetPathEdit, zoneplugin.SetPathArgs{Path: typed}
	}
	s.send(ctx, meaning.ownerID, edit, args, func(result protocol.Result, err error) {
		if err != nil {
			s.path.Field.Failed()
			ctx.Frame.Report("")(result, err)

			return
		}
		s.focus.Set(nil)
		ctx.Frame.Status("Path name saved")
	})

	return true
}
