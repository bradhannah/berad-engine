// Package tilefloor is the editor half of the tile floor feature: its document
// type, painting either layer, resizing, and adding, changing and removing
// anchors.
package tilefloor

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"image"
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
	ResizeEdit       = "tilefloor.resize"
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

// MaxResizeCells bounds the cells one resize may add and remove together.
const MaxResizeCells = 1 << 16

// BoundsKey names a floor's bounds for undo-clash detection.
const BoundsKey = "bounds"

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

// NewDocument returns a tile floor document whose top-left cell is the zone
// origin, with every cell's ground set.
func NewDocument(width, height int, ground string) (*Document, error) {
	return NewDocumentAt(image.Rect(0, 0, width, height), ground)
}

// NewDocumentAt returns a tile floor document covering the given bounds, with
// every cell's ground set.
func NewDocumentAt(bounds image.Rectangle, ground string) (*Document, error) {
	created, err := floor.NewAt(bounds, ground)
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

// Bounds is a floor's extent: its top-left cell and its size.
type Bounds struct {
	Col    int `json:"col"`
	Row    int `json:"row"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Rect returns the bounds as a rectangle of cells.
func (b Bounds) Rect() image.Rectangle {
	return image.Rect(b.Col, b.Row, b.Col+b.Width, b.Row+b.Height)
}

// BoundsOf returns a rectangle of cells as Bounds.
func BoundsOf(rect image.Rectangle) Bounds {
	return Bounds{Col: rect.Min.X, Row: rect.Min.Y, Width: rect.Dx(), Height: rect.Dy()}
}

// ResizeArgs changes a floor's bounds, adding or removing rows and columns on
// any side. Added cells get Ground and nothing on top; then Cells, Anchors and
// Placements, which must lie in added cells, are put back. Undo uses them to
// restore what a resize removed.
type ResizeArgs struct {
	Bounds     Bounds             `json:"bounds"`
	Ground     string             `json:"ground"`
	Cells      []Cell             `json:"cells,omitempty"`
	Anchors    []AddAnchorArgs    `json:"anchors,omitempty"`
	Placements []AddPlacementArgs `json:"placements,omitempty"`
}

// Resize is the edit kind that changes a floor's bounds.
type Resize struct{}

// Name identifies the edit kind.
func (Resize) Name() string { return ResizeEdit }

// Format names the document format the edit applies to.
func (Resize) Format() string { return floor.FormatName }

// Apply resizes the floor. Its inverse resizes back and restores the removed
// cells and anchors, so undo brings back exactly what was there.
func (Resize) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	floorDoc, err := target(document)
	if err != nil {
		return model.Applied{}, err
	}
	resize, err := decodeArgs[ResizeArgs](args)
	if err != nil {
		return model.Applied{}, err
	}
	old, next := floorDoc.Floor.Bounds(), resize.Bounds.Rect()
	if err := checkResize(old, next, resize); err != nil {
		return model.Applied{}, err
	}
	inverse := resizeInverse(floorDoc.Floor, next)
	working := floorDoc.Floor.Clone()
	if _, err := working.Resize(next, resize.Ground); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	for _, cell := range resize.Cells {
		set := working.SetGround
		if cell.Layer == LayerTop {
			set = working.SetTop
		}
		if _, err := set(cell.Col, cell.Row, cell.Tile); err != nil {
			return model.Applied{}, fmt.Errorf("restore after validation: %w", err)
		}
	}
	for _, restored := range resize.Anchors {
		if err := working.InsertAnchor(restored.Index, restored.Anchor); err != nil {
			return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
	}
	for _, restored := range resize.Placements {
		if err := working.InsertPlacement(restored.Index, restored.Placement); err != nil {
			return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
	}
	inverseArgs, err := encodeArgs(inverse)
	if err != nil {
		return model.Applied{}, err
	}
	floorDoc.Floor = working

	return model.Applied{
		Inverse: inverseArgs, InverseEdit: "", Touched: resizeTouched(old, next, inverse, resize), Covers: nil,
	}, nil
}

// checkResize validates a resize: real new bounds, a bounded number of cells
// added and removed, and restored cells and anchors only in added cells.
func checkResize(old, next image.Rectangle, resize ResizeArgs) error {
	if err := floor.CheckBounds(next); err != nil {
		return fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if next == old {
		return fmt.Errorf("%w: the floor already has bounds %v", model.ErrArgs, next)
	}
	kept := area(old.Intersect(next))
	if changed := area(old) - kept + area(next) - kept; changed > MaxResizeCells {
		return fmt.Errorf("%w: a resize may add and remove at most %d cells, not %d", model.ErrArgs, MaxResizeCells, changed)
	}
	added := func(col, row int) bool {
		cell := image.Pt(col, row)

		return cell.In(next) && !cell.In(old)
	}
	if len(resize.Cells) > 2*MaxResizeCells {
		return fmt.Errorf("%w: %d restored cells", model.ErrArgs, len(resize.Cells))
	}
	for _, cell := range resize.Cells {
		if !added(cell.Col, cell.Row) || (cell.Layer != LayerGround && cell.Layer != LayerTop) {
			return fmt.Errorf("%w: restored cell %+v is not a new cell", model.ErrArgs, cell)
		}
	}
	for _, restored := range resize.Anchors {
		if !added(restored.Anchor.Col, restored.Anchor.Row) {
			return fmt.Errorf("%w: restored anchor %q is not on a new cell", model.ErrArgs, restored.Anchor.Name)
		}
	}
	for _, restored := range resize.Placements {
		if !added(restored.Placement.Col, restored.Placement.Row) {
			return fmt.Errorf("%w: restored placement %s is not on a new cell", model.ErrArgs, restored.Placement.ID)
		}
	}

	return nil
}

func area(rect image.Rectangle) int { return rect.Dx() * rect.Dy() }

// cellsOutside returns the cells of from that lie outside other, row by row.
func cellsOutside(from, other image.Rectangle) []image.Point {
	var cells []image.Point
	for row := from.Min.Y; row < from.Max.Y; row++ {
		for col := from.Min.X; col < from.Max.X; col++ {
			if cell := image.Pt(col, row); !cell.In(other) {
				cells = append(cells, cell)
			}
		}
	}

	return cells
}

// resizeInverse returns the resize that undoes resizing current to next: back
// to the current bounds, with the most common removed ground as the fill and
// every other removed tile, anchor and placement restored.
func resizeInverse(current *floor.Floor, next image.Rectangle) ResizeArgs {
	removed := cellsOutside(current.Bounds(), next)
	counts := map[string]int{}
	for _, cell := range removed {
		ground, _ := current.Ground(cell.X, cell.Y) //nolint:errcheck // The cell is on the floor.
		counts[ground]++
	}
	fill, best := "", 0
	for ground, count := range counts {
		if count > best || (count == best && cmp.Less(ground, fill)) {
			fill, best = ground, count
		}
	}
	var cells []Cell
	for _, cell := range removed {
		ground, _ := current.Ground(cell.X, cell.Y) //nolint:errcheck // The cell is on the floor.
		top, _ := current.Top(cell.X, cell.Y)       //nolint:errcheck // The cell is on the floor.
		if ground != fill {
			cells = append(cells, Cell{Col: cell.X, Row: cell.Y, Layer: LayerGround, Tile: ground})
		}
		if top != "" {
			cells = append(cells, Cell{Col: cell.X, Row: cell.Y, Layer: LayerTop, Tile: top})
		}
	}
	var anchors []AddAnchorArgs
	for index, anchor := range current.Anchors() {
		if !image.Pt(anchor.Col, anchor.Row).In(next) {
			anchors = append(anchors, AddAnchorArgs{Anchor: anchor, Index: index})
		}
	}

	var placements []AddPlacementArgs
	for index, placement := range current.Placements() {
		if !image.Pt(placement.Col, placement.Row).In(next) {
			placements = append(placements, AddPlacementArgs{Placement: placement, Index: index})
		}
	}

	return ResizeArgs{
		Bounds: BoundsOf(current.Bounds()), Ground: fill, Cells: cells, Anchors: anchors, Placements: placements,
	}
}

// resizeTouched lists what a resize changes: the bounds, both layers of every
// cell added or removed, and every anchor and placement removed or restored.
func resizeTouched(old, next image.Rectangle, inverse, resize ResizeArgs) []string {
	touched := []string{BoundsKey}
	for _, cells := range [][]image.Point{cellsOutside(old, next), cellsOutside(next, old)} {
		for _, cell := range cells {
			touched = append(touched, CellKey(LayerGround, cell.X, cell.Y), CellKey(LayerTop, cell.X, cell.Y))
		}
	}
	for _, anchors := range [][]AddAnchorArgs{inverse.Anchors, resize.Anchors} {
		for _, anchor := range anchors {
			touched = append(touched, AnchorKey(anchor.Anchor.ID))
		}
	}
	for _, placements := range [][]AddPlacementArgs{inverse.Placements, resize.Placements} {
		for _, placement := range placements {
			touched = append(touched, PlacementKey(placement.Placement.ID))
		}
	}

	return touched
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
		Documents: []model.DocumentType{{Format: floor.Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits: []model.EditKind{
			Paint{}, Resize{}, AddAnchor{}, RemoveAnchor{}, UpdateAnchor{},
			AddPlacement{}, RemovePlacement{}, UpdatePlacement{},
		},
		Required: nil,
	}
}
