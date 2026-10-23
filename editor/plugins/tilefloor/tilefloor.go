// Package tilefloor is the editor half of the tile floor feature: its document
// type, painting either layer, and adding, changing and removing anchors.
package tilefloor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// PluginName identifies the plug-in.
const PluginName = "tilefloor"

// Edit kind names.
const (
	PaintEdit        = "tilefloor.paint"
	AddAnchorEdit    = "tilefloor.anchor.add"
	RemoveAnchorEdit = "tilefloor.anchor.remove"
	UpdateAnchorEdit = "tilefloor.anchor.update"
)

// Layer names used in paint arguments.
const (
	LayerGround = "ground"
	LayerTop    = "top"
)

// MaxCells bounds the cells one paint command may change.
const MaxCells = 4096

// ErrNotFloor reports a document that is not a tile floor.
var ErrNotFloor = errors.New("document is not a tile floor")

// Document is a tile floor held by the editor.
type Document struct {
	Floor *floor.Floor
}

// Format names the document format.
func (d *Document) Format() string { return floor.FormatName }

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	data, err := d.Floor.Encode()
	if err != nil {
		return nil, fmt.Errorf("tile floor document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document {
	return &Document{Floor: d.Floor.Clone()}
}

// Decode reads a saved tile floor.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	decoded, err := floor.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("tile floor document: %w", err)
	}

	return &Document{Floor: decoded}, nil
}

// NewDocument returns a tile floor document with every cell's ground set.
func NewDocument(width, height int, ground string) (*Document, error) {
	created, err := floor.New(width, height, ground)
	if err != nil {
		return nil, fmt.Errorf("new tile floor: %w", err)
	}

	return &Document{Floor: created}, nil
}

// PickAnchor is what a pick of an anchor asks for; it matches the protocol.
const PickAnchor = "anchor"

// CanPick reports whether the floor holds something of the kind a pick asks
// for.
func (d *Document) CanPick(what string) bool {
	return what == PickAnchor && len(d.Floor.Anchors()) > 0
}

func target(document model.Document) (*Document, error) {
	floorDoc, ok := document.(*Document)
	if !ok {
		return nil, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotFloor)
	}

	return floorDoc, nil
}

//nolint:ireturn // T is the caller-chosen concrete argument type.
func decodeArgs[T any](args json.RawMessage) (T, error) {
	var decoded T
	if err := json.Unmarshal(args, &decoded); err != nil {
		return decoded, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}

	return decoded, nil
}

func encodeArgs(args any) (json.RawMessage, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("encode inverse: %w", err)
	}

	return data, nil
}

// Cell is one painted position on one layer.
type Cell struct {
	Col   int    `json:"col"`
	Row   int    `json:"row"`
	Layer string `json:"layer"`
	Tile  string `json:"tile"`
}

// PaintArgs sets tiles at positions, applied in order.
type PaintArgs struct {
	Cells []Cell `json:"cells"`
}

// Paint is the paint edit kind.
type Paint struct{}

// Name identifies the edit kind.
func (Paint) Name() string { return PaintEdit }

// Format names the document format the edit applies to.
func (Paint) Format() string { return floor.FormatName }

// Apply paints every cell, or none if any cell is invalid.
func (Paint) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	paint, err := decodeArgs[PaintArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	if len(paint.Cells) == 0 || len(paint.Cells) > MaxCells {
		return model.Applied{}, fmt.Errorf("%w: %d cells", model.ErrArgs, len(paint.Cells))
	}
	for _, cell := range paint.Cells {
		if !floorDoc.Floor.In(cell.Col, cell.Row) {
			return model.Applied{}, fmt.Errorf("%w: (%d, %d) is outside the floor", model.ErrArgs, cell.Col, cell.Row)
		}
		if cell.Layer != LayerGround && cell.Layer != LayerTop {
			return model.Applied{}, fmt.Errorf("%w: layer %q", model.ErrArgs, cell.Layer)
		}
	}

	inverse := make([]Cell, 0, len(paint.Cells))
	touched := make([]string, 0, len(paint.Cells))
	for _, cell := range paint.Cells {
		set := floorDoc.Floor.SetGround
		if cell.Layer == LayerTop {
			set = floorDoc.Floor.SetTop
		}
		previous, err := set(cell.Col, cell.Row, cell.Tile)
		if err != nil {
			return model.Applied{}, fmt.Errorf("paint after validation: %w", err)
		}
		inverse = append(inverse, Cell{Col: cell.Col, Row: cell.Row, Layer: cell.Layer, Tile: previous})
		touched = append(touched, CellKey(cell.Layer, cell.Col, cell.Row))
	}
	slices.Reverse(inverse)
	inverseArgs, err := encodeArgs(PaintArgs{Cells: inverse})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{Inverse: inverseArgs, InverseEdit: "", Touched: touched, Covers: nil}, nil
}

