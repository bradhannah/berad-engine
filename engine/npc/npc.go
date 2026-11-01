// Package npc is the NPC document: one NPC's definition, kept once however
// many floors place it. It names the workflow that is the NPC's behaviour,
// gives the NPC a path name that workflows read its values by, such as
// george in npc.george.mood, and holds the author's private notes about the
// NPC, which players never see.
package npc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// FormatName identifies NPCs inside saved envelopes.
const FormatName = "npc"

// FormatVersion is the NPC format version this package writes. Version 2
// gave NPCs a path name.
const FormatVersion = 2

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
	// Behaviour is the workflow the NPC follows; zero when none is chosen.
	Behaviour ids.ULID `json:"behaviour,omitzero"`
	// Path is the name workflows read the NPC's values by, such as george;
	// "" until one is given. It follows the rules for value names and is
	// unique in the project.
	Path string `json:"path,omitempty"`
	// Notes are the author's own notes about the NPC, never shown to
	// players.
	Notes string `json:"notes,omitempty"`
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

// SuggestPath suggests a path name from an NPC's display name: lower case,
// with each run of other characters an underscore, so George becomes george
// and Old Tom old_tom. A name with no letters gives npc.
func SuggestPath(display string) string {
	var built strings.Builder
	gap := false
	for _, char := range strings.ToLower(display) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			if gap && built.Len() > 0 {
				built.WriteByte('_')
			}
			built.WriteRune(char)
			gap = false

			continue
		}
		gap = true
	}
	path := built.String()
	if path == "" {
		return "npc"
	}
	if path[0] >= '0' && path[0] <= '9' {
		path = "npc_" + path
	}
	if len(path) > worldstate.MaxName {
		path = strings.TrimRight(path[:worldstate.MaxName], "_")
	}

	return path
}

// UniquePath returns a path name not in taken: the suggestion, or it with a
// number added.
func UniquePath(suggestion string, taken map[string]bool) string {
	if !taken[suggestion] {
		return suggestion
	}
	for number := 2; ; number++ {
		suffix := "_" + strconv.Itoa(number)
		base := suggestion
		if len(base)+len(suffix) > worldstate.MaxName {
			base = base[:worldstate.MaxName-len(suffix)]
		}
		if candidate := base + suffix; !taken[candidate] {
			return candidate
		}
	}
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
		return NPC{Behaviour: ids.ULID{}, Path: "", Notes: ""}, fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := CheckNotes(decoded.Notes); err != nil {
		return NPC{Behaviour: ids.ULID{}, Path: "", Notes: ""}, fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := CheckPath(decoded.Path); err != nil {
		return NPC{Behaviour: ids.ULID{}, Path: "", Notes: ""}, fmt.Errorf("%w: %w", ErrData, err)
	}

	return decoded, nil
}

// Format returns the NPC format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: map[int]content.Migration{1: upgradeFrom1}}
}

// upgradeFrom1 reads a version 1 NPC unchanged: it has no path name until
// its author gives it one.
func upgradeFrom1(data json.RawMessage) (json.RawMessage, error) { return data, nil }
