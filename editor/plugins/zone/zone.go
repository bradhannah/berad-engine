// Package zone is the editor half of zones: the zone document and the edits
// that add, remove and reorder its floors and list the palettes it uses.
package zone

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// PluginName identifies the plug-in.
const PluginName = "zone"

// Edit kind names.
const (
	InsertFloorEdit = "zone.floor.insert"
	RemoveFloorEdit = "zone.floor.remove"
	MoveFloorEdit   = "zone.floor.move"

	InsertPaletteEdit = "zone.palette.insert"
	RemovePaletteEdit = "zone.palette.remove"
)

// floorsItem is the undo-clash item for the order of a zone's floors, and
// palettesItem for the palettes it lists.
const (
	floorsItem   = "floors"
	palettesItem = "palettes"
)

// ErrNotZone reports a document that is not a zone.
var ErrNotZone = errors.New("document is not a zone")

// Document is a zone held by the editor.
type Document struct {
	Zone *zone.Zone
}

// NewDocument returns a zone with no floors.
func NewDocument() *Document {
	return &Document{Zone: zone.New()}
}

// Format names the document format.
func (d *Document) Format() string { return zone.FormatName }

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	data, err := d.Zone.Encode()
	if err != nil {
		return nil, fmt.Errorf("zone document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document {
	return &Document{Zone: d.Zone.Clone()}
}

// Parts lists the floors: floor names are unique within their zone.
func (d *Document) Parts() []ids.ULID { return d.Zone.Floors() }

// References lists the zone's floors and the palettes it lists, so a palette
// a zone lists is not deleted from under it.
func (d *Document) References() []model.Reference {
	floors, palettes := d.Zone.Floors(), d.Zone.Palettes()
	references := make([]model.Reference, 0, len(floors)+len(palettes))
	for index, floor := range floors {
		references = append(references, model.Reference{
			Document: floor, Item: "", Label: "floor " + strconv.Itoa(index+1) + " from the bottom",
		})
	}
	for _, palette := range palettes {
		references = append(references, model.Reference{Document: palette, Item: "", Label: "listed by the zone"})
	}

	return references
}

// Decode reads a saved zone.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	decoded, err := zone.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("zone document: %w", err)
	}

	return &Document{Zone: decoded}, nil
}

// FloorArgs names a floor and, for insert and move, its position from the
// bottom.
type FloorArgs struct {
	Floor ids.ULID `json:"floor"`
	Index int      `json:"index"`
}

// InsertFloor is the edit kind that adds a floor to a zone.
type InsertFloor struct{}

// Name identifies the edit kind.
func (InsertFloor) Name() string { return InsertFloorEdit }

// Format names the document format the edit applies to.
func (InsertFloor) Format() string { return zone.FormatName }

// Apply inserts the floor.
func (InsertFloor) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, func(z *zone.Zone, floor FloorArgs) (string, FloorArgs, error) {
		return RemoveFloorEdit, FloorArgs{Floor: floor.Floor, Index: 0}, z.Insert(floor.Index, floor.Floor)
	})
}

// RemoveFloor is the edit kind that takes a floor out of a zone.
type RemoveFloor struct{}

// Name identifies the edit kind.
func (RemoveFloor) Name() string { return RemoveFloorEdit }

// Format names the document format the edit applies to.
func (RemoveFloor) Format() string { return zone.FormatName }

// Apply removes the floor.
func (RemoveFloor) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, func(z *zone.Zone, floor FloorArgs) (string, FloorArgs, error) {
		index, err := z.Remove(floor.Floor)
		if err != nil {
			return "", FloorArgs{Floor: floor.Floor, Index: 0}, fmt.Errorf("remove floor: %w", err)
		}

		return InsertFloorEdit, FloorArgs{Floor: floor.Floor, Index: index}, nil
	})
}

// MoveFloor is the edit kind that changes a floor's position.
type MoveFloor struct{}

// Name identifies the edit kind.
func (MoveFloor) Name() string { return MoveFloorEdit }

// Format names the document format the edit applies to.
func (MoveFloor) Format() string { return zone.FormatName }

// Apply moves the floor.
func (MoveFloor) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, func(z *zone.Zone, floor FloorArgs) (string, FloorArgs, error) {
		previous, err := z.Move(floor.Floor, floor.Index)
		if err != nil {
			return "", FloorArgs{Floor: floor.Floor, Index: 0}, fmt.Errorf("move floor: %w", err)
		}

		return "", FloorArgs{Floor: floor.Floor, Index: previous}, nil
	})
}

// PaletteArgs names a palette a zone lists and, for insert, its place in the
// list.
type PaletteArgs struct {
	Palette ids.ULID `json:"palette"`
	Index   int      `json:"index"`
}

// InsertPalette is the edit kind that lists a palette for a zone.
type InsertPalette struct{}

// Name identifies the edit kind.
func (InsertPalette) Name() string { return InsertPaletteEdit }

// Format names the document format the edit applies to.
func (InsertPalette) Format() string { return zone.FormatName }

// Apply lists the palette; its inverse takes it off again.
func (InsertPalette) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return changePalettes(document, args, func(z *zone.Zone, listed PaletteArgs) (string, PaletteArgs, error) {
		if err := z.InsertPalette(listed.Index, listed.Palette); err != nil {
			return "", listed, fmt.Errorf("list palette: %w", err)
		}

		return RemovePaletteEdit, PaletteArgs{Palette: listed.Palette, Index: 0}, nil
	})
}

// RemovePalette is the edit kind that takes a palette off a zone's list.
type RemovePalette struct{}

// Name identifies the edit kind.
func (RemovePalette) Name() string { return RemovePaletteEdit }

// Format names the document format the edit applies to.
func (RemovePalette) Format() string { return zone.FormatName }

// Apply takes the palette off; its inverse lists it again where it was.
func (RemovePalette) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return changePalettes(document, args, func(z *zone.Zone, listed PaletteArgs) (string, PaletteArgs, error) {
		index, err := z.RemovePalette(listed.Palette)
		if err != nil {
			return "", listed, fmt.Errorf("remove palette: %w", err)
		}

		return InsertPaletteEdit, PaletteArgs{Palette: listed.Palette, Index: index}, nil
	})
}

func change(
	document model.Document, args json.RawMessage,
	apply func(*zone.Zone, FloorArgs) (string, FloorArgs, error),
) (model.Applied, error) {
	return changeZone(document, args, floorsItem, apply)
}

func changePalettes(
	document model.Document, args json.RawMessage,
	apply func(*zone.Zone, PaletteArgs) (string, PaletteArgs, error),
) (model.Applied, error) {
	return changeZone(document, args, palettesItem, apply)
}

// changeZone applies one edit to a zone; apply returns the inverse edit's
// name, when it is another kind, and its arguments.
func changeZone[A any](
	document model.Document, args json.RawMessage, touched string,
	apply func(*zone.Zone, A) (string, A, error),
) (model.Applied, error) {
	zoneDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotZone)
	}
	var decoded A
	if err := json.Unmarshal(args, &decoded); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseEdit, inverse, err := apply(zoneDoc.Zone, decoded)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseArgs, err := json.Marshal(inverse)
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{Inverse: inverseArgs, InverseEdit: inverseEdit, Touched: []string{touched}, Covers: nil}, nil
}

// Plugin returns the zone plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name:      PluginName,
		Documents: []model.DocumentType{{Format: zone.Format(), Decode: Decode, Layout: nil, Prepare: nil}},
		Edits:     []model.EditKind{InsertFloor{}, RemoveFloor{}, MoveFloor{}, InsertPalette{}, RemovePalette{}},
		Required:  nil,
	}
}
