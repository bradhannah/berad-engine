// Package watchview is the Watch window: during a preview it follows the NPC
// selected on the map and shows where that NPC is in its workflow, the
// workflow drawn read-only with the node it is on outlined in orange and the
// line it just took flashing, what it is doing now, where it stands, and the
// clock. Pause and Step to next change control the preview's NPCs, so each
// change from one node to the next can be checked: a step goes straight to
// the watched NPC's next node, and the way it went stays lit until the NPCs
// resume or step again.
package watchview

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ViewName is the Watch window's view name; its target is a zone.
const ViewName = "watch"

// Layout, in screen pixels.
const (
	headerHeight = 44
	footerHeight = 92 + ui.LineHeight
	valuesWidth  = 260
	buttonWidth  = 170
	buttonGap    = 8
	shortID      = 6
)

// Footer labels and the step button's labels.
const (
	labelNow      = "NOW"
	labelStep     = "STEP"
	labelLast     = "LAST"
	labelAt       = "AT"
	textStep      = "Step to next change"
	textStepping  = "Stepping…"
	pathSeparator = " → "
)

// Messages shown in place of a workflow.
const (
	textNoPreview   = "Start a preview in a Map, then click an NPC."
	textChooseNPC   = "Click an NPC on the map to watch it."
	textChooseLoop  = "Click an NPC on the map, or Watch zone loop, to watch it."
	textNoBehaviour = "This NPC has no behaviour."
	textWatchLoop   = "Watch zone loop"
)

// State is one Watch window.
type State struct {
	window ids.ULID
	zone   ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	run    ids.ULID
	tick   uint64
	state  *protocol.PreviewState
	flow   *workflowview.State
	shown  ids.ULID
	pause  *ui.Button
	step   *ui.Button
	// watchLoop watches the zone's loop, while the preview runs one.
	watchLoop *ui.Button
	// chosen is the workflow of the path clicked, by its place and ID,
	// -1 to follow the NPC; parts are where each workflow of the path is
	// drawn.
	chosen     int
	chosenFlow ids.ULID
	parts      []image.Rectangle
	// log is what the Logs wrote in this preview.
	log *logPanel
}

// New returns the Watch window for a zone.
func New(window, zone ids.ULID, theme *ui.Theme) *State {
	return &State{
		window: window, zone: zone, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{}, run: ids.ULID{},
		tick: 0, state: nil, flow: nil, shown: ids.ULID{},
		pause: ui.NewButton("pause", "Pause NPCs"), step: ui.NewButton("step", textStep),
		watchLoop: ui.NewButton("watch-loop", textWatchLoop), chosen: -1,
		chosenFlow: ids.ULID{}, parts: nil, log: newLogPanel(),
	}
}

// Start subscribes to the project index, for names.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] {
		return
	}
	s.subs[document] = true
	ctx.Requests.Subscribe(document, func(err error) {
		if err != nil {
			delete(s.subs, document)
		}
	})
}

// Focused returns no field: the Watch window takes no typing.
func (s *State) Focused() *ui.TextField { return nil }

// Title is the window title.
func (s *State) Title(docs frame.Docs) string {
	entity, ok := s.watched()
	if !ok {
		return "Watch  (view " + s.window.Short(shortID) + ")"
	}

	return "Watch: " + s.entityName(docs, entity, "NPC") + "  (view " + s.window.Short(shortID) + ")"
}

// entityName names what is watched: an NPC, or a zone loop by its zone.
func (s *State) entityName(docs frame.Docs, entity protocol.PreviewEntity, fallback string) string {
	if !entity.Loop.IsZero() {
		return s.name(docs, entity.Loop, fallback)
	}

	return s.name(docs, entity.NPC, fallback)
}

// loop returns the zone's loop in the preview, if it runs one.
func (s *State) loop() (protocol.PreviewEntity, bool) {
	if s.state != nil {
		for _, entity := range s.state.Entities {
			if !entity.Loop.IsZero() {
				return entity, true
			}
		}
	}

	var none protocol.PreviewEntity

	return none, false
}

// Changed follows the zone's preview: its start, which starts the log
// afresh, its latest state and the lines it brings, and its end, after
// which the log stays to read.
func (s *State) Changed(update client.Update, _ *frame.Context) {
	switch {
	case update.PreviewStart != nil:
		s.run, s.tick, s.state = update.PreviewStart.Run, 0, nil
		s.log.restart()
	case update.PreviewState != nil && update.PreviewState.Run == s.run && update.PreviewState.Tick > s.tick:
		state := *update.PreviewState
		s.tick, s.state = state.Tick, &state
		s.log.keep(state.Log)
	case update.PreviewStop != nil && update.PreviewStop.Run == s.run:
		s.run, s.tick, s.state, s.flow, s.shown = ids.ULID{}, 0, nil, nil, ids.ULID{}
	}
}

