// Package views is the plug-in for the Project window's saved views: named
// sets of facet values and a search, kept with the project so everyone who
// opens it finds them (increment 30). A game never reads them.
package views

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
const PluginName = "views"

// FormatName identifies saved views inside saved envelopes. The document's
// folder is the format's name with an s.
const FormatName = "view"

// Name is the name of the saved views every project holds.
const Name = "Project views"

// SetEdit is the edit kind that replaces the saved views.
const SetEdit = "views.set"

// Limits on a view, in bytes and in views.
const (
	MaxNameLength = 60
	MaxViews      = 100
)

// viewsID is the saved views' fixed ID. Its time part is zero, so no
// generated ID ever equals it.
const viewsID = "00000000000000000000000031"

// viewsItem is the undo-clash item every change to the views touches.
const viewsItem = "views"

// ID is the ID of a project's saved views.
func ID() ids.ULID { return ids.MustParse(viewsID) }

// ErrNotViews reports a document that is not the saved views.
var ErrNotViews = errors.New("document is not the saved views")

// ErrView reports a view that cannot be saved.
var ErrView = errors.New("invalid view")

// View is one saved view: its name, what is typed in the search, and the
// values ticked in each facet, by the facet's and the values' keys.
type View struct {
	Name   string              `json:"name"`
	Search string              `json:"search,omitempty"`
	Ticks  map[string][]string `json:"ticks,omitempty"`
}

// Same reports whether two views narrow the same way, whatever their names.
func (v View) Same(other View) bool {
	if strings.TrimSpace(v.Search) != strings.TrimSpace(other.Search) {
		return false
	}
	ticked := func(view View) map[string][]string {
		kept := map[string][]string{}
		for facet, values := range view.Ticks {
			if len(values) > 0 {
				sorted := slices.Clone(values)
				slices.Sort(sorted)
				kept[facet] = sorted
			}
		}

		return kept
	}
	mine, theirs := ticked(v), ticked(other)
	if len(mine) != len(theirs) {
		return false
	}
	for facet, values := range mine {
		if !slices.Equal(values, theirs[facet]) {
			return false
		}
	}

	return true
}

// Document is a project's saved views, in the order they were saved.
type Document struct {
	Views []View
}

type wire struct {
	Views []View `json:"views"`
}

// Format names the document format.
func (d *Document) Format() string { return FormatName }

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	views := d.Views
	if views == nil {
		views = []View{}
	}
	data, err := json.Marshal(wire{Views: views})
	if err != nil {
		return nil, fmt.Errorf("saved views: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document { return &Document{Views: cloneViews(d.Views)} }

func cloneViews(views []View) []View {
	cloned := make([]View, 0, len(views))
	for _, view := range views {
		ticks := make(map[string][]string, len(view.Ticks))
		for facet, values := range view.Ticks {
			ticks[facet] = slices.Clone(values)
		}
		cloned = append(cloned, View{Name: view.Name, Search: view.Search, Ticks: ticks})
	}

	return cloned
}

// Decode reads saved views.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	var read wire
	if err := json.Unmarshal(data, &read); err != nil {
		return nil, fmt.Errorf("%w: saved views: %w", content.ErrMalformed, err)
	}
	if err := Check(read.Views); err != nil {
		return nil, err
	}

	return &Document{Views: read.Views}, nil
}

// Check validates a list of views: at most MaxViews, each named, trimmed,
// at most MaxNameLength bytes, and no two with one name, ignoring case.
func Check(views []View) error {
	if len(views) > MaxViews {
		return fmt.Errorf("%w: at most %d views", ErrView, MaxViews)
	}
	for index, view := range views {
		name := view.Name
		if name == "" || strings.TrimSpace(name) != name || len(name) > MaxNameLength {
			return fmt.Errorf("%w: name %q", ErrView, name)
		}
		if slices.ContainsFunc(views[:index], func(other View) bool { return strings.EqualFold(other.Name, name) }) {
			return fmt.Errorf("%w: there is already a view called %q", ErrView, name)
		}
	}

	return nil
}

// SetArgs replaces the saved views.
type SetArgs struct {
	Views []View `json:"views"`
}

// Set is the edit kind that replaces the saved views.
type Set struct{}

// Name identifies the edit kind.
func (Set) Name() string { return SetEdit }

// Format names the document format the edit applies to.
func (Set) Format() string { return FormatName }

// Apply replaces the views; its inverse puts the previous ones back.
func (Set) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	viewsDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotViews)
	}
	var set SetArgs
	if err := json.Unmarshal(args, &set); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if err := Check(set.Views); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := json.Marshal(SetArgs{Views: cloneViews(viewsDoc.Views)})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	viewsDoc.Views = cloneViews(set.Views)

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{viewsItem}, Covers: nil}, nil
}

// Plugin returns the saved views plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name: PluginName,
		Documents: []model.DocumentType{
			{Format: content.Format{Name: FormatName, Current: 1, Migrations: nil}, Decode: Decode, Layout: nil, Prepare: nil},
		},
		Edits: []model.EditKind{Set{}},
		Required: []model.RequiredDocument{{
			ID: ID(), Format: FormatName, Name: Name, Data: (&Document{Views: nil}).Encode, ReadOnly: false,
		}},
	}
}
