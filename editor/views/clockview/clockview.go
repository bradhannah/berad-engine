// Package clockview is the Clock window: the preview's clock and its
// controls, kept on top of the other windows by default. Before a preview it
// shows and sets the time the next preview starts at; while one runs, in any
// zone, it shows the preview's time and rate and runs its clock: an hour back
// or on, a jump to a time of day, stopping and starting it, and its rate.
// Every control goes through the orchestrator to the Map window running the
// preview, and a jump starts every NPC's workflow again, as one from the
// Map window would.
package clockview

import (
	"encoding/json"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ViewName is the Clock window's view name; its target is the project index.
const ViewName = "clock"

// Width and Height are the Clock window's size when it opens, in screen
// pixels.
const (
	Width  = 320
	Height = 300
)

// Layout, in screen pixels.
const (
	hourWidth     = 56
	applyWidth    = 88
	displayHeight = 44
	fieldBytes    = 8
)

// The note under the controls.
const textNote = "A jump starts every NPC's workflow again."

// rates are the clock rates offered, in button order.
func rates() []clock.Rate {
	return []clock.Rate{clock.Real, clock.MinutePerSecond, clock.TenMinutesPerSecond}
}

// State is the Clock window.
type State struct {
	theme *ui.Theme
	size  image.Point
	subs  map[ids.ULID]bool
	// start is the time of day the next preview starts at.
	start float64
	// run is the preview running, its zone and its latest state.
	run   ids.ULID
	zone  ids.ULID
	tick  uint64
	state *protocol.PreviewState
	zones string
	back  *ui.Button
	on    *ui.Button
	stop  *ui.Button
	apply *ui.Button
	rate  []*ui.Button
	field *ui.TextField
	focus ui.Focus
}

// New returns the Clock window.
func New(theme *ui.Theme) *State {
	rateButtons := make([]*ui.Button, 0, len(rates()))
	for _, rate := range rates() {
		rateButtons = append(rateButtons, ui.NewButton("rate-"+strconv.Itoa(int(rate)), clock.RateLabel(rate)))
	}
	view := &State{
		theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{}, start: protocol.DefaultClockStart,
		run: ids.ULID{}, zone: ids.ULID{}, tick: 0, state: nil, zones: "",
		back: ui.NewButton("hour-back", "−1h"), on: ui.NewButton("hour-on", "+1h"),
		stop: ui.NewButton("stop-clock", "Stop clock"), apply: ui.NewButton("apply", "Set start"), rate: rateButtons,
		field: ui.NewTextField("time", textedit.NewField("", false, fieldBytes), false, theme), focus: ui.Focus{},
	}
	view.field.Placeholder = "17:00"
	view.focus.Register(view.field)

	return view
}

// Start subscribes to the project index, for the zone's name.
func (s *State) Start(ctx *frame.Context) {
	if s.subs[project.IndexID()] {
		return
	}
	s.subs[project.IndexID()] = true
	ctx.Requests.Subscribe(project.IndexID(), nil)
}

// Title is the window title.
func (s *State) Title(frame.Docs) string { return "Clock" }

// Focused returns the time field when it has focus.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// running reports whether a preview runs and has sent its state.
func (s *State) running() bool { return s.state != nil }

// Changed follows the preview of any zone, and the time the next preview
// starts at, which every window shares.
func (s *State) Changed(update client.Update, _ *frame.Context) {
	switch {
	case update.PreviewStart != nil:
		s.run, s.zone, s.tick, s.state = update.PreviewStart.Run, update.PreviewStart.Zone, 0, nil
	case update.PreviewState != nil && update.PreviewState.Run == s.run && update.PreviewState.Tick > s.tick:
		state := *update.PreviewState
		s.tick, s.state = state.Tick, &state
	case update.PreviewStop != nil && update.PreviewStop.Run == s.run:
		s.run, s.zone, s.tick, s.state = ids.ULID{}, ids.ULID{}, 0, nil
	case update.Session != nil && update.Session.Topic == protocol.TopicClockStart:
		var start protocol.ClockStart
		if err := json.Unmarshal(update.Session.Value, &start); err == nil {
			s.start = start.Seconds
		}
	}
}

// layout places the controls from the top of the window down.
func (s *State) layout() {
	left, width := ui.Inset, s.size.X-ui.Half*ui.Inset
	row := ui.Inset + ui.LineHeight + displayHeight + ui.LineHeight + ui.Gap
	s.back.Rect = image.Rect(left, row, left+hourWidth, row+ui.ButtonH)
	s.on.Rect = image.Rect(left+hourWidth+ui.Gap, row, left+ui.Half*hourWidth+ui.Gap, row+ui.ButtonH)
	s.stop.Rect = image.Rect(left+ui.Half*(hourWidth+ui.Gap), row, left+width, row+ui.ButtonH)
	row += ui.ButtonH + ui.Half*ui.Gap
	s.field.Rect = image.Rect(left, row, left+width-applyWidth-ui.Gap, row+ui.ButtonH)
	s.apply.Rect = image.Rect(left+width-applyWidth, row, left+width, row+ui.ButtonH)
	row += ui.ButtonH + ui.Half*ui.Gap + ui.LineHeight
	gaps := (len(s.rate) - 1) * ui.Gap
	for index, button := range s.rate {
		from := left + index*(width-gaps)/len(s.rate) + index*ui.Gap
		to := left + (index+1)*(width-gaps)/len(s.rate) + index*ui.Gap
		button.Rect = image.Rect(from, row, to, row+ui.ButtonH)
		button.On = s.running() && s.state.Rate == int(rates()[index])
		button.Disabled = !s.running()
	}
	s.apply.Label, s.stop.Label, s.stop.Disabled = "Set start", "Stop clock", !s.running()
	if s.running() {
		s.apply.Label = "Jump"
		if s.state.ClockStopped {
			s.stop.Label = "Start clock"
		}
	}
}

// shown is the time the window shows: the preview's, or the next start.
func (s *State) shown() float64 {
	if s.running() {
		return s.state.Clock
	}

	return s.start
}

// Update handles the controls.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.layout()
	s.names(ctx.Docs)
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field == s.field && outcome == textedit.OutcomeCommit {
		s.applyTime(ctx)
	}
	switch {
	case s.back.Update(input):
		s.setTime(ctx, s.shown()-clock.Hour)
	case s.on.Update(input):
		s.setTime(ctx, s.shown()+clock.Hour)
	case s.apply.Update(input):
		s.applyTime(ctx)
	case s.stop.Update(input) && s.running():
		action := protocol.ControlStopClock
		if s.state.ClockStopped {
			action = protocol.ControlStartClock
		}
		s.control(ctx, protocol.PreviewControl{Run: s.run, Action: action, Placement: ids.ULID{}, Rate: 0, Seconds: 0})
	}
	for index, button := range s.rate {
		if button.Update(input) && s.running() {
			s.control(ctx, protocol.PreviewControl{
				Run: s.run, Action: protocol.ControlRate, Placement: ids.ULID{}, Rate: int(rates()[index]), Seconds: 0,
			})
		}
	}
	frame.CommonKeys(input, ctx)
}

