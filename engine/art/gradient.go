package art

import (
	"fmt"
	"math"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// MinSteps is the fewest colours a gradient makes: its two ends.
const MinSteps = 2

// Gradient returns the colours a gradient fills a ramp with: steps colours
// from the left one to the right one, both included. It mixes the colour,
// the opacity or both; what it does not mix stays the left colour's. Colours
// mix in OKLab, so the steps look evenly spaced, and opacities evenly. The
// colours have no IDs or labels yet.
func Gradient(left, right Shade, steps int, mixColor, mixOpacity bool) ([]Shade, error) {
	if steps < MinSteps || steps > MaxShades {
		return nil, fmt.Errorf("%w: %d steps", ErrShade, steps)
	}
	if err := checkShadeValue(left); err != nil {
		return nil, err
	}
	if err := checkShadeValue(right); err != nil {
		return nil, err
	}
	start, end := toOKLab(left.Color), toOKLab(right.Color)
	shades := make([]Shade, 0, steps)
	for step := range steps {
		along := float64(step) / float64(steps-1)
		shade := Shade{ID: ids.ULID{}, Color: left.Color, Opacity: left.Opacity, Label: ""}
		if mixColor {
			shade.Color = fromOKLab(oklab{
				l: start.l + (end.l-start.l)*along, a: start.a + (end.a-start.a)*along, b: start.b + (end.b-start.b)*along,
			})
		}
		if mixOpacity {
			opacity := float64(left.Opacity) + float64(right.Opacity-left.Opacity)*along
			shade.Opacity = min(max(int(math.Round(opacity)), MinOpacity), MaxOpacity)
		}
		shades = append(shades, shade)
	}

	return shades, nil
}

// oklab is a colour in the OKLab space: its lightness and two opponent axes.
type oklab struct {
	l, a, b float64
}

// toOKLab converts a colour, by Björn Ottosson's published matrices.
func toOKLab(value RGB) oklab {
	red := toLinear(uint8(value >> redShift & channelMask))
	green := toLinear(uint8(value >> greenShift & channelMask))
	blue := toLinear(uint8(value & channelMask))
	long := math.Cbrt(0.4122214708*red + 0.5363325363*green + 0.0514459929*blue)
	medium := math.Cbrt(0.2119034982*red + 0.6806995451*green + 0.1073969566*blue)
	short := math.Cbrt(0.0883024619*red + 0.2817188376*green + 0.6299787005*blue)

	return oklab{
		l: 0.2104542553*long + 0.7936177850*medium - 0.0040720468*short,
		a: 1.9779984951*long - 2.4285922050*medium + 0.4505937099*short,
		b: 0.0259040371*long + 0.7827717662*medium - 0.8086757660*short,
	}
}

// fromOKLab converts back, clipping a colour outside sRGB to its edge.
func fromOKLab(lab oklab) RGB {
	long := cube(lab.l + 0.3963377774*lab.a + 0.2158037573*lab.b)
	medium := cube(lab.l - 0.1055613458*lab.a - 0.0638541728*lab.b)
	short := cube(lab.l - 0.0894841775*lab.a - 1.2914855480*lab.b)
	red := fromLinear(4.0767416621*long - 3.3077115913*medium + 0.2309699292*short)
	green := fromLinear(-1.2684380046*long + 2.6097574011*medium - 0.3413193965*short)
	blue := fromLinear(-0.0041960863*long - 0.7034186147*medium + 1.7076147010*short)

	return RGB(red)<<redShift | RGB(green)<<greenShift | RGB(blue)
}

func cube(value float64) float64 { return value * value * value }

// toLinear undoes sRGB's gamma for one channel.
//
//nolint:mnd // The sRGB transfer function's constants.
func toLinear(channel uint8) float64 {
	value := float64(channel) / opaque
	if value <= 0.04045 {
		return value / 12.92
	}

	return math.Pow((value+0.055)/1.055, 2.4)
}

// fromLinear applies sRGB's gamma to one channel, rounded and clipped.
//
//nolint:mnd // The sRGB transfer function's constants.
func fromLinear(value float64) uint8 {
	value = min(max(value, 0), 1)
	if value <= 0.0031308 {
		value *= 12.92
	} else {
		value = 1.055*math.Pow(value, 1/2.4) - 0.055
	}

	return uint8(math.Round(value * opaque))
}
