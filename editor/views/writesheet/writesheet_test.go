package writesheet_test

import (
	"image"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/writesheet"
)

type clipboard struct{ text string }

func (c *clipboard) Read() (string, error) { return c.text, nil }
func (c *clipboard) Write(text string) error {
	c.text = text

	return nil
}

type fixture struct {
	t      *testing.T
	sheet  *writesheet.Sheet
	window image.Rectangle
	clip   *clipboard
	saved  []writesheet.Saving
	open   bool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	sheet := writesheet.New(theme, "npc.shop.greeting", []writesheet.Holder{
		{Key: "", Label: "Default", Text: "Welcome in!"},
		{Key: "marta", Label: "Marta's own", Text: "Fresh off the boat!"},
	}, 0, "used in Serve a customer")
	f := &fixture{t: t, sheet: sheet, window: image.Rect(0, 0, 1200, 800), clip: &clipboard{text: ""}, saved: nil,
		open: true}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func (f *fixture) frame(input ui.Input) {
	if !f.open {
		return
	}
	saving, open := f.sheet.Update(&input, f.window, f.clip)
	if saving != nil {
		f.saved = append(f.saved, *saving)
	}
	f.open = open
	f.sheet.Draw(ebiten.NewImage(f.window.Dx(), f.window.Dy()), f.window)
}

func (f *fixture) click(widget string) {
	f.t.Helper()
	at, found := f.sheet.Locate(widget)
	if !found {
		f.t.Fatalf("cannot locate %s", widget)
	}
	input := ui.NewInput(at)
	input.Pressed, input.Down = true, true
	f.frame(input)
	input = ui.NewInput(at)
	input.Released = true
	f.frame(input)
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) typed(text string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

// TestWritingWithMarkup types a line, makes a word bold and another gold
// with the buttons, switches to Marta's own, which saves the default, and
// saves hers with Done.
func TestWritingWithMarkup(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if found, _ := f.sheet.HasState("sheet-holder=Default"); !found {
		t.Fatal("the sheet opens on the default")
	}
	f.keys("Cmd+A", "Backspace")
	f.typed("Fresh bread")
	f.keys("Shift+Left", "Shift+Left", "Shift+Left", "Shift+Left", "Shift+Left")
	f.click("ui:sheet-bold")
	if found, _ := f.sheet.HasState("sheet-text=Fresh [b]bread[/b]"); !found {
		t.Fatalf("B puts bold around what is selected: %q", f.sheet.Text())
	}
	f.click("ui:sheet-colour")
	f.click("menu:sheet-colour-gold")
	if found, _ := f.sheet.HasState("sheet-text=Fresh [b][color=gold]bread[/color][/b]"); !found {
		t.Fatalf("a colour goes around what is still selected: %q", f.sheet.Text())
	}
	f.click("ui:sheet-holder")
	f.click("menu:sheet-holder-1")
	if len(f.saved) != 1 || f.saved[0].Key != "" || !strings.Contains(f.saved[0].Text, "[b]") {
		t.Fatalf("switching saves the default: %+v", f.saved)
	}
	if found, _ := f.sheet.HasState("sheet-text=Fresh off the boat!"); !found {
		t.Fatalf("Marta's own is written next: %q", f.sheet.Text())
	}
	f.keys("Cmd+A", "Backspace")
	f.typed("Mackerel today.\nAsk about the eels.")
	f.click("ui:sheet-done")
	if f.open || len(f.saved) != 2 || f.saved[1].Key != "marta" || f.saved[1].Text != "Mackerel today.\nAsk about the eels." {
		t.Fatalf("Done saves Marta's and closes: %+v", f.saved)
	}
}

// TestNotesWarn says how long a text is and how long it takes to read,
// and warns of a tag left open, of a text too long to read in a box's time
// and of one too long for one box.
func TestNotesWarn(t *testing.T) {
	t.Parallel()
	about, warnings := writesheet.Notes("Hello [b]there", "", 1)
	if len(about) != 1 || !strings.Contains(about[0], "14 of 4,000") || len(warnings) != 1 {
		t.Fatalf("a short text with a tag left open: %v %v", about, warnings)
	}
	_, warnings = writesheet.Notes(strings.Repeat("A long letter. ", 20), "", 12)
	if len(warnings) != 2 {
		t.Fatalf("a long text is too long to read and for one box: %v", warnings)
	}
}
