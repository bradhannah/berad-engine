// Package project is the project index: the list of a project's documents,
// and the edits that create, rename and delete them.
//
// The orchestrator holds the index in memory under the well-known ID
// IndexID and never saves it: it is rebuilt from the document files on
// load. Windows subscribe to it like any document. Making document creation,
// renaming and deletion ordinary edits gives them journaling, undo and events
// for free. The orchestrator carries out what each edit means (D4): it assigns
// the new document's ID, fills a deletion's arguments with the document's data
// so the deletion can be undone, and creates, renames or removes the document.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// PluginName identifies the plug-in.
const PluginName = "project"

// FormatName identifies the project index. It is never saved.
const FormatName = "project"

// MaxNameLength bounds a document name, in bytes.
const MaxNameLength = 200

// Edit kind names.
const (
	CreateEdit = "project.create"
	DeleteEdit = "project.delete"
	RenameEdit = "project.rename"
)

// IndexID is the ID windows subscribe to for the project index. The zero
// ULID is never assigned to a document.
func IndexID() ids.ULID { return ids.ULID{} }

// ErrNotIndex reports a document that is not the project index.
var ErrNotIndex = errors.New("document is not the project index")

// ErrName reports an empty, untrimmed or overlong document name.
var ErrName = errors.New("invalid document name")

// Entry describes one document in the project.
type Entry struct {
	ID     ids.ULID `json:"id"`
	Name   string   `json:"name"`
	Format string   `json:"format"`
}

// Index lists a project's documents, ordered by ID.
type Index struct {
	entries []Entry
}

type wireIndex struct {
	Entries []Entry `json:"entries"`
}

// NewIndex returns an index of the given documents.
func NewIndex(entries []Entry) *Index {
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(a, b Entry) int { return a.ID.Compare(b.ID) })

	return &Index{entries: sorted}
}

// Entries returns the documents ordered by ID.
func (x *Index) Entries() []Entry { return slices.Clone(x.entries) }

// Entry returns one document's entry.
func (x *Index) Entry(id ids.ULID) (Entry, bool) {
	index, found := x.find(id)
	if !found {
		return noEntry(), false
	}

	return x.entries[index], true
}

func noEntry() Entry {
	var none Entry

	return none
}

// Name returns a document's name, or fallback when the index lacks it.
func (x *Index) Name(id ids.ULID, fallback string) string {
	if entry, ok := x.Entry(id); ok {
		return entry.Name
	}

	return fallback
}

// OfFormat returns the documents of one format, ordered by name.
func (x *Index) OfFormat(format string) []Entry {
	var matching []Entry
	for _, entry := range x.entries {
		if entry.Format == format {
			matching = append(matching, entry)
		}
	}
	slices.SortFunc(matching, func(a, b Entry) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return matching
}

func (x *Index) find(id ids.ULID) (int, bool) {
	return slices.BinarySearchFunc(x.entries, id, func(entry Entry, target ids.ULID) int {
		return entry.ID.Compare(target)
	})
}

// Format names the document format.
func (x *Index) Format() string { return FormatName }

// Encode returns the index's data, sent to windows in snapshots.
func (x *Index) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireIndex{Entries: x.entries})
	if err != nil {
		return nil, fmt.Errorf("project index: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (x *Index) Clone() model.Document {
	return &Index{entries: slices.Clone(x.entries)}
}

// Decode reads index data from a snapshot.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	var wire wireIndex
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("project index: %w", err)
	}
	if wire.Entries == nil {
		wire.Entries = []Entry{}
	}

	return NewIndex(wire.Entries), nil
}

// Format returns the index's format registration, so windows can read
// snapshots of it. Index data is never saved to a file.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: 1, Migrations: nil}
}

// CheckName validates a document name.
func CheckName(name string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name || len(name) > MaxNameLength {
		return fmt.Errorf("%w: %q", ErrName, name)
	}

	return nil
}

