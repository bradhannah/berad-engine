// Package art is the editor half of a project's art: the palette, tile,
// sprite, settings and stand-in sprite documents, the edits that change them,
// and the documents every project holds: Default, the engine's palette, which
// is read only, the art settings and the stand-in sprites.
package art

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// PluginName identifies the plug-in.
const PluginName = "art"

// ErrWrongDocument reports an art edit given a document of another kind.
var ErrWrongDocument = errors.New("not the art document this edit changes")

// Item prefixes, for references and undo clashes.
const (
	rampPrefix     = "ramp:"
	rampNamePrefix = "ramp-name:"
	shadePrefix    = "shade:"
	standInPrefix  = "standin:"
	rampOrderItem  = "ramp-order"
	tileSizeItem   = "tile-size"
)

// RampItem names a ramp, as a tile's references and a ramp's removal name it.
func RampItem(ramp ids.ULID) string { return rampPrefix + ramp.String() }

// ShadeItem names a shade, as a tile's references and a shade's removal name
// it.
func ShadeItem(shade ids.ULID) string { return shadePrefix + shade.String() }

// PaletteDocument is a palette held by the editor.
type PaletteDocument struct {
	Palette *art.Palette
}

// Format names the document format.
func (d *PaletteDocument) Format() string { return art.PaletteFormatName }

