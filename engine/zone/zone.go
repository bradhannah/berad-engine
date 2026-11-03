// Package zone is the zone document: the live unit of the world, holding its
// floors in order from the bottom up. Each floor is its own document; the zone
// refers to them by ID and the floors do not refer back. A zone also lists the
// palettes its art uses, which the pixel editor offers first; it is a
// convenience, not a limit. It lists its sounds: the cues, such as music or a
// bed, that play the whole time the player is in it. And a zone has a path
// name that workflows read its values by, such as market in
// zone.market.crowded, holds the values it owns and the templates applied
// to it, and keeps the author's private notes about it, which players
// never see.
package zone

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// FormatName identifies zones inside saved envelopes.
const FormatName = "zone"

// FormatVersion is the zone format version this package writes. Version 2
// added the palettes a zone uses, version 3 its sounds, version 4 a path
// name and the zone's own values and templates, and version 5 its notes;
// each reads the version before it unchanged.
const FormatVersion = 5

// MaxNotes bounds a zone's notes, in characters.
const MaxNotes = 20000

// ErrNotes reports notes that are too long.
var ErrNotes = errors.New("invalid zone notes")

// MaxFloors bounds the floors in one zone.
const MaxFloors = 256

// MaxPalettes bounds the palettes one zone lists.
const MaxPalettes = 256

// MaxSounds bounds the sounds one zone lists, and MaxCueLength a cue's ID.
const (
	MaxSounds    = 16
	MaxCueLength = 256
)

// ErrSound reports an invalid change to the sounds a zone lists.
var ErrSound = errors.New("invalid zone sound change")

// ErrFloor reports an invalid change to a zone's floors.
var ErrFloor = errors.New("invalid zone floor change")

// ErrData reports saved zone data that cannot be read.
var ErrData = errors.New("invalid zone data")

// ErrPalette reports an invalid change to the palettes a zone lists.
var ErrPalette = errors.New("invalid zone palette change")

// Zone lists its floors from the bottom up, the palettes it uses and its
// sounds.
type Zone struct {
	floors   []ids.ULID
	palettes []ids.ULID
	sounds   []string
	// Path is the name workflows read the zone's values by, such as market;
	// "" until one is given. It follows the rules for value names and is
	// unique in the project.
	Path string
	// Notes are the author's own notes about the zone, never shown to
	// players.
	Notes string
	// Owned is the zone's own values and the templates applied to it.
	Owned worldstate.Owned
}

type wireZone struct {
	Floors   []ids.ULID `json:"floors"`
	Palettes []ids.ULID `json:"palettes,omitempty"`
	Sounds   []string   `json:"sounds,omitempty"`
	Path     string     `json:"path,omitempty"`
	Notes    string     `json:"notes,omitempty"`
	worldstate.WireOwned
}

// New returns a zone with no floors.
func New() *Zone {
	return &Zone{
		floors: []ids.ULID{}, palettes: nil, sounds: nil, Path: "", Notes: "",
		Owned: worldstate.NewOwned(worldstate.ScopeZone),
	}
}

// Sounds returns the cues the zone plays throughout, in the order they were
// added.
func (z *Zone) Sounds() []string {
	return slices.Clone(z.sounds)
}

// InsertSound lists a cue at a position; an index past the end appends.
func (z *Zone) InsertSound(index int, cue string) error {
	if cue == "" || len(cue) > MaxCueLength || slices.Contains(z.sounds, cue) {
		return fmt.Errorf("%w: %q is missing, too long or already listed", ErrSound, cue)
	}
	if len(z.sounds) >= MaxSounds {
		return fmt.Errorf("%w: a zone lists at most %d sounds", ErrSound, MaxSounds)
	}
	index = min(max(index, 0), len(z.sounds))
	z.sounds = slices.Insert(slices.Clone(z.sounds), index, cue)

	return nil
}

// RemoveSound takes a cue off the zone's list and returns where it was.
func (z *Zone) RemoveSound(cue string) (int, error) {
	index := slices.Index(z.sounds, cue)
	if index < 0 {
		return -1, fmt.Errorf("%w: %q is not listed", ErrSound, cue)
	}
	z.sounds = slices.Delete(slices.Clone(z.sounds), index, index+1)

	return index, nil
}

// Palettes returns the palettes the zone lists, in the order they were added.
func (z *Zone) Palettes() []ids.ULID {
	return slices.Clone(z.palettes)
}

// InsertPalette lists a palette at a position; an index past the end appends.
func (z *Zone) InsertPalette(index int, palette ids.ULID) error {
	if palette.IsZero() || slices.Contains(z.palettes, palette) {
		return fmt.Errorf("%w: %s is missing or already listed", ErrPalette, palette)
	}
	if len(z.palettes) >= MaxPalettes {
		return fmt.Errorf("%w: a zone lists at most %d palettes", ErrPalette, MaxPalettes)
	}
	index = min(max(index, 0), len(z.palettes))
	z.palettes = slices.Insert(slices.Clone(z.palettes), index, palette)

	return nil
}

// RemovePalette takes a palette off the zone's list and returns where it was.
func (z *Zone) RemovePalette(palette ids.ULID) (int, error) {
	index := slices.Index(z.palettes, palette)
	if index < 0 {
		return -1, fmt.Errorf("%w: %s is not listed", ErrPalette, palette)
	}
	z.palettes = slices.Delete(slices.Clone(z.palettes), index, index+1)

	return index, nil
}

