package mapview

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Preview clock and NPCs.
const (
	// watchView is the Watch window's view name.
	watchView = "watch"
	// defaultStart is where the start-time field begins: 08:00.
	defaultStart = "08:00"
	secondsPerHr = 3600
	secondsPerMn = 60
	clockDigits  = 4
	hoursPerDay  = 24
	// previewRows is how many lines of text the clock and watch part of the
	// inspector has while a preview runs, besides its rows of buttons.
	previewRows    = 4
	previewButtons = 3
	idleRows       = 2
	// jumpWidth is the width of the Jump button beside its field.
	jumpWidth = 52
	// clockFieldBytes bounds what can be typed into a time field.
	clockFieldBytes = 8
	// stepBudget is how many ticks a step runs in one frame: a minute of
	// real time, so most steps end in the frame they start.
	stepBudget = 60 * clock.TicksPerSecond
)

// npcRun is one NPC in this window's preview: its runner, and a copy of its
// workflow at the revision it last saw, so an edit restarts it.
type npcRun struct {
	npc      *npcrun.NPC
	workflow *flow.Workflow
	source   ids.ULID
	revision uint64
}

// rates are the clock rates the preview offers, in button order.
func rates() []clock.Rate {
	return []clock.Rate{clock.Real, clock.MinutePerSecond, clock.TenMinutesPerSecond}
}

// newPreviewButtons makes the preview's clock and watch controls.
func newPreviewButtons() ([]*ui.Button, *ui.Button, *ui.Button) {
	rateButtons := make([]*ui.Button, 0, len(rates()))
	for _, rate := range rates() {
		rateButtons = append(rateButtons, ui.NewButton("rate-"+strconv.Itoa(int(rate)), clock.RateLabel(rate)))
	}

	return rateButtons, ui.NewButton("stop-clock", "Stop clock"), ui.NewButton("watch", "Watch")
}

// parseClock reads "8", "8:30", "08:30" or "0830" as seconds from midnight.
func parseClock(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	hours, minutes, found := strings.Cut(value, ":")
	if !found {
		hours, minutes = value, "0"
		if len(value) == clockDigits {
			hours, minutes = value[:2], value[2:]
		}
	}
	hour, hourErr := strconv.Atoi(hours)
	minute, minuteErr := strconv.Atoi(minutes)
	if hourErr != nil || minuteErr != nil || hour < 0 || hour >= hoursPerDay || minute < 0 || minute >= secondsPerMn {
		return 0, false
	}

	return float64(hour*secondsPerHr + minute*secondsPerMn), true
}

// startClock starts the preview's clock at the time in the start field.
func (s *State) startClock() *clock.Clock {
	seconds, ok := parseClock(s.startField.Field.Value())
	if !ok {
		seconds, _ = parseClock(defaultStart)
	}

	return clock.New(seconds, clock.MinutePerSecond)
}

// seed is an NPC's random seed: the run and its placement, so a run repeats
// and NPCs differ.
func seed(run, placement ids.ULID) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write(run[:])
	_, _ = hash.Write(placement[:])

	return hash.Sum64()
}

// watchClicked watches the NPC clicked during a preview, and reports whether
// a click went to one.
func (s *State) watchClicked(input *ui.Input, ctx *frame.Context, cell Cell, onFloor bool) bool {
	if (s.preview == nil && s.remote == nil) || !input.Pressed || !onFloor {
		return false
	}
	placement, onNPC := s.liveNPCAt(cell)
	if onNPC {
		s.watch(placement, ctx)
	}

	return onNPC
}

