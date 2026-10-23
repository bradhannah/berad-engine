// Package tilefloor is the editor half of the tile floor feature: its document
// type and the paint edit.
package tilefloor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// PluginName identifies the plug-in.
const PluginName = "tilefloor"

// PaintEdit is the name of the paint edit kind.
const PaintEdit = "tilefloor.paint"

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

// Cell is one painted position.
type Cell struct {
	Col  int    `json:"col"`
	Row  int    `json:"row"`
	Tile string `json:"tile"`
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
	target, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotFloor)
	}
	var paint PaintArgs
	if err := json.Unmarshal(args, &paint); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if len(paint.Cells) == 0 || len(paint.Cells) > MaxCells {
		return model.Applied{}, fmt.Errorf("%w: %d cells", model.ErrArgs, len(paint.Cells))
	}
	for _, cell := range paint.Cells {
		if _, err := target.Floor.At(cell.Col, cell.Row); err != nil {
			return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
	}

	inverse := make([]Cell, 0, len(paint.Cells))
	touched := make([]string, 0, len(paint.Cells))
	for _, cell := range paint.Cells {
		previous, err := target.Floor.Set(cell.Col, cell.Row, cell.Tile)
		if err != nil {
			return model.Applied{}, fmt.Errorf("paint after validation: %w", err)
		}
		inverse = append(inverse, Cell{Col: cell.Col, Row: cell.Row, Tile: previous})
		touched = append(touched, CellKey(cell.Col, cell.Row))
	}
	slices.Reverse(inverse)
	inverseArgs, err := json.Marshal(PaintArgs{Cells: inverse})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{Inverse: inverseArgs, Touched: touched}, nil
}

// CellKey names a cell for undo-clash detection.
func CellKey(col, row int) string {
	return strconv.Itoa(col) + "," + strconv.Itoa(row)
}

// NewDocument returns a filled tile floor document.
func NewDocument(width, height int, fill string) (*Document, error) {
	created, err := floor.New(width, height, fill)
	if err != nil {
		return nil, fmt.Errorf("new tile floor: %w", err)
	}

	return &Document{Floor: created}, nil
}

// Plugin returns the tile floor plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: floor.Format(), Decode: Decode}},
		Edits:     []model.EditKind{Paint{}},
	}
}
