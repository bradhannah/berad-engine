package window

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Quick open (increment 38): Cmd+P in any tab opens the project browser's
// search in a sheet over the tab in front. The browser's pane runs the
// sheet, wherever its own tab is: while it shows, the browser's pane takes
// the window's input, and the sheet is drawn over the tab in front. A window
// opened with Cmd+Enter is put beside the tab that was in front.

// Quick open's sheet: its most width and height, and how far under the bar
// it starts. besideTicks is how long a window asked for beside the tab in
// front is waited for.
const (
	quickWidth  = 680
	quickHeight = 460
	quickTop    = 40
	besideTicks = 600
)

// quickOpener is the project browser's quick open.
type quickOpener interface {
	OpenQuick()
	QuickOpen() bool
	SetQuick(rect image.Rectangle, room bool)
	DrawQuick(screen *ebiten.Image, docs frame.Docs)
	TakeBeside() (browser.Beside, bool)
}

// besideAsk is a window asked for beside a tab, waited for until ticks run
// out.
type besideAsk struct {
	view   string
	target ids.ULID
	tab    *tab
	ticks  int
}

// browserPane is the project browser's pane, and its quick open.
//
//nolint:ireturn // The browser's quick open is held as the interface, so it can be stood in for.
func (s *shell) browserPane() (*pane, quickOpener, bool) {
	for _, each := range s.tabs {
		for _, shown := range each.panes {
			if shown.host == nil || !shown.host.closeQuits {
				continue
			}
			if quick, ok := shown.host.view.(quickOpener); ok {
				return shown, quick, true
			}
		}
	}

	return nil, nil, false
}

// quickShowing reports whether quick open's sheet shows.
func (s *shell) quickShowing() bool {
	_, quick, ok := s.browserPane()

	return ok && quick.QuickOpen()
}

// openQuick opens quick open's sheet, for Cmd+P.
func (s *shell) openQuick(status func(string)) {
	_, quick, ok := s.browserPane()
	if !ok {
		status("Quick open needs the Project's tab, which is not open")

		return
	}
	quick.OpenQuick()
}

// quickRect is where the sheet shows: across the middle of the tab in front,
// near its top.
func (s *shell) quickRect() image.Rectangle {
	area := s.area()
	width := min(quickWidth, area.Dx()-ui.Half*ui.Inset)
	height := min(quickHeight, area.Dy()-ui.Half*quickTop)
	left := area.Min.X + (area.Dx()-width)/ui.Half

	return image.Rect(left, area.Min.Y+quickTop, left+width, area.Min.Y+quickTop+height)
}

// placeQuick tells the browser where its sheet shows, in its pane's
// coordinates, and whether the tab in front has room for a window beside it.
func (s *shell) placeQuick() {
	shown, quick, ok := s.browserPane()
	if !ok {
		return
	}
	front := s.front()
	quick.SetQuick(s.quickRect().Sub(shown.rect.Min), front != nil && len(front.panes) < splitPanes)
}

// takeBeside waits for a window the sheet asked for beside the tab in front,
// and stops waiting once its time runs out.
func (s *shell) takeBeside() {
	if s.beside != nil {
		s.beside.ticks--
		if s.beside.ticks <= 0 {
			s.beside = nil
		}
	}
	_, quick, ok := s.browserPane()
	if !ok {
		return
	}
	if asked, found := quick.TakeBeside(); found && s.front() != nil {
		s.beside = &besideAsk{view: asked.View, target: asked.Target, tab: s.front(), ticks: besideTicks}
	}
}

// besideFor reports whether a pane is the window asked for beside a tab, and
// that tab still has room for it.
func (s *shell) besideFor(shown *pane) (*tab, bool) {
	asked := s.beside
	if asked == nil || shown.view != asked.view || shown.target != asked.target ||
		!slices.Contains(s.tabs, asked.tab) || len(asked.tab.panes) >= splitPanes {
		return nil, false
	}

	return asked.tab, true
}

// putBeside puts a pane beside the tab it was asked for, taking it out of
// the tab it was in, and brings that tab forward with the pane focused.
func (s *shell) putBeside(shown *pane, beside *tab) {
	s.beside = nil
	if slices.Contains(beside.panes, shown) {
		return
	}
	for index, each := range s.tabs {
		if slot := slices.Index(each.panes, shown); slot >= 0 {
			if len(each.panes) == 1 {
				s.tabs = slices.Delete(s.tabs, index, index+1)
			} else {
				each.panes = slices.Delete(each.panes, slot, slot+1)
				each.focus = 0
			}

			break
		}
	}
	beside.panes = append(beside.panes, shown)
	beside.focus = len(beside.panes) - 1
	s.active = slices.Index(s.tabs, beside)
}

// drawQuick draws the sheet over the tab in front while it shows: the
// browser draws it into an image as large as its pane, and the sheet's part
// of that is drawn over the window.
func (s *shell) drawQuick(screen *ebiten.Image) {
	shown, quick, ok := s.browserPane()
	if !ok || !quick.QuickOpen() {
		return
	}
	size := shown.rect.Size()
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	if s.quickImage == nil || s.quickImage.Bounds().Size() != size {
		if s.quickImage != nil {
			s.quickImage.Deallocate()
		}
		s.quickImage = ebiten.NewImage(size.X, size.Y)
	}
	s.quickImage.Clear()
	if err := safely(func() error {
		quick.DrawQuick(s.quickImage, shown.host.conn)

		return nil
	}); err != nil {
		logLine("draw quick open: " + err.Error())

		return
	}
	sheet := s.quickRect().Sub(shown.rect.Min)
	sheet.Max.Y += ui.FloatDrop
	part, isImage := s.quickImage.SubImage(sheet).(*ebiten.Image)
	if !isImage {
		return
	}
	var options ebiten.DrawImageOptions
	at := shown.rect.Min.Add(sheet.Min)
	options.GeoM.Translate(float64(at.X), float64(at.Y))
	screen.DrawImage(part, &options)
}