// watched returns the NPC being watched, if the preview has one.
func (s *State) watched() (protocol.PreviewEntity, bool) {
	if s.state == nil || s.state.Watched.IsZero() {
		var none protocol.PreviewEntity

		return none, false
	}
	for _, entity := range s.state.Entities {
		if entity.ID == s.state.Watched.String() {
			return entity, true
		}
	}

	var none protocol.PreviewEntity

	return none, false
}

// canvas is where the workflow is drawn, between the header and the footer.
func (s *State) canvas() image.Rectangle {
	content := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	right := content.Max.X
	if !s.valuesColumn().Empty() {
		right -= valuesWidth
	}

	return image.Rect(0, headerHeight, right, s.logArea().Min.Y)
}

// valuesColumn is where the watched NPC's values are listed, beside its
// workflow, while it has any.
func (s *State) valuesColumn() image.Rectangle {
	if s.state == nil || len(s.state.Values) == 0 {
		return ui.Zero()
	}

	return image.Rect(s.size.X-valuesWidth, headerHeight, s.size.X, s.logArea().Min.Y)
}

// drawValues lists the watched NPC's values: its workflow's locals and its
// own, its zone's and the world's.
func (s *State) drawValues(screen *ebiten.Image) {
	column := s.valuesColumn()
	if column.Empty() {
		return
	}
	ui.Divider(screen, column.Min, image.Pt(column.Min.X, column.Max.Y))
	left := column.Min.X + ui.Inset
	s.theme.DrawLabel(screen, "VALUES", left, column.Min.Y+ui.Inset, ui.ColorMuted())
	top := column.Min.Y + ui.Inset + ui.LineHeight
	for _, value := range s.state.Values {
		if top+ui.LineHeight > column.Max.Y {
			break
		}
		width := s.theme.TextWidth(value.Value)
		room := column.Max.X - ui.Inset - left - width - ui.Inset
		s.theme.DrawMono(screen, s.fit([]string{value.Path}, room), left, top, ui.ColorInk())
		s.theme.DrawText(screen, value.Value, column.Max.X-ui.Inset-width, top, ui.ColorLive())
		top += ui.LineHeight
	}
}

// layout places the buttons in the footer, each as wide as its label
// needs, Watch zone loop only while the preview runs one.
func (s *State) layout() {
	bottom := s.size.Y - frame.StatusHeight - ui.Inset
	s.pause.Label, s.step.Label = "Pause NPCs", textStep
	if s.state != nil && s.state.NPCsPaused {
		s.pause.Label = "Resume NPCs"
	}
	stepping := s.state != nil && s.state.Stepping
	if stepping {
		s.step.Label = textStepping
	}
	s.pause.Disabled, s.step.Disabled = s.state == nil, s.state == nil || s.state.Watched.IsZero() || stepping
	left := ui.Inset
	place := func(button *ui.Button) {
		width := max(buttonWidth/ui.Half, s.theme.TextWidth(button.Label)+ui.Half*ui.Inset)
		button.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
		left += width + buttonGap
	}
	place(s.pause)
	place(s.step)
	s.watchLoop.Rect = ui.Zero()
	if loop, runs := s.loop(); runs {
		place(s.watchLoop)
		s.watchLoop.Disabled = s.state.Watched.String() == loop.ID
	}
}

