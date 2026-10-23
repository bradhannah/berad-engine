// Package npc is the NPC document: one NPC's definition, kept once however
// many floors place it. It names the workflow that is the NPC's behaviour and
// holds the author's private notes about the NPC, which players never see.
package npc

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// FormatName identifies NPCs inside saved envelopes.
const FormatName = "npc"

// FormatVersion is the NPC format version this package writes.
const FormatVersion = 1

// MaxNotes bounds an NPC's notes, in characters.
const MaxNotes = 20000

// ErrData reports saved NPC data that cannot be read.
var ErrData = errors.New("invalid NPC data")

// ErrNotes reports notes that are too long.
var ErrNotes = errors.New("invalid NPC notes")

// NPC is one NPC's definition.
type NPC struct {
	// Behaviour is the workflow the NPC follows; zero when none is chosen.
	Behaviour ids.ULID `json:"behaviour,omitzero"`
	// Notes are the author's own notes about the NPC, never shown to
	// players.
	Notes string `json:"notes,omitempty"`
}

// CheckNotes reports notes that are too long.
func CheckNotes(notes string) error {
	if utf8.RuneCountInString(notes) > MaxNotes {
		return fmt.Errorf("%w: longer than %d characters", ErrNotes, MaxNotes)
	}

	return nil
}

// Encode returns the NPC's saved data.
func (n NPC) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("encode NPC: %w", err)
	}

	return data, nil
}

// Decode reads saved NPC data.
func Decode(data json.RawMessage) (NPC, error) {
	var decoded NPC
	if err := json.Unmarshal(data, &decoded); err != nil {
		return NPC{Behaviour: ids.ULID{}, Notes: ""}, fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := CheckNotes(decoded.Notes); err != nil {
		return NPC{Behaviour: ids.ULID{}, Notes: ""}, fmt.Errorf("%w: %w", ErrData, err)
	}

	return decoded, nil
}

// Format returns the NPC format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: nil}
}
