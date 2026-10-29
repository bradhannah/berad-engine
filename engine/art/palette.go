package art

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Palette size bounds.
const (
	MaxRamps  = 256
	MaxShades = 64
)

// Shade is one colour of a ramp, with its opacity in percent and an optional
// short label. Its number, counted from 1 at the dark end, is its place in
// the ramp; its ID never changes, even when it moves to another ramp.
type Shade struct {
	ID      ids.ULID `json:"id"`
	Color   RGB      `json:"color"`
	Opacity int      `json:"opacity"`
	Label   string   `json:"label,omitempty"`
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

// Moved is a shadow record of a shade's old place: the ramp it was moved out
// of and its name there, such as "ember 1". A tile painted with it there still
// finds it, and says so until the pixels are repainted.
type Moved struct {
	Ramp  ids.ULID `json:"ramp"`
	Shade ids.ULID `json:"shade"`
	Name  string   `json:"name"`
}

// Palette is a document of named colours that art is painted with, in ramps,
// with shadow records of the shades moved between them.
type Palette struct {
	ramps []Ramp
	moved []Moved
}

type wirePalette struct {
	Ramps []Ramp  `json:"ramps"`
	Moved []Moved `json:"moved,omitempty"`
}

// NewPalette returns a palette with no ramps.
func NewPalette() *Palette { return &Palette{ramps: nil, moved: nil} }

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

// Resolve returns the shade a pixel's ramp and shade name: the shade in that
// ramp, or, through a shadow record, the shade moved out of it, reported as
// moved.
func (p *Palette) Resolve(rampID, shadeID ids.ULID) (Shade, bool, bool) {
	if shade, _, ok := p.Lookup(rampID, shadeID); ok {
		return shade, false, true
	}
	if _, ok := p.MovedFrom(rampID, shadeID); ok {
		if ramp, ok := p.Place(shadeID); ok {
			shade, _, found := p.Lookup(ramp, shadeID)

			return shade, true, found
		}
	}
	var none Shade

	return none, false, false
}

// Place returns the ramp a shade is in now.
func (p *Palette) Place(shadeID ids.ULID) (ids.ULID, bool) {
	for _, ramp := range p.ramps {
		if shadeIndex(ramp.Shades, shadeID) >= 0 {
			return ramp.ID, true
		}
	}

	return ids.ULID{}, false
}

// MovedFrom returns the shadow record of a shade moved out of a ramp.
func (p *Palette) MovedFrom(rampID, shadeID ids.ULID) (Moved, bool) {
	at := slices.IndexFunc(p.moved, func(record Moved) bool { return record.Ramp == rampID && record.Shade == shadeID })
	if at < 0 {
		return Moved{Ramp: ids.ULID{}, Shade: ids.ULID{}, Name: ""}, false
	}

	return p.moved[at], true
}

// MovedShades returns a copy of the palette's shadow records, oldest first.
func (p *Palette) MovedShades() []Moved { return slices.Clone(p.moved) }

// ShadeName returns a shade's name: its ramp's name and its number, such as
// "grass 2".
func (p *Palette) ShadeName(rampID, shadeID ids.ULID) (string, bool) {
	_, number, ok := p.Lookup(rampID, shadeID)
	if !ok {
		return "", false
	}

	return p.ramps[p.rampIndex(rampID)].Name + " " + strconv.Itoa(number), true
}

// Labelled returns the shade with a label and its ramp.
func (p *Palette) Labelled(label string) (ids.ULID, Shade, bool) {
	for _, ramp := range p.ramps {
		for _, shade := range ramp.Shades {
			if label != "" && shade.Label == label {
				return ramp.ID, shade, true
			}
		}
	}
	var none Shade

	return ids.ULID{}, none, false
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
	for position, shade := range ramp.Shades {
		if err := p.checkShade(shade); err != nil {
			return err
		}
		if err := p.checkLabel(shade); err != nil {
			return err
		}
		if shadeIndex(ramp.Shades[:position], shade.ID) >= 0 {
			return fmt.Errorf("%w: shade ID %s repeated", ErrShade, shade.ID)
		}
		if shade.Label != "" && slices.ContainsFunc(ramp.Shades[:position], func(other Shade) bool {
			return other.Label == shade.Label
		}) {
			return fmt.Errorf("%w: %q repeated", ErrLabel, shade.Label)
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

// MoveRamp moves a ramp to an index, returning where it was.
func (p *Palette) MoveRamp(id ids.ULID, index int) (int, error) {
	from := p.rampIndex(id)
	if from < 0 {
		return 0, fmt.Errorf("%w: %s", ErrNoRamp, id)
	}
	if index < 0 || index >= len(p.ramps) {
		return 0, fmt.Errorf("%w: move to %d of %d", ErrData, index, len(p.ramps))
	}
	ramp := p.ramps[from]
	ramps := slices.Delete(slices.Clone(p.ramps), from, from+1)
	p.ramps = slices.Insert(ramps, index, ramp)

	return from, nil
}

// MoveShade moves a shade to an index in a ramp, its own or another,
// returning the ramp and index it left. Leaving a ramp keeps a shadow record
// of its old place, and coming back to a ramp it left drops that ramp's
// record. When records is not nil, the palette's shadow records become
// exactly those instead, as undoing a move needs.
func (p *Palette) MoveShade(shadeID, toRamp ids.ULID, index int, records []Moved) (ids.ULID, int, error) {
	fromRamp, found := p.Place(shadeID)
	toAt := p.rampIndex(toRamp)
	if !found || toAt < 0 {
		return ids.ULID{}, 0, fmt.Errorf("%w: %s to %s", ErrNoShade, shadeID, toRamp)
	}
	fromAt := p.rampIndex(fromRamp)
	name, _ := p.ShadeName(fromRamp, shadeID)
	from := shadeIndex(p.ramps[fromAt].Shades, shadeID)
	limit := len(p.ramps[toAt].Shades)
	if fromRamp == toRamp {
		limit--
	} else if limit >= MaxShades {
		return ids.ULID{}, 0, fmt.Errorf("%w: %s holds %d shades", ErrShade, toRamp, limit)
	}
	if index < 0 || index > limit {
		return ids.ULID{}, 0, fmt.Errorf("%w: move to %d of %d", ErrShade, index, limit)
	}
	shade := p.ramps[fromAt].Shades[from]
	p.ramps[fromAt].Shades = slices.Delete(slices.Clone(p.ramps[fromAt].Shades), from, from+1)
	p.ramps[toAt].Shades = slices.Insert(slices.Clone(p.ramps[toAt].Shades), index, shade)
	switch {
	case records != nil:
		p.moved = slices.Clone(records)
	case fromRamp != toRamp:
		p.moved = slices.DeleteFunc(slices.Clone(p.moved), func(record Moved) bool {
			return record.Shade == shadeID && record.Ramp == toRamp
		})
		if _, ok := p.MovedFrom(fromRamp, shadeID); !ok {
			p.moved = append(p.moved, Moved{Ramp: fromRamp, Shade: shadeID, Name: name})
		}
	}

	return fromRamp, from, nil
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
	if err := p.checkLabel(shade); err != nil {
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

// SetShade changes the colour, opacity and label of the shade with the given
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
	if err := p.checkLabel(shade); err != nil {
		return none, err
	}
	previous := p.ramps[rampAt].Shades[index]
	p.ramps[rampAt].Shades = slices.Clone(p.ramps[rampAt].Shades)
	p.ramps[rampAt].Shades[index] = shade

	return previous, nil
}

// Clone returns an independent copy.
func (p *Palette) Clone() *Palette {
	return &Palette{ramps: p.Ramps(), moved: slices.Clone(p.moved)}
}

// Equal reports whether two palettes hold the same ramps and shades, in the
// same order, and the same shadow records.
func (p *Palette) Equal(other *Palette) bool {
	return slices.EqualFunc(p.ramps, other.ramps, Ramp.equal) && slices.Equal(p.moved, other.moved)
}

// Encode returns the palette's saved data.
func (p *Palette) Encode() (json.RawMessage, error) {
	ramps := p.ramps
	if ramps == nil {
		ramps = []Ramp{}
	}
	data, err := json.Marshal(wirePalette{Ramps: ramps, Moved: p.moved})
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
	for at, record := range wire.Moved {
		if record.Ramp.IsZero() || record.Shade.IsZero() || slices.ContainsFunc(wire.Moved[:at], func(other Moved) bool {
			return other.Ramp == record.Ramp && other.Shade == record.Shade
		}) {
			return nil, fmt.Errorf("%w: moved colour %d", ErrData, at+1)
		}
	}
	palette.moved = wire.Moved

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
	if shade.Label != strings.TrimSpace(shade.Label) || utf8.RuneCountInString(shade.Label) > MaxLabelLength {
		return fmt.Errorf("%w: %q", ErrLabel, shade.Label)
	}

	return nil
}

// checkLabel refuses a label another shade of the palette has.
func (p *Palette) checkLabel(shade Shade) error {
	if _, other, ok := p.Labelled(shade.Label); ok && other.ID != shade.ID {
		return fmt.Errorf("%w: %q is taken", ErrLabel, shade.Label)
	}

	return nil
}
