package art

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Edit kind names.
const (
	AddRampEdit     = "art.ramp.add"
	RemoveRampEdit  = "art.ramp.remove"
	RenameRampEdit  = "art.ramp.rename"
	MoveRampEdit    = "art.ramp.move"
	MoveShadeEdit   = "art.shade.move"
	AddShadeEdit    = "art.shade.add"
	RemoveShadeEdit = "art.shade.remove"
	SetShadeEdit    = "art.shade.set"
	PaintEdit       = "art.tile.paint"
	TileSizeEdit    = "art.settings.tile-size"
	StandInEdit     = "art.standin.set"
)

// MaxPixels bounds the pixels one paint command may change.
const MaxPixels = art.MaxTileSize * art.MaxTileSize

// AddRampArgs adds a ramp at an index. Zero IDs are filled in before it is
// applied.
type AddRampArgs struct {
	Index int      `json:"index"`
	Ramp  art.Ramp `json:"ramp"`
}

// RemoveRampArgs removes a ramp.
type RemoveRampArgs struct {
	Ramp ids.ULID `json:"ramp"`
}

// RenameRampArgs renames a ramp.
type RenameRampArgs struct {
	Ramp ids.ULID `json:"ramp"`
	Name string   `json:"name"`
}

// MoveRampArgs moves a ramp to an index.
type MoveRampArgs struct {
	Ramp  ids.ULID `json:"ramp"`
	Index int      `json:"index"`
}

// MoveShadeArgs moves a shade to an index in a ramp, its own or another.
// Moved, when set, is exactly the shadow records the palette keeps after the
// move, as undoing one restores them; otherwise leaving a ramp records the
// shade's old place there.
type MoveShadeArgs struct {
	Shade ids.ULID    `json:"shade"`
	Ramp  ids.ULID    `json:"ramp"`
	Index int         `json:"index"`
	Moved []art.Moved `json:"moved"`
}

// AddShadeArgs adds a shade to a ramp at an index. A zero ID is filled in
// before it is applied.
type AddShadeArgs struct {
	Ramp  ids.ULID  `json:"ramp"`
	Index int       `json:"index"`
	Shade art.Shade `json:"shade"`
}

// RemoveShadeArgs removes a shade from a ramp.
type RemoveShadeArgs struct {
	Ramp  ids.ULID `json:"ramp"`
	Shade ids.ULID `json:"shade"`
}

// SetShadeArgs changes a shade's colour, opacity and label.
type SetShadeArgs struct {
	Ramp  ids.ULID  `json:"ramp"`
	Shade art.Shade `json:"shade"`
}

// Pixel is one painted pixel of a tile.
type Pixel struct {
	Col   int          `json:"col"`
	Row   int          `json:"row"`
	Color art.ColorRef `json:"color"`
}

// PaintArgs paints pixels, applied in order.
type PaintArgs struct {
	Pixels []Pixel `json:"pixels"`
}

// TileSizeArgs sets the project's tile size.
type TileSizeArgs struct {
	Size int `json:"size"`
}

// StandInArgs chooses the sprite that shows a stand-in; a zero sprite puts it
// back to its flat colour.
type StandInArgs struct {
	StandIn string   `json:"standIn"`
	Sprite  ids.ULID `json:"sprite"`
}

// change applies one edit to a document of type D with arguments of type A.
// apply returns the inverse arguments and the items it touched, and must
// leave the document untouched when it fails.
func change[D model.Document, A any](
	document model.Document, args json.RawMessage, inverseEdit string,
	apply func(target D, decoded A) (any, []string, error),
) (model.Applied, error) {
	target, ok := document.(D)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w: %s", model.ErrWrongDocument, ErrWrongDocument, document.Format())
	}
	decoded, err := decode[A](args)
	if err != nil {
		return model.Applied{}, err
	}
	inverse, touched, err := apply(target, decoded)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseArgs, err := json.Marshal(inverse)
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{Inverse: inverseArgs, InverseEdit: inverseEdit, Touched: touched, Covers: nil}, nil
}

