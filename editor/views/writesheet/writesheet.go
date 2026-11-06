// Package writesheet is the sheet long text is written in, over a window:
// the text in a field of many lines; the bold, italic and colour buttons,
// which put markup's tags around what is selected; and beside it a preview
// drawn as the game draws a message box. A constant in a template has a
// list switching between its default and each NPC's or zone's own value,
// to write them in turn. Done, Escape or switching saves what was typed.
package writesheet

import (
	"image"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/showdraw"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/show"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Layout, in screen pixels: the sheet's largest size, its toolbar's
// buttons, and the lines of notes under the text.
const (
	sheetWidth   = 1000
	sheetHeight  = 600
	toolWidth    = 40
	colourWidth  = 110
	holderWidth  = 220
	doneWidth    = 100
	noteLines    = 4
	previewHead  = 18
	holderMenuID = "sheet-holder-"
	colourMenuID = "sheet-colour-"
)

// Locate names for input scripts: the text, "ui:sheet-text"; the buttons,
// "ui:sheet-bold", "ui:sheet-italic", "ui:sheet-colour" and "ui:sheet-done";
// the holder list, "ui:sheet-holder"; and in an open menu, "menu:<id>".
const (
	textID   = "sheet-text"
	boldID   = "sheet-bold"
	italicID = "sheet-italic"
	colourID = "sheet-colour"
	holderID = "sheet-holder"
	doneID   = "sheet-done"
)

// Holder is one value the sheet writes: a constant's default, or one NPC's
// or zone's own value of it. Key is the window's own name for it; Text is
// as saved, or the default for one with no value of its own yet.
type Holder struct {
	Key   string
	Label string
	Text  string
}

// Saving is what the window saves: whose value, and its text.
type Saving struct {
	Key  string
	Text string
}

// Sheet is an open writing sheet.
type Sheet struct {
	theme   *ui.Theme
	path    string
	holders []Holder
	chosen  int
	said    string
	field   *ui.TextField
	focus   ui.Focus
	bold    *ui.Button
	italic  *ui.Button
	colour  *ui.Button
	holder  *ui.Button
	done    *ui.Button
	menu    ui.Menu
	menuOf  string
	rect    image.Rectangle
	preview image.Rectangle
	notesAt image.Point
}

// New opens a sheet for a constant by its path: who holds a value of it, the
// one written first, and where it is said, such as "Said in Serve a
// customer".
func New(theme *ui.Theme, path string, holders []Holder, chosen int, said string) *Sheet {
	sheet := &Sheet{
		theme: theme, path: path, holders: holders, chosen: min(max(chosen, 0), max(len(holders)-1, 0)), said: said,
		field: ui.NewTextField(textID, textedit.NewField("", true, worldstate.MaxConstantText), true, theme),
		focus: ui.Focus{}, bold: ui.NewButton(boldID, "B"), italic: ui.NewButton(italicID, "I"),
		colour: ui.NewButton(colourID, "Colour ▾"), holder: ui.NewButton(holderID, ""),
		done: ui.NewButton(doneID, "Done"), menu: ui.Menu{Items: nil, Width: 0}, menuOf: "", rect: ui.Zero(),
		preview: ui.Zero(), notesAt: image.Pt(0, 0),
	}
	sheet.done.Primary = true
	sheet.focus.Register(sheet.field)
	sheet.show()
	sheet.focus.Set(sheet.field)
	sheet.field.Field.Focus()

	return sheet
}

// show puts the chosen holder's text in the field.
func (s *Sheet) show() {
	s.field.Field.Revert()
	if len(s.holders) > 0 {
		s.field.Field.SetSaved(s.holders[s.chosen].Text)
		s.holder.Label = s.holders[s.chosen].Label + " ▾"
	}
}

// Chosen returns the key of the holder being written.
func (s *Sheet) Chosen() string {
	if len(s.holders) == 0 {
		return ""
	}

	return s.holders[s.chosen].Key
}

// SetSaved updates a holder's saved text, as its document changes.
func (s *Sheet) SetSaved(key, text string) {
	for index := range s.holders {
		if s.holders[index].Key != key {
			continue
		}
		s.holders[index].Text = text
		if index == s.chosen {
			s.field.Field.SetSaved(text)
		}
	}
}

// Focused returns the sheet's text field, which takes what is typed.
func (s *Sheet) Focused() *ui.TextField { return s.focus.Current() }

// layout places the sheet in the middle of a window.
func (s *Sheet) layout(window image.Rectangle) {
	width, height := min(sheetWidth, window.Dx()-ui.Half*ui.Inset), min(sheetHeight, window.Dy()-ui.Half*ui.Inset)
	left, top := window.Min.X+(window.Dx()-width)/ui.Half, window.Min.Y+(window.Dy()-height)/ui.Half
	s.rect = image.Rect(left, top, left+width, top+height)
	inner := s.rect.Inset(ui.Inset)
	titleBottom := inner.Min.Y + ui.ButtonH
	s.holder.Rect = ui.Zero()
	if len(s.holders) > 1 {
		s.holder.Rect = image.Rect(inner.Max.X-holderWidth, inner.Min.Y, inner.Max.X, titleBottom)
	}
	toolsTop := titleBottom + ui.Gap
	half := (inner.Dx() - ui.Inset) / ui.Half
	s.bold.Rect = image.Rect(inner.Min.X, toolsTop, inner.Min.X+toolWidth, toolsTop+ui.ButtonH)
	s.italic.Rect = s.bold.Rect.Add(image.Pt(toolWidth+ui.Gap, 0))
	s.colour.Rect = image.Rect(s.italic.Rect.Max.X+ui.Gap, toolsTop, s.italic.Rect.Max.X+ui.Gap+colourWidth,
		toolsTop+ui.ButtonH)
	bodyTop := toolsTop + ui.ButtonH + ui.Gap
	bodyBottom := inner.Max.Y - ui.ButtonH - ui.Gap - noteLines*ui.LineHeight
	s.field.Rect = image.Rect(inner.Min.X, bodyTop, inner.Min.X+half, bodyBottom)
	s.preview = image.Rect(inner.Max.X-half, bodyTop+previewHead, inner.Max.X, bodyBottom)
	s.notesAt = image.Pt(inner.Min.X, bodyBottom+ui.Gap)
	s.done.Rect = image.Rect(inner.Max.X-doneWidth, inner.Max.Y-ui.ButtonH, inner.Max.X, inner.Max.Y)
}

// Update handles a frame over a window: typing, the markup buttons, the
// holder list and Done. It returns a value to save, if there is one, and
// whether the sheet stays open.
func (s *Sheet) Update(input *ui.Input, window image.Rectangle, clipboard textedit.Clipboard) (*Saving, bool) {
	s.layout(window)
	if s.menu.IsOpen() {
		return s.chooseFromMenu(s.menu.Update(input))
	}
	if input.Consume(ui.KeyEscape, ui.Plain()) || s.done.Update(input) {
		return s.saving(), false
	}
	switch {
	case s.bold.Update(input):
		s.wrap("[b]", "[/b]")
	case s.italic.Update(input):
		s.wrap("[i]", "[/i]")
	case s.colour.Update(input):
		s.openMenu(colourID, colourItems(), s.colour.Rect, window)
	case s.holder.Update(input):
		s.openMenu(holderID, s.holderItems(), s.holder.Rect, window)
	}
	s.focus.Update(input, clipboard)
	if s.focus.Current() == nil {
		s.focus.Set(s.field)
	}

	return nil, true
}

// openMenu opens the colour list or the holder list below its button.
func (s *Sheet) openMenu(of string, items []ui.MenuItem, below, window image.Rectangle) {
	s.menu.Items, s.menuOf = items, of
	s.menu.Open(below, window)
}

// colourItems lists the colours markup names.
func colourItems() []ui.MenuItem {
	items := make([]ui.MenuItem, 0, len(show.Colours()))
	for _, name := range show.Colours() {
		items = append(items, ui.MenuItem{
			ID: colourMenuID + name, Label: name, Icon: ui.IconNone, Disabled: false, Danger: false, Rule: false,
		})
	}

	return items
}

// holderItems lists the default and each NPC's or zone's own value.
func (s *Sheet) holderItems() []ui.MenuItem {
	items := make([]ui.MenuItem, 0, len(s.holders))
	for index, holder := range s.holders {
		items = append(items, ui.MenuItem{
			ID: holderMenuID + strconv.Itoa(index), Label: holder.Label, Icon: ui.IconNone, Disabled: index == s.chosen,
			Danger: false, Rule: false,
		})
	}

	return items
}

// chooseFromMenu wraps the selection in a colour, or saves what was typed
// and switches to another holder.
func (s *Sheet) chooseFromMenu(chosen string) (*Saving, bool) {
	if colour, isColour := strings.CutPrefix(chosen, colourMenuID); isColour {
		s.wrap("[color="+colour+"]", "[/color]")

		return nil, true
	}
	index, err := strconv.Atoi(strings.TrimPrefix(chosen, holderMenuID))
	if !strings.HasPrefix(chosen, holderMenuID) || err != nil || index < 0 || index >= len(s.holders) {
		return nil, true
	}
	saving := s.saving()
	s.chosen = index
	s.show()
	s.field.Field.Focus()

	return saving, true
}

// wrap puts tags around what is selected, keeping it selected, or at the
// caret with the caret between them.
func (s *Sheet) wrap(open, closing string) {
	field := s.field.Field
	field.Focus()
	buffer := field.Buffer()
	start, _ := buffer.Selection()
	selected := buffer.SelectedText()
	field.Type(open + selected + closing)
	if buffer = field.Buffer(); buffer != nil {
		buffer.Select(start+len(open), start+len(open)+len(selected))
	}
	s.focus.Set(s.field)
}

// saving returns what was typed to save, if anything was.
func (s *Sheet) saving() *Saving {
	if len(s.holders) == 0 || !s.field.Field.Dirty() {
		return nil
	}
	text, changed := s.field.Field.Commit()
	if !changed {
		return nil
	}
	s.holders[s.chosen].Text = text

	return &Saving{Key: s.holders[s.chosen].Key, Text: text}
}

// Text returns what the field holds now, typed or saved.
func (s *Sheet) Text() string { return s.field.Field.Value() }

// Notes says what is worth knowing about the text: how long it is, how long
// it takes to read and where it is said, then anything wrong with its
// markup, and when it is too long for one box or to read in the time a box
// shows.
func Notes(text, said string, lines int) ([]string, []string) {
	count := utf8.RuneCountInString(text)
	seconds := show.ReadingTicks(text) / clock.TicksPerSecond
	about := thousands(count) + " of " + thousands(worldstate.MaxConstantText) + " · line breaks kept · " +
		"reads in about " + strconv.Itoa(seconds) + " seconds"
	if said != "" {
		about += " · " + said
	}
	warnings := show.Problems(text)
	if show.TooLongToRead(text) {
		warnings = append(warnings, "Longer than a line or a box shows for, ten seconds: one that goes when read "+
			"would go before it was read. A Show message that goes when the player closes it suits it.")
	}
	if lines > showdraw.BoxLines {
		warnings = append(warnings, "Longer than one message box holds. Paging is not built yet.")
	}

	return []string{about}, warnings
}

// Draw draws the sheet over a dimmed window.
func (s *Sheet) Draw(screen *ebiten.Image, window image.Rectangle) {
	s.layout(window)
	ui.FillRect(screen, window, ui.ColorShade())
	ui.FillRound(screen, s.rect, ui.Radius, ui.ColorBackground())
	ui.StrokeRound(screen, s.rect, ui.Radius, ui.Border, ui.ColorRule())
	inner := s.rect.Inset(ui.Inset)
	s.theme.DrawTitle(screen, "Write", inner.Min.X, inner.Min.Y, ui.ColorInk())
	pathLeft := inner.Min.X + s.theme.StrongWidth("Write") + ui.Inset
	s.theme.DrawMono(screen, s.theme.FitText(s.path, holderWidth+ui.Inset), pathLeft, inner.Min.Y+ui.Half,
		ui.ColorMuted())
	for _, button := range []*ui.Button{s.bold, s.italic, s.colour, s.holder, s.done} {
		button.Draw(screen, s.theme)
	}
	s.field.Draw(screen)
	s.theme.Heading(screen, "IN THE GAME", s.preview.Min.X, s.preview.Min.Y-previewHead)
	ui.FillRect(screen, s.preview, ui.ColorCanvas())
	box := &protocol.PreviewBox{Text: s.Text(), Name: "", Left: 1, Queued: 0, Closing: true}
	showdraw.DrawBox(screen, s.theme, s.preview, box)
	_, lines := showdraw.BoxRect(s.theme, s.preview, box)
	about, warnings := Notes(s.Text(), s.said, len(lines))
	top := s.notesAt.Y
	for _, line := range about {
		s.theme.DrawText(screen, s.theme.FitText(line, s.rect.Dx()-ui.Half*ui.Inset), s.notesAt.X, top, ui.ColorMuted())
		top += ui.LineHeight
	}
	for _, warning := range warnings {
		s.theme.DrawText(screen, s.theme.FitText("⚠ "+warning, s.done.Rect.Min.X-s.notesAt.X-ui.Inset), s.notesAt.X,
			top, ui.ColorWarn())
		top += ui.LineHeight
	}
	s.menu.Draw(screen, s.theme)
}

// Locate finds a widget for input scripts, "ui:<id>", or an item of an
// open menu, "menu:<id>".
func (s *Sheet) Locate(name string) (image.Point, bool) {
	kind, widget, _ := strings.Cut(name, ":")
	if kind == "menu" {
		return s.menu.Locate(widget)
	}
	if widget == textID && !s.field.Rect.Empty() {
		return s.field.Center(), true
	}
	for _, button := range []*ui.Button{s.bold, s.italic, s.colour, s.holder, s.done} {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// HasState answers "sheet-holder=<label>", the holder being written, and
// "sheet-text=<text>", what the field holds.
func (s *Sheet) HasState(state string) (bool, bool) {
	if label, asked := strings.CutPrefix(state, "sheet-holder="); asked {
		return len(s.holders) > 0 && s.holders[s.chosen].Label == label, true
	}
	if text, asked := strings.CutPrefix(state, "sheet-text="); asked {
		return s.Text() == text, true
	}

	return false, false
}

// thousands writes a count with a comma between each three digits, as
// 4,000.
func thousands(count int) string {
	digits := strconv.Itoa(count)
	for at := len(digits) - groupDigits; at > 0; at -= groupDigits {
		digits = digits[:at] + "," + digits[at:]
	}

	return digits
}

// groupDigits is how many digits sit between commas.
const groupDigits = 3
