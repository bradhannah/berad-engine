// Package npc is the editor half of NPCs: the NPC document and the edits that
// choose the zone it lives in and its behaviour workflow, give it a path name, change its private
// notes, declare its own values and apply templates to it.
package npc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// PluginName identifies the plug-in.
const PluginName = "npc"

// Edit kind names.
const (
	SetZoneEdit      = "npc.zone.set"
	SetBehaviourEdit = "npc.behaviour.set"
	SetSpriteEdit    = "npc.sprite.set"
	SetNotesEdit     = "npc.notes.set"
	SetPathEdit      = "npc.path.set"
	// ChangeValuesEdit changes the NPC's own values; ApplyTemplateEdit and
	// UnapplyTemplateEdit put a template on it and take one off.
	ChangeValuesEdit    = "npc.values.change"
	ApplyTemplateEdit   = "npc.template.apply"
	UnapplyTemplateEdit = "npc.template.unapply"
	// SetConstantEdit sets or clears its own value of a constant of one of
	// its templates.
	SetConstantEdit = "npc.constant.set"
)

// Undo-clash items.
const (
	zoneItem      = "zone"
	behaviourItem = "behaviour"
	spriteItem    = "sprite"
	notesItem     = "notes"
	pathItem      = "path"
)

// ErrNotNPC reports a document that is not an NPC.
var ErrNotNPC = errors.New("document is not an NPC")

// Document is an NPC held by the editor.
type Document struct {
	NPC npc.NPC
}

// NewDocument returns an NPC with no behaviour and no notes.
func NewDocument() *Document {
	return &Document{NPC: npc.New()}
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
	return &Document{NPC: d.NPC.Clone()}
}

// LivesHere labels an NPC's reference to the zone it lives in.
const LivesHere = "lives here"

// References lists the NPC's zone, behaviour workflow and sprite, so
// deleting one shows which NPCs use it, and the templates applied to it.
func (d *Document) References() []model.Reference {
	references := stateplugin.TemplateReferences(d.NPC.Owned)
	if !d.NPC.Zone.IsZero() {
		references = append(references, model.Reference{Document: d.NPC.Zone, Item: "", Label: LivesHere})
	}
	if !d.NPC.Behaviour.IsZero() {
		references = append(references, model.Reference{Document: d.NPC.Behaviour, Item: "", Label: "behaviour"})
	}
	if !d.NPC.Sprite.IsZero() {
		references = append(references, model.Reference{Document: d.NPC.Sprite, Item: "", Label: "sprite"})
	}

	return references
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

// SetZoneArgs chooses the zone the NPC lives in; zero is none.
type SetZoneArgs struct {
	Zone ids.ULID `json:"zone"`
}

// SetBehaviourArgs chooses the NPC's behaviour workflow; zero clears it.
type SetBehaviourArgs struct {
	Behaviour ids.ULID `json:"behaviour"`
}

// SetSpriteArgs chooses the sprite the NPC shows; zero goes back to its token.
type SetSpriteArgs struct {
	Sprite ids.ULID `json:"sprite"`
}

// SetNotesArgs replaces the NPC's notes.
type SetNotesArgs struct {
	Notes string `json:"notes"`
}

// SetPathArgs gives the NPC a path name; "" takes it away.
type SetPathArgs struct {
	Path string `json:"path"`
}

// SetPath is the edit kind that gives an NPC its path name.
type SetPath struct{}

// Name identifies the edit kind.
func (SetPath) Name() string { return SetPathEdit }

// Format names the document format the edit applies to.
func (SetPath) Format() string { return npc.FormatName }

// Apply sets the path name; its inverse restores the previous one.
func (SetPath) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set SetPathArgs

	return change(document, args, &set, pathItem, func(changed *npc.NPC) (any, error) {
		if err := npc.CheckPath(set.Path); err != nil {
			return nil, fmt.Errorf("set path name: %w", err)
		}
		previous := SetPathArgs{Path: changed.Path}
		changed.Path = set.Path

		return previous, nil
	})
}

// SetZone is the edit kind that chooses the zone an NPC lives in.
type SetZone struct{}

// Name identifies the edit kind.
func (SetZone) Name() string { return SetZoneEdit }

// Format names the document format the edit applies to.
func (SetZone) Format() string { return npc.FormatName }

// Apply sets the zone; its inverse restores the previous one.
func (SetZone) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set SetZoneArgs

	return change(document, args, &set, zoneItem, func(changed *npc.NPC) (any, error) {
		previous := SetZoneArgs{Zone: changed.Zone}
		changed.Zone = set.Zone

		return previous, nil
	})
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

// SetSprite is the edit kind that chooses the sprite an NPC shows.
type SetSprite struct{}

// Name identifies the edit kind.
func (SetSprite) Name() string { return SetSpriteEdit }

// Format names the document format the edit applies to.
func (SetSprite) Format() string { return npc.FormatName }

// Apply sets the sprite; its inverse restores the previous one.
func (SetSprite) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set SetSpriteArgs

	return change(document, args, &set, spriteItem, func(changed *npc.NPC) (any, error) {
		previous := SetSpriteArgs{Sprite: changed.Sprite}
		changed.Sprite = set.Sprite

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

// owned is what an NPC document owns, for the edits that change it.
func owned(document model.Document) (*worldstate.Owned, error) {
	npcDoc, ok := document.(*Document)
	if !ok {
		return nil, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotNPC)
	}

	return &npcDoc.NPC.Owned, nil
}

// Plugin returns the NPC plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: npc.Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits: append([]model.EditKind{SetZone{}, SetBehaviour{}, SetSprite{}, SetNotes{}, SetPath{}},
			stateplugin.OwnerEdits("npc", npc.FormatName, owned)...),
		Required: nil,
	}
}
