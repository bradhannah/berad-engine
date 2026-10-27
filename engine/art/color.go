package art

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// RGB is a colour as 0xRRGGBB, saved as "#RRGGBB".
type RGB uint32

// MaxRGB is the largest colour value.
const MaxRGB RGB = 0xFFFFFF

// Opacity bounds, in percent. A shade is never fully see-through: that is the
// clear colour's job.
const (
	MinOpacity = 1
	MaxOpacity = 100
)

const (
	hexDigits   = 6
	hexBase     = 16
	parseBits   = 32
	redShift    = 16
	greenShift  = 8
	channelMask = 0xFF
	opaque      = 255
	halfPercent = MaxOpacity / 2
)

// MarshalText writes the colour as "#RRGGBB".
func (c RGB) MarshalText() ([]byte, error) {
	if c > MaxRGB {
		return nil, fmt.Errorf("%w: colour %#x", ErrShade, uint32(c))
	}

	return []byte(c.String()), nil
}

// UnmarshalText reads a colour written as "#RRGGBB".
func (c *RGB) UnmarshalText(text []byte) error {
	value, err := ParseRGB(string(text))
	if err != nil {
		return err
	}
	*c = value

	return nil
}

// ParseRGB reads a colour written as "#RRGGBB" or "RRGGBB", in either case.
func ParseRGB(text string) (RGB, error) {
	digits := strings.TrimPrefix(text, "#")
	if len(digits) != hexDigits {
		return 0, fmt.Errorf("%w: colour %q", ErrShade, text)
	}
	value, err := strconv.ParseUint(digits, hexBase, parseBits)
	if err != nil {
		return 0, fmt.Errorf("%w: colour %q: %w", ErrShade, text, err)
	}

	return RGB(value), nil
}

// String returns the colour as "#RRGGBB".
func (c RGB) String() string {
	return fmt.Sprintf("#%06X", uint32(c))
}

// NRGBA returns the colour at an opacity in percent.
func (c RGB) NRGBA(opacity int) color.NRGBA {
	alpha := (min(max(opacity, 0), MaxOpacity)*opaque + halfPercent) / MaxOpacity

	return color.NRGBA{
		R: uint8(c >> redShift & channelMask), G: uint8(c >> greenShift & channelMask),
		B: uint8(c & channelMask), A: uint8(alpha),
	}
}

// ColorRef names one colour a pixel is painted with: a shade of a ramp in a
// palette, by their permanent IDs. The zero ColorRef is the clear colour,
// which erases.
type ColorRef struct {
	Palette ids.ULID `json:"palette"`
	Ramp    ids.ULID `json:"ramp"`
	Shade   ids.ULID `json:"shade"`
}

// IsClear reports whether the reference is the clear colour.
func (r ColorRef) IsClear() bool {
	return r.Palette.IsZero() && r.Ramp.IsZero() && r.Shade.IsZero()
}

func (r ColorRef) valid() bool {
	return r.IsClear() || (!r.Palette.IsZero() && !r.Ramp.IsZero() && !r.Shade.IsZero())
}
