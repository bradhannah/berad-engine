// Package model defines the editor's document and edit seams: what a document
// is, what an edit kind must do, and the shared contract every edit kind meets.
package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/content"
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

// DocumentType registers a document format and how to read it.
type DocumentType struct {
	Format content.Format
	Decode func(data json.RawMessage) (Document, error)
}

// Applied describes what an applied edit changed.
type Applied struct {
	// Inverse is the argument set that undoes the edit with the same kind.
	Inverse json.RawMessage
	// Touched lists the items the edit changed, for detecting undo clashes.
	Touched []string
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

// VerifyUndo checks the shared contract every edit kind must meet: applying an
// edit and then applying its inverse restores exactly the same data. The
// document itself is not modified.
func VerifyUndo(document Document, kind EditKind, args json.RawMessage) error {
	before, err := document.Encode()
	if err != nil {
		return fmt.Errorf("encode before: %w", err)
	}
	working := document.Clone()
	applied, err := kind.Apply(working, args)
	if err != nil {
		return fmt.Errorf("apply %s: %w", kind.Name(), err)
	}
	if _, err := kind.Apply(working, applied.Inverse); err != nil {
		return fmt.Errorf("apply inverse of %s: %w", kind.Name(), err)
	}
	after, err := working.Encode()
	if err != nil {
		return fmt.Errorf("encode after: %w", err)
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("%w: %s", ErrUndoContract, kind.Name())
	}

	return nil
}
