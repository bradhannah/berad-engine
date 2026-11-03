package watchview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/clock"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The log: how many lines Watch keeps, how many it shows at once, and its
// buttons.
const (
	MaxLog         = 500
	logRows        = 6
	logWatchedID   = "log-watched"
	logEveryID     = "log-every"
	logClearID     = "log-clear"
	textEveryNPC   = "Every NPC"
	textNoLines    = "Lines a Log node writes show here."
	textNoneHere   = "No lines from this NPC yet."
	linePart       = "log-line:"
	logLinesState  = "log-lines="
	logLineState   = "log-line="
	logHeaderSpace = ui.ButtonH + ui.Gap*2
)

// logPanel is the log under the workflow: the lines the Logs wrote in this
// preview, the newest at the foot, the watched NPC's or every NPC's.
type logPanel struct {
	lines []protocol.PreviewLine
	// seen is the number of the latest line kept; back how many lines up
	// from the newest the list is scrolled.
	seen uint64
	back int
	// every shows every NPC's lines, not only the watched one's.
	every   bool
	watched *ui.Button
	all     *ui.Button
	clear   *ui.Button
	// shown are the rows drawn, oldest first, each with where its workflow
	// and node are written, which opens them.
	shown []logRow
}

type logRow struct {
	line protocol.PreviewLine
	top  int
	link image.Rectangle
}

func newLogPanel() *logPanel {
	return &logPanel{
		lines: nil, seen: 0, back: 0, every: false, watched: ui.NewButton(logWatchedID, "Watched NPC"),
		all: ui.NewButton(logEveryID, textEveryNPC), clear: ui.NewButton(logClearID, "Clear"), shown: nil,
	}
}

// restart empties the log, as a preview starts.
func (l *logPanel) restart() {
	l.lines, l.seen, l.back = nil, 0, 0
}

// keep adds the lines of a preview's state not kept yet, keeping the
// latest MaxLog.
func (l *logPanel) keep(lines []protocol.PreviewLine) {
	for _, line := range lines {
		if line.Seq > l.seen {
			l.lines, l.seen = append(l.lines, line), line.Seq
		}
	}
	if over := len(l.lines) - MaxLog; over > 0 {
		l.lines = slices.Delete(l.lines, 0, over)
	}
}

// panelHeight is how tall the log is: its buttons and its rows.
func panelHeight() int { return logHeaderSpace + logRows*ui.LineHeight + ui.Gap }

// logArea is where the log is drawn, between the workflow and the footer.
func (s *State) logArea() image.Rectangle {
	bottom := s.size.Y - frame.StatusHeight - footerHeight

	return image.Rect(0, bottom-panelHeight(), s.size.X, bottom)
}

// everyNPC reports whether the log shows every NPC's lines: chosen, or
// while no NPC is watched.
func (s *State) everyNPC() bool {
	return s.log.every || s.state == nil || s.state.Watched.IsZero()
}

// logged lists the lines the log shows, oldest first.
func (s *State) logged() []protocol.PreviewLine {
	if s.everyNPC() {
		return s.log.lines
	}
	watched := s.state.Watched.String()
	var lines []protocol.PreviewLine
	for _, line := range s.log.lines {
		if line.Entity == watched {
			lines = append(lines, line)
		}
	}

	return lines
}

// layoutLog places the log's buttons, each as wide as its label, the
// watched NPC's named for it, and its rows, the newest at the foot.
func (s *State) layoutLog(docs frame.Docs) {
	area := s.logArea()
	panel := s.log
	panel.watched.Label = "Watched NPC"
	if entity, watching := s.watched(); watching {
		panel.watched.Label = s.entityName(docs, entity, "Watched NPC")
	}
	panel.watched.On, panel.all.On = !s.everyNPC(), s.everyNPC()
	panel.watched.Disabled = s.state == nil || s.state.Watched.IsZero()
	left := ui.Inset + s.theme.LabelWidth("LOG") + ui.Inset
	top := area.Min.Y + ui.Gap
	for _, button := range []*ui.Button{panel.watched, panel.all, panel.clear} {
		width := s.theme.TextWidth(button.Label) + ui.Half*ui.Inset
		button.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		left += width + buttonGap
	}
	lines := s.logged()
	panel.back = max(0, min(panel.back, len(lines)-logRows))
	end := len(lines) - panel.back
	shown := lines[max(0, end-logRows):end]
	panel.shown = panel.shown[:0]
	rowTop := area.Max.Y - ui.Gap - len(shown)*ui.LineHeight
	for index, line := range shown {
		label := s.lineSource(docs, line)
		width := s.theme.TextWidth(label)
		at := rowTop + index*ui.LineHeight
		panel.shown = append(panel.shown, logRow{
			line: line, top: at, link: image.Rect(area.Max.X-ui.Inset-width, at, area.Max.X-ui.Inset, at+ui.LineHeight),
		})
	}
}

