package tilefloor

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Ambient sound edit kind names (increment 14).
const (
	AddSoundEdit    = "tilefloor.sound.add"
	RemoveSoundEdit = "tilefloor.sound.remove"
	UpdateSoundEdit = "tilefloor.sound.update"
)

// SoundKey names an ambient sound for undo-clash detection.
func SoundKey(id ids.ULID) string {
	return "sound:" + id.String()
}

// AddSoundArgs places an ambient sound on the floor at a position in its
// stored order. The orchestrator assigns the sound's ID when it is missing.
type AddSoundArgs struct {
	Sound floor.Sound `json:"sound"`
	Index int         `json:"index"`
}

// RemoveSoundArgs takes a sound off the floor.
type RemoveSoundArgs struct {
	ID ids.ULID `json:"id"`
}

// UpdateSoundArgs moves a sound or changes its cue.
type UpdateSoundArgs struct {
	Sound floor.Sound `json:"sound"`
}

// AddSound is the edit kind that places an ambient sound on the floor.
type AddSound struct{}

// Name identifies the edit kind.
func (AddSound) Name() string { return AddSoundEdit }

// Format names the document format the edit applies to.
func (AddSound) Format() string { return floor.FormatName }

// Prepare gives a new sound its permanent ID.
func (AddSound) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	add, err := decodeArgs[AddSoundArgs](args)
	if err != nil {
		return nil, err
	}
	if !add.Sound.ID.IsZero() {
		return args, nil
	}
	add.Sound.ID, err = newID()
	if err != nil {
		return nil, fmt.Errorf("sound id: %w", err)
	}

	return encodeArgs(add)
}

// Apply places the sound on the floor.
func (AddSound) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	add, err := decodeArgs[AddSoundArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	if err := floorDoc.Floor.InsertSound(add.Index, add.Sound); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(RemoveSoundArgs{ID: add.Sound.ID})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: RemoveSoundEdit, Touched: []string{SoundKey(add.Sound.ID)},
		Covers: nil,
	}, nil
}

// RemoveSound is the edit kind that takes an ambient sound off the floor.
type RemoveSound struct{}

// Name identifies the edit kind.
func (RemoveSound) Name() string { return RemoveSoundEdit }

// Format names the document format the edit applies to.
func (RemoveSound) Format() string { return floor.FormatName }

// Apply removes the sound; its inverse puts it back in the same place.
func (RemoveSound) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	remove, err := decodeArgs[RemoveSoundArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	removed, index, err := floorDoc.Floor.RemoveSound(remove.ID)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(AddSoundArgs{Sound: removed, Index: index})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: AddSoundEdit, Touched: []string{SoundKey(remove.ID)}, Covers: nil,
	}, nil
}

// UpdateSound is the edit kind that moves an ambient sound or changes its cue.
type UpdateSound struct{}

// Name identifies the edit kind.
func (UpdateSound) Name() string { return UpdateSoundEdit }

// Format names the document format the edit applies to.
func (UpdateSound) Format() string { return floor.FormatName }

// Apply moves the sound or changes its cue; its inverse puts it back.
func (UpdateSound) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	update, err := decodeArgs[UpdateSoundArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	previous, err := floorDoc.Floor.UpdateSound(update.Sound)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(UpdateSoundArgs{Sound: previous})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: "", Touched: []string{SoundKey(update.Sound.ID)}, Covers: nil,
	}, nil
}
