// Package npc is the NPC document: one NPC's definition, placed once in the
// whole project. It names the zone the NPC lives in, the workflow that is
// its behaviour and the sprite it shows, gives the NPC a path name that workflows read its values
// by, such as george in npc.george.secret, holds the values the NPC owns and
// the templates applied to it, and the author's private notes about the NPC,
// which players never see.
package npc

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// FormatName identifies NPCs inside saved envelopes.
const FormatName = "npc"

// FormatVersion is the NPC format version this package writes. Version 2
// added the sprite an NPC shows, version 3 a path name, version 4 the NPC's
// own values and templates, version 5 the zone it lives in, and version 6
// its own values of its templates' constants; each reads the version
// before it unchanged.
const FormatVersion = 6

// MaxNotes bounds an NPC's notes, in characters.
const MaxNotes = 20000

// ErrData reports saved NPC data that cannot be read.
var ErrData = errors.New("invalid NPC data")

// ErrNotes reports notes that are too long.
var ErrNotes = errors.New("invalid NPC notes")

// ErrPath reports a path name that is not fit for a path.
var ErrPath = errors.New("invalid NPC path name")

// NPC is one NPC's definition.
type NPC struct {
	// Zone is the zone the NPC lives in: it is placed only on that zone's
	// floors and goes only to places on them. Zero is none yet.
	Zone ids.ULID `json:"zone,omitzero"`
	// Behaviour is the workflow the NPC follows; zero when none is chosen.
	Behaviour ids.ULID `json:"behaviour,omitzero"`
	// Sprite is the sprite the NPC shows; zero for its token.
	Sprite ids.ULID `json:"sprite,omitzero"`
	// Path is the name workflows read the NPC's values by, such as george;
	// "" until one is given. It follows the rules for value names and is
	// unique in the project.
	Path string `json:"path,omitempty"`
	// Notes are the author's own notes about the NPC, never shown to
	// players.
	Notes string `json:"notes,omitempty"`
	// Owned is the NPC's own values and the templates applied to it.
	Owned worldstate.Owned `json:"-"`
}

// New returns an NPC with nothing set.
func New() NPC {
	return NPC{
		Zone: ids.ULID{}, Behaviour: ids.ULID{}, Sprite: ids.ULID{}, Path: "", Notes: "",
		Owned: worldstate.NewOwned(worldstate.ScopeNPC),
	}
}

// Clone returns an independent copy.
func (n NPC) Clone() NPC {
	n.Owned = n.Owned.Clone()

	return n
}

// Equal reports whether two NPCs are the same.
func (n NPC) Equal(other NPC) bool {
	return n.Zone == other.Zone && n.Behaviour == other.Behaviour && n.Sprite == other.Sprite && n.Path == other.Path &&
		n.Notes == other.Notes && n.Owned.Equal(other.Owned)
}

type wireNPC struct {
	Zone      ids.ULID `json:"zone,omitzero"`
	Behaviour ids.ULID `json:"behaviour,omitzero"`
	Sprite    ids.ULID `json:"sprite,omitzero"`
	Path      string   `json:"path,omitempty"`
	Notes     string   `json:"notes,omitempty"`
	worldstate.WireOwned
}

// CheckPath reports a path name that is not fit for a path; "" is none yet.
func CheckPath(path string) error {
	if path == "" {
		return nil
	}
	if err := worldstate.CheckName(path); err != nil {
		return fmt.Errorf("%w: %w", ErrPath, err)
	}

	return nil
}

// SuggestPath suggests a path name from an NPC's display name, as
// worldstate.SuggestName does, so George becomes george and Old Tom
// old_tom. A name with no letters gives npc.
func SuggestPath(display string) string { return worldstate.SuggestName(display, "npc") }

// UniquePath returns a path name not in taken: the suggestion, or it with a
// number added.
func UniquePath(suggestion string, taken map[string]bool) string {
	return worldstate.UniqueName(suggestion, taken)
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
	owned, err := n.Owned.Wire()
	if err != nil {
		return nil, fmt.Errorf("encode NPC: %w", err)
	}
	data, err := json.Marshal(wireNPC{
		Zone: n.Zone, Behaviour: n.Behaviour, Sprite: n.Sprite, Path: n.Path, Notes: n.Notes, WireOwned: owned,
	})
	if err != nil {
		return nil, fmt.Errorf("encode NPC: %w", err)
	}

	return data, nil
}

// Decode reads saved NPC data.
func Decode(data json.RawMessage) (NPC, error) {
	var wire wireNPC
	if err := json.Unmarshal(data, &wire); err != nil {
		return New(), fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := CheckNotes(wire.Notes); err != nil {
		return New(), fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := CheckPath(wire.Path); err != nil {
		return New(), fmt.Errorf("%w: %w", ErrData, err)
	}
	owned, err := worldstate.ReadOwned(worldstate.ScopeNPC, wire.WireOwned)
	if err != nil {
		return New(), fmt.Errorf("%w: %w", ErrData, err)
	}

	return NPC{
		Zone: wire.Zone, Behaviour: wire.Behaviour, Sprite: wire.Sprite, Path: wire.Path, Notes: wire.Notes,
		Owned: owned,
	}, nil
}

// Format returns the NPC format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: map[int]content.Migration{
		1: unchanged, 2: unchanged, 3: unchanged, 4: unchanged, 5: unchanged,
	}}
}

// unchanged reads an older NPC as it is: one from version 1 shows its token,
// one from version 2 has no path name until its author gives it one, one
// from version 3 owns no values yet, one from version 4 lives in no zone
// until its author or the editor gives it one, and one from version 5 has
// the default of every constant.
func unchanged(data json.RawMessage) (json.RawMessage, error) { return data, nil }
