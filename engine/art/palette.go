package art

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Palette size bounds.
const (
	MaxRamps  = 256
	MaxShades = 64
)

// StarterRampName names the one ramp a new shared palette starts with.
const StarterRampName = "grey"

// The starter ramp's shades, dark to light.
const (
	starterGrey1 RGB = 0x202020
	starterGrey2 RGB = 0x4A4A4A
	starterGrey3 RGB = 0x7A7A7A
	starterGrey4 RGB = 0xABABAB
	starterGrey5 RGB = 0xDCDCDC
)

// Shade is one colour of a ramp, with its opacity in percent. Its number,
// counted from 1 at the dark end, is its place in the ramp; its ID never
// changes.
type Shade struct {
	ID      ids.ULID `json:"id"`
	Color   RGB      `json:"color"`
	Opacity int      `json:"opacity"`
}

// Ramp is a palette's shades for one use, named for that use and ordered dark
// to light, such as "grass".
type Ramp struct {
	ID     ids.ULID `json:"id"`
	Name   string   `json:"name"`
	Shades []Shade  `json:"shades"`
}

func (r Ramp) clone() Ramp {
	return Ramp{ID: r.ID, Name: r.Name, Shades: slices.Clone(r.Shades)}
}

func (r Ramp) equal(other Ramp) bool {
	return r.ID == other.ID && r.Name == other.Name && slices.Equal(r.Shades, other.Shades)
}

// Palette is a document of named colours that art is painted with, in ramps.
type Palette struct {
	ramps []Ramp
}

type wirePalette struct {
	Ramps []Ramp `json:"ramps"`
}

// NewPalette returns a palette with no ramps.
func NewPalette() *Palette { return &Palette{ramps: nil} }

// StarterPalette returns the palette a project's shared palette starts with:
// one ramp, "grey", of five shades. newID supplies the permanent IDs.
func StarterPalette(newID func() (ids.ULID, error)) (*Palette, error) {
	rampID, err := newID()
	if err != nil {
		return nil, fmt.Errorf("starter palette: %w", err)
	}
	ramp := Ramp{ID: rampID, Name: StarterRampName, Shades: nil}
	for _, value := range []RGB{starterGrey1, starterGrey2, starterGrey3, starterGrey4, starterGrey5} {
		shadeID, err := newID()
		if err != nil {
			return nil, fmt.Errorf("starter palette: %w", err)
		}
		ramp.Shades = append(ramp.Shades, Shade{ID: shadeID, Color: value, Opacity: MaxOpacity})
	}
	palette := NewPalette()
	if err := palette.InsertRamp(0, ramp); err != nil {
		return nil, err
	}

	return palette, nil
}

// Ramps returns a copy of the palette's ramps, in order.
func (p *Palette) Ramps() []Ramp {
	ramps := make([]Ramp, 0, len(p.ramps))
	for _, ramp := range p.ramps {
		ramps = append(ramps, ramp.clone())
	}

	return ramps
}

// Ramp returns a copy of a ramp by ID.
func (p *Palette) Ramp(id ids.ULID) (Ramp, bool) {
	index := p.rampIndex(id)
	if index < 0 {
		var none Ramp

		return none, false
	}

	return p.ramps[index].clone(), true
}

// Lookup returns a shade of a ramp and its number, counted from 1 at the dark
// end.
func (p *Palette) Lookup(rampID, shadeID ids.ULID) (Shade, int, bool) {
	index := p.rampIndex(rampID)
	if index >= 0 {
		shades := p.ramps[index].Shades
		if at := shadeIndex(shades, shadeID); at >= 0 {
			return shades[at], at + 1, true
		}
	}
	var none Shade

	return none, 0, false
}

// ShadeName returns a shade's name: its ramp's name and its number, such as
// "grass 2".
func (p *Palette) ShadeName(rampID, shadeID ids.ULID) (string, bool) {
	_, number, ok := p.Lookup(rampID, shadeID)
	if !ok {
		return "", false
	}

	return p.ramps[p.rampIndex(rampID)].Name + " " + strconv.Itoa(number), true
}

// InsertRamp adds a ramp at an index, with its shades.
func (p *Palette) InsertRamp(index int, ramp Ramp) error {
	if index < 0 || index > len(p.ramps) || len(p.ramps) >= MaxRamps {
		return fmt.Errorf("%w: insert at %d of %d", ErrData, index, len(p.ramps))
	}
	if ramp.ID.IsZero() || p.rampIndex(ramp.ID) >= 0 {
		return fmt.Errorf("%w: ramp ID %s is missing or taken", ErrData, ramp.ID)
	}
	if err := p.checkName(ramp.ID, ramp.Name); err != nil {
		return err
	}
	if len(ramp.Shades) > MaxShades {
		return fmt.Errorf("%w: %d shades", ErrShade, len(ramp.Shades))
	}
	for at, shade := range ramp.Shades {
		if err := p.checkShade(shade); err != nil {
			return err
		}
		if shadeIndex(ramp.Shades[:at], shade.ID) >= 0 {
			return fmt.Errorf("%w: shade ID %s repeated", ErrShade, shade.ID)
		}
	}
	p.ramps = slices.Insert(p.ramps, index, ramp.clone())

	return nil
}

// RemoveRamp removes a ramp, returning it and where it was.
func (p *Palette) RemoveRamp(id ids.ULID) (Ramp, int, error) {
	index := p.rampIndex(id)
	if index < 0 {
		var none Ramp

		return none, 0, fmt.Errorf("%w: %s", ErrNoRamp, id)
	}
	removed := p.ramps[index]
	p.ramps = slices.Delete(p.ramps, index, index+1)

	return removed, index, nil
}

