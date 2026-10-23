// Package content defines the versioned envelope every saved document uses and
// the step-by-step upgrades that bring old documents to the current version.
//
// Games and the editor share this package: a game upgrades documents in memory
// when loading them, and the editor additionally saves the upgraded result.
package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrMalformed reports bytes that are not a valid envelope.
var ErrMalformed = errors.New("malformed document")

// ErrUnknownFormat reports a document whose format is not registered.
var ErrUnknownFormat = errors.New("unknown document format")

// ErrNewerVersion reports a document written by a newer version of its format.
var ErrNewerVersion = errors.New("document is newer than this program supports")

// ErrBadFormat reports an invalid format registration.
var ErrBadFormat = errors.New("invalid format registration")

// ErrDuplicateFormat reports a format name registered twice.
var ErrDuplicateFormat = errors.New("format already registered")

// ErrMigration reports a failed upgrade step.
var ErrMigration = errors.New("document upgrade failed")

// Envelope wraps every saved document with its format, format version,
// permanent ID, display name and revision.
type Envelope struct {
	Format   string          `json:"format"`
	Version  int             `json:"version"`
	ID       ids.ULID        `json:"id"`
	Name     string          `json:"name"`
	Revision uint64          `json:"revision"`
	Data     json.RawMessage `json:"data"`
}

// Migration upgrades a document's data by exactly one version.
type Migration func(data json.RawMessage) (json.RawMessage, error)

// Format describes a document format: its current version and the migrations
// that upgrade each older version to the next.
type Format struct {
	// Name identifies the format inside saved envelopes.
	Name string
	// Current is the version this program writes. Versions start at 1.
	Current int
	// Migrations maps a version to the step that upgrades it to version+1.
	// Every version from 1 to Current-1 needs one.
	Migrations map[int]Migration
}

// Registry holds the formats a program understands.
type Registry struct {
	formats map[string]Format
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{formats: map[string]Format{}}
}

// Register adds a format. Names must be unique and every upgrade step present.
func (r *Registry) Register(format Format) error {
	if format.Name == "" || format.Current < 1 {
		return fmt.Errorf("%w: %q version %d", ErrBadFormat, format.Name, format.Current)
	}
	if _, exists := r.formats[format.Name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateFormat, format.Name)
	}
	for version := 1; version < format.Current; version++ {
		if format.Migrations[version] == nil {
			return fmt.Errorf("%w: %q has no upgrade from version %d", ErrBadFormat, format.Name, version)
		}
	}
	r.formats[format.Name] = format

	return nil
}

// Lookup returns a registered format.
func (r *Registry) Lookup(name string) (Format, bool) {
	format, ok := r.formats[name]

	return format, ok
}

// Names returns the registered format names in sorted order.
func (r *Registry) Names() []string {
	return slices.Sorted(maps.Keys(r.formats))
}

// Upgrade brings an envelope to its format's current version, one step at a
// time. It reports whether anything changed.
func (r *Registry) Upgrade(envelope Envelope) (Envelope, bool, error) {
	format, ok := r.formats[envelope.Format]
	if !ok {
		return envelope, false, fmt.Errorf("%w: %q", ErrUnknownFormat, envelope.Format)
	}
	if envelope.Version < 1 {
		return envelope, false, fmt.Errorf("%w: version %d", ErrMalformed, envelope.Version)
	}
	if envelope.Version > format.Current {
		return envelope, false, fmt.Errorf("%w: %q version %d > %d",
			ErrNewerVersion, envelope.Format, envelope.Version, format.Current)
	}
	upgraded := envelope.Version < format.Current
	for envelope.Version < format.Current {
		data, err := format.Migrations[envelope.Version](envelope.Data)
		if err != nil {
			return envelope, false, fmt.Errorf("%w: %q from version %d: %w",
				ErrMigration, envelope.Format, envelope.Version, err)
		}
		envelope.Data = data
		envelope.Version++
	}

	return envelope, upgraded, nil
}

// Decode parses an envelope without upgrading it.
func Decode(raw []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if envelope.Format == "" || envelope.ID.IsZero() || len(envelope.Data) == 0 {
		return Envelope{}, fmt.Errorf("%w: missing format, id or data", ErrMalformed)
	}

	return envelope, nil
}

// Encode writes an envelope as indented JSON with a trailing newline. The
// output is stable for equal input, which keeps version-control diffs small.
func Encode(envelope Envelope) ([]byte, error) {
	var data bytes.Buffer
	if err := json.Compact(&data, envelope.Data); err != nil {
		return nil, fmt.Errorf("%w: data: %w", ErrMalformed, err)
	}
	envelope.Data = data.Bytes()
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %q: %w", envelope.Format, err)
	}

	return append(encoded, '\n'), nil
}
