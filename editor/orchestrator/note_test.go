package orchestrator_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// The tests' own kind of document, a note: a body of text and an optional
// location, an anchor on a floor. It gives the orchestrator's tests a
// document that nothing else depends on, saved under notes/, with one edit
// that changes its text and one that makes it refer to another document.
const (
	noteFormat       = "note"
	noteBodyEdit     = "note.body.set"
	noteLocationEdit = "note.location.set"
)

var errNotNote = errors.New("document is not a note")

type noteDoc struct {
	Body     string          `json:"body"`
	Location floor.AnchorRef `json:"location"`
}

type noteBodyArgs struct {
	Body string `json:"body"`
}

type noteLocationArgs struct {
	Location floor.AnchorRef `json:"location"`
}

func newNote() *noteDoc {
	return &noteDoc{Body: "", Location: floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}}
}

func (d *noteDoc) Format() string { return noteFormat }

func (d *noteDoc) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("note: %w", err)
	}
	return data, nil
}

func (d *noteDoc) Clone() model.Document {
	clone := *d
	return &clone
}

func (d *noteDoc) References() []model.Reference {
	if d.Location.IsZero() {
		return nil
	}
	return []model.Reference{{Document: d.Location.Floor, Item: d.Location.Anchor.String(), Label: "location"}}
}

func decodeNote(data json.RawMessage) (model.Document, error) {
	note := newNote()
	if err := json.Unmarshal(data, note); err != nil {
		return nil, fmt.Errorf("note: %w", err)
	}
	return note, nil
}

type noteBody struct{}

func (noteBody) Name() string   { return noteBodyEdit }
func (noteBody) Format() string { return noteFormat }

func (noteBody) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	note, ok := document.(*noteDoc)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, errNotNote)
	}
	var set noteBodyArgs
	if err := json.Unmarshal(args, &set); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := json.Marshal(noteBodyArgs{Body: note.Body})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	note.Body = set.Body
	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{"body"}, Covers: nil}, nil
}

type noteLocation struct{}

func (noteLocation) Name() string   { return noteLocationEdit }
func (noteLocation) Format() string { return noteFormat }

func (noteLocation) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	note, ok := document.(*noteDoc)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, errNotNote)
	}
	var set noteLocationArgs
	if err := json.Unmarshal(args, &set); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := json.Marshal(noteLocationArgs{Location: note.Location})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	note.Location = set.Location
	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{"location"}, Covers: nil}, nil
}

func notePlugin() plugin.Plugin {
	return plugin.Plugin{
		Name: "note",
		Documents: []model.DocumentType{{
			Format: content.Format{Name: noteFormat, Current: 1, Migrations: nil}, Decode: decodeNote, Layout: nil,
			Prepare: nil,
		}},
		Edits:    []model.EditKind{noteBody{}, noteLocation{}},
		Required: nil,
	}
}
