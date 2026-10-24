package ui

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

// Save bar geometry, in screen pixels. A bar narrower than wideBar has two
// rows: the message above, Revert and Save below.
const (
	wideBar     = 420
	barPad      = 6
	saveWidth   = 64
	revertWidth = 72
	saveDot     = 4
)

// Save bar button IDs, for input scripts.
const (
	SaveID   = "save"
	RevertID = "revert"
)

// SaveBarHeight returns the height of a save bar of a width.
func SaveBarHeight(width int) int {
	if width >= wideBar {
		return ButtonH + Half*barPad
	}

	return LineHeight + ButtonH + third*barPad
}

// SaveBar sits at the foot of an inspector, or of the Text window, and saves
// or reverts its typed changes. It names how many are unsaved, and its
// buttons are greyed out when there is nothing to save.
type SaveBar struct {
	Rect    image.Rectangle
	Unsaved int
	save    *Button
	revert  *Button
}

// NewSaveBar returns a save bar, placed later by setting Rect.
func NewSaveBar() *SaveBar {
	return &SaveBar{Rect: Zero(), Unsaved: 0, save: NewButton(SaveID, "Save"), revert: NewButton(RevertID, "Revert")}
}

// wide reports whether the bar fits on one row.
func (b *SaveBar) wide() bool { return b.Rect.Dx() >= wideBar }

// layout places the buttons: at the right of a wide bar, or sharing the
// lower row of a narrow one.
func (b *SaveBar) layout() {
	b.save.Disabled, b.revert.Disabled = b.Unsaved == 0, b.Unsaved == 0
	b.save.Primary = b.Unsaved > 0
	if b.Rect.Empty() {
		b.save.Rect, b.revert.Rect = Zero(), Zero()

		return
	}
	bottom := b.Rect.Max.Y - barPad
	right := b.Rect.Max.X - barPad
	if b.wide() {
		b.save.Rect = image.Rect(right-saveWidth, bottom-ButtonH, right, bottom)
		right -= saveWidth + Gap
		b.revert.Rect = image.Rect(right-revertWidth, bottom-ButtonH, right, bottom)

		return
	}
	middle := b.Rect.Min.X + b.Rect.Dx()/Half
	b.revert.Rect = image.Rect(b.Rect.Min.X+barPad, bottom-ButtonH, middle-Gap/Half, bottom)
	b.save.Rect = image.Rect(middle+Gap/Half, bottom-ButtonH, right, bottom)
}

// Update returns SaveID or RevertID when one is clicked, or "".
func (b *SaveBar) Update(input *Input) string {
	b.layout()
	switch {
	case b.save.Update(input):
		return SaveID
	case b.revert.Update(input):
		return RevertID
	default:
		return ""
	}
}

// Locate returns the centre of a button, for input scripts.
func (b *SaveBar) Locate(id string) (image.Point, bool) {
	b.layout()
	for _, button := range []*Button{b.save, b.revert} {
		if button.ID == id && !button.Rect.Empty() {
			return Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// Message is what the bar says: how many changes are unsaved, or that all
// are saved.
func (b *SaveBar) Message() string {
	switch b.Unsaved {
	case 0:
		return "All changes saved"
	case 1:
		return "1 unsaved change"
	default:
		return strconv.Itoa(b.Unsaved) + " unsaved changes"
	}
}

// Draw renders the bar: a quiet panel, or a red outline and dot when
// something is unsaved; the message with the Cmd+S hint; and the buttons.
func (b *SaveBar) Draw(screen *ebiten.Image, theme *Theme) {
	if b.Rect.Empty() {
		return
	}
	b.layout()
	fill, edge, ink := ColorPanel(), ColorRuleSoft(), ColorSubdued()
	if b.Unsaved > 0 {
		fill, edge, ink = ColorBackground(), ColorWarn(), ColorInk()
	}
	FillRound(screen, b.Rect, Radius, fill)
	StrokeRound(screen, b.Rect, Radius, Border, edge)
	row := image.Rect(b.Rect.Min.X+barPad, b.Rect.Min.Y+barPad, b.Rect.Max.X-barPad, b.Rect.Min.Y+barPad+LineHeight)
	if b.wide() {
		row = image.Rect(b.Rect.Min.X+barPad, b.Rect.Min.Y, b.revert.Rect.Min.X-Gap, b.Rect.Max.Y)
	}
	left, textTop := row.Min.X+Gap, row.Min.Y+(row.Dy()-TextSize)/Half-1
	if b.Unsaved > 0 {
		centre := image.Pt(left+saveDot, row.Min.Y+row.Dy()/Half)
		FillRound(screen, image.Rect(centre.X-saveDot, centre.Y-saveDot, centre.X+saveDot, centre.Y+saveDot), saveDot,
			ColorWarn())
		left += saveDot*Half + Gap*Half
		hint := "Cmd+S"
		theme.DrawText(screen, hint, row.Max.X-Gap-theme.TextWidth(hint), textTop, ColorSubdued())
	}
	theme.DrawText(screen, b.Message(), left, textTop, ink)
	b.revert.Draw(screen, theme)
	b.save.Draw(screen, theme)
}
