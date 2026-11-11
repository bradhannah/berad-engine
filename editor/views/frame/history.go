package frame

import (
	"image"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

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
// refusal.
type Said struct {
	At      time.Time
	Text    string
	Refusal bool
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
}

func newHistoryState() historyState {
	return historyState{
		log: nil, refusal: "", open: false, list: ui.NewList("history", ""),
		button: ui.NewButton("history", "History"), dismiss: ui.NewButton("dismiss", "Dismiss"), panel: ui.Zero(),
		hand: "",
	}
}

// keep adds a message to the log.
func (f *Frame) keep(text string, refusal bool) {
	f.history.log = append(f.history.log, Said{At: f.now(), Text: text, Refusal: refusal})
	if len(f.history.log) > logLength {
		f.history.log = f.history.log[len(f.history.log)-logLength:]
	}
}

// Refuse shows why something was refused. It stays in the status bar until
// Dismiss is pressed, and goes into the log as a refusal.
func (f *Frame) Refuse(reason string) {
	f.history.refusal = reason
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
// Esc, History again or a click outside closes it.
func (f *Frame) UpdateHistory(input *ui.Input, size image.Point) {
	if !f.history.open {
		return
	}
	defer input.ClearAll()
	f.layoutStatus(size)
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
}

// historyRows lists the log newest first, each with its time.
func (f *Frame) historyRows() []ui.Row {
	rows := make([]ui.Row, 0, len(f.history.log))
	for index := len(f.history.log) - 1; index >= 0; index-- {
		said := f.history.log[index]
		row := ui.NewRow("said:"+said.At.Format(time.RFC3339Nano)+":"+said.Text, said.Text)
		row.Detail = said.At.Format(historyStamp)
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
	note := "newest first · Esc closes"
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
// and "state:said:<words>" when a message in the log has the words.
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
