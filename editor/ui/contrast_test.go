package ui_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// minContrast is the least contrast text may have against its ground.
const minContrast = 4.5

// named is a colour with the name a failure gives it.
type named struct {
	name  string
	color color.RGBA
}

// TestTextReadsOnEveryGround measures every text colour against every ground
// it is drawn on, and fails any pairing under 4.5 to 1.
func TestTextReadsOnEveryGround(t *testing.T) {
	t.Parallel()
	light := []named{
		{"field", ui.ColorField()}, {"page", ui.ColorPage()}, {"desk", ui.ColorDesk()},
		{"hover", ui.ColorHover()}, {"selection wash", ui.ColorSelectWash()}, {"live wash", ui.ColorLiveWash()},
		{"group", ui.ColorGroup()},
	}
	text := []named{
		{"ink", ui.ColorInk()}, {"muted", ui.ColorMuted()}, {"subdued", ui.ColorSubdued()},
		{"select", ui.ColorSelect()}, {"live", ui.ColorLive()}, {"warn", ui.ColorWarn()},
		{"caution", ui.ColorCaution()}, {"constant", ui.ColorConstant()}, {"value", ui.ColorValue()},
	}
	for _, ground := range light {
		for _, ink := range text {
			check(t, ink, ground)
		}
	}
	for _, tint := range ui.Tints() {
		check(t, named{"ink", ui.ColorInk()}, named{tint.Name + " wash", tint.Wash})
		check(t, named{"ink", ui.ColorInk()}, named{tint.Name + " strong wash", tint.Strong})
		check(t, named{"reverse", ui.ColorReverse()}, named{tint.Name + " ink", tint.Ink})
	}
	filled := []named{
		{"select", ui.ColorSelect()}, {"ink", ui.ColorInk()}, {"warn", ui.ColorWarn()}, {"live", ui.ColorLive()},
		{"zone", ui.ColorZone()}, {"workflow", ui.ColorWorkflow()}, {"NPC", ui.ColorNPC()},
		{"sound", ui.ColorSound()}, {"art", ui.ColorArt()},
	}
	for _, ground := range filled {
		check(t, named{"reverse", ui.ColorReverse()}, ground)
	}
	check(t, named{"constant", ui.ColorConstant()}, named{"constant wash", ui.ColorConstantWash()})
	check(t, named{"value", ui.ColorValue()}, named{"value wash", ui.ColorValueWash()})
	check(t, named{"warn", ui.ColorWarn()}, named{"warn wash", ui.ColorWarnWash()})
	check(t, named{"stage ink", ui.ColorStageInk()}, named{"stage", ui.ColorStage()})
	check(t, named{"stage muted", ui.ColorStageMuted()}, named{"stage", ui.ColorStage()})
	check(t, named{"stage muted", ui.ColorStageMuted()}, named{"ink", ui.ColorInk()})
}

// TestTonesStepApart checks that the four tones are told apart by lightness,
// not by hue alone: each is lighter than the next by a clear step.
func TestTonesStepApart(t *testing.T) {
	t.Parallel()
	tones := []named{{"field", ui.ColorField()}, {"page", ui.ColorPage()}, {"desk", ui.ColorDesk()}, {"stage", ui.ColorStage()}}
	const minStep = 1.05
	for index := 1; index < len(tones); index++ {
		lighter, darker := tones[index-1], tones[index]
		if ratio := contrast(lighter.color, darker.color); ratio < minStep {
			t.Errorf("%s and %s are %.2f to 1 apart, want at least %.2f", lighter.name, darker.name, ratio, minStep)
		}
	}
}

func check(t *testing.T, ink, ground named) {
	t.Helper()
	if ratio := contrast(ink.color, ground.color); ratio < minContrast {
		t.Errorf("%s on %s is %.2f to 1, want at least %.1f", ink.name, ground.name, ratio, minContrast)
	}
}

// contrast is the WCAG contrast ratio of two opaque colours.
func contrast(first, second color.RGBA) float64 {
	lighter, darker := luminance(first), luminance(second)
	if lighter < darker {
		lighter, darker = darker, lighter
	}

	return (lighter + 0.05) / (darker + 0.05)
}

// luminance is a colour's relative luminance, by the WCAG formula.
func luminance(shade color.RGBA) float64 {
	channel := func(value uint8) float64 {
		linear := float64(value) / 255
		if linear <= 0.03928 {
			return linear / 12.92
		}

		return math.Pow((linear+0.055)/1.055, 2.4)
	}

	return 0.2126*channel(shade.R) + 0.7152*channel(shade.G) + 0.0722*channel(shade.B)
}