//nolint:ireturn // T is the caller-chosen concrete argument type.
func decode[T any](args json.RawMessage) (T, error) {
	var decoded T
	if err := json.Unmarshal(args, &decoded); err != nil {
		return decoded, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}

	return decoded, nil
}

func encode(args any) (json.RawMessage, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("encode arguments: %w", err)
	}

	return data, nil
}

func rampItems(ramp art.Ramp) []string {
	items := make([]string, 0, len(ramp.Shades)+2) //nolint:mnd // The ramp and its name.
	items = append(items, RampItem(ramp.ID), rampNamePrefix+ramp.ID.String())
	for _, shade := range ramp.Shades {
		items = append(items, ShadeItem(shade.ID))
	}

	return items
}

// AddRamp is the edit kind that adds a ramp to a palette.
type AddRamp struct{}

// Name identifies the edit kind.
func (AddRamp) Name() string { return AddRampEdit }

// Format names the document format the edit applies to.
func (AddRamp) Format() string { return art.PaletteFormatName }

// Prepare gives a new ramp and its shades their permanent IDs.
func (AddRamp) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	add, err := decode[AddRampArgs](args)
	if err != nil {
		return nil, err
	}
	changed := false
	if add.Ramp.ID.IsZero() {
		if add.Ramp.ID, err = newID(); err != nil {
			return nil, fmt.Errorf("ramp id: %w", err)
		}
		changed = true
	}
	for at := range add.Ramp.Shades {
		if add.Ramp.Shades[at].ID.IsZero() {
			if add.Ramp.Shades[at].ID, err = newID(); err != nil {
				return nil, fmt.Errorf("shade id: %w", err)
			}
			changed = true
		}
	}
	if !changed {
		return args, nil
	}

	return encode(add)
}

// Apply adds the ramp; its inverse removes it.
func (AddRamp) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, RemoveRampEdit, func(target *PaletteDocument, add AddRampArgs) (any, []string, error) {
		if err := target.Palette.InsertRamp(add.Index, add.Ramp); err != nil {
			return nil, nil, fmt.Errorf("add ramp: %w", err)
		}

		return RemoveRampArgs{Ramp: add.Ramp.ID}, rampItems(add.Ramp), nil
	})
}

// RemoveRamp is the edit kind that removes a ramp, refused while a tile is
// painted with it.
type RemoveRamp struct{}

// Name identifies the edit kind.
func (RemoveRamp) Name() string { return RemoveRampEdit }

// Format names the document format the edit applies to.
func (RemoveRamp) Format() string { return art.PaletteFormatName }

// Removes names the ramp.
func (RemoveRamp) Removes(_ model.Document, args json.RawMessage) ([]string, error) {
	remove, err := decode[RemoveRampArgs](args)
	if err != nil {
		return nil, err
	}

	return []string{RampItem(remove.Ramp)}, nil
}

// Apply removes the ramp; its inverse puts it back where it was.
func (RemoveRamp) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, AddRampEdit,
		func(target *PaletteDocument, remove RemoveRampArgs) (any, []string, error) {
			removed, index, err := target.Palette.RemoveRamp(remove.Ramp)
			if err != nil {
				return nil, nil, fmt.Errorf("remove ramp: %w", err)
			}

			return AddRampArgs{Index: index, Ramp: removed}, rampItems(removed), nil
		})
}

// RenameRamp is the edit kind that renames a ramp.
type RenameRamp struct{}

// Name identifies the edit kind.
func (RenameRamp) Name() string { return RenameRampEdit }

// Format names the document format the edit applies to.
func (RenameRamp) Format() string { return art.PaletteFormatName }

// Apply renames the ramp; its inverse restores the previous name.
func (RenameRamp) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *PaletteDocument, rename RenameRampArgs) (any, []string, error) {
		previous, err := target.Palette.RenameRamp(rename.Ramp, rename.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("rename ramp: %w", err)
		}

		return RenameRampArgs{Ramp: rename.Ramp, Name: previous}, []string{rampNamePrefix + rename.Ramp.String()}, nil
	})
}

