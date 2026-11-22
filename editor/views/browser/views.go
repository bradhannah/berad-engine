package browser

import (
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The main pane's views (increment 32): Rows; Pictures for art; Cards for
// NPCs, zones and workflows; and Preview. A strip over the pane names the
// place shown and offers its views, which the place keeps, and Pictures'
// sizes.

// The views, as places keep them.
const (
	viewRows     = "rows"
	viewCards    = "cards"
	viewPictures = "pictures"
	viewPreview  = "preview"
)

// Sizes of the strip and its buttons, in pixels, and the buttons' IDs.
const (
	headHeight   = 36
	viewBnWidth  = 76
	sizeBnWidth  = 28
	viewBnPrefix = "view-"
	sizeBnPrefix = "size-"
)

// pictureSizes are Pictures' three sizes, small to large, in pixels.
func pictureSizes() []int { return []int{48, 72, 112} }

// viewsOf lists the views a place offers, its kind's default first, or
// none for a place that is not a kind's.
func viewsOf(place string) []string {
	switch kindOfPlace(place) {
	case sectionSprites, sectionPalettes, sectionTemplates:
		return []string{viewPictures, viewRows, viewPreview}
	case sectionNPCs, sectionZones:
		return []string{viewCards, viewRows, viewPreview}
	case sectionWorkflows:
		return []string{viewRows, viewCards, viewPreview}
	case sectionAudio:
		return []string{viewRows, viewPreview}
	}

	return nil
}

// viewLabel names a view on its button.
func viewLabel(view string) string {
	switch view {
	case viewCards:
		return "Cards"
	case viewPictures:
		return "Pictures"
	case viewPreview:
		return "Preview"
	}

	return "Rows"
}

// newViewButtons returns a button for each view and each size.
func newViewButtons() (map[string]*ui.Button, []*ui.Button) {
	views := map[string]*ui.Button{}
	for _, view := range []string{viewRows, viewCards, viewPictures, viewPreview} {
		views[view] = ui.NewButton(viewBnPrefix+view, viewLabel(view))
	}
	sizes := make([]*ui.Button, 0, len(pictureSizes()))
	for position, label := range []string{"S", "M", "L"} {
		sizes = append(sizes, ui.NewButton(sizeBnPrefix+string(rune('0'+position)), label))
	}

	return views, sizes
}

// viewShown returns the view the place shown is in: the one it keeps, or
// its kind's default.
func (s *State) viewShown() string {
	offered := viewsOf(s.place)
	if len(offered) == 0 {
		return viewRows
	}
	if chosen := s.choice().View; slices.Contains(offered, chosen) {
		return chosen
	}

	return offered[0]
}

// gridShown reports whether the main pane shows tiles rather than rows.
func (s *State) gridShown() bool {
	view := s.viewShown()

	return view == viewPictures || view == viewCards || view == viewPreview
}

// pictureSize returns the size of Pictures' pictures for the place shown.
func (s *State) pictureSize() int {
	sizes := pictureSizes()
	chosen := s.choice().Size - 1
	if chosen < 0 || chosen >= len(sizes) {
		chosen = 1
	}

	return sizes[chosen]
}

// layoutHead places the strip over the main pane: the place's path at the
// left, its views at the right, and Pictures' sizes before them. When they
// do not fit beside the path, the buttons go on a line below it (increment
// 61). It returns where the strip ends; a place without views has none.
func (s *State) layoutHead(top, right int) int {
	for _, button := range s.viewBns {
		button.Rect = ui.Zero()
	}
	for _, button := range s.sizeBns {
		button.Rect = ui.Zero()
	}
	s.headPath = ui.Zero()
	offered := viewsOf(s.place)
	if len(offered) == 0 || s.narrowed() {
		return top
	}
	shown := s.viewShown()
	var trail []int
	if shown == viewPictures {
		trail = append(trail, len(s.sizeBns)*(sizeBnWidth+ui.Gap)-ui.Gap+ui.Gap)
	}
	for range offered {
		trail = append(trail, viewBnWidth)
	}
	pad := (headHeight - ui.ButtonH) / ui.Half
	row := image.Rect(s.places.Rect.Max.X+ui.Inset, top+pad, right-ui.Inset, top+pad)
	leads, trails, bottom := ui.WrapRow(row, []int{s.theme.TextWidth(s.placePath())}, trail, ui.Gap)
	s.headPath = leads[0]
	if shown == viewPictures {
		sizes := trails[0]
		trails = trails[1:]
		for position, button := range s.sizeBns {
			button.On = pictureSizes()[position] == s.pictureSize()
			left := sizes.Min.X + position*(sizeBnWidth+ui.Gap)
			button.Rect = image.Rect(left, sizes.Min.Y, left+sizeBnWidth, sizes.Max.Y)
		}
	}
	for position, view := range offered {
		button := s.viewBns[view]
		button.On = view == shown
		button.Rect = trails[position]
	}
	s.joinViews(offered)

	return bottom + pad
}

// joinViews moves the views' buttons up against one another when they
// stand on one line, ending where the last one ends, so they are drawn as
// one control (increment 66). Wrapped onto lines of their own in a narrow
// window, they stay apart.
func (s *State) joinViews(offered []string) {
	for position := 1; position < len(offered); position++ {
		if s.viewBns[offered[position]].Rect.Min.Y != s.viewBns[offered[0]].Rect.Min.Y {
			return
		}
	}
	for position := len(offered) - ui.Half; position >= 0; position-- {
		button, next := s.viewBns[offered[position]], s.viewBns[offered[position+1]]
		button.Rect = image.Rect(next.Rect.Min.X-button.Rect.Dx(), button.Rect.Min.Y, next.Rect.Min.X, button.Rect.Max.Y)
	}
}

// viewsJoined lists the views' buttons shown, in order, and reports whether
// they stand against one another.
func (s *State) viewsJoined() ([]*ui.Button, bool) {
	offered := viewsOf(s.place)
	shown := make([]*ui.Button, 0, len(offered))
	joined := true
	for _, view := range offered {
		button := s.viewBns[view]
		if len(shown) > 0 && button.Rect.Min != image.Pt(shown[len(shown)-1].Rect.Max.X, shown[0].Rect.Min.Y) {
			joined = false
		}
		shown = append(shown, button)
	}

	return shown, joined && len(shown) > 1
}

// updateHead handles the strip: a view's button shows the place in that
// view, and a size's button sets Pictures' size; the place keeps both.
func (s *State) updateHead(input *ui.Input, ctx *frame.Context) {
	choice := s.choice()
	changed := false
	for view, button := range s.viewBns {
		if button.Update(input) {
			choice.View, changed = view, true
		}
	}
	for position, button := range s.sizeBns {
		if button.Update(input) {
			choice.Size, changed = position+1, true
		}
	}
	if changed {
		s.home.kept.Folders[s.place] = choice
		s.keepHome(ctx)
	}
}

// placePath names the place shown, as "Workflows › Market › Guards".
func (s *State) placePath() string {
	folder, isFolder := parseFolderRow(s.place)
	if !isFolder {
		return s.placeLabel()
	}
	label, _ := kindLabel(formatSection(folder.Format))

	return label + " › " + strings.ReplaceAll(folder.Path, project.FolderSeparator, " › ")
}

// drawHead draws the strip: the place's path and its buttons.
func (s *State) drawHead(screen *ebiten.Image) {
	if len(viewsOf(s.place)) == 0 || s.narrowed() {
		return
	}
	top := s.headPath.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	s.theme.DrawText(screen, s.theme.FitText(s.placePath(), s.headPath.Dx()), s.headPath.Min.X, top, ui.ColorInk())
	if shown, joined := s.viewsJoined(); joined {
		ui.DrawJoined(screen, s.theme, shown)
	} else {
		for _, button := range shown {
			button.Draw(screen, s.theme)
		}
	}
	for _, button := range s.sizeBns {
		button.Draw(screen, s.theme)
	}
}

// viewButtons lists the strip's buttons, for input scripts.
func (s *State) viewButtons() []*ui.Button {
	buttons := make([]*ui.Button, 0, len(s.viewBns)+len(s.sizeBns))
	for _, view := range []string{viewRows, viewCards, viewPictures, viewPreview} {
		buttons = append(buttons, s.viewBns[view])
	}

	return append(buttons, s.sizeBns...)
}