// RenameRamp renames a ramp, returning its previous name.
func (p *Palette) RenameRamp(id ids.ULID, name string) (string, error) {
	index := p.rampIndex(id)
	if index < 0 {
		return "", fmt.Errorf("%w: %s", ErrNoRamp, id)
	}
	if err := p.checkName(id, name); err != nil {
		return "", err
	}
	previous := p.ramps[index].Name
	p.ramps[index].Name = name

	return previous, nil
}

// InsertShade adds a shade to a ramp at an index.
func (p *Palette) InsertShade(rampID ids.ULID, index int, shade Shade) error {
	rampAt := p.rampIndex(rampID)
	if rampAt < 0 {
		return fmt.Errorf("%w: %s", ErrNoRamp, rampID)
	}
	shades := p.ramps[rampAt].Shades
	if index < 0 || index > len(shades) || len(shades) >= MaxShades {
		return fmt.Errorf("%w: insert rampAt %d of %d", ErrShade, index, len(shades))
	}
	if err := p.checkShade(shade); err != nil {
		return err
	}
	p.ramps[rampAt].Shades = slices.Insert(slices.Clone(shades), index, shade)

	return nil
}

// RemoveShade removes a shade from a ramp, returning it and where it was.
func (p *Palette) RemoveShade(rampID, shadeID ids.ULID) (Shade, int, error) {
	var none Shade
	rampAt := p.rampIndex(rampID)
	if rampAt < 0 {
		return none, 0, fmt.Errorf("%w: %s", ErrNoRamp, rampID)
	}
	shades := p.ramps[rampAt].Shades
	index := shadeIndex(shades, shadeID)
	if index < 0 {
		return none, 0, fmt.Errorf("%w: %s", ErrNoShade, shadeID)
	}
	removed := shades[index]
	p.ramps[rampAt].Shades = slices.Delete(slices.Clone(shades), index, index+1)

	return removed, index, nil
}

// SetShade changes the colour and opacity of the shade with the given
// shade's ID, returning the shade as it was.
func (p *Palette) SetShade(rampID ids.ULID, shade Shade) (Shade, error) {
	var none Shade
	rampAt := p.rampIndex(rampID)
	if rampAt < 0 {
		return none, fmt.Errorf("%w: %s", ErrNoRamp, rampID)
	}
	index := shadeIndex(p.ramps[rampAt].Shades, shade.ID)
	if index < 0 {
		return none, fmt.Errorf("%w: %s", ErrNoShade, shade.ID)
	}
	if err := checkShadeValue(shade); err != nil {
		return none, err
	}
	previous := p.ramps[rampAt].Shades[index]
	p.ramps[rampAt].Shades = slices.Clone(p.ramps[rampAt].Shades)
	p.ramps[rampAt].Shades[index] = shade

	return previous, nil
}

// Clone returns an independent copy.
func (p *Palette) Clone() *Palette {
	return &Palette{ramps: p.Ramps()}
}

// Equal reports whether two palettes hold the same ramps and shades, in the
// same order.
func (p *Palette) Equal(other *Palette) bool {
	return slices.EqualFunc(p.ramps, other.ramps, Ramp.equal)
}

// Encode returns the palette's saved data.
func (p *Palette) Encode() (json.RawMessage, error) {
	ramps := p.ramps
	if ramps == nil {
		ramps = []Ramp{}
	}
	data, err := json.Marshal(wirePalette{Ramps: ramps})
	if err != nil {
		return nil, fmt.Errorf("encode palette: %w", err)
	}

	return data, nil
}

// DecodePalette reads saved palette data in the current format.
func DecodePalette(data json.RawMessage) (*Palette, error) {
	var wire wirePalette
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	palette := NewPalette()
	for _, ramp := range wire.Ramps {
		if err := palette.InsertRamp(len(palette.ramps), ramp); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
	}

	return palette, nil
}

func (p *Palette) rampIndex(id ids.ULID) int {
	return slices.IndexFunc(p.ramps, func(ramp Ramp) bool { return ramp.ID == id })
}

func shadeIndex(shades []Shade, id ids.ULID) int {
	return slices.IndexFunc(shades, func(shade Shade) bool { return shade.ID == id })
}

func (p *Palette) checkName(id ids.ULID, name string) error {
	if name == "" || name != strings.TrimSpace(name) || len(name) > MaxNameLength {
		return fmt.Errorf("%w: %q", ErrName, name)
	}
	for _, ramp := range p.ramps {
		if ramp.ID != id && ramp.Name == name {
			return fmt.Errorf("%w: %q is taken", ErrName, name)
		}
	}

	return nil
}

func (p *Palette) checkShade(shade Shade) error {
	if shade.ID.IsZero() {
		return fmt.Errorf("%w: missing ID", ErrShade)
	}
	for _, ramp := range p.ramps {
		if shadeIndex(ramp.Shades, shade.ID) >= 0 {
			return fmt.Errorf("%w: shade ID %s is taken", ErrShade, shade.ID)
		}
	}

	return checkShadeValue(shade)
}

func checkShadeValue(shade Shade) error {
	if shade.Color > MaxRGB || shade.Opacity < MinOpacity || shade.Opacity > MaxOpacity {
		return fmt.Errorf("%w: colour %#x at %d%%", ErrShade, uint32(shade.Color), shade.Opacity)
	}

	return nil
}