// MoveRamp is the edit kind that moves a ramp to another place in its palette.
type MoveRamp struct{}

// Name identifies the edit kind.
func (MoveRamp) Name() string { return MoveRampEdit }

// Format names the document format the edit applies to.
func (MoveRamp) Format() string { return art.PaletteFormatName }

// Apply moves the ramp; its inverse moves it back.
func (MoveRamp) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *PaletteDocument, move MoveRampArgs) (any, []string, error) {
		from, err := target.Palette.MoveRamp(move.Ramp, move.Index)
		if err != nil {
			return nil, nil, fmt.Errorf("move ramp: %w", err)
		}

		return MoveRampArgs{Ramp: move.Ramp, Index: from}, []string{rampOrderItem}, nil
	})
}

// MoveShade is the edit kind that moves a shade within its ramp or to
// another. Tiles painted with it keep finding it through the shadow record of
// its old place.
type MoveShade struct{}

// Name identifies the edit kind.
func (MoveShade) Name() string { return MoveShadeEdit }

// Format names the document format the edit applies to.
func (MoveShade) Format() string { return art.PaletteFormatName }

// Apply moves the shade; its inverse moves it back, with the shadow records
// as they were.
func (MoveShade) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *PaletteDocument, move MoveShadeArgs) (any, []string, error) {
		before := target.Palette.MovedShades()
		if before == nil {
			before = []art.Moved{}
		}
		from, index, err := target.Palette.MoveShade(move.Shade, move.Ramp, move.Index, move.Moved)
		if err != nil {
			return nil, nil, fmt.Errorf("move shade: %w", err)
		}
		touched := []string{
			ShadeItem(move.Shade), rampOrderItem + ":" + from.String(), rampOrderItem + ":" + move.Ramp.String(),
		}

		return MoveShadeArgs{Shade: move.Shade, Ramp: from, Index: index, Moved: before}, touched, nil
	})
}

// AddShade is the edit kind that adds a shade to a ramp.
type AddShade struct{}

// Name identifies the edit kind.
func (AddShade) Name() string { return AddShadeEdit }

// Format names the document format the edit applies to.
func (AddShade) Format() string { return art.PaletteFormatName }

// Prepare gives a new shade its permanent ID.
func (AddShade) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	add, err := decode[AddShadeArgs](args)
	if err != nil {
		return nil, err
	}
	if !add.Shade.ID.IsZero() {
		return args, nil
	}
	if add.Shade.ID, err = newID(); err != nil {
		return nil, fmt.Errorf("shade id: %w", err)
	}

	return encode(add)
}

// Apply adds the shade; its inverse removes it.
func (AddShade) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, RemoveShadeEdit, func(target *PaletteDocument, add AddShadeArgs) (any, []string, error) {
		if err := target.Palette.InsertShade(add.Ramp, add.Index, add.Shade); err != nil {
			return nil, nil, fmt.Errorf("add shade: %w", err)
		}

		return RemoveShadeArgs{Ramp: add.Ramp, Shade: add.Shade.ID}, []string{ShadeItem(add.Shade.ID)}, nil
	})
}

// RemoveShade is the edit kind that removes a shade, refused while a tile is
// painted with it.
type RemoveShade struct{}

// Name identifies the edit kind.
func (RemoveShade) Name() string { return RemoveShadeEdit }

// Format names the document format the edit applies to.
func (RemoveShade) Format() string { return art.PaletteFormatName }

// Removes names the shade.
func (RemoveShade) Removes(_ model.Document, args json.RawMessage) ([]string, error) {
	remove, err := decode[RemoveShadeArgs](args)
	if err != nil {
		return nil, err
	}

	return []string{ShadeItem(remove.Shade)}, nil
}

// Apply removes the shade; its inverse puts it back where it was.
func (RemoveShade) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, AddShadeEdit,
		func(target *PaletteDocument, remove RemoveShadeArgs) (any, []string, error) {
			removed, index, err := target.Palette.RemoveShade(remove.Ramp, remove.Shade)
			if err != nil {
				return nil, nil, fmt.Errorf("remove shade: %w", err)
			}

			return AddShadeArgs{Ramp: remove.Ramp, Index: index, Shade: removed}, []string{ShadeItem(removed.ID)}, nil
		})
}

