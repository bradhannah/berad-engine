// Package plugin holds the explicit list of plug-ins a program uses. Each
// program builds one Registry from a literal list; nothing registers itself
// through import side effects, and duplicate names fail at startup.
package plugin

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrDuplicate reports a plug-in, document format or edit name used twice.
var ErrDuplicate = errors.New("duplicate plug-in registration")

// ErrInvalid reports an incomplete plug-in.
var ErrInvalid = errors.New("invalid plug-in")

// Plugin contributes document types and edit kinds, and the documents every
// project must hold.
type Plugin struct {
	Name      string
	Documents []model.DocumentType
	Edits     []model.EditKind
	Required  []model.RequiredDocument
}

// Registry is the immutable set of plug-ins a program was built with.
type Registry struct {
	names     []string
	documents map[string]model.DocumentType
	edits     map[string]model.EditKind
	formats   *content.Registry
	required  []model.RequiredDocument
}

// NewRegistry validates and registers plug-ins in the order given.
func NewRegistry(plugins ...Plugin) (*Registry, error) {
	registry := &Registry{
		names:     nil,
		documents: map[string]model.DocumentType{},
		edits:     map[string]model.EditKind{},
		formats:   content.NewRegistry(),
		required:  nil,
	}
	for _, entry := range plugins {
		if err := registry.add(entry); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func (r *Registry) add(entry Plugin) error {
	if entry.Name == "" {
		return fmt.Errorf("%w: unnamed plug-in", ErrInvalid)
	}
	if slices.Contains(r.names, entry.Name) {
		return fmt.Errorf("%w: plug-in %q", ErrDuplicate, entry.Name)
	}
	for _, documentType := range entry.Documents {
		if documentType.Decode == nil {
			return fmt.Errorf("%w: %q has no decoder for %q", ErrInvalid, entry.Name, documentType.Format.Name)
		}
		if err := r.formats.Register(documentType.Format); err != nil {
			return fmt.Errorf("%w: plug-in %q: %w", ErrDuplicate, entry.Name, err)
		}
		if documentType.Layout != nil {
			if err := r.formats.Register(*documentType.Layout); err != nil {
				return fmt.Errorf("%w: plug-in %q: %w", ErrDuplicate, entry.Name, err)
			}
		}
		r.documents[documentType.Format.Name] = documentType
	}
	for _, kind := range entry.Edits {
		if _, exists := r.edits[kind.Name()]; exists {
			return fmt.Errorf("%w: edit %q", ErrDuplicate, kind.Name())
		}
		r.edits[kind.Name()] = kind
	}
	if err := r.addRequired(entry); err != nil {
		return err
	}
	r.names = append(r.names, entry.Name)

	return nil
}

func (r *Registry) addRequired(entry Plugin) error {
	for _, required := range entry.Required {
		if required.ID.IsZero() || required.Name == "" || required.Data == nil {
			return fmt.Errorf("%w: %q has an incomplete required document", ErrInvalid, entry.Name)
		}
		if _, ok := r.documents[required.Format]; !ok {
			return fmt.Errorf("%w: %q requires a %q document, a format no plug-in has", ErrInvalid, entry.Name, required.Format)
		}
		if r.IsRequired(required.ID) {
			return fmt.Errorf("%w: required document %s", ErrDuplicate, required.ID)
		}
		r.required = append(r.required, required)
	}

	return nil
}

// Required returns the documents every project must hold, in registration
// order.
func (r *Registry) Required() []model.RequiredDocument {
	return slices.Clone(r.required)
}

// IsRequired reports whether a document is one every project must hold.
func (r *Registry) IsRequired(id ids.ULID) bool {
	return slices.ContainsFunc(r.required, func(required model.RequiredDocument) bool { return required.ID == id })
}

// IsReadOnly reports whether a document is one every project holds as the
// engine made it, which nothing changes.
func (r *Registry) IsReadOnly(id ids.ULID) bool {
	return slices.ContainsFunc(r.required, func(required model.RequiredDocument) bool {
		return required.ID == id && required.ReadOnly
	})
}

// Plugins returns plug-in names in registration order.
func (r *Registry) Plugins() []string {
	return slices.Clone(r.names)
}

// Document returns the document type for a format.
func (r *Registry) Document(format string) (model.DocumentType, bool) {
	documentType, ok := r.documents[format]

	return documentType, ok
}

// Edit returns an edit kind by name.
//
//nolint:ireturn // Edit kinds are polymorphic plug-in implementations.
func (r *Registry) Edit(name string) (model.EditKind, bool) {
	kind, ok := r.edits[name]

	return kind, ok
}

// EditNames returns the registered edit names in sorted order.
func (r *Registry) EditNames() []string {
	return slices.Sorted(maps.Keys(r.edits))
}

// Formats returns the content registry used to read and upgrade documents.
func (r *Registry) Formats() *content.Registry {
	return r.formats
}
