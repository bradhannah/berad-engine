// Package npc is the editor half of NPCs: the NPC document and the edits that
// choose its behaviour workflow and change its private notes.
package npc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
)

// PluginName identifies the plug-in.
const PluginName = "npc"

// Edit kind names.
const (
	SetBehaviourEdit = "npc.behaviour.set"
	SetNotesEdit     = "npc.notes.set"
)

// Undo-clash items.
const (
	behaviourItem = "behaviour"
	notesItem     = "notes"
)

// ErrNotNPC reports a document that is not an NPC.
var ErrNotNPC = errors.New("document is not an NPC")

// Document is an NPC held by the editor.
type Document struct {
	NPC npc.NPC
}

// NewDocument returns an NPC with no behaviour and no notes.
func NewDocument() *Document {
	return &Document{NPC: npc.NPC{Behaviour: ids.ULID{}, Notes: ""}}
}

// Format names the document format.
func (d *Document) Format() string { return npc.FormatName }

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	data, err := d.NPC.Encode()
	if err != nil {
		return nil, fmt.Errorf("NPC document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document {
	return &Document{NPC: d.NPC}
}

// References lists the NPC's behaviour workflow, so deleting a workflow
// shows which NPCs follow it.
func (d *Document) References() []model.Reference {
	if d.NPC.Behaviour.IsZero() {
		return nil
	}

	return []model.Reference{{Document: d.NPC.Behaviour, Item: "", Label: "behaviour"}}
}

// Decode reads a saved NPC.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	decoded, err := npc.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("NPC document: %w", err)
	}

	return &Document{NPC: decoded}, nil
}

// SetBehaviourArgs chooses the NPC's behaviour workflow; zero clears it.
type SetBehaviourArgs struct {
	Behaviour ids.ULID `json:"behaviour"`
}

// SetNotesArgs replaces the NPC's notes.
type SetNotesArgs struct {
	Notes string `json:"notes"`
}

// SetBehaviour is the edit kind that chooses an NPC's behaviour.
type SetBehaviour struct{}

// Name identifies the edit kind.
func (SetBehaviour) Name() string { return SetBehaviourEdit }

// Format names the document format the edit applies to.
func (SetBehaviour) Format() string { return npc.FormatName }

// Apply sets the behaviour; its inverse restores the previous one.
func (SetBehaviour) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set SetBehaviourArgs

	return change(document, args, &set, behaviourItem, func(changed *npc.NPC) (any, error) {
		previous := SetBehaviourArgs{Behaviour: changed.Behaviour}
		changed.Behaviour = set.Behaviour

		return previous, nil
	})
}

// SetNotes is the edit kind that changes an NPC's notes.
type SetNotes struct{}

// Name identifies the edit kind.
func (SetNotes) Name() string { return SetNotesEdit }

// Format names the document format the edit applies to.
func (SetNotes) Format() string { return npc.FormatName }

// Apply sets the notes; its inverse restores the previous notes.
func (SetNotes) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set SetNotesArgs

	return change(document, args, &set, notesItem, func(changed *npc.NPC) (any, error) {
		if err := npc.CheckNotes(set.Notes); err != nil {
			return nil, fmt.Errorf("set notes: %w", err)
		}
		previous := SetNotesArgs{Notes: changed.Notes}
		changed.Notes = set.Notes

		return previous, nil
	})
}

func change(
	document model.Document, args json.RawMessage, target any, item string, apply func(*npc.NPC) (any, error),
) (model.Applied, error) {
	npcDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotNPC)
	}
	if err := json.Unmarshal(args, target); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := apply(&npcDoc.NPC)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseArgs, err := json.Marshal(inverse)
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{Inverse: inverseArgs, InverseEdit: "", Touched: []string{item}, Covers: nil}, nil
}

// Plugin returns the NPC plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: npc.Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits:     []model.EditKind{SetBehaviour{}, SetNotes{}},
	}
}
