// Package zone is the zone document: the live unit of the world, holding its
// floors in order from the bottom up. Each floor is its own document; the zone
// refers to them by ID and the floors do not refer back.
package zone

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// FormatName identifies zones inside saved envelopes.
const FormatName = "zone"

// FormatVersion is the zone format version this package writes.
const FormatVersion = 1

// MaxFloors bounds the floors in one zone.
const MaxFloors = 256

// ErrFloor reports an invalid change to a zone's floors.
var ErrFloor = errors.New("invalid zone floor change")

// ErrData reports saved zone data that cannot be read.
var ErrData = errors.New("invalid zone data")

// Zone lists its floors from the bottom up.
type Zone struct {
	floors []ids.ULID
}

type wireZone struct {
	Floors []ids.ULID `json:"floors"`
}

// New returns a zone with no floors.
func New() *Zone {
	return &Zone{floors: []ids.ULID{}}
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
	return &Zone{floors: slices.Clone(z.floors)}
}

// Equal reports whether two zones list the same floors in the same order.
func (z *Zone) Equal(other *Zone) bool {
	return slices.Equal(z.floors, other.floors)
}

// Encode returns the zone's saved data.
func (z *Zone) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireZone{Floors: z.floors})
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

	return decoded, nil
}

// Format returns the zone format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: nil}
}
