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
	textNoPreview   = "Start a preview in a Map window, then click an NPC."
	textChooseNPC   = "Click an NPC on the map to watch it."
	textNoBehaviour = "This NPC has no behaviour."
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
}

// New returns the Watch window for a zone.
func New(window, zone ids.ULID, theme *ui.Theme) *State {
	return &State{
		window: window, zone: zone, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{}, run: ids.ULID{},
		tick: 0, state: nil, flow: nil, shown: ids.ULID{},
		pause: ui.NewButton("pause", "Pause NPCs"), step: ui.NewButton("step", textStep),
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

	return "Watch: " + s.name(docs, entity.NPC, "NPC") + "  (view " + s.window.Short(shortID) + ")"
}

// Changed follows the zone's preview: its start, its latest state and its
// end.
func (s *State) Changed(update client.Update, _ *frame.Context) {
	switch {
	case update.PreviewStart != nil:
		s.run, s.tick, s.state = update.PreviewStart.Run, 0, nil
	case update.PreviewState != nil && update.PreviewState.Run == s.run && update.PreviewState.Tick > s.tick:
		state := *update.PreviewState
		s.tick, s.state = state.Tick, &state
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

	return image.Rect(0, headerHeight, content.Max.X, content.Max.Y-frame.StatusHeight-footerHeight)
}

// layout places the buttons in the footer.
func (s *State) layout() {
	bottom := s.size.Y - frame.StatusHeight - ui.Inset
	s.pause.Rect = image.Rect(ui.Inset, bottom-ui.ButtonH, ui.Inset+buttonWidth, bottom)
	s.step.Rect = image.Rect(ui.Inset+buttonWidth+buttonGap, bottom-ui.ButtonH, ui.Inset+ui.Half*buttonWidth+buttonGap,
		bottom)
	s.pause.Label, s.step.Label = "Pause NPCs", textStep
	if s.state != nil && s.state.NPCsPaused {
		s.pause.Label = "Resume NPCs"
	}
	stepping := s.state != nil && s.state.Stepping
	if stepping {
		s.step.Label = textStepping
	}
	s.pause.Disabled, s.step.Disabled = s.state == nil, s.state == nil || s.state.Watched.IsZero() || stepping
}

// Update follows the watched NPC's workflow and sends the buttons' controls.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.layout()
	entity, ok := s.watched()
	switch {
	case !ok || entity.Workflow.IsZero():
		s.flow, s.shown = nil, ids.ULID{}
	case entity.Workflow != s.shown:
		s.subscribe(entity.Workflow, ctx)
		s.flow, s.shown = workflowview.NewLive(s.window, entity.Workflow, s.theme), entity.Workflow
	}
	if s.flow != nil {
		s.flow.SetArea(s.canvas())
		s.flow.SetLive(entity.Node, entity.Last.From, entity.Last.Output, entity.Changes)
		s.flow.SetStep(s.state.Step)
		s.flow.Update(input, ctx)
	}
	frame.CommonKeys(input, ctx)
	if s.state == nil {
		return
	}
	control := protocol.PreviewControl{Run: s.run, Action: "", Placement: s.state.Watched}
	switch {
	case s.pause.Update(input):
		control.Action = protocol.ControlPause
		if s.state.NPCsPaused {
			control.Action = protocol.ControlResume
		}
	case s.step.Update(input):
		control.Action = protocol.ControlStep
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
	s.drawHeader(screen, docs, entity, watching)
	s.drawFooter(screen, docs, entity, watching)
}

func (s *State) drawHeader(screen *ebiten.Image, docs frame.Docs, entity protocol.PreviewEntity, watching bool) {
	width := s.size.X
	textY := (headerHeight - ui.TextSize) / ui.Half
	s.theme.DrawLabel(screen, "WATCH", ui.Inset, (headerHeight-ui.LabelSize)/ui.Half-1, ui.ColorMuted())
	nameX := ui.Inset + s.theme.LabelWidth("WATCH") + ui.Inset
	if watching {
		s.theme.DrawStrong(screen, s.name(docs, entity.NPC, "Deleted NPC"), nameX, textY, ui.ColorInk())
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
	top := s.canvas().Max.Y
	ui.Divider(screen, image.Pt(0, top), image.Pt(s.size.X, top))
	if watching {
		now := entity.Activity
		if entity.Status != "" {
			now = entity.Status
		}
		at := fmt.Sprintf("cell %d, %d on %s", int(math.Round(entity.X)), int(math.Round(entity.Y)),
			s.name(docs, entity.Floor, "a floor"))
		label, path := s.way(docs, entity)
		column := ui.Inset + max(s.theme.LabelWidth(labelStep), s.theme.LabelWidth(labelLast)) + ui.Inset
		rows := []struct{ label, text string }{{labelNow, now}, {label, s.fit(path, s.size.X-column-ui.Inset)}, {labelAt, at}}
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
// "ui:step", or "state:..." (see hasState).
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout()
	kind, value, _ := strings.Cut(name, ":")
	switch kind {
	case "ui":
		for _, button := range []*ui.Button{s.pause, s.step} {
			if button.ID == value {
				return ui.Center(button.Rect), true
			}
		}
	case "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts: "previewing" while a preview runs,
// "watching:<name>" while the named NPC is watched, "live:<kind>" while it is
// on a node of that kind, "paused" while its NPCs are paused, and "stepped"
// while they are paused after a step that changed node.
func (s *State) hasState(state string, docs frame.Docs) bool {
	entity, watching := s.watched()
	switch {
	case state == "previewing":
		return s.state != nil
	case state == "paused":
		return s.state != nil && s.state.NPCsPaused
	case state == "stepped":
		return s.state != nil && s.state.NPCsPaused && len(s.state.Step) > 0
	case strings.HasPrefix(state, "watching:"):
		return watching && s.name(docs, entity.NPC, "") == strings.TrimPrefix(state, "watching:")
	case strings.HasPrefix(state, "live:"):
		return watching && s.nodeKind(docs, entity) == strings.TrimPrefix(state, "live:")
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