// locatePreviewWidget finds the preview's fields and buttons.
func (s *State) locatePreviewWidget(widget string) (image.Point, bool) {
	for _, field := range []*ui.TextField{s.startField, s.jumpField} {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	for _, button := range append(slices.Clone(s.rateBns), s.stopBn, s.jumpBn, s.watchBn) {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// syncRunners keeps a runner for every placement on the zone's floors:
// placements added during the preview start running, and removed ones stop.
// Floors are read one at a time.
func (s *State) syncRunners(docs frame.Docs) {
	present := map[ids.ULID]bool{}
	for _, floorID := range s.floors {
		docs.View(floorID, func(document client.Document) {
			floorDoc, ok := document.Model.(*tilefloor.Document)
			if !ok {
				return
			}
			for _, placement := range floorDoc.Floor.Placements() {
				present[placement.ID] = true
				if _, running := s.preview.npcs[placement.ID]; !running {
					s.preview.npcs[placement.ID] = &npcRun{
						npc: npcrun.Start(placement, floorID, seed(s.preview.id, placement.ID)), workflow: nil,
						source: ids.ULID{}, revision: 0,
					}
				}
			}
		})
	}
	for placement := range s.preview.npcs {
		if !present[placement] {
			delete(s.preview.npcs, placement)
		}
	}
}

// runners lists the preview's NPCs in placement order, so they step the same
// way every run.
func (s *State) runners() []*npcRun {
	list := make([]*npcRun, 0, len(s.preview.npcs))
	for _, run := range s.preview.npcs {
		list = append(list, run)
	}
	slices.SortFunc(list, func(a, b *npcRun) int { return a.npc.Placement.Compare(b.npc.Placement) })

	return list
}

// refreshWorkflow keeps an NPC's copy of its behaviour current: a changed
// behaviour, or an edit to it, restarts the NPC. Documents are read one at a
// time.
func (s *State) refreshWorkflow(ctx *frame.Context, run *npcRun) {
	behaviour := ids.ULID{}
	if s.names.Name(run.npc.NPC, "") != "" {
		s.subscribe(run.npc.NPC, ctx)
	}
	ctx.Docs.View(run.npc.NPC, func(document client.Document) {
		if npcDoc, ok := document.Model.(*npcplugin.Document); ok {
			behaviour = npcDoc.NPC.Behaviour
		}
	})
	if behaviour.IsZero() || s.names.Name(behaviour, "") == "" {
		run.workflow, run.source = nil, ids.ULID{}

		return
	}
	s.subscribe(behaviour, ctx)
	ctx.Docs.View(behaviour, func(document client.Document) {
		workflowDoc, ok := document.Model.(*workflowplugin.Document)
		if !ok || (behaviour == run.source && document.Revision == run.revision && run.workflow != nil) {
			return
		}
		restart := run.workflow != nil
		run.workflow, run.source, run.revision = workflowDoc.Workflow.Clone(), behaviour, document.Revision
		if restart {
			run.npc.Restart()
		}
	})
}

// stepNPCs runs every NPC for one tick, unless they are paused. During a
// step it runs them as fast as it can, up to stepBudget ticks a frame, until
// the watched NPC changes node, then pauses them.
func (s *State) stepNPCs(ctx *frame.Context) {
	s.syncRunners(ctx.Docs)
	if s.preview.paused {
		return
	}
	runners := s.runners()
	for _, run := range runners {
		s.refreshWorkflow(ctx, run)
	}
	zone := docsZone{floors: s.floors, docs: ctx.Docs}
	ticks := 1
	if s.preview.stepping {
		ticks = stepBudget
	}
	for range ticks {
		passed := s.preview.clock.Advance()
		for _, run := range runners {
			world := npcrun.World{Zone: zone, Workflow: run.workflow, Clock: s.preview.clock, Passed: passed}
			run.npc.Step(world, s.preview.run.Tick())
		}
		if s.preview.stepping && s.stepped() {
			return
		}
	}
}

// stepped keeps the watched NPC's changes during a step, and pauses the NPCs
// once it has changed node, or has gone. It reports whether the step is over.
func (s *State) stepped() bool {
	watched, ok := s.preview.npcs[s.preview.watched]
	if ok && len(watched.npc.Taken) == 0 {
		return false
	}
	if ok {
		for _, change := range watched.npc.Taken {
			s.preview.step = append(s.preview.step, previewChange(change))
		}
	}
	s.preview.paused, s.preview.stepping = true, false

	return true
}

// control carries out a preview control from another window, or this one.
func (s *State) control(control protocol.PreviewControl, ctx *frame.Context) {
	if s.preview == nil || control.Run != s.preview.id {
		return
	}
	switch control.Action {
	case protocol.ControlWatch:
		s.preview.watched, s.preview.step = control.Placement, nil
	case protocol.ControlPause:
		s.preview.paused, s.preview.stepping = true, false
		ctx.Frame.Status("NPCs paused")
	case protocol.ControlResume:
		s.preview.paused, s.preview.stepping, s.preview.step = false, false, nil
		ctx.Frame.Status("NPCs running")
	case protocol.ControlStep:
		if _, ok := s.preview.npcs[s.preview.watched]; ok {
			s.preview.paused, s.preview.stepping, s.preview.step = false, true, nil
		}
	}
}

// watch watches the NPC at a placement: this window's own preview at once,
// another window's through it.
func (s *State) watch(placement ids.ULID, ctx *frame.Context) {
	control := protocol.PreviewControl{Run: ids.ULID{}, Action: protocol.ControlWatch, Placement: placement}
	switch {
	case s.preview != nil:
		control.Run = s.preview.id
		s.control(control, ctx)
	case s.remote != nil:
		control.Run = s.remote.id
		ctx.Requests.PreviewControl(control)
	default:
		return
	}
	ctx.Frame.Status("Watching " + s.names.Name(s.liveNPCOf(placement), "the NPC") + ". Watch shows its workflow.")
}

// liveNPCOf returns the NPC a live placement shows.
func (s *State) liveNPCOf(placement ids.ULID) ids.ULID {
	for _, entity := range s.liveNPCs() {
		if entity.ID == placement.String() {
			return entity.NPC
		}
	}

	return ids.ULID{}
}

// previewChange is a change of node as the preview's state carries it.
func previewChange(change npcrun.Change) protocol.PreviewChange {
	return protocol.PreviewChange{
		From: change.From, Output: change.Output, To: change.To, Tick: change.Tick, Status: change.Status,
	}
}

// npcEntity is an NPC as the preview's state carries it.
func npcEntity(run *npcRun) protocol.PreviewEntity {
	x, y := run.npc.At()

	return protocol.PreviewEntity{
		ID: run.npc.Placement.String(), Floor: run.npc.Floor, X: x, Y: y,
		Face: [2]int{run.npc.Facing.X, run.npc.Facing.Y}, NPC: run.npc.NPC, Workflow: run.source,
		Node: run.npc.Node, Activity: run.npc.Activity(run.workflow), Status: run.npc.Status,
		Last: previewChange(run.npc.Last), Changes: run.npc.Changes,
	}
}

// previewState is this window's preview as its state carries it.
func (s *State) previewState() protocol.PreviewState {
	runners := s.runners()
	entities := make([]protocol.PreviewEntity, 0, 1+len(runners))
	entities = append(entities, entityOf(s.preview.run.Player()))
	for _, run := range runners {
		entities = append(entities, npcEntity(run))
	}
	game := s.preview.clock

	return protocol.PreviewState{
		Run: s.preview.id, Tick: s.preview.run.Tick(), Entities: entities, Clock: game.Seconds(), Rate: int(game.Rate()),
		ClockStopped: game.Stopped(), NPCsPaused: s.preview.paused, Watched: s.preview.watched,
		Stepping: s.preview.stepping, Step: slices.Clone(s.preview.step),
	}
}

// liveNPCs lists the NPCs of the preview this window shows, its own or
// another window's.
func (s *State) liveNPCs() []protocol.PreviewEntity {
	switch {
	case s.preview != nil:
		state := s.previewState()

		return state.Entities[1:]
	case s.remote != nil && s.remote.state != nil:
		var npcs []protocol.PreviewEntity
		for _, entity := range s.remote.state.Entities {
			if !entity.NPC.IsZero() {
				npcs = append(npcs, entity)
			}
		}

		return npcs
	default:
		return nil
	}
}

// watched returns the placement of the NPC being watched in the preview this
// window shows.
func (s *State) watched() ids.ULID {
	switch {
	case s.preview != nil:
		return s.preview.watched
	case s.remote != nil && s.remote.state != nil:
		return s.remote.state.Watched
	default:
		return ids.ULID{}
	}
}

// liveNPCAt returns the NPC standing on, or stepping onto, a cell of the
// shown floor.
func (s *State) liveNPCAt(cell Cell) (ids.ULID, bool) {
	for _, entity := range s.liveNPCs() {
		if entity.Floor == s.floor && int(math.Round(entity.X)) == cell.X && int(math.Round(entity.Y)) == cell.Y {
			placement, err := ids.Parse(entity.ID)

			return placement, err == nil
		}
	}

	return ids.ULID{}, false
}

// drawLiveNPCs draws the preview's NPCs where they are now, on the shown
// floor, as dark tokens with their initials; the watched one has a blue ring.
func (s *State) drawLiveNPCs(area *ebiten.Image, grid image.Rectangle) {
	size := float64(s.camera.cellSize())
	origin := grid.Min.Add(s.camera.offset)
	watched := s.watched()
	for _, entity := range s.liveNPCs() {
		if entity.Floor != s.floor {
			continue
		}
		centreX := float32(float64(origin.X) + (entity.X+cellCentre)*size)
		centreY := float32(float64(origin.Y) + (entity.Y+cellCentre)*size)
		radius := max(float32(size)*tokenRadius, minTokenRadius)
		name := s.names.Name(entity.NPC, "Deleted NPC")
		vector.FillCircle(area, centreX, centreY, radius, ui.Hex(hexToken), true)
		if entity.ID == watched.String() {
			vector.StrokeCircle(area, centreX, centreY, radius+tokenRing, tokenRing, ui.ColorSelect(), true)
		}
		if int(size) >= minLabel {
			label := initials(name)
			width := s.theme.SmallWidth(label)
			s.theme.DrawSmall(area, label, int(centreX)-width/ui.Half, int(centreY)-ui.LabelSize/ui.Half-ui.Half,
				ui.ColorBackground())
		}
	}
}

// layoutPreview places the preview's controls above the keys: the start time
// before a preview, and the clock and Watch while this window runs one.
func (s *State) layoutPreview(left, width, foot int) {
	for _, button := range slices.Concat(s.rateBns, []*ui.Button{s.stopBn, s.jumpBn, s.watchBn}) {
		button.Rect = ui.Zero()
	}
	s.startField.Rect, s.jumpField.Rect = ui.Zero(), ui.Zero()
	switch {
	case s.preview != nil:
		rowY := clockTop(foot) + ui.Half*ui.LineHeight
		gaps := (len(s.rateBns) - 1) * ui.Gap
		for index, button := range s.rateBns {
			from := left + index*(width-gaps)/len(s.rateBns) + index*ui.Gap
			to := left + (index+1)*(width-gaps)/len(s.rateBns) + index*ui.Gap
			button.Rect = image.Rect(from, rowY, to, rowY+ui.ButtonH)
			button.On = s.preview.clock.Rate() == rates()[index]
		}
		rowY += ui.ButtonH + ui.Gap
		half := (width - ui.Gap) / ui.Half
		s.stopBn.Rect = image.Rect(left, rowY, left+half, rowY+ui.ButtonH)
		s.stopBn.Label = "Stop clock"
		if s.preview.clock.Stopped() {
			s.stopBn.Label = "Start clock"
		}
		s.jumpField.Rect = image.Rect(left+half+ui.Gap, rowY, left+width-jumpWidth-ui.Gap, rowY+ui.ButtonH)
		s.jumpBn.Rect = image.Rect(left+width-jumpWidth, rowY, left+width, rowY+ui.ButtonH)
		rowY += ui.ButtonH + ui.Gap + ui.Half*ui.LineHeight
		s.watchBn.Rect = image.Rect(left, rowY, left+width, rowY+ui.ButtonH)
	case s.remote == nil:
		top := foot - idleRows*ui.LineHeight - ui.ButtonH - ui.Gap
		s.startField.Rect = image.Rect(left+width/ui.Half, top+ui.LineHeight, left+width, top+ui.LineHeight+ui.ButtonH)
	}
}

// clockTop is where the clock and watch part of the inspector starts, a line
// above the keys.
func clockTop(foot int) int {
	return foot - (previewRows+1)*ui.LineHeight - previewButtons*ui.ButtonH - ui.Half*ui.Gap
}

// updatePreviewControls handles the clock's buttons and fields and Watch.
func (s *State) updatePreviewControls(input *ui.Input, ctx *frame.Context) {
	if s.preview == nil {
		return
	}
	for index, button := range s.rateBns {
		if button.Update(input) {
			s.preview.clock.SetRate(rates()[index])
		}
	}
	if s.stopBn.Update(input) {
		s.preview.clock.Stop(!s.preview.clock.Stopped())
	}
	if s.jumpBn.Update(input) {
		s.jumpClock(ctx)
	}
	if s.watchBn.Update(input) {
		ctx.Requests.Open(watchView, s.zone, false)
	}
}

// setStart keeps the time typed into the start field for the next preview,
// or says what is wrong with it.
func (s *State) setStart(ctx *frame.Context) {
	value, _ := s.startField.Field.Commit()
	seconds, ok := parseClock(value)
	if !ok {
		s.startField.Field.Failed()
		ctx.Frame.Status("Enter a time of day such as 08:00")

		return
	}
	s.startField.Field.SetSaved(clock.Format(seconds))
	s.focus.Set(nil)
	ctx.Frame.Status("The next preview starts at " + clock.Format(seconds))
}

// jumpClock jumps the preview's clock to the time typed into the jump field;
// every NPC starts its workflow again from where it stands.
func (s *State) jumpClock(ctx *frame.Context) {
	value, _ := s.jumpField.Field.Commit()
	seconds, ok := parseClock(value)
	if !ok || s.preview == nil {
		s.jumpField.Field.Failed()
		ctx.Frame.Status("Enter a time of day such as 12:30")

		return
	}
	s.preview.clock.Jump(seconds)
	s.jumpField.Field.SetSaved("")
	s.focus.Set(nil)
	ctx.Frame.Status("Jumped to " + clock.Format(seconds) + ". NPCs start their workflows again.")
}

// drawNPCPreview draws the preview's part of the inspector: before a
// preview, the start time; while one runs, the clock, its controls and the
// watched NPC.
func (s *State) drawNPCPreview(screen *ebiten.Image, left, foot int) {
	switch {
	case s.preview != nil:
		top := clockTop(foot)
		game := s.preview.clock
		s.theme.DrawLabel(screen, "CLOCK", left, top, ui.ColorLive())
		state := clock.Format(game.Seconds()) + " · " + clock.RateLabel(game.Rate())
		if game.Stopped() {
			state += " · stopped"
		}
		if s.preview.paused {
			state += " · NPCs paused"
		}
		s.theme.DrawMono(screen, state, left, top+ui.LineHeight, ui.ColorInk())
		for _, button := range append(slices.Clone(s.rateBns), s.stopBn, s.jumpBn) {
			button.Draw(screen, s.theme)
		}
		s.jumpField.Draw(screen)
		watchTop := s.jumpField.Rect.Max.Y + ui.Gap
		s.theme.DrawLabel(screen, "WATCHING", left, watchTop, ui.ColorLive())
		s.theme.DrawText(screen, s.watchedLine(), left, watchTop+ui.LineHeight, ui.ColorInk())
		s.watchBn.Draw(screen, s.theme)
	case s.remote == nil:
		top := foot - idleRows*ui.LineHeight - ui.ButtonH - ui.Gap
		s.theme.DrawLabel(screen, "PREVIEW", left, top, ui.ColorMuted())
		s.theme.DrawText(screen, "Clock starts at", left, top+ui.LineHeight+(ui.ButtonH-ui.TextSize)/ui.Half-1,
			ui.ColorInk())
		s.startField.Draw(screen)
	}
}

// remoteClock describes the clock of another window's preview.
func (s *State) remoteClock() string {
	if s.remote == nil || s.remote.state == nil {
		return ""
	}
	state := s.remote.state

	return "Clock " + clock.Format(state.Clock) + " · " + clock.RateLabel(clock.Rate(state.Rate))
}

// watchedLine names the watched NPC and what it is doing, or says how to
// choose one.
func (s *State) watchedLine() string {
	for _, entity := range s.liveNPCs() {
		if entity.ID == s.watched().String() {
			return s.names.Name(entity.NPC, "Deleted NPC") + ": " + cmp.Or(entity.Status, entity.Activity)
		}
	}
	if len(s.liveNPCs()) == 0 {
		return "No NPCs on this zone's floors."
	}

	return "Click an NPC to watch it."
}

// hasNPCState answers "npcs-moved" once an NPC has left its placement,
// "watching" while an NPC is watched, and "npc:<name>=C,R" when the named NPC
// stands on that cell.
func (s *State) hasNPCState(state string, docs frame.Docs) bool {
	switch {
	case state == "watching":
		return !s.watched().IsZero()
	case state == "npcs-moved":
		return s.npcsMoved(docs)
	case strings.HasPrefix(state, "npc:"):
		name, where, _ := strings.Cut(strings.TrimPrefix(state, "npc:"), "=")
		var col, row int
		if _, err := fmt.Sscanf(where, "%d,%d", &col, &row); err != nil {
			return false
		}
		for _, entity := range s.liveNPCs() {
			if s.names.Name(entity.NPC, "") == name && entity.X == float64(col) && entity.Y == float64(row) {
				return true
			}
		}
	}

	return false
}

// npcsMoved reports whether a live NPC stands away from its placement.
func (s *State) npcsMoved(docs frame.Docs) bool {
	placed := map[string]image.Point{}
	for _, floorID := range s.floors {
		docs.View(floorID, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				for _, placement := range floorDoc.Floor.Placements() {
					placed[placement.ID.String()] = image.Pt(placement.Col, placement.Row)
				}
			}
		})
	}
	for _, entity := range s.liveNPCs() {
		if at, ok := placed[entity.ID]; ok && (entity.X != float64(at.X) || entity.Y != float64(at.Y)) {
			return true
		}
	}

	return false
}

// locateLiveNPC finds a live NPC by name on screen, for input scripts.
func (s *State) locateLiveNPC(name string) (image.Point, bool) {
	size := float64(s.camera.cellSize())
	origin := s.Grid().Min.Add(s.camera.offset)
	for _, entity := range s.liveNPCs() {
		if entity.Floor == s.floor && s.names.Name(entity.NPC, "") == name {
			return image.Pt(origin.X+int(math.Round((entity.X+cellCentre)*size)),
				origin.Y+int(math.Round((entity.Y+cellCentre)*size))), true
		}
	}

	return image.Pt(0, 0), false
}