// Floors returns the floor IDs from the bottom up.
func (z *Zone) Floors() []ids.ULID {
	return slices.Clone(z.floors)
}

// Index returns a floor's position from the bottom, or -1.
func (z *Zone) Index(floor ids.ULID) int {
	return slices.Index(z.floors, floor)
}

// Below returns the floor directly under another one.
func (z *Zone) Below(floor ids.ULID) (ids.ULID, bool) {
	index := z.Index(floor)
	if index <= 0 {
		return ids.ULID{}, false
	}

	return z.floors[index-1], true
}

// Insert adds a floor at a position from the bottom; an index past the top
// appends.
func (z *Zone) Insert(index int, floor ids.ULID) error {
	if floor.IsZero() || z.Index(floor) >= 0 {
		return fmt.Errorf("%w: %s is missing or already in the zone", ErrFloor, floor)
	}
	if len(z.floors) >= MaxFloors {
		return fmt.Errorf("%w: a zone holds at most %d floors", ErrFloor, MaxFloors)
	}
	index = min(max(index, 0), len(z.floors))
	z.floors = slices.Insert(z.floors, index, floor)

	return nil
}

// Remove takes a floor out of the zone and returns where it was.
func (z *Zone) Remove(floor ids.ULID) (int, error) {
	index := z.Index(floor)
	if index < 0 {
		return -1, fmt.Errorf("%w: %s is not in the zone", ErrFloor, floor)
	}
	z.floors = slices.Delete(z.floors, index, index+1)

	return index, nil
}

// Move puts a floor at a new position and returns its previous one.
func (z *Zone) Move(floor ids.ULID, index int) (int, error) {
	previous, err := z.Remove(floor)
	if err != nil {
		return -1, err
	}
	index = min(max(index, 0), len(z.floors))
	z.floors = slices.Insert(z.floors, index, floor)

	return previous, nil
}

// Clone returns an independent copy.
func (z *Zone) Clone() *Zone {
	return &Zone{
		floors: slices.Clone(z.floors), palettes: slices.Clone(z.palettes), sounds: slices.Clone(z.sounds),
		Path: z.Path, Notes: z.Notes, Owned: z.Owned.Clone(),
	}
}

// Equal reports whether two zones list the same floors, palettes and sounds
// in the same order, with the same path name, notes and values.
func (z *Zone) Equal(other *Zone) bool {
	return slices.Equal(z.floors, other.floors) && slices.Equal(z.palettes, other.palettes) &&
		slices.Equal(z.sounds, other.sounds) && z.Path == other.Path && z.Notes == other.Notes &&
		z.Owned.Equal(other.Owned)
}

// CheckNotes reports notes that are too long.
func CheckNotes(notes string) error {
	if utf8.RuneCountInString(notes) > MaxNotes {
		return fmt.Errorf("%w: longer than %d characters", ErrNotes, MaxNotes)
	}

	return nil
}

// CheckPath reports a path name that is not fit for a path; "" is none yet.
func CheckPath(path string) error {
	if path == "" {
		return nil
	}
	if err := worldstate.CheckName(path); err != nil {
		return fmt.Errorf("%w: %w", ErrData, err)
	}

	return nil
}

// SuggestPath suggests a path name from a zone's display name, as
// worldstate.SuggestName does, so Market Square becomes market_square. A
// name with no letters gives zone.
func SuggestPath(display string) string { return worldstate.SuggestName(display, "zone") }

// Encode returns the zone's saved data.
func (z *Zone) Encode() (json.RawMessage, error) {
	owned, err := z.Owned.Wire()
	if err != nil {
		return nil, fmt.Errorf("encode zone: %w", err)
	}
	data, err := json.Marshal(wireZone{
		Floors: z.floors, Palettes: z.palettes, Sounds: z.sounds, Path: z.Path, Notes: z.Notes, WireOwned: owned,
	})
	if err != nil {
		return nil, fmt.Errorf("encode zone: %w", err)
	}

	return data, nil
}

// Decode reads saved zone data.
func Decode(data json.RawMessage) (*Zone, error) {
	var wire wireZone
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	decoded := New()
	for _, floor := range wire.Floors {
		if err := decoded.Insert(len(decoded.floors), floor); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
	}
	for _, palette := range wire.Palettes {
		if err := decoded.InsertPalette(len(decoded.palettes), palette); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
	}
	for _, cue := range wire.Sounds {
		if err := decoded.InsertSound(len(decoded.sounds), cue); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
	}
	if err := CheckPath(wire.Path); err != nil {
		return nil, err
	}
	if err := CheckNotes(wire.Notes); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	owned, err := worldstate.ReadOwned(worldstate.ScopeZone, wire.WireOwned)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	decoded.Path, decoded.Notes, decoded.Owned = wire.Path, wire.Notes, owned

	return decoded, nil
}

// Format returns the zone format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: map[int]content.Migration{
		1: unchanged, 2: unchanged, 3: unchanged, 4: unchanged,
	}}
}

// unchanged upgrades a version that the next reads as it is.
func unchanged(data json.RawMessage) (json.RawMessage, error) { return data, nil }