// Update follows the watched NPC's workflow and sends the buttons' controls.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.layout()
	s.updateLog(input, ctx)
	entity, watching := s.watched()
	if watching {
		s.updatePath(input, entity)
	}
	shown, following := s.shownLevel(entity)
	switch {
	case !watching || shown.workflow.IsZero():
		s.flow, s.shown = nil, ids.ULID{}
	case shown.workflow != s.shown:
		s.subscribe(shown.workflow, ctx)
		s.flow, s.shown = workflowview.NewLive(s.window, shown.workflow, s.theme), shown.workflow
	}
	if s.flow != nil {
		s.flow.SetArea(s.canvas())
		if following {
			s.flow.SetLive(entity.Node, entity.Last.From, entity.Last.Output, entity.Changes)
			s.flow.SetStep(s.state.Step)
			s.flow.SetLanes(entity.Lanes)
		} else {
			s.flow.SetLive(shown.node, ids.ULID{}, "", entity.Changes)
			s.flow.SetStep(nil)
			s.flow.SetLanes(nil)
		}
		s.flow.Update(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	if s.state == nil {
		return
	}
	control := protocol.PreviewControl{Run: s.run, Action: "", Placement: s.state.Watched, Rate: 0, Seconds: 0}
	switch {
	case s.pause.Update(input):
		control.Action = protocol.ControlPause
		if s.state.NPCsPaused {
			control.Action = protocol.ControlResume
		}
	case s.step.Update(input):
		control.Action = protocol.ControlStep
	case s.watchLoop.Update(input):
		loop, _ := s.loop()
		control.Action, control.Placement = protocol.ControlWatch, loop.Loop
	default:
		return
	}
	ctx.Requests.PreviewControl(control)
}

// Draw renders the window: the header, the workflow and the footer.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout()
	screen.Fill(ui.ColorBackground())
	if area, ok := screen.SubImage(s.canvas()).(*ebiten.Image); ok {
		area.Fill(ui.ColorCanvas())
	}
	entity, watching := s.watched()
	message := ""
	switch {
	case s.state == nil:
		message = textNoPreview
	case !watching:
		message = textChooseNPC
		if _, runs := s.loop(); runs {
			message = textChooseLoop
		}
	case entity.Workflow.IsZero():
		message = textNoBehaviour
	case s.flow != nil:
		s.flow.SetArea(s.canvas())
		s.flow.Draw(screen, docs)
	}
	if message != "" {
		canvas := s.canvas()
		s.theme.DrawText(screen, message, canvas.Min.X+ui.Inset, canvas.Min.Y+ui.Inset, ui.ColorMuted())
	}
	s.drawValues(screen)
	s.layoutLog(docs)
	s.drawLog(screen, docs)
	s.drawHeader(screen, docs, entity, watching)
	s.drawFooter(screen, docs, entity, watching)
}

func (s *State) drawHeader(screen *ebiten.Image, docs frame.Docs, entity protocol.PreviewEntity, watching bool) {
	width := s.size.X
	textY := (headerHeight - ui.TextSize) / ui.Half
	s.theme.DrawLabel(screen, "WATCH", ui.Inset, (headerHeight-ui.LabelSize)/ui.Half-1, ui.ColorMuted())
	nameX := ui.Inset + s.theme.LabelWidth("WATCH") + ui.Inset
	if watching {
		name := s.entityName(docs, entity, "Deleted NPC")
		s.theme.DrawStrong(screen, name, nameX, textY, ui.ColorInk())
		pathX := nameX + s.theme.StrongWidth(name) + ui.Inset
		if !entity.Loop.IsZero() {
			s.theme.DrawText(screen, "zone loop", pathX, textY, ui.ColorMuted())
			pathX += s.theme.TextWidth("zone loop") + ui.Inset
		}
		s.drawPath(screen, docs, entity, pathX)
	}
	if s.state != nil {
		clockText := "LIVE  " + clock.Format(s.state.Clock) + " · " + clock.RateLabel(clock.Rate(s.state.Rate))
		if s.state.ClockStopped {
			clockText += " · clock stopped"
		}
		if s.state.Stepping {
			clockText += " · stepping"
		}
		if s.state.NPCsPaused {
			clockText += " · paused"
		}
		s.theme.DrawMono(screen, clockText, width-ui.Inset-s.theme.MonoWidth(clockText),
			(headerHeight-ui.MonoSize)/ui.Half-1, ui.ColorLive())
	}
	ui.Divider(screen, image.Pt(0, headerHeight-ui.Outline), image.Pt(width, headerHeight-ui.Outline))
}

// drawFooter draws what the watched NPC is doing now, the way its latest
// step went (or its latest change while it runs), where it stands, and the
// buttons.
func (s *State) drawFooter(screen *ebiten.Image, docs frame.Docs, entity protocol.PreviewEntity, watching bool) {
	top := s.logArea().Max.Y
	ui.Divider(screen, image.Pt(0, top), image.Pt(s.size.X, top))
	if watching {
		now := entity.Activity
		if entity.Status != "" {
			now = entity.Status
		}
		now = handlingLine(entity) + now + lanesLine(entity.Lanes) + heldLine(entity)
		if way := s.wayLine(docs, entity); way != "" {
			now += " · " + way
		}
		where := s.whereLine(docs, entity)
		label, path := s.way(docs, entity)
		column := ui.Inset + max(s.theme.LabelWidth(labelStep), s.theme.LabelWidth(labelLast)) + ui.Inset
		rows := []struct{ label, text string }{
			{labelNow, s.theme.FitText(now, s.size.X-column-ui.Inset)}, {label, s.fit(path, s.size.X-column-ui.Inset)},
			{labelAt, where},
		}
		for index, row := range rows {
			if row.text == "" {
				continue
			}
			y := top + index*ui.LineHeight
			s.theme.DrawLabel(screen, row.label, ui.Inset, y+ui.Gap*ui.Half, ui.ColorMuted())
			s.theme.DrawText(screen, row.text, column, y+ui.Gap, ui.ColorInk())
		}
	}
	s.pause.Draw(screen, s.theme)
	s.step.Draw(screen, s.theme)
	if !s.watchLoop.Rect.Empty() {
		s.watchLoop.Draw(screen, s.theme)
	}
}

