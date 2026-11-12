package frame

import (
	"image"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// A history that can be read (increment 39): every message a window is
// given is kept, newest first, in a list History in the status bar opens.
// A refusal stays in the status bar, in red with its reason, until Dismiss
// is pressed; other messages still fade.

// History's limits and sizes, in screen pixels.
const (
	logLength     = 200
	historyWidth  = 76
	dismissWidth  = 76
	historyPanelW = 560
	historyRows   = 14
	historyLeast  = 3
	historyStamp  = "15:04:05"
)

// Said is one message a window was given, when, and whether it was a
// refusal. Touched is what the change it says touched; Item and Name are
// the thing of it the view can still select and its name, found while
// History is open (increment 45).
type Said struct {
	At      time.Time
	Text    string
	Refusal bool
	Touched protocol.Touch
	Item    string
	Name    string
}

// Namer is a view that can say which of its things a change touched: an
// item it can be asked to show, as Back and Forward ask, and the thing's
// name; ok is false while none it can select is still there (increment 45).
type Namer interface {
	Named(touched protocol.Touch, docs Docs) (item, name string, ok bool)
}

// historyState is the window's log, the refusal it shows until dismissed,
// and the History list while it is open.
type historyState struct {
	log     []Said
	refusal string
	open    bool
	list    *ui.List
	button  *ui.Button
	dismiss *ui.Button
	panel   image.Rectangle
	// hand is what the status bar says is in hand, at its right.
	hand string
	// chosen is the item of the line last clicked in History, until the
	// window takes it, and chosenName its name.
	chosen     string
	chosenName string
}

func newHistoryState() historyState {
	return historyState{
		log: nil, refusal: "", open: false, list: ui.NewList("history", ""),
		button: ui.NewButton("history", "History"), dismiss: ui.NewButton("dismiss", "Dismiss"), panel: ui.Zero(),
		hand: "", chosen: "", chosenName: "",
	}
}

// keep adds a message to the log.
func (f *Frame) keep(text string, refusal bool) {
	f.history.log = append(f.history.log, Said{
		At: f.now(), Text: text, Refusal: refusal, Touched: protocol.Untouched(), Item: "", Name: "",
	})
	if len(f.history.log) > logLength {
		f.history.log = f.history.log[len(f.history.log)-logLength:]
	}
}

// StatusAbout says a change, as Status does, and keeps what it touched with
// its line in the log, so a click on the line in History selects it.
func (f *Frame) StatusAbout(message string, touched protocol.Touch) {
	f.Status(message)
	if last := len(f.history.log) - 1; last >= 0 && f.history.log[last].Text == message {
		f.history.log[last].Touched = touched
	}
}

// StatusAgain says a change that goes on one said just before, as a stroke
// does: its line in the log takes the new words, rather than another line
// being added (increment 51).
func (f *Frame) StatusAgain(message string, touched protocol.Touch) {
	last := len(f.history.log) - 1
	if last < 0 || f.history.log[last].Refusal {
		f.StatusAbout(message, touched)

		return
	}
	f.status, f.statusUntil = message, f.now().Add(statusDuration)
	f.history.log[last].At, f.history.log[last].Text, f.history.log[last].Touched = f.now(), message, touched
}

// TakeChosen is the item of the line clicked in History and its name, once:
// the window asks its view to show it.
func (f *Frame) TakeChosen() (string, string, bool) {
	item, name := f.history.chosen, f.history.chosenName
	f.history.chosen, f.history.chosenName = "", ""

	return item, name, item != ""
}

// Refuse shows why something was refused. It stays in the status bar until
// Dismiss is pressed, and goes into the log as a refusal. It takes the place
// of a message said before it, which a later message takes from it again.
func (f *Frame) Refuse(reason string) {
	f.history.refusal = reason
	f.statusUntil = f.now()
	f.keep(reason, true)
}

// SetHand sets what the status bar says is in hand: a view that has tools
// names the one in hand and what the pointer is over, every frame.
func (f *Frame) SetHand(text string) { f.history.hand = text }

// Refusal is the refusal the status bar shows until it is dismissed.
func (f *Frame) Refusal() string { return f.history.refusal }

// Log is every message the window was given, oldest first.
func (f *Frame) Log() []Said { return f.history.log }

// HistoryOpen reports whether the History list shows.
func (f *Frame) HistoryOpen() bool { return f.history.open }

// layoutHistory places History, and Dismiss before it while a refusal shows,
// left of the Inspector toggle; and the list above the status bar.
func (f *Frame) layoutHistory(size image.Point) {
	top := size.Y - StatusHeight + (StatusHeight-ui.ButtonH)/ui.Half
	right := f.ShowInspector.Rect.Min.X - statusGap/ui.Half
	f.history.button.Rect = image.Rect(right-historyWidth, top, right, top+ui.ButtonH)
	f.history.dismiss.Rect = ui.Zero()
	if f.history.refusal != "" {
		right = f.history.button.Rect.Min.X - ui.Gap
		f.history.dismiss.Rect = image.Rect(right-dismissWidth, top, right, top+ui.ButtonH)
	}
	rows := min(max(len(f.history.log), historyLeast), historyRows)
	height := min(rows*ui.RowHeight+ui.Half*ui.Inset+ui.LineHeight, size.Y-StatusHeight-ui.Inset)
	width := min(historyPanelW, size.X-ui.Half*ui.Inset)
	bottom := size.Y - StatusHeight - ui.Gap
	f.history.panel = image.Rect(ui.Inset, bottom-height, ui.Inset+width, bottom)
	f.history.list.Rect = image.Rect(f.history.panel.Min.X+ui.Gap, f.history.panel.Min.Y+ui.Inset+ui.LineHeight,
		f.history.panel.Max.X-ui.Gap, f.history.panel.Max.Y-ui.Gap)
}

// statusLeft is where the status bar's buttons start, which its message
// stops short of.
func (f *Frame) statusLeft() int {
	if !f.history.dismiss.Rect.Empty() {
		return f.history.dismiss.Rect.Min.X
	}

	return f.history.button.Rect.Min.X
}

// updateHistoryButtons handles History and Dismiss in the status bar.
func (f *Frame) updateHistoryButtons(input *ui.Input) {
	if f.history.dismiss.Update(input) {
		f.history.refusal = ""
	}
	if f.history.button.Update(input) {
		f.history.open = !f.history.open
		f.history.list.Selected = ""
	}
}

// UpdateHistory runs the open History list for a frame, taking every press:
// Esc, History again or a click outside closes it. A click on a line that
// names something the view can select closes it and chooses that thing.
// named finds, for each line, what the view can still select.
func (f *Frame) UpdateHistory(input *ui.Input, size image.Point, named func(protocol.Touch) (string, string, bool)) {
	if !f.history.open {
		return
	}
	defer input.ClearAll()
	f.layoutStatus(size)
	f.nameLines(named)
	f.history.list.Rows = f.historyRows()
	if input.Has(ui.KeyEscape, ui.Plain()) {
		f.history.open = false

		return
	}
	if f.history.button.Update(input) {
		f.history.open = false

		return
	}
	if input.Pressed && !input.Cursor.In(f.history.panel) {
		f.history.open = false

		return
	}
	f.history.list.Update(input)
	f.chooseLine()
}

// nameLines finds what each line's change touched that the view can still
// select.
func (f *Frame) nameLines(named func(protocol.Touch) (string, string, bool)) {
	for index := range f.history.log {
		said := &f.history.log[index]
		said.Item, said.Name = "", ""
		if named == nil || said.Touched.Document.IsZero() {
			continue
		}
		if item, name, ok := named(said.Touched); ok {
			said.Item, said.Name = item, name
		}
	}
}

// chooseLine takes a click on a line that names something to select: it
// closes History and chooses the thing. A line that names nothing does
// nothing.
func (f *Frame) chooseLine() {
	clicked := f.history.list.Selected
	f.history.list.Selected = ""
	index, ok := lineIndex(clicked)
	if !ok || index >= len(f.history.log) || f.history.log[index].Item == "" {
		return
	}
	f.history.chosen, f.history.chosenName = f.history.log[index].Item, f.history.log[index].Name
	f.history.open = false
}

// lineRow is the row ID of the log's line at an index, and lineIndex the
// index of a row ID.
func lineRow(index int) string { return linePrefix + strconv.Itoa(index) }

func lineIndex(row string) (int, bool) {
	number, ok := strings.CutPrefix(row, linePrefix)
	if !ok {
		return 0, false
	}
	index, err := strconv.Atoi(number)

	return index, err == nil && index >= 0
}

// linePrefix starts the row ID of a line in History.
const linePrefix = "line:"

// historyRows lists the log newest first, each with its time, and what a
// click selects where a line names something still there.
func (f *Frame) historyRows() []ui.Row {
	rows := make([]ui.Row, 0, len(f.history.log))
	for index := len(f.history.log) - 1; index >= 0; index-- {
		said := f.history.log[index]
		row := ui.NewRow(lineRow(index), said.Text)
		row.Detail = said.At.Format(historyStamp)
		if said.Item != "" {
			row.Detail = "selects " + said.Name + " · " + row.Detail
		}
		if said.Refusal {
			row.Label = "Refused: " + said.Text
		}
		rows = append(rows, row)
	}

	return rows
}

// DrawHistory draws the History list over the window while it is open,
// floating on a hard shadow above the status bar.
func (f *Frame) DrawHistory(screen *ebiten.Image, theme *ui.Theme) {
	if !f.history.open {
		return
	}
	f.layoutStatus(screen.Bounds().Size())
	f.history.list.Rows = f.historyRows()
	panel := f.history.panel
	ui.FloatBox(screen, panel, ui.ColorPage())
	theme.DrawStrong(screen, "History", panel.Min.X+ui.Gap, panel.Min.Y+ui.Gap, ui.ColorInk())
	note := "newest first · a click selects what a line names · Esc closes"
	theme.DrawText(screen, note, panel.Min.X+ui.Gap+theme.StrongWidth("History")+ui.Inset, panel.Min.Y+ui.Gap,
		ui.ColorSubdued())
	if len(f.history.log) == 0 {
		theme.DrawText(screen, "Nothing yet", f.history.list.Rect.Min.X+ui.Gap, f.history.list.Rect.Min.Y+ui.Gap,
			ui.ColorSubdued())

		return
	}
	f.history.list.Draw(screen, theme)
}

// locateHistory finds History's parts for input scripts: "ui:history",
// "ui:dismiss", "state:history-open", "state:refused" while a refusal shows,
// "state:said:<words>" when a message in the log has the words, and
// "history:<words>" for the newest line with the words in the open list.
func (f *Frame) locateHistory(name string) (image.Point, bool, bool) {
	switch name {
	case "ui:history":
		return ui.Center(f.history.button.Rect), !f.history.button.Rect.Empty(), true
	case "ui:dismiss":
		return ui.Center(f.history.dismiss.Rect), !f.history.dismiss.Rect.Empty(), true
	case "state:history-open":
		return image.Pt(0, 0), f.history.open, true
	case "state:refused":
		return image.Pt(0, 0), f.history.refusal != "", true
	}
	if words, ok := strings.CutPrefix(name, "state:hand:"); ok {
		return image.Pt(0, 0), strings.Contains(f.history.hand, words), true
	}
	if words, ok := strings.CutPrefix(name, "history:"); ok {
		return f.locateLine(words)
	}
	if words, ok := strings.CutPrefix(name, "state:said:"); ok {
		for _, said := range f.history.log {
			if strings.Contains(said.Text, words) {
				return image.Pt(0, 0), true, true
			}
		}

		return image.Pt(0, 0), false, true
	}

	return image.Pt(0, 0), false, false
}

// locateLine finds the newest line in the open History list with the words.
func (f *Frame) locateLine(words string) (image.Point, bool, bool) {
	if !f.history.open {
		return image.Pt(0, 0), false, true
	}
	for index := len(f.history.log) - 1; index >= 0; index-- {
		if strings.Contains(f.history.log[index].Text, words) {
			point, ok := f.history.list.Locate(lineRow(index))

			return point, ok && point.In(f.history.list.Rect), true
		}
	}

	return image.Pt(0, 0), false, true
}