// DocumentArgs creates or deletes a document. A window creating a document
// sends Format, Name and Data, and Scope when the name only needs to be unique
// within a zone; the orchestrator adds the ID. A window deleting a document
// sends only the ID; the orchestrator adds the rest, so the inverse can
// recreate exactly what was deleted.
type DocumentArgs struct {
	ID       ids.ULID        `json:"id"`
	Format   string          `json:"format"`
	Name     string          `json:"name"`
	Version  int             `json:"version,omitempty"`
	Revision uint64          `json:"revision,omitempty"`
	Scope    ids.ULID        `json:"scope"`
	Data     json.RawMessage `json:"data,omitempty"`
	// Layout is the document's layout, for documents that keep one (D17).
	Layout json.RawMessage `json:"layout,omitempty"`
}

// RenameArgs renames a document.
type RenameArgs struct {
	ID   ids.ULID `json:"id"`
	Name string   `json:"name"`
}

// DocumentItem names a document's existence for undo-clash detection.
func DocumentItem(id ids.ULID) string { return "document:" + id.String() }

func nameItem(id ids.ULID) string { return "name:" + id.String() }

// Create is the edit kind that adds a document.
type Create struct{}

// Name identifies the edit kind.
func (Create) Name() string { return CreateEdit }

// Format names the document format the edit applies to.
func (Create) Format() string { return FormatName }

// Apply adds the document's entry. Its inverse deletes it.
func (Create) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, created, err := decode[DocumentArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	if created.ID.IsZero() || created.Format == "" {
		return model.Applied{}, fmt.Errorf("%w: a new document needs an ID and a format", model.ErrArgs)
	}
	if err := CheckName(created.Name); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	position, exists := index.find(created.ID)
	if exists {
		return model.Applied{}, fmt.Errorf("%w: document %s already exists", model.ErrArgs, created.ID)
	}
	entry := Entry{ID: created.ID, Name: created.Name, Format: created.Format}
	index.entries = slices.Insert(index.entries, position, entry)

	return lifecycle(args, DeleteEdit, created.ID)
}

// Delete is the edit kind that removes a document.
type Delete struct{}

// Name identifies the edit kind.
func (Delete) Name() string { return DeleteEdit }

// Format names the document format the edit applies to.
func (Delete) Format() string { return FormatName }

// Apply removes the document's entry. Its inverse recreates it from the data
// the orchestrator added to the arguments.
func (Delete) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, deleted, err := decode[DocumentArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	position, exists := index.find(deleted.ID)
	if !exists {
		return model.Applied{}, fmt.Errorf("%w: no document %s", model.ErrArgs, deleted.ID)
	}
	index.entries = slices.Delete(index.entries, position, position+1)

	return lifecycle(args, CreateEdit, deleted.ID)
}

func lifecycle(args json.RawMessage, inverseEdit string, id ids.ULID) (model.Applied, error) {
	return model.Applied{
		Inverse: args, InverseEdit: inverseEdit, Touched: []string{DocumentItem(id)}, Covers: []ids.ULID{id},
	}, nil
}

// Rename is the edit kind that renames a document.
type Rename struct{}

// Name identifies the edit kind.
func (Rename) Name() string { return RenameEdit }

// Format names the document format the edit applies to.
func (Rename) Format() string { return FormatName }

// Apply renames the document's entry.
func (Rename) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, rename, err := decode[RenameArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	if err := CheckName(rename.Name); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	position, exists := index.find(rename.ID)
	if !exists {
		return model.Applied{}, fmt.Errorf("%w: no document %s", model.ErrArgs, rename.ID)
	}
	previous := index.entries[position].Name
	index.entries[position].Name = rename.Name
	inverse, err := json.Marshal(RenameArgs{ID: rename.ID, Name: previous})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{nameItem(rename.ID)}, Covers: nil}, nil
}

//nolint:ireturn // T is the caller-chosen concrete argument type.
func decode[T any](document model.Document, args json.RawMessage) (*Index, T, error) {
	var decoded T
	index, ok := document.(*Index)
	if !ok {
		return nil, decoded, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotIndex)
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, decoded, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}

	return index, decoded, nil
}

// Plugin returns the project index plug-in. Every program that runs the
// orchestrator or a window must list it.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits:     []model.EditKind{Create{}, Delete{}, Rename{}},
	}
}