// way is the way the watched NPC went: its latest step's changes, under
// STEP, or while it runs its latest change, under LAST.
func (s *State) way(docs frame.Docs, entity protocol.PreviewEntity) (string, []string) {
	if s.flow == nil {
		return labelLast, nil
	}
	if len(s.state.Step) > 0 {
		return labelStep, s.flow.Path(s.state.Step, docs)
	}
	if entity.Last.From.IsZero() {
		return labelLast, nil
	}

	return labelLast, s.flow.Path([]protocol.PreviewChange{entity.Last}, docs)
}

// fit joins a path's parts, leaving out the earliest when they are too wide.
func (s *State) fit(parts []string, width int) string {
	text := strings.Join(parts, pathSeparator)
	for len(parts) > 1 && s.theme.TextWidth(text) > width {
		parts = parts[1:]
		text = "…" + pathSeparator + strings.Join(parts, pathSeparator)
	}

	return text
}

// name returns a document's name from the project index.
func (s *State) name(docs frame.Docs, id ids.ULID, fallback string) string {
	name := fallback
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(id, fallback)
		}
	})

	return name
}

// Locate finds a named part of the window for input scripts: "ui:pause",
// "ui:step", the log's buttons, "log-line:<n>", or "state:..." (see
// hasState).
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout()
	s.layoutLog(docs)
	kind, value, _ := strings.Cut(name, ":")
	switch kind {
	case strings.TrimSuffix(linePart, ":"):
		return s.locateLine(value)
	case "ui":
		for _, button := range []*ui.Button{s.pause, s.step, s.watchLoop, s.log.watched, s.log.all, s.log.clear} {
			if button.ID == value {
				return ui.Center(button.Rect), true
			}
		}

		return s.locatePart(value)
	case "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts: "previewing" while a preview runs,
// "watching:<name>" while the named NPC is watched, "live:<kind>" while it is
// on a node of that kind, "paused" while its NPCs are paused, and "stepped"
// while they are paused after a step that changed node, and
// "value:<path>=<value>" while the watched NPC's value is that,
// "path=<path>" while the path down through its calls reads so, and
// "showing:<workflow>" while that workflow is drawn; and the log's states
// (see hasLogState).
func (s *State) hasState(state string, docs frame.Docs) bool {
	if found, handled := s.hasPartState(state); handled {
		return found
	}
	entity, watching := s.watched()
	switch {
	case state == "previewing":
		return s.state != nil
	case state == "paused":
		return s.state != nil && s.state.NPCsPaused
	case state == "stepped":
		return s.state != nil && s.state.NPCsPaused && len(s.state.Step) > 0
	case strings.HasPrefix(state, "watching:"):
		return watching && s.entityName(docs, entity, "") == strings.TrimPrefix(state, "watching:")
	case strings.HasPrefix(state, "live:"):
		return watching && s.nodeKind(docs, entity) == strings.TrimPrefix(state, "live:")
	case strings.HasPrefix(state, "path="), strings.HasPrefix(state, "showing:"), strings.HasPrefix(state, "way="),
		strings.HasPrefix(state, "at="):
		return s.hasCallState(state, docs, entity, watching)
	case strings.HasPrefix(state, "value:"):
		return s.state != nil && slices.ContainsFunc(s.state.Values, func(value protocol.PreviewValue) bool {
			return value.Path+"="+value.Value == strings.TrimPrefix(state, "value:")
		})
	}

	return false
}

// nodeKind is the kind of the node a watched NPC is on.
func (s *State) nodeKind(docs frame.Docs, entity protocol.PreviewEntity) string {
	kind := ""
	docs.View(entity.Workflow, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			if node, found := workflowDoc.Workflow.Node(entity.Node); found {
				kind = node.Kind
			}
		}
	})

	return kind
}

