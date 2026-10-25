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
	s.drawButtons(screen, s.addSound, s.newCue, s.stopAll)
	s.drawButtons(screen, s.tabs...)
	s.drawButtons(screen, s.filters...)
	s.search.Draw(screen)
	s.list.Draw(screen, s.theme)
	if s.library != nil && s.library.Problem != "" {
		s.theme.DrawText(screen, "No sounds yet.", ui.Inset, s.list.Rect.Min.Y+ui.Inset, ui.ColorMuted())
	}
	s.drawPanel(screen)
	s.drawInspector(screen)
	switch {
	case s.dialog != nil:
		s.drawDialog(screen)
	case s.confirm != nil:
		s.confirm.Draw(screen, s.theme)
	}
	s.guard.Draw(screen, s.theme)
}

// Locate finds a named part of the window for input scripts: "ui:<id>" for
// a button, field or checkbox, in a dialog while one shows; "row:<label>"
// for a list row by its label; "cue:<id>", "rec:<id>" and "plays:<id>"
// for a cue, a recording and one of a cue's recordings; "section:<id>" for a
// section's heading; or "state:selected=<row>", "state:listed",
// "state:dialog" and "state:cues=<n>" as readiness flags.
func (s *State) Locate(name string, _ frame.Docs) (image.Point, bool) {
	s.layout()
	kind, value, _ := strings.Cut(name, ":")
	switch {
	case s.guard.Active():
		return s.guard.Locate(value, s.size)
	case kind == "state":
		return image.Pt(0, 0), s.state(value)
	case s.dialog != nil:
		s.dialog.layout(image.Rectangle{Min: image.Pt(0, 0), Max: s.size})

		return s.dialog.locate(value)
	case s.confirm != nil:
		return s.confirm.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
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
	case "ui":
		return s.locateWidget(value)
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
	}

	return false
}

// locateWidget finds a button or field outside dialogs.
func (s *State) locateWidget(widget string) (image.Point, bool) {
	for _, field := range append([]*ui.TextField{s.search}, s.inspector.fields()...) {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	buttons := append([]*ui.Button{s.addSound, s.newCue, s.stopAll}, s.tabs...)
	buttons = append(buttons, s.filters...)
	buttons = append(buttons, s.panel.all()...)
	buttons = append(buttons, s.inspector.buttons()...)
	for _, button := range buttons {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}
