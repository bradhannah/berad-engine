package tilefloor

import (
	"errors"
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// MaxCueLength bounds a sound's cue ID, in bytes.
const MaxCueLength = 256

// ErrSound reports an invalid change to a floor's ambient sounds.
var ErrSound = errors.New("invalid ambient sound")

// ErrNoSound reports a sound ID the floor does not hold.
var ErrNoSound = errors.New("no such ambient sound")

// Sound is an ambient sound placed on a cell: an ambient cue, heard within
// its cue's distance of the cell, louder closer, while the player is on this
// floor. Each has its own permanent ID; a cell holds at most one.
type Sound struct {
	ID  ids.ULID `json:"id"`
	Cue string   `json:"cue"`
	Col int      `json:"col"`
	Row int      `json:"row"`
}

// Sounds returns the floor's ambient sounds in their stored order.
func (f *Floor) Sounds() []Sound {
	return slices.Clone(f.sounds)
}

// Sound returns a sound by ID and its position in the stored order.
func (f *Floor) Sound(id ids.ULID) (Sound, int, bool) {
	index := slices.IndexFunc(f.sounds, func(sound Sound) bool { return sound.ID == id })
	if index < 0 {
		return noSound(), -1, false
	}

	return f.sounds[index], index, true
}

// SoundAt returns the sound on a cell, if any.
func (f *Floor) SoundAt(col, row int) (Sound, bool) {
	for _, sound := range f.sounds {
		if sound.Col == col && sound.Row == row {
			return sound, true
		}
	}

	return noSound(), false
}

func noSound() Sound {
	var none Sound

	return none
}

// InsertSound adds a sound at a position in the stored order; an index past
// the end appends. A sound needs its own ID and a cue, must lie on the floor,
// and may not share a cell with another.
func (f *Floor) InsertSound(index int, sound Sound) error {
	if sound.ID.IsZero() {
		return fmt.Errorf("%w: missing ID", ErrSound)
	}
	if _, _, exists := f.Sound(sound.ID); exists {
		return fmt.Errorf("%w: ID %s already used", ErrSound, sound.ID)
	}
	if err := f.checkSound(sound, ids.ULID{}); err != nil {
		return err
	}
	index = min(max(index, 0), len(f.sounds))
	f.sounds = slices.Insert(f.sounds, index, sound)

	return nil
}

// RemoveSound deletes a sound and returns it with its former position.
func (f *Floor) RemoveSound(id ids.ULID) (Sound, int, error) {
	sound, index, ok := f.Sound(id)
	if !ok {
		return noSound(), -1, fmt.Errorf("%w: %s", ErrNoSound, id)
	}
	f.sounds = slices.Delete(f.sounds, index, index+1)

	return sound, index, nil
}

// UpdateSound moves a sound or changes its cue, returning its previous state.
func (f *Floor) UpdateSound(changed Sound) (Sound, error) {
	previous, index, ok := f.Sound(changed.ID)
	if !ok {
		return noSound(), fmt.Errorf("%w: %s", ErrNoSound, changed.ID)
	}
	if err := f.checkSound(changed, changed.ID); err != nil {
		return noSound(), err
	}
	f.sounds[index] = changed

	return previous, nil
}

// checkSound validates a sound, ignoring the sound with the given ID when
// checking cells.
func (f *Floor) checkSound(sound Sound, self ids.ULID) error {
	if sound.Cue == "" || len(sound.Cue) > MaxCueLength {
		return fmt.Errorf("%w: a sound names a cue of 1 to %d bytes", ErrSound, MaxCueLength)
	}
	if !f.In(sound.Col, sound.Row) {
		return fmt.Errorf("%w: (%d, %d) is outside the floor", ErrSound, sound.Col, sound.Row)
	}
	for _, other := range f.sounds {
		if other.ID != self && other.Col == sound.Col && other.Row == sound.Row {
			return fmt.Errorf("%w: another sound is already at (%d, %d)", ErrSound, sound.Col, sound.Row)
		}
	}

	return nil
}