// names keeps the running preview's zone name.
func (s *State) names(docs frame.Docs) {
	s.zones = ""
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			s.zones = index.Name(s.zone, "")
		}
	})
}

// applyTime jumps the preview to the typed time, or sets the next start to
// it.
func (s *State) applyTime(ctx *frame.Context) {
	value, _ := s.field.Field.Commit()
	seconds, ok := clock.Parse(value)
	if !ok {
		s.field.Field.Failed()
		ctx.Frame.Status("Enter a time of day such as 17:00")

		return
	}
	s.field.Field.SetSaved("")
	s.focus.Set(nil)
	s.setTime(ctx, seconds)
}

// setTime jumps the preview's clock to a time of day, or sets the time the
// next preview starts at for every window.
func (s *State) setTime(ctx *frame.Context, seconds float64) {
	seconds = math.Mod(seconds+clock.Day, clock.Day)
	if s.running() {
		s.control(ctx, protocol.PreviewControl{
			Run: s.run, Action: protocol.ControlJump, Placement: ids.ULID{}, Rate: 0, Seconds: seconds,
		})
		ctx.Frame.Status("Jumped to " + clock.Format(seconds) + ". NPCs start their workflows again.")

		return
	}
	s.start = seconds
	ctx.Requests.Publish(protocol.TopicClockStart, protocol.ClockStart{Seconds: seconds})
	ctx.Frame.Status("The next preview starts at " + clock.Format(seconds))
}

