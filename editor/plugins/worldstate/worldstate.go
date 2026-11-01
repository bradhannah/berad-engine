// Package worldstate is the editor half of the world state: the document
// that declares a project's world, zone and NPC values, and the edits that
// add, change and remove them. Every project holds one world-state document,
// with a fixed ID.
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

// Edit kind names.
const (
	AddValueEdit    = "worldstate.value.add"
	RemoveValueEdit = "worldstate.value.remove"
	SetValueEdit    = "worldstate.value.set"
)

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

// ValueArgs adds or changes one declared value.
type ValueArgs struct {
	Value worldstate.Declaration `json:"value"`
}

// RemoveValueArgs removes one declared value.
type RemoveValueArgs struct {
	ID ids.ULID `json:"id"`
}

// Values returns the list an edit changes, and the scopes it takes values
// in: the world state's, or a workflow's local values. A plug-in whose
// documents declare values gives its edits one.
type Values func(document model.Document) (*worldstate.Declarations, error)

// worldValues is the world-state document's list.
func worldValues(document model.Document) (*worldstate.Declarations, error) {
	stateDoc, ok := document.(*Document)
	if !ok {
		return nil, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotWorldState)
	}

	return stateDoc.State.Values, nil
}

// AddValue is an edit kind that declares a value. Arguments without an ID get
// a permanent one when prepared (D4).
type AddValue struct {
	Kind, On, Inverse string
	List              Values
}

// Name identifies the edit kind.
func (a AddValue) Name() string { return a.Kind }

// Format names the document format the edit applies to.
func (a AddValue) Format() string { return a.On }

// Prepare gives a new value its permanent ID.
func (a AddValue) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	var add ValueArgs
	if err := json.Unmarshal(args, &add); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if !add.Value.ID.IsZero() {
		return args, nil
	}
	made, err := newID()
	if err != nil {
		return nil, fmt.Errorf("new value ID: %w", err)
	}
	add.Value.ID = made

	return encode(add)
}

// Apply declares the value; its inverse removes it.
func (a AddValue) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	values, err := a.List(document)
	if err != nil {
		return model.Applied{}, err
	}
	var add ValueArgs
	if err := json.Unmarshal(args, &add); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if err := values.Insert(add.Value); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}

	return applied(RemoveValueArgs{ID: add.Value.ID}, a.Inverse, add.Value.ID)
}

// RemoveValue is an edit kind that takes a declared value out. Nodes that
// used it are marked as problems; undo brings it back.
type RemoveValue struct {
	Kind, On, Inverse string
	List              Values
}

// Name identifies the edit kind.
func (r RemoveValue) Name() string { return r.Kind }

// Format names the document format the edit applies to.
func (r RemoveValue) Format() string { return r.On }

// Apply removes the value; its inverse declares it again.
func (r RemoveValue) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	values, err := r.List(document)
	if err != nil {
		return model.Applied{}, err
	}
	var remove RemoveValueArgs
	if err := json.Unmarshal(args, &remove); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	removed, err := values.Remove(remove.ID)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}

	return applied(ValueArgs{Value: removed}, r.Inverse, remove.ID)
}

// SetValue is an edit kind that changes a declared value: its name, type,
// choices, default or description.
type SetValue struct {
	Kind, On string
	List     Values
}

// Name identifies the edit kind.
func (s SetValue) Name() string { return s.Kind }

// Format names the document format the edit applies to.
func (s SetValue) Format() string { return s.On }

// Apply changes the value; its inverse puts it back as it was.
func (s SetValue) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	values, err := s.List(document)
	if err != nil {
		return model.Applied{}, err
	}
	var set ValueArgs
	if err := json.Unmarshal(args, &set); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	previous, err := values.Replace(set.Value)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}

	return applied(ValueArgs{Value: previous}, "", set.Value.ID)
}

func applied(inverse any, inverseEdit string, value ids.ULID) (model.Applied, error) {
	data, err := encode(inverse)
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{Inverse: data, InverseEdit: inverseEdit, Touched: []string{value.String()}, Covers: nil}, nil
}

func encode(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode edit arguments: %w", err)
	}

	return data, nil
}

// Edits returns a set of add, remove and change edits for a list of values,
// named with a prefix such as "worldstate.value".
func Edits(prefix, format string, list Values) []model.EditKind {
	add, remove, set := prefix+".add", prefix+".remove", prefix+".set"

	return []model.EditKind{
		AddValue{Kind: add, On: format, Inverse: remove, List: list},
		RemoveValue{Kind: remove, On: format, Inverse: add, List: list},
		SetValue{Kind: set, On: format, List: list},
	}
}

// Plugin returns the world-state plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: worldstate.Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits:     Edits("worldstate.value", worldstate.FormatName, worldValues),
		Required: []model.RequiredDocument{{
			ID: worldstate.ID(), Format: worldstate.FormatName, Name: worldstate.Name, Data: emptyState,
			ReadOnly: false,
		}},
	}
}
