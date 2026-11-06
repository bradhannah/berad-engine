package project

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TouchEdit is the edit kind that says when a document last changed. Only
// the orchestrator makes it, as a document changes; it is not undone.
const TouchEdit = "project.touch"

// TouchArgs says when a document last changed.
type TouchArgs struct {
	ID      ids.ULID  `json:"id"`
	Changed time.Time `json:"changed"`
}

// Touch is the edit kind that sets when a document last changed.
type Touch struct{}

// Name identifies the edit kind.
func (Touch) Name() string { return TouchEdit }

// Format names the document format the edit applies to.
func (Touch) Format() string { return FormatName }

// Apply sets when the document last changed. Its inverse sets it back.
func (Touch) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, touch, err := decode[TouchArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	position, exists := index.find(touch.ID)
	if !exists {
		return model.Applied{}, fmt.Errorf("%w: no document %s", model.ErrArgs, touch.ID)
	}
	inverse, err := json.Marshal(TouchArgs{ID: touch.ID, Changed: index.entries[position].Changed})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	index.entries[position].Changed = touch.Changed

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: nil, Covers: nil}, nil
}