// lineSource names the workflow and the node that wrote a line, as
// "Customer shops · Log".
func (s *State) lineSource(docs frame.Docs, line protocol.PreviewLine) string {
	kind := "a deleted node"
	docs.View(line.Workflow, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			if node, found := workflowDoc.Workflow.Node(line.Node); found {
				declared, _ := flow.KindOf(node.Kind)
				kind = declared.Label
			}
		}
	})

	return s.name(docs, line.Workflow, "A deleted workflow") + " · " + kind
}

// lineWho names whose workflow wrote a line, for every NPC's lines: the
// NPC, or the zone whose loop it is.
func (s *State) lineWho(docs frame.Docs, line protocol.PreviewLine) string {
	if !line.Loop.IsZero() {
		return s.name(docs, line.Loop, "The zone") + " zone loop"
	}

	return s.name(docs, line.NPC, "A deleted NPC")
}

// updateLog handles the log's buttons, a click on a line's workflow and
// node, which opens them, and the wheel over it, which scrolls back
// through older lines.
func (s *State) updateLog(input *ui.Input, ctx *frame.Context) {
	panel := s.log
	s.layoutLog(ctx.Docs)
	switch {
	case panel.watched.Update(input):
		panel.every, panel.back = false, 0
	case panel.all.Update(input):
		panel.every, panel.back = true, 0
	case panel.clear.Update(input):
		panel.lines, panel.back = nil, 0
		ctx.Frame.Status("Log cleared. New lines show as Log nodes write them.")
	}
	for _, row := range panel.shown {
		if input.Pressed && input.Cursor.In(row.link) {
			input.ClearPointer()
			ctx.Requests.OpenItem(workflowview.ViewName, row.line.Workflow, workflowview.NodeItem+row.line.Node.String())
		}
	}
	if input.Cursor.In(s.logArea()) && input.WheelY != 0 {
		step := 1
		if input.WheelY < 0 {
			step = -1
		}
		panel.back += step
		input.WheelY = 0
	}
}

// drawLog draws the log: its label and buttons, and its rows, each with
// its game time, its text, for every NPC whose it is, and the workflow and
// node that wrote it.
func (s *State) drawLog(screen *ebiten.Image, docs frame.Docs) {
	area := s.logArea()
	panel := s.log
	ui.Divider(screen, area.Min, image.Pt(area.Max.X, area.Min.Y))
	s.theme.DrawLabel(screen, "LOG", ui.Inset, panel.clear.Rect.Min.Y+(ui.ButtonH-ui.LabelSize)/ui.Half-1,
		ui.ColorMuted())
	for _, button := range []*ui.Button{panel.watched, panel.all, panel.clear} {
		button.Draw(screen, s.theme)
	}
	if len(panel.shown) == 0 {
		empty := textNoLines
		if !s.everyNPC() && len(panel.lines) > 0 {
			empty = textNoneHere
		}
		s.theme.DrawText(screen, empty, ui.Inset, area.Min.Y+logHeaderSpace, ui.ColorMuted())

		return
	}
	timeWidth := s.theme.MonoWidth("00:00")
	for _, row := range panel.shown {
		s.theme.DrawMono(screen, clock.Format(row.line.Clock), ui.Inset, row.top, ui.ColorMuted())
		left := ui.Inset + timeWidth + ui.Inset
		if s.everyNPC() {
			who := s.lineWho(docs, row.line) + "  "
			s.theme.DrawStrong(screen, who, left, row.top, ui.ColorInk())
			left += s.theme.StrongWidth(who)
		}
		text := s.theme.FitText(row.line.Text, row.link.Min.X-ui.Inset-left)
		s.theme.DrawText(screen, text, left, row.top, ui.ColorInk())
		s.theme.DrawText(screen, s.lineSource(docs, row.line), row.link.Min.X, row.top, ui.ColorSelect())
	}
}

// locateLine finds "log-line:<n>", the workflow and node of the nth line
// shown, counting up from the newest, 1.
func (s *State) locateLine(value string) (image.Point, bool) {
	number, err := strconv.Atoi(strings.TrimPrefix(value, linePart))
	shown := s.log.shown
	if err != nil || number < 1 || number > len(shown) {
		return image.Pt(0, 0), false
	}

	return ui.Center(shown[len(shown)-number].link), true
}

// hasLogState answers "log-lines=<count>", how many lines the log holds as
// it shows them, and "log-line=<text>", whether it holds a line reading so.
func (s *State) hasLogState(state string) (bool, bool) {
	switch {
	case strings.HasPrefix(state, logLinesState):
		return strconv.Itoa(len(s.logged())) == strings.TrimPrefix(state, logLinesState), true
	case strings.HasPrefix(state, logLineState):
		return slices.ContainsFunc(s.logged(), func(line protocol.PreviewLine) bool {
			return line.Text == strings.TrimPrefix(state, logLineState)
		}), true
	}

	return false, false
}