// CellKey names one layer of a cell for undo-clash detection.
func CellKey(layer string, col, row int) string {
	return layer + ":" + strconv.Itoa(col) + "," + strconv.Itoa(row)
}

// AnchorKey names an anchor for undo-clash detection.
func AnchorKey(id ids.ULID) string {
	return "anchor:" + id.String()
}

// AddAnchorArgs adds an anchor at a position in the floor's anchor order. The
// orchestrator assigns the ID when it is missing.
type AddAnchorArgs struct {
	Anchor floor.Anchor `json:"anchor"`
	Index  int          `json:"index"`
}

// RemoveAnchorArgs removes an anchor.
type RemoveAnchorArgs struct {
	ID ids.ULID `json:"id"`
}

// UpdateAnchorArgs renames or moves an anchor.
type UpdateAnchorArgs struct {
	Anchor floor.Anchor `json:"anchor"`
}

// AddAnchor is the edit kind that adds an anchor.
type AddAnchor struct{}

// Name identifies the edit kind.
func (AddAnchor) Name() string { return AddAnchorEdit }

// Format names the document format the edit applies to.
func (AddAnchor) Format() string { return floor.FormatName }

// Prepare gives a new anchor its permanent ID.
func (AddAnchor) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	add, err := decodeArgs[AddAnchorArgs](args)
	if err != nil {
		return nil, err
	}
	if !add.Anchor.ID.IsZero() {
		return args, nil
	}
	add.Anchor.ID, err = newID()
	if err != nil {
		return nil, fmt.Errorf("anchor id: %w", err)
	}

	return encodeArgs(add)
}

// Apply adds the anchor.
func (AddAnchor) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	add, err := decodeArgs[AddAnchorArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	if err := floorDoc.Floor.InsertAnchor(add.Index, add.Anchor); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(RemoveAnchorArgs{ID: add.Anchor.ID})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: RemoveAnchorEdit, Touched: []string{AnchorKey(add.Anchor.ID)}, Covers: nil,
	}, nil
}

// RemoveAnchor is the edit kind that removes an anchor.
type RemoveAnchor struct{}

// Name identifies the edit kind.
func (RemoveAnchor) Name() string { return RemoveAnchorEdit }

// Format names the document format the edit applies to.
func (RemoveAnchor) Format() string { return floor.FormatName }

// Apply removes the anchor; its inverse puts it back in the same place.
func (RemoveAnchor) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	remove, err := decodeArgs[RemoveAnchorArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	removed, index, err := floorDoc.Floor.RemoveAnchor(remove.ID)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(AddAnchorArgs{Anchor: removed, Index: index})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: AddAnchorEdit, Touched: []string{AnchorKey(remove.ID)}, Covers: nil,
	}, nil
}

// UpdateAnchor is the edit kind that renames or moves an anchor.
type UpdateAnchor struct{}

// Name identifies the edit kind.
func (UpdateAnchor) Name() string { return UpdateAnchorEdit }

// Format names the document format the edit applies to.
func (UpdateAnchor) Format() string { return floor.FormatName }

// Apply changes the anchor; its inverse restores the previous name and place.
func (UpdateAnchor) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	update, err := decodeArgs[UpdateAnchorArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	previous, err := floorDoc.Floor.UpdateAnchor(update.Anchor)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverse, err := encodeArgs(UpdateAnchorArgs{Anchor: previous})
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: "", Touched: []string{AnchorKey(update.Anchor.ID)}, Covers: nil,
	}, nil
}

// Plugin returns the tile floor plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: floor.Format(), Decode: Decode}},
		Edits:     []model.EditKind{Paint{}, AddAnchor{}, RemoveAnchor{}, UpdateAnchor{}},
	}
}