func (s *State) control(ctx *frame.Context, control protocol.PreviewControl) {
	ctx.Requests.PreviewControl(control)
}

// Readout sums the clock up for a shell's window bar (increment 10): the
// time it shows, and while a preview runs, that it is live and at what rate.
// It reports whether a preview runs.
func (s *State) Readout() (string, bool) {
	if !s.running() {
		return clock.Format(s.start), false
	}
	if s.state.ClockStopped {
		return clock.Format(s.state.Clock) + " · live · stopped", true
	}

	return clock.Format(s.state.Clock) + " · live · " + clock.RateLabel(clock.Rate(s.state.Rate)), true
}

// stateLine says what the clock is showing.
func (s *State) stateLine() string {
	if !s.running() {
		return "No preview. The next starts at this time."
	}
	line := clock.RateLabel(clock.Rate(s.state.Rate))
	if s.zones != "" {
		line = "Preview of " + s.zones + " · " + line
	}
	if s.state.ClockStopped {
		line += " · stopped"
	}

	return line
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, _ frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout()
	screen.Fill(ui.ColorBackground())
	left, top := ui.Inset, ui.Inset
	label, ink := "NEXT PREVIEW STARTS", ui.ColorMuted()
	if s.running() {
		label, ink = "CLOCK", ui.ColorLive()
	}
	s.theme.DrawLabel(screen, label, left, top, ink)
	top += ui.LineHeight
	s.theme.DrawDisplay(screen, clock.Format(s.shown()), left, top, ui.ColorInk())
	top += displayHeight
	s.theme.DrawText(screen, s.stateLine(), left, top, ui.ColorMuted())
	for _, button := range []*ui.Button{s.back, s.on, s.stop, s.apply} {
		button.Draw(screen, s.theme)
	}
	s.field.Draw(screen)
	s.theme.DrawLabel(screen, "RATE", left, s.rate[0].Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
	for _, button := range s.rate {
		button.Draw(screen, s.theme)
	}
	s.theme.DrawText(screen, textNote, left, s.rate[0].Rect.Max.Y+ui.Half*ui.Gap, ui.ColorSubdued())
}

// Locate finds a control, "ui:<id>", or answers "state:running" while a
// preview runs and "state:time:09:12" while that time shows, for input
// scripts; "state:time:09:" matches any minute of the hour.
func (s *State) Locate(name string, _ frame.Docs) (image.Point, bool) {
	s.layout()
	kind, value, _ := strings.Cut(name, ":")
	switch kind {
	case "state":
		switch {
		case value == "running":
			return image.Pt(0, 0), s.running()
		case strings.HasPrefix(value, "time:"):
			return image.Pt(0, 0), strings.HasPrefix(clock.Format(s.shown()), strings.TrimPrefix(value, "time:"))
		}
	case "ui":
		if value == s.field.ID {
			return s.field.Center(), true
		}
		for _, button := range append([]*ui.Button{s.back, s.on, s.stop, s.apply}, s.rate...) {
			if button.ID == value {
				return ui.Center(button.Rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}
