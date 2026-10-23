// Package textdoc is a placeholder text document for proving text editing and
// pick on map: a body of text and an optional location (an anchor).
//
// It is editor-only and holds no game content; it exists so the editor's text
// field and cross-window picking can be judged before real documents need them.
package textdoc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// PluginName identifies the plug-in.
const PluginName = "textdoc"

// FormatName identifies text documents inside saved envelopes.
const FormatName = "text"

// FormatVersion is the text document format version this package writes.
const FormatVersion = 1

// MaxBody bounds a body, in bytes.
const MaxBody = 64 << 10

// Edit kind names.
const (
	SetBodyEdit     = "text.body.set"
	SetLocationEdit = "text.location.set"
)

// Undo-clash items.
const (
	bodyItem     = "body"
	locationItem = "location"
)

// ErrNotText reports a document that is not a text document.
var ErrNotText = errors.New("document is not a text document")

// Document is a text document.
type Document struct {
	Body     string
	Location tilefloor.AnchorRef
}

type wireDocument struct {
	Body     string              `json:"body"`
	Location tilefloor.AnchorRef `json:"location"`
}

// NewDocument returns an empty text document.
func NewDocument() *Document {
	return &Document{Body: "", Location: tilefloor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}}
}

// Format names the document format.
func (d *Document) Format() string { return FormatName }

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireDocument{Body: d.Body, Location: d.Location})
	if err != nil {
		return nil, fmt.Errorf("text document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document {
	clone := *d

	return &clone
}

// References lists the location's anchor, if set.
func (d *Document) References() []model.Reference {
	if d.Location.IsZero() {
		return nil
	}

	return []model.Reference{{Document: d.Location.Floor, Item: d.Location.Anchor.String(), Label: "location"}}
}

// Decode reads a saved text document.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	var wire wireDocument
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("text document: %w", err)
	}

	return &Document{Body: wire.Body, Location: wire.Location}, nil
}

// Format returns the text document format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: nil}
}

// SetBodyArgs replaces the body.
type SetBodyArgs struct {
	Body string `json:"body"`
}

// SetLocationArgs replaces the location; a zero reference clears it.
type SetLocationArgs struct {
	Location tilefloor.AnchorRef `json:"location"`
}

// SetBody is the edit kind that replaces the body.
type SetBody struct{}

// Name identifies the edit kind.
func (SetBody) Name() string { return SetBodyEdit }

// Format names the document format the edit applies to.
func (SetBody) Format() string { return FormatName }

// Apply replaces the body.
func (SetBody) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	textDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotText)
	}
	var set SetBodyArgs
	if err := json.Unmarshal(args, &set); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if len(set.Body) > MaxBody {
		return model.Applied{}, fmt.Errorf("%w: body is %d bytes, at most %d", model.ErrArgs, len(set.Body), MaxBody)
	}
	inverse, err := json.Marshal(SetBodyArgs{Body: textDoc.Body})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	textDoc.Body = set.Body

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{bodyItem}, Covers: nil}, nil
}

// SetLocation is the edit kind that replaces the location.
type SetLocation struct{}

// Name identifies the edit kind.
func (SetLocation) Name() string { return SetLocationEdit }

// Format names the document format the edit applies to.
func (SetLocation) Format() string { return FormatName }

// Apply replaces the location.
func (SetLocation) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	textDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotText)
	}
	var set SetLocationArgs
	if err := json.Unmarshal(args, &set); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if set.Location.Floor.IsZero() != set.Location.Anchor.IsZero() {
		return model.Applied{}, fmt.Errorf("%w: a location needs both a floor and an anchor", model.ErrArgs)
	}
	inverse, err := json.Marshal(SetLocationArgs{Location: textDoc.Location})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	textDoc.Location = set.Location

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{locationItem}, Covers: nil}, nil
}

// Plugin returns the text document plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits:     []model.EditKind{SetBody{}, SetLocation{}},
	}
}