// Encode returns the saved data.
func (d *PaletteDocument) Encode() (json.RawMessage, error) {
	data, err := d.Palette.Encode()
	if err != nil {
		return nil, fmt.Errorf("palette document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *PaletteDocument) Clone() model.Document { return &PaletteDocument{Palette: d.Palette.Clone()} }

// KeptWhileUsed keeps a palette while a zone lists it or a tile is painted
// with it: each reference is removed first.
func (d *PaletteDocument) KeptWhileUsed() bool { return true }

// DecodePalette reads a saved palette.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodePalette(data json.RawMessage) (model.Document, error) {
	palette, err := art.DecodePalette(data)
	if err != nil {
		return nil, fmt.Errorf("palette document: %w", err)
	}

	return &PaletteDocument{Palette: palette}, nil
}

// TileDocument is a tile held by the editor.
type TileDocument struct {
	Tile *art.Tile
}

// Format names the document format.
func (d *TileDocument) Format() string { return art.TileFormatName }

// Encode returns the saved data.
func (d *TileDocument) Encode() (json.RawMessage, error) {
	data, err := d.Tile.Encode()
	if err != nil {
		return nil, fmt.Errorf("tile document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *TileDocument) Clone() model.Document { return &TileDocument{Tile: d.Tile.Clone()} }

// References lists every ramp and shade the tile is painted with, so a ramp
// or shade it uses is not removed from under it.
func (d *TileDocument) References() []model.Reference {
	var references []model.Reference
	seen := map[model.Reference]bool{}
	for _, ref := range d.Tile.Colors() {
		if ref.IsLoose() {
			continue
		}
		for _, item := range []string{RampItem(ref.Ramp), ShadeItem(ref.Shade)} {
			reference := model.Reference{Document: ref.Palette, Item: item, Label: "painted with it"}
			if !seen[reference] {
				seen[reference] = true
				references = append(references, reference)
			}
		}
	}

	return references
}

// DecodeTile reads a saved tile.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodeTile(data json.RawMessage) (model.Document, error) {
	tile, err := art.DecodeTile(data)
	if err != nil {
		return nil, fmt.Errorf("tile document: %w", err)
	}

	return &TileDocument{Tile: tile}, nil
}

// SpriteDocument is a sprite held by the editor. Its tile is its part, so the
// tile's name is unique only among the sprite's parts and goes with it.
type SpriteDocument struct {
	Sprite art.Sprite
}

// Format names the document format.
func (d *SpriteDocument) Format() string { return art.SpriteFormatName }

// Encode returns the saved data.
func (d *SpriteDocument) Encode() (json.RawMessage, error) {
	data, err := d.Sprite.Encode()
	if err != nil {
		return nil, fmt.Errorf("sprite document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *SpriteDocument) Clone() model.Document { return &SpriteDocument{Sprite: d.Sprite} }

// Parts lists the sprite's tile.
func (d *SpriteDocument) Parts() []ids.ULID { return []ids.ULID{d.Sprite.Tile} }

// DecodeSprite reads a saved sprite.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodeSprite(data json.RawMessage) (model.Document, error) {
	sprite, err := art.DecodeSprite(data)
	if err != nil {
		return nil, fmt.Errorf("sprite document: %w", err)
	}

	return &SpriteDocument{Sprite: sprite}, nil
}

// SettingsDocument is a project's art settings held by the editor.
type SettingsDocument struct {
	Settings art.Settings
}

// Format names the document format.
func (d *SettingsDocument) Format() string { return art.SettingsFormatName }

// Encode returns the saved data.
func (d *SettingsDocument) Encode() (json.RawMessage, error) {
	data, err := d.Settings.Encode()
	if err != nil {
		return nil, fmt.Errorf("art settings document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *SettingsDocument) Clone() model.Document { return &SettingsDocument{Settings: d.Settings} }

// DecodeSettings reads saved art settings.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodeSettings(data json.RawMessage) (model.Document, error) {
	settings, err := art.DecodeSettings(data)
	if err != nil {
		return nil, fmt.Errorf("art settings document: %w", err)
	}

	return &SettingsDocument{Settings: settings}, nil
}

// StandInsDocument records which sprite shows each stand-in.
type StandInsDocument struct {
	Sprites *art.StandInSprites
}

// Format names the document format.
func (d *StandInsDocument) Format() string { return art.StandInsFormatName }

// Encode returns the saved data.
func (d *StandInsDocument) Encode() (json.RawMessage, error) {
	data, err := d.Sprites.Encode()
	if err != nil {
		return nil, fmt.Errorf("stand-in sprites document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *StandInsDocument) Clone() model.Document {
	return &StandInsDocument{Sprites: d.Sprites.Clone()}
}

// References lists the sprite that shows each stand-in, so deleting a sprite
// shows the stand-ins it shows.
func (d *StandInsDocument) References() []model.Reference {
	standIns := d.Sprites.StandIns()
	references := make([]model.Reference, 0, len(standIns))
	for _, standIn := range standIns {
		sprite, _ := d.Sprites.Sprite(standIn)
		name := standIn
		if definition, ok := tiles.Lookup(standIn); ok {
			name = definition.Name
		}
		references = append(references, model.Reference{Document: sprite, Item: "", Label: "shows " + name})
	}

	return references
}

// DecodeStandIns reads saved stand-in sprites.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodeStandIns(data json.RawMessage) (model.Document, error) {
	sprites, err := art.DecodeStandInSprites(data)
	if err != nil {
		return nil, fmt.Errorf("stand-in sprites document: %w", err)
	}

	return &StandInsDocument{Sprites: sprites}, nil
}

func defaultPalette() (json.RawMessage, error) {
	palette, err := art.DefaultPalette()
	if err != nil {
		return nil, fmt.Errorf("default palette: %w", err)
	}
	data, err := palette.Encode()
	if err != nil {
		return nil, fmt.Errorf("default palette: %w", err)
	}

	return data, nil
}

// preparePalette gives a new palette's ramps and shades their permanent IDs,
// as a palette made from colours chosen out of another needs.
func preparePalette(data json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	var wire struct {
		Ramps []art.Ramp  `json:"ramps"`
		Moved []art.Moved `json:"moved,omitempty"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	changed := false
	for at := range wire.Ramps {
		ramp := &wire.Ramps[at]
		for _, slot := range append([]*ids.ULID{&ramp.ID}, shadeIDs(ramp.Shades)...) {
			if !slot.IsZero() {
				continue
			}
			made, err := newID()
			if err != nil {
				return nil, fmt.Errorf("palette id: %w", err)
			}
			*slot, changed = made, true
		}
	}
	if !changed {
		return data, nil
	}
	if wire.Ramps == nil {
		wire.Ramps = []art.Ramp{}
	}

	return encode(wire)
}

func shadeIDs(shades []art.Shade) []*ids.ULID {
	found := make([]*ids.ULID, 0, len(shades))
	for at := range shades {
		found = append(found, &shades[at].ID)
	}

	return found
}

// Plugin returns the art plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name: PluginName,
		Documents: []model.DocumentType{
			{Format: art.PaletteFormat(), Decode: DecodePalette, Layout: nil, Prepare: preparePalette},
			{Format: art.TileFormat(), Decode: DecodeTile, Layout: nil, Prepare: nil},
			{Format: art.SpriteFormat(), Decode: DecodeSprite, Layout: nil, Prepare: nil},
			{Format: art.SettingsFormat(), Decode: DecodeSettings, Layout: nil, Prepare: nil},
			{Format: art.StandInsFormat(), Decode: DecodeStandIns, Layout: nil, Prepare: nil},
		},
		Edits: []model.EditKind{
			AddRamp{}, RemoveRamp{}, RenameRamp{}, MoveRamp{}, AddShade{}, RemoveShade{}, SetShade{},
			MoveShade{}, Paint{}, SetTileSize{}, SetStandIn{},
		},
		Required: []model.RequiredDocument{
			{
				ID: art.DefaultPaletteID(), Format: art.PaletteFormatName, Name: art.DefaultPaletteName,
				Data: defaultPalette, ReadOnly: true,
			},
			{
				ID: art.SettingsID(), Format: art.SettingsFormatName, Name: art.SettingsName,
				Data: art.DefaultSettings().Encode, ReadOnly: false,
			},
			{
				ID: art.StandInsID(), Format: art.StandInsFormatName, Name: art.StandInsName,
				Data: art.NewStandInSprites().Encode, ReadOnly: false,
			},
		},
	}
}