// wayLine says which floors a watched NPC's Go to crosses, as "Ground floor
// → stairs down at 12, 4 → Cellar", or "" while it stays on its floor.
func (s *State) wayLine(docs frame.Docs, entity protocol.PreviewEntity) string {
	if len(entity.Way) == 0 {
		return ""
	}
	parts := []string{s.name(docs, entity.Way[0].From, "a floor")}
	for _, climb := range entity.Way {
		way := "down"
		if climb.Up {
			way = "up"
		}
		parts = append(parts, fmt.Sprintf("stairs %s at %d, %d", way, climb.Col, climb.Row),
			s.name(docs, climb.To, "a floor"))
	}

	return strings.Join(parts, " → ")
}

// whereLine says where a watched NPC stands, as "cell 2, 0 on Upper floor";
// a zone loop stands nowhere.
func (s *State) whereLine(docs frame.Docs, entity protocol.PreviewEntity) string {
	if !entity.Loop.IsZero() {
		return ""
	}

	return fmt.Sprintf("cell %d, %d on %s", int(math.Round(entity.X)), int(math.Round(entity.Y)),
		s.name(docs, entity.Floor, "a floor"))
}

// laneDoing is what a lane is doing, in a word or two when it is done.
func laneDoing(lane protocol.PreviewLane) string {
	if lane.Done {
		return "done"
	}

	return lane.Doing
}

// hasLaneState answers "lane:<n>=<what it is doing>" and "lane-done:<n>"
// for the watched NPC's Together's lanes.
func hasLaneState(entity protocol.PreviewEntity, state string) bool {
	if number, done := strings.CutPrefix(state, "lane-done:"); done {
		index, err := strconv.Atoi(number)

		return err == nil && index >= 1 && index <= len(entity.Lanes) && entity.Lanes[index-1].Done
	}
	number, doing, _ := strings.Cut(strings.TrimPrefix(state, "lane:"), "=")
	index, err := strconv.Atoi(number)

	return err == nil && index >= 1 && index <= len(entity.Lanes) && !entity.Lanes[index-1].Done &&
		entity.Lanes[index-1].Doing == doing
}

// hasPartState answers the log's states (see hasLogState) and the watched
// NPC's lanes', and whether the state is one of them.
func (s *State) hasPartState(state string) (bool, bool) {
	if found, handled := s.hasLogState(state); handled {
		return found, true
	}
	entity, watching := s.watched()
	if strings.HasPrefix(state, "lane:") || strings.HasPrefix(state, "lane-done:") {
		return watching && hasLaneState(entity, state), true
	}
	if found, handled := hasSignalState(entity, state); handled {
		return watching && found, true
	}

	return false, false
}

// lanesLine says what each lane of a Together is doing, after what the NPC
// is doing: " · 1 Shake: Shake screen · 2 Chime: done".
func lanesLine(lanes []protocol.PreviewLane) string {
	var line strings.Builder
	for index, lane := range lanes {
		line.WriteString(" · " + strconv.Itoa(index+1) + " " + lane.Label + ": " + laneDoing(lane))
	}

	return line.String()
}

// handlingLine says which signal the NPC is handling and from whom, and what
// it will resume, before what it is doing: "Handling bread_called from
// Baker, then Wait 7 min of 10 min · ".
func handlingLine(entity protocol.PreviewEntity) string {
	if entity.Handling == "" {
		return ""
	}
	line := "Handling " + entity.Handling
	if entity.HandlingFrom != "" {
		line += " from " + entity.HandlingFrom
	}
	if entity.Resumes != "" {
		line += ", then " + entity.Resumes
	}

	return line + " · "
}

// heldLine names the signals waiting to land.
func heldLine(entity protocol.PreviewEntity) string {
	if len(entity.Held) == 0 {
		return ""
	}

	return " · waiting: " + strings.Join(entity.Held, ", ")
}

// hasSignalState answers "handling=<signal>", "resumes=<what>" and
// "held=<signal>" for the watched NPC, and whether the state is one of them.
func hasSignalState(entity protocol.PreviewEntity, state string) (bool, bool) {
	if signal, asked := strings.CutPrefix(state, "handling="); asked {
		return entity.Handling == signal, true
	}
	if what, asked := strings.CutPrefix(state, "resumes="); asked {
		return entity.Resumes == what, true
	}
	if signal, asked := strings.CutPrefix(state, "held="); asked {
		return slices.Contains(entity.Held, signal), true
	}

	return false, false
}
