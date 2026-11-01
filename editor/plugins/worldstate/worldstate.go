// Package worldstate is the editor half of the world state: the document
// that declares the world's values and the NPC and zone templates, and the
// edits that change any list of declared values, which the workflow, NPC and
// zone plug-ins use for their own. Every project holds one world-state
// document, with a fixed ID.
package worldstate

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// PluginName identifies the plug-in.
const PluginName = "world-state"

// ChangeEdit changes the world state's declarations.
const ChangeEdit = "worldstate.values.change"

// ErrNotWorldState reports a document that is not the world state.
var ErrNotWorldState = errors.New("document is not the world state")

// Document is the world state held by the editor.
type Document struct {
	State *worldstate.Document
}

// NewDocument returns a world state that declares nothing yet.
func NewDocument() *Document { return &Document{State: worldstate.NewDocument()} }

// Format names the document format.
func (d *Document) Format() string { return worldstate.FormatName }

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	data, err := d.State.Encode()
	if err != nil {
		return nil, fmt.Errorf("world-state document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document { return &Document{State: d.State.Clone()} }

// Decode reads a saved world state.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	decoded, err := worldstate.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("world-state document: %w", err)
	}

	return &Document{State: decoded}, nil
}

// emptyState is what a project's world state holds before anything is
// declared.
func emptyState() (json.RawMessage, error) {
	data, err := worldstate.NewDocument().Encode()
	if err != nil {
		return nil, fmt.Errorf("world-state document: %w", err)
	}

	return data, nil
}

// tempMark marks a temporary ID: its time part is zero, its second-last
// byte this mark and its last a number from 1, so no generated or fixed ID
// equals one.
const tempMark = 0xFE

// TempID returns the nth temporary ID, from 1, for a declaration a change
// adds: preparing the change gives it a permanent one, and any declaration
// in it whose parent is that temporary ID the same (D4). Temporary IDs let
// one change declare groups and a value inside them.
func TempID(n int) ids.ULID {
	var temp ids.ULID
	temp[len(temp)-2], temp[len(temp)-1] = tempMark, byte(n) //nolint:gosec // A change adds a handful of declarations.

	return temp
}

func isTemp(candidate ids.ULID) bool {
	for _, part := range candidate[:len(candidate)-2] {
		if part != 0 {
			return false
		}
	}

	return candidate[len(candidate)-2] == tempMark && candidate[len(candidate)-1] != 0
}

// ChangeArgs changes a list of declarations at once, all or none: it
// declares Add, then changes each of Set, keeping its ID, then removes
// Remove. Its inverse is another change, which adds back what it removed,
// puts back what it set and removes what it added.
type ChangeArgs struct {
	Add    []worldstate.Declaration `json:"add,omitempty"`
	Set    []worldstate.Declaration `json:"set,omitempty"`
	Remove []ids.ULID               `json:"remove,omitempty"`
}

// Values returns the list a change applies to: the world state's, an NPC's
// or zone's own, or a workflow's local values. A plug-in whose documents
// declare values gives its change edit one.
type Values func(document model.Document) (*worldstate.Declarations, error)

// worldValues is the world-state document's list.
func worldValues(document model.Document) (*worldstate.Declarations, error) {
	stateDoc, ok := document.(*Document)
	if !ok {
		return nil, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotWorldState)
	}

	return stateDoc.State.Values, nil
}

// Change is an edit kind that changes a list of declarations.
type Change struct {
	Kind, On string
	List     Values
}

// Name identifies the edit kind.
func (c Change) Name() string { return c.Kind }

// Format names the document format the edit applies to.
func (c Change) Format() string { return c.On }

// Prepare gives each declaration added with a temporary ID its permanent
// one, in order, so the last added is the edit's Created, and points what
// was inside it at the new ID.
func (c Change) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	var change ChangeArgs
	if err := json.Unmarshal(args, &change); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	permanent := map[ids.ULID]ids.ULID{}
	for index, added := range change.Add {
		if !isTemp(added.ID) {
			continue
		}
		made, err := newID()
		if err != nil {
			return nil, fmt.Errorf("new value ID: %w", err)
		}
		permanent[added.ID] = made
		change.Add[index].ID = made
	}
	if len(permanent) == 0 {
		return args, nil
	}
	for _, list := range [][]worldstate.Declaration{change.Add, change.Set} {
		for index, declared := range list {
			if made, ok := permanent[declared.Parent]; ok {
				list[index].Parent = made
			}
		}
	}

	return encode(change)
}

// Apply makes the change; its inverse undoes it.
func (c Change) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	values, err := c.List(document)
	if err != nil {
		return model.Applied{}, err
	}
	var change ChangeArgs
	if err := json.Unmarshal(args, &change); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	working := values.Clone()
	inverse, err := change.apply(working)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	*values = *working
	data, err := encode(inverse)
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{Inverse: data, InverseEdit: "", Touched: change.touched(), Covers: nil}, nil
}