// SetShade is the edit kind that changes a shade's colour and opacity.
type SetShade struct{}

// Name identifies the edit kind.
func (SetShade) Name() string { return SetShadeEdit }

// Format names the document format the edit applies to.
func (SetShade) Format() string { return art.PaletteFormatName }

// Apply changes the shade; its inverse restores it.
func (SetShade) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *PaletteDocument, set SetShadeArgs) (any, []string, error) {
		previous, err := target.Palette.SetShade(set.Ramp, set.Shade)
		if err != nil {
			return nil, nil, fmt.Errorf("set shade: %w", err)
		}

		return SetShadeArgs{Ramp: set.Ramp, Shade: previous}, []string{ShadeItem(previous.ID)}, nil
	})
}

// Paint is the edit kind that paints a tile's pixels.
type Paint struct{}

// Name identifies the edit kind.
func (Paint) Name() string { return PaintEdit }

// Format names the document format the edit applies to.
func (Paint) Format() string { return art.TileFormatName }

// Apply paints the pixels in order; its inverse restores them in reverse
// order, so a pixel painted twice in one command comes back as it was.
func (Paint) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *TileDocument, paint PaintArgs) (any, []string, error) {
		if len(paint.Pixels) == 0 || len(paint.Pixels) > MaxPixels {
			return nil, nil, fmt.Errorf("%w: %d pixels", model.ErrArgs, len(paint.Pixels))
		}
		painted := target.Tile.Clone()
		inverse := make([]Pixel, 0, len(paint.Pixels))
		var touched []string
		for _, pixel := range paint.Pixels {
			previous, err := painted.Set(pixel.Col, pixel.Row, pixel.Color)
			if err != nil {
				return nil, nil, fmt.Errorf("paint: %w", err)
			}
			inverse = append(inverse, Pixel{Col: pixel.Col, Row: pixel.Row, Color: previous})
			item := strconv.Itoa(pixel.Col) + "," + strconv.Itoa(pixel.Row)
			if !slices.Contains(touched, item) {
				touched = append(touched, item)
			}
		}
		target.Tile = painted
		slices.Reverse(inverse)

		return PaintArgs{Pixels: inverse}, touched, nil
	})
}

// SetTileSize is the edit kind that sets the project's tile size.
type SetTileSize struct{}

// Name identifies the edit kind.
func (SetTileSize) Name() string { return TileSizeEdit }

// Format names the document format the edit applies to.
func (SetTileSize) Format() string { return art.SettingsFormatName }

// Apply sets the tile size; its inverse restores the previous size.
func (SetTileSize) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *SettingsDocument, set TileSizeArgs) (any, []string, error) {
		settings := target.Settings
		settings.TileSize = set.Size
		if err := settings.Validate(); err != nil {
			return nil, nil, fmt.Errorf("tile size: %w", err)
		}
		previous := target.Settings.TileSize
		target.Settings = settings

		return TileSizeArgs{Size: previous}, []string{tileSizeItem}, nil
	})
}

// SetStandIn is the edit kind that chooses the sprite that shows a stand-in.
type SetStandIn struct{}

// Name identifies the edit kind.
func (SetStandIn) Name() string { return StandInEdit }

// Format names the document format the edit applies to.
func (SetStandIn) Format() string { return art.StandInsFormatName }

// Apply sets the stand-in's sprite; its inverse restores the previous one.
func (SetStandIn) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	return change(document, args, "", func(target *StandInsDocument, set StandInArgs) (any, []string, error) {
		previous, err := target.Sprites.Set(set.StandIn, set.Sprite)
		if err != nil {
			return nil, nil, fmt.Errorf("stand-in sprite: %w", err)
		}

		return StandInArgs{StandIn: set.StandIn, Sprite: previous}, []string{standInPrefix + set.StandIn}, nil
	})
}
