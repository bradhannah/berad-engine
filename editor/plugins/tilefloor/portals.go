package tilefloor

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Portal edit kind names (increment 64).
const (
	AddPortalEdit    = "tilefloor.portal.add"
	RemovePortalEdit = "tilefloor.portal.remove"
	UpdatePortalEdit = "tilefloor.portal.update"
)

// PortalLabel starts what a where-used list says of a portal that leads to
// an anchor; its cell follows.
const PortalLabel = "a portal at "

// PortalKey names a portal for undo-clash detection.
func PortalKey(id ids.ULID) string {
	return "portal:" + id.String()
}

// AddPortalArgs puts a portal on the floor at a position in its
// stored order. The orchestrator assigns the portal's ID when it is missing.
type AddPortalArgs struct {
	Portal floor.Portal `json:"portal"`
	Index  int          `json:"index"`
}

// RemovePortalArgs takes a portal off the floor.
type RemovePortalArgs struct {
	ID ids.ULID `json:"id"`
}

// UpdatePortalArgs moves a portal, or changes where it leads or how the
// screen changes.
type UpdatePortalArgs struct {
	Portal floor.Portal `json:"portal"`
}

// AddPortal is the edit kind that puts a portal on the floor.
type AddPortal struct{}

// Name identifies the edit kind.
func (AddPortal) Name() string { return AddPortalEdit }

// Format names the document format the edit applies to.
func (AddPortal) Format() string { return floor.FormatName }

// Prepare gives a new portal its permanent ID.
func (AddPortal) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	add, err := decodeArgs[AddPortalArgs](args)
	if err != nil {
		return nil, err
	}
	if !add.Portal.ID.IsZero() {
		return args, nil
	}
	add.Portal.ID, err = newID()
	if err != nil {
		return nil, fmt.Errorf("portal id: %w", err)
	}

	return encodeArgs(add)
}

// Apply puts the portal on the floor.
func (AddPortal) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	add, err := decodeArgs[AddPortalArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	if err := floorDoc.Floor.InsertPortal(add.Index, add.Portal); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(RemovePortalArgs{ID: add.Portal.ID})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: RemovePortalEdit, Touched: []string{PortalKey(add.Portal.ID)},
		Covers: nil,
	}, nil
}

// RemovePortal is the edit kind that takes a portal off the floor.
type RemovePortal struct{}

// Name identifies the edit kind.
func (RemovePortal) Name() string { return RemovePortalEdit }

// Format names the document format the edit applies to.
func (RemovePortal) Format() string { return floor.FormatName }

// Apply removes the portal; its inverse puts it back in the same place.
func (RemovePortal) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	remove, err := decodeArgs[RemovePortalArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	removed, index, err := floorDoc.Floor.RemovePortal(remove.ID)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(AddPortalArgs{Portal: removed, Index: index})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: AddPortalEdit, Touched: []string{PortalKey(remove.ID)}, Covers: nil,
	}, nil
}

// UpdatePortal is the edit kind that moves a portal, or changes where it leads
// or how the screen changes.
type UpdatePortal struct{}

// Name identifies the edit kind.
func (UpdatePortal) Name() string { return UpdatePortalEdit }

// Format names the document format the edit applies to.
func (UpdatePortal) Format() string { return floor.FormatName }

// Apply changes the portal; its inverse puts it back.
func (UpdatePortal) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	update, err := decodeArgs[UpdatePortalArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	previous, err := floorDoc.Floor.UpdatePortal(update.Portal)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(UpdatePortalArgs{Portal: previous})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: "", Touched: []string{PortalKey(update.Portal.ID)}, Covers: nil,
	}, nil
}
