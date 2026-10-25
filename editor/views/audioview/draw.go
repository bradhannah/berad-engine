package audioview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

func itoa(value int) string { return strconv.Itoa(value) }

// summary says which library the window shows.
func (s *State) summary() string {
	switch {
	case s.library == nil:
		return "listing…"
	case s.library.Problem != "":
		return "no library yet"
	}

	return "audio library · schema " + itoa(s.library.Schema)
}

// Draw renders the window: the toolbar, the list, the panel, the inspector
// and anything showing over them.
func (s *State) Draw(screen *ebiten.Image, _ frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout()
	screen.Fill(ui.ColorBackground())
	content := frame.Content(s.size)
	s.theme.DrawTitle(screen, "Audio", ui.Inset, (toolbarHeight-ui.TitleSize)/ui.Half-1, ui.ColorInk())
	s.theme.DrawMono(screen, s.summary(), ui.Inset+s.theme.TextWidth("Audio")+ui.Inset*ui.Half,
		(toolbarHeight-ui.MonoSize)/ui.Half, ui.ColorSubdued())
	ui.Divider(screen, image.Pt(0, toolbarHeight-ui.Outline), image.Pt(content.Max.X, toolbarHeight-ui.Outline))
	ui.Divider(screen, image.Pt(listWidth, toolbarHeight), image.Pt(listWidth, content.Max.Y))
	s.drawButtons(screen, s.addSound, s.newCue, s.checkFiles, s.soundscape.toggle, s.stopAll)
	s.drawButtons(screen, s.tabs...)
	s.drawButtons(screen, s.filters...)
	s.search.Draw(screen)
	s.list.Draw(screen, s.theme)
	if s.library != nil && s.library.Problem != "" {
		s.theme.DrawText(screen, "No sounds yet.", ui.Inset, s.list.Rect.Min.Y+ui.Inset, ui.ColorMuted())
	}
	s.drawPanel(screen)
	if s.soundscape.shown {
		s.drawSoundscape(screen)
	} else {
		s.drawInspector(screen)
	}
	switch {
	case s.dialog != nil:
		s.drawDialog(screen)
	case s.confirm != nil:
		s.confirm.Draw(screen, s.theme)
	case s.prompt != nil:
		s.prompt.draw(screen, s.theme)
	case s.inspector.classes.menu.IsOpen():
		s.inspector.classes.menu.Draw(screen, s.theme)
	}
	s.guard.Draw(screen, s.theme)
}

// Locate finds a named part of the window for input scripts: "ui:<id>" for
// a button, field or checkbox, in a dialog while one shows; "row:<label>"
// for a list row by its label; "cue:<id>", "rec:<id>" and "plays:<id>"
// for a cue, a recording and one of a cue's recordings; "section:<id>" for a
// section's heading; "wave:start" and "wave:end" for the region's ends; or
// "state:selected=<row>", "state:listed", "state:dialog", "state:cues=<n>",
// "state:waveform", "state:region=<start>-<end>" and "state:stop=<button>"
// as readiness flags.
func (s *State) Locate(name string, _ frame.Docs) (image.Point, bool) {
	s.layout()
	kind, value, _ := strings.Cut(name, ":")
	switch {
	case s.guard.Active():
		return s.guard.Locate(value, s.size)
	case kind == "state":
		return image.Pt(0, 0), s.state(value)
	case s.dialogShows():
		return s.locateInDialog(value)
	case s.shownMenu() != nil:
		return s.shownMenu().Locate(value)
	}
	switch kind {
	case "row":
		for _, row := range s.list.Rows {
			if row.Label == value && !row.Heading {
				return s.list.Locate(row.ID)
			}
		}
	case "cue", "rec":
		return s.list.Locate(name)
	case "plays":
		return s.played.Locate(name)
	case "section":
		return s.list.Locate(sectionPrefix + value)
	case "add":
		return s.list.LocateAdd(categorySectionOf + value)
	case "wave":
		return s.trim.wave.Locate(value)
	case "ui":
		return s.locateWidget(value)
	}

	return image.Pt(0, 0), false
}

// shownMenu is the menu open in the window, or nil.
func (s *State) shownMenu() *ui.Menu {
	for _, menu := range []*ui.Menu{&s.inspector.classes.menu, &s.soundscape.replace} {
		if menu.IsOpen() {
			return menu
		}
	}

	return nil
}

// dialogShows reports whether a dialog takes the window's input.
func (s *State) dialogShows() bool {
	return s.dialog != nil || s.confirm != nil || s.prompt != nil
}

// locateInDialog finds a button or field in the dialog that shows.
func (s *State) locateInDialog(value string) (image.Point, bool) {
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	switch {
	case s.dialog != nil:
		s.dialog.layout(window)

		return s.dialog.locate(value)
	case s.confirm != nil:
		return s.confirm.Locate(value, window)
	case s.prompt != nil:
		s.prompt.layout(window)

		return s.prompt.locate(value)
	}

	return image.Pt(0, 0), false
}

// state answers a readiness flag.
func (s *State) state(value string) bool {
	flag, argument, _ := strings.Cut(value, "=")
	switch flag {
	case "selected":
		return s.list.Selected == argument
	case "listed":
		return s.library != nil && !s.asking
	case "dialog":
		return s.dialog != nil
	case "cues":
		return itoa(s.count(modeCues)) == argument
	case "waveform":
		return s.trim.analysis != nil && s.trim.analysis.Problem == ""
	case "layer":
		slot, cue, _ := strings.Cut(argument, ":")
		found := s.soundscape.slot(slot)

		return found != nil && found.cue == cue
	case "stop":
		return s.trim.auditioned != nil && s.trim.auditioned.ID == argument && s.trim.auditioned.Label == labelStop
	case "region":
		start, end := s.trim.region()

		return seconds(start)+"-"+seconds(end) == argument
	}

	return false
}

// locateWidget finds a button or field outside dialogs.
func (s *State) locateWidget(widget string) (image.Point, bool) {
	for _, field := range append([]*ui.TextField{s.search, s.trim.start, s.trim.end}, s.inspector.fields()...) {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	buttons := append([]*ui.Button{s.addSound, s.newCue, s.checkFiles, s.soundscape.toggle, s.stopAll}, s.tabs...)
	buttons = append(buttons, s.soundscape.buttons()...)
	buttons = append(buttons, s.trim.buttons()...)
	buttons = append(buttons, s.filters...)
	buttons = append(buttons, s.panel.all()...)
	buttons = append(buttons, s.inspector.buttons()...)
	buttons = append(buttons, s.attribution.edit)
	for _, button := range buttons {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	for _, toggle := range append([]*ui.Toggle{s.trim.loop, s.attribution.cleared}, s.soundscape.toggles()...) {
		if widget == toggle.ID && !toggle.Rect.Empty() {
			return image.Pt(toggle.Rect.Min.X+ui.Inset/ui.Half, ui.Center(toggle.Rect).Y), true
		}
	}

	return image.Pt(0, 0), false
}