// apply makes the change to a list and returns its inverse.
func (c ChangeArgs) apply(values *worldstate.Declarations) (ChangeArgs, error) {
	inverse := ChangeArgs{Add: nil, Set: nil, Remove: nil}
	if err := values.InsertAll(c.Add); err != nil {
		return inverse, err //nolint:wrapcheck // Wrapped by Apply.
	}
	for _, added := range c.Add {
		inverse.Remove = append(inverse.Remove, added.ID)
	}
	for _, set := range c.Set {
		previous, err := values.Replace(set)
		if err != nil {
			return inverse, err //nolint:wrapcheck // Wrapped by Apply.
		}
		inverse.Set = append(inverse.Set, previous)
	}
	removed, err := values.RemoveAll(c.Remove)
	if err != nil {
		return inverse, err //nolint:wrapcheck // Wrapped by Apply.
	}
	inverse.Add = removed

	return inverse, nil
}

// touched lists every declaration the change touches.
func (c ChangeArgs) touched() []string {
	items := make([]string, 0, len(c.Add)+len(c.Set)+len(c.Remove))
	for _, list := range [][]worldstate.Declaration{c.Add, c.Set} {
		for _, declared := range list {
			items = append(items, declared.ID.String())
		}
	}
	for _, removed := range c.Remove {
		items = append(items, removed.String())
	}

	return items
}

func encode(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode edit arguments: %w", err)
	}

	return data, nil
}

// TemplateArgs applies a template to an NPC or zone, or takes it off; Index
// is where it goes back when its removal is undone, past the end to append.
type TemplateArgs struct {
	Template ids.ULID `json:"template"`
	Index    int      `json:"index"`
}

// OwnedOf returns what an NPC or zone document owns, for the edits that
// apply templates to it.
type OwnedOf func(document model.Document) (*worldstate.Owned, error)

// ApplyTemplate is an edit kind that applies a template to an NPC or zone.
type ApplyTemplate struct {
	Kind, On, Inverse string
	Owned             OwnedOf
}

// Name identifies the edit kind.
func (a ApplyTemplate) Name() string { return a.Kind }

// Format names the document format the edit applies to.
func (a ApplyTemplate) Format() string { return a.On }

// Apply applies the template; its inverse takes it off.
func (a ApplyTemplate) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	owned, err := a.Owned(document)
	if err != nil {
		return model.Applied{}, err
	}
	var apply TemplateArgs
	if err := json.Unmarshal(args, &apply); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	index, err := owned.ApplyAt(apply.Template, apply.Index)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	data, err := encode(TemplateArgs{Template: apply.Template, Index: index})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: data, InverseEdit: a.Inverse, Touched: []string{templateItem(apply.Template)}, Covers: nil,
	}, nil
}

// UnapplyTemplate is an edit kind that takes a template off an NPC or zone.
type UnapplyTemplate struct {
	Kind, On, Inverse string
	Owned             OwnedOf
}

// Name identifies the edit kind.
func (u UnapplyTemplate) Name() string { return u.Kind }

// Format names the document format the edit applies to.
func (u UnapplyTemplate) Format() string { return u.On }

// Apply takes the template off; its inverse applies it where it was.
func (u UnapplyTemplate) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	owned, err := u.Owned(document)
	if err != nil {
		return model.Applied{}, err
	}
	var unapply TemplateArgs
	if err := json.Unmarshal(args, &unapply); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	index, err := owned.Unapply(unapply.Template)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	data, err := encode(TemplateArgs{Template: unapply.Template, Index: index})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: data, InverseEdit: u.Inverse, Touched: []string{templateItem(unapply.Template)}, Covers: nil,
	}, nil
}

// TemplateItem names an applied template as an edit touches it.
func templateItem(template ids.ULID) string { return "template:" + template.String() }

// TemplateReferences lists the templates applied to an NPC or zone, in the
// world state, so a template shows where it is applied.
func TemplateReferences(owned worldstate.Owned) []model.Reference {
	references := make([]model.Reference, 0, len(owned.Templates))
	for _, template := range owned.Templates {
		references = append(references, model.Reference{
			Document: worldstate.ID(), Item: template.String(), Label: "applied template",
		})
	}

	return references
}

// OwnerEdits returns the edits an NPC or zone plug-in needs for what its
// documents own: a change to its own values, and applying and taking off
// templates, named with a prefix such as "npc".
func OwnerEdits(prefix, format string, owned OwnedOf) []model.EditKind {
	values := func(document model.Document) (*worldstate.Declarations, error) {
		held, err := owned(document)
		if err != nil {
			return nil, err
		}

		return held.Values, nil
	}
	apply, unapply := prefix+".template.apply", prefix+".template.unapply"

	return []model.EditKind{
		Change{Kind: prefix + ".values.change", On: format, List: values},
		ApplyTemplate{Kind: apply, On: format, Inverse: unapply, Owned: owned},
		UnapplyTemplate{Kind: unapply, On: format, Inverse: apply, Owned: owned},
	}
}

// Plugin returns the world-state plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: worldstate.Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits:     []model.EditKind{Change{Kind: ChangeEdit, On: worldstate.FormatName, List: worldValues}},
		Required: []model.RequiredDocument{{
			ID: worldstate.ID(), Format: worldstate.FormatName, Name: worldstate.Name, Data: emptyState,
			ReadOnly: false,
		}},
	}
}
