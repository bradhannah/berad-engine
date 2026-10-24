package tilefloor

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Placement edit kind names.
const (
	AddPlacementEdit    = "tilefloor.placement.add"
	RemovePlacementEdit = "tilefloor.placement.remove"
	UpdatePlacementEdit = "tilefloor.placement.update"
)

// PlacementKey names a placement for undo-clash detection.
func PlacementKey(id ids.ULID) string {
	return "placement:" + id.String()
}

// References lists the NPCs the floor places, so deleting an NPC shows where
// it stands.
func (d *Document) References() []model.Reference {
	placements := d.Floor.Placements()
	references := make([]model.Reference, 0, len(placements))
	for _, placement := range placements {
		references = append(references, model.Reference{
			Document: placement.NPC, Item: "",
			Label: "placed at " + strconv.Itoa(placement.Col) + ", " + strconv.Itoa(placement.Row),
		})
	}

	return references
}

// AddPlacementArgs puts an NPC on the floor at a position in its placement
// order. The orchestrator assigns the placement's ID when it is missing.
type AddPlacementArgs struct {
	Placement floor.Placement `json:"placement"`
	Index     int             `json:"index"`
}

// RemovePlacementArgs takes a placement off the floor.
type RemovePlacementArgs struct {
	ID ids.ULID `json:"id"`
}

// UpdatePlacementArgs moves a placement.
type UpdatePlacementArgs struct {
	Placement floor.Placement `json:"placement"`
}

// AddPlacement is the edit kind that puts an NPC on the floor.
type AddPlacement struct{}

// Name identifies the edit kind.
func (AddPlacement) Name() string { return AddPlacementEdit }

// Format names the document format the edit applies to.
func (AddPlacement) Format() string { return floor.FormatName }

// Prepare gives a new placement its permanent ID.
func (AddPlacement) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	add, err := decodeArgs[AddPlacementArgs](args)
	if err != nil {
		return nil, err
	}
	if !add.Placement.ID.IsZero() {
		return args, nil
	}
	add.Placement.ID, err = newID()
	if err != nil {
		return nil, fmt.Errorf("placement id: %w", err)
	}

	return encodeArgs(add)
}

// Apply puts the NPC on the floor.
func (AddPlacement) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	add, err := decodeArgs[AddPlacementArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	if err := floorDoc.Floor.InsertPlacement(add.Index, add.Placement); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(RemovePlacementArgs{ID: add.Placement.ID})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: RemovePlacementEdit, Touched: []string{PlacementKey(add.Placement.ID)},
		Covers: nil,
	}, nil
}

// RemovePlacement is the edit kind that takes an NPC off the floor.
type RemovePlacement struct{}

// Name identifies the edit kind.
func (RemovePlacement) Name() string { return RemovePlacementEdit }

// Format names the document format the edit applies to.
func (RemovePlacement) Format() string { return floor.FormatName }

// Apply removes the placement; its inverse puts it back in the same place.
func (RemovePlacement) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	remove, err := decodeArgs[RemovePlacementArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	removed, index, err := floorDoc.Floor.RemovePlacement(remove.ID)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(AddPlacementArgs{Placement: removed, Index: index})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: AddPlacementEdit, Touched: []string{PlacementKey(remove.ID)}, Covers: nil,
	}, nil
}

// UpdatePlacement is the edit kind that moves an NPC on the floor.
type UpdatePlacement struct{}

// Name identifies the edit kind.
func (UpdatePlacement) Name() string { return UpdatePlacementEdit }

// Format names the document format the edit applies to.
func (UpdatePlacement) Format() string { return floor.FormatName }

// Apply moves the placement; its inverse moves it back.
func (UpdatePlacement) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	update, err := decodeArgs[UpdatePlacementArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	previous, err := floorDoc.Floor.UpdatePlacement(update.Placement)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(UpdatePlacementArgs{Placement: previous})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: "", Touched: []string{PlacementKey(update.Placement.ID)}, Covers: nil,
	}, nil
}
