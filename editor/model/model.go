// Package model defines the editor's document and edit seams: what a document
// is, what an edit kind must do, and the shared contract every edit kind meets.
package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrWrongDocument reports an edit applied to a document of another format.
var ErrWrongDocument = errors.New("edit does not apply to this document")

// ErrArgs reports invalid edit arguments.
var ErrArgs = errors.New("invalid edit arguments")

// ErrUndoContract reports an edit kind whose undo does not restore the document.
var ErrUndoContract = errors.New("undo did not restore the document")

// Document is an editable document held in memory.
type Document interface {
	// Format names the document's content format.
	Format() string
	// Encode returns the document's saved data.
	Encode() (json.RawMessage, error)
	// Clone returns an independent copy.
	Clone() Document
}

// DocumentType registers a document format and how to read it. Layout, when
// set, is the format of the layout file a Layered document keeps beside its
// own.
type DocumentType struct {
	Format content.Format
	Decode func(data json.RawMessage) (Document, error)
	Layout *content.Format
	// Prepare, when set, completes a new document's data before it is
	// created, such as giving its parts permanent IDs (D4).
	Prepare func(data json.RawMessage, newID IDSource) (json.RawMessage, error)
}

// Layered is implemented by documents that save their layout, such as where
// a workflow's nodes sit, in a file of its own beside the document, so moving
// things never shows up as a change to the logic (D17).
type Layered interface {
	EncodeLayout() (json.RawMessage, error)
	DecodeLayout(data json.RawMessage) error
}

// LayoutOf returns a document's layout data, or nil for a document that has
// none.
func LayoutOf(document Document) (json.RawMessage, error) {
	layered, ok := document.(Layered)
	if !ok {
		return nil, nil
	}
	data, err := layered.EncodeLayout()
	if err != nil {
		return nil, fmt.Errorf("encode layout: %w", err)
	}

	return data, nil
}

// DecodeWithLayout reads a document and, when given, its layout.
//
//nolint:ireturn // Documents are polymorphic.
func (t DocumentType) DecodeWithLayout(data, layout json.RawMessage) (Document, error) {
	document, err := t.Decode(data)
	if err != nil {
		return nil, err
	}
	layered, ok := document.(Layered)
	if !ok || len(layout) == 0 {
		return document, nil
	}
	if err := layered.DecodeLayout(layout); err != nil {
		return nil, fmt.Errorf("decode layout: %w", err)
	}

	return document, nil
}

// Applied describes what an applied edit changed.
type Applied struct {
	// Inverse is the argument set that undoes the edit. It is applied with
	// InverseEdit, or with the same edit kind when InverseEdit is empty.
	Inverse json.RawMessage
	// InverseEdit names the edit kind that applies Inverse, for edits whose
	// opposite is a different kind (adding versus removing an anchor).
	InverseEdit string
	// Touched lists the items the edit changed, for detecting undo clashes.
	Touched []string
	// Covers lists documents whose whole content the edit creates or removes.
	// Undoing it clashes with any later change to them from another window.
	Covers []ids.ULID
}

// EditKind is one kind of change. Apply must either change the document and
// succeed, or leave it untouched and fail.
type EditKind interface {
	// Name identifies the edit kind, unique across all plug-ins.
	Name() string
	// Format names the document format the edit applies to.
	Format() string
	// Apply changes the document in place.
	Apply(document Document, args json.RawMessage) (Applied, error)
}

// IDSource supplies new permanent IDs.
type IDSource func() (ids.ULID, error)

// Preparer is implemented by edit kinds whose arguments need values only the
// orchestrator can supply, such as new permanent IDs (D4). The orchestrator
// prepares the arguments before applying them, then journals and broadcasts
// the prepared arguments, so every copy applies exactly the same change.
// Preparing arguments that are already complete must leave them unchanged, so
// redo applies the original change again.
type Preparer interface {
	Prepare(document Document, args json.RawMessage, newID IDSource) (json.RawMessage, error)
}

// Reference is a link from one document to another document or to an item
// inside it, such as an anchor on a floor.
type Reference struct {
	// Document is the document referred to.
	Document ids.ULID
	// Item names an item inside it; empty refers to the whole document.
	Item string
	// Label says where the reference is, for where-used lists.
	Label string
}

// Referencer is implemented by documents that refer to others, so deleting
// something can show where it is used.
type Referencer interface {
	References() []Reference
}

// Container is implemented by documents that list other documents as their
// parts, such as a zone listing its floors. A part's name must be unique among
// the parts of its container; other documents' names are unique within their
// format.
type Container interface {
	Parts() []ids.ULID
}

// Pickable is implemented by documents that hold things a pick on map can
// choose, such as a floor holding anchors.
type Pickable interface {
	CanPick(what string) bool
}

// Lookup returns the edit kind with a name, for VerifyUndo to apply inverses
// that use a different kind.
type Lookup func(name string) (EditKind, bool)

// VerifyUndo checks the shared contract every edit kind must meet: applying an
// edit and then applying its inverse restores exactly the same data. The
// document itself is not modified.
func VerifyUndo(document Document, kind EditKind, args json.RawMessage, lookup Lookup) error {
	before, err := encodeAll(document)
	if err != nil {
		return fmt.Errorf("encode before: %w", err)
	}
	working := document.Clone()
	applied, err := kind.Apply(working, args)
	if err != nil {
		return fmt.Errorf("apply %s: %w", kind.Name(), err)
	}
	inverse := kind
	if applied.InverseEdit != "" {
		found, ok := lookup(applied.InverseEdit)
		if !ok {
			return fmt.Errorf("%w: %s has unknown inverse %s", ErrUndoContract, kind.Name(), applied.InverseEdit)
		}
		inverse = found
	}
	if _, err := inverse.Apply(working, applied.Inverse); err != nil {
		return fmt.Errorf("apply inverse of %s: %w", kind.Name(), err)
	}
	after, err := encodeAll(working)
	if err != nil {
		return fmt.Errorf("encode after: %w", err)
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("%w: %s", ErrUndoContract, kind.Name())
	}

	return nil
}

// encodeAll returns a document's data followed by its layout, if it has one,
// so the undo contract covers both.
func encodeAll(document Document) ([]byte, error) {
	data, err := document.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	layout, err := LayoutOf(document)
	if err != nil {
		return nil, err
	}

	return append(append(data, '\n'), layout...), nil
}
