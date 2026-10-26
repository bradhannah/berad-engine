package mapview

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"image"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
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
	// watchView is the Watch window's view name, and clockView the Clock
	// window's.
	watchView = "watch"
	clockView = "clock"
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

// startClock starts the preview's clock at the time set in the Clock
// window.
func (s *State) startClock() *clock.Clock {
	return clock.New(s.start, clock.MinutePerSecond)
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

// locatePreviewWidget finds the preview's buttons.
func (s *State) locatePreviewWidget(widget string) (image.Point, bool) {
	for _, button := range []*ui.Button{s.clockBn, s.watchBn} {
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
		if cues := cuesOf(run.workflow); len(cues) > 0 {
			ctx.Sounds.Prepare(cues)
		}
	})
}

// cuesOf lists the cues a workflow's Play sounds play, so they can be
// fetched before an NPC reaches them.
func cuesOf(workflow *flow.Workflow) []string {
	var cues []string
	for _, node := range workflow.Nodes() {
		if cue, err := node.Cue("cue"); node.Kind == flow.KindPlaySound && err == nil && cue != "" {
			cues = append(cues, cue)
		}
	}

	return cues
}

// stepNPCs runs every NPC for one tick, unless they are paused. During a
// step it runs them as fast as it can, up to stepBudget ticks a frame, until
// the watched NPC changes node, then pauses them. The cues the NPCs' Play
// sounds asked for in the tick they ran, or the tick a step ended in, play;
// those of the ticks a step hurries through do not.
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
	var asked []npcrun.Sound
	for range ticks {
		passed := s.preview.clock.Advance()
		asked = asked[:0]
		for _, run := range runners {
			world := npcrun.World{Zone: zone, Workflow: run.workflow, Clock: s.preview.clock, Passed: passed}
			run.npc.Step(world, s.preview.run.Tick())
			asked = append(asked, run.npc.Sounds...)
		}
		if s.preview.stepping && s.stepped() {
			s.hear(ctx, asked)

			return
		}
	}
	if !s.preview.stepping {
		s.hear(ctx, asked)
	}
}

// hear plays the cues NPCs asked for in one tick, each once however many
// NPCs asked for it, at the loudness the first asked for.
func (s *State) hear(ctx *frame.Context, sounds []npcrun.Sound) {
	played := map[string]bool{}
	for _, sound := range sounds {
		if played[sound.Cue] {
			continue
		}
		played[sound.Cue] = true
		s.preview.heard[sound.Cue]++
		if sound.Loudness != "" {
			ctx.Sounds.PlayAt(sound.Cue, sound.Loudness)
		} else {
			ctx.Sounds.Play(sound.Cue)
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

// control carries out a preview control from another window, or this one:
// the Watch window's controls for the NPCs, and the Clock window's for the
// clock. A jump starts every NPC's workflow again.
func (s *State) control(control protocol.PreviewControl, ctx *frame.Context) {
	if s.preview == nil || control.Run != s.preview.id {
		return
	}
	switch control.Action {
	case protocol.ControlRate:
		s.preview.clock.SetRate(clock.Rate(control.Rate))
	case protocol.ControlJump:
		s.preview.clock.Jump(control.Seconds)
		ctx.Frame.Status("Jumped to " + clock.Format(control.Seconds) + ". NPCs start their workflows again.")
	case protocol.ControlStopClock, protocol.ControlStartClock:
		s.preview.clock.Stop(control.Action == protocol.ControlStopClock)
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
	control := protocol.PreviewControl{
		Run: ids.ULID{}, Action: protocol.ControlWatch, Placement: placement, Rate: 0, Seconds: 0,
	}
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

// layoutPreview places the Clock button, and Watch while a preview runs, at
// the foot of the inspector.
func (s *State) layoutPreview(left, width, foot int) {
	top := foot - ui.ButtonH
	half := (width - ui.Gap) / ui.Half
	s.clockBn.Rect = image.Rect(left, top, left+half, foot)
	s.watchBn.Rect = ui.Zero()
	if s.preview != nil || s.remote != nil {
		s.watchBn.Rect = image.Rect(left+half+ui.Gap, top, left+width, foot)
	}
}

// updatePreviewControls handles Clock and Watch.
func (s *State) updatePreviewControls(input *ui.Input, ctx *frame.Context) {
	if s.clockBn.Update(input) {
		ctx.Requests.Open(clockView, project.IndexID(), false)
	}
	if s.watchBn.Update(input) {
		ctx.Requests.Open(watchView, s.zone, false)
	}
}

// labelPreview heads the preview's part of the inspector.
const labelPreview = "PREVIEW"

// previewGroup is one headed group of lines in the preview's part of the
// inspector.
type previewGroup struct {
	label string
	lines []string
}

// previewGroups describe the preview: when the next one starts; where the
// player starts, while choosing; and while this window or another view of
// the zone runs one, the player, the clock and the watched NPC.
func (s *State) previewGroups(docs frame.Docs) []previewGroup {
	switch {
	case s.choosing || s.starting:
		return []previewGroup{{label: labelPreview, lines: []string{
			"Click the cell where the player starts.", "Escape cancels.",
		}}}
	case s.preview != nil:
		player := s.preview.run.Player()
		game := s.preview.clock
		state := clock.Format(game.Seconds()) + " · " + clock.RateLabel(game.Rate())
		if game.Stopped() {
			state += " · stopped"
		}
		if s.preview.paused {
			state += " · NPCs paused"
		}

		return []previewGroup{
			{label: labelPreview, lines: []string{
				fmt.Sprintf("Player on %s, cell %d, %d", s.floorName(docs, player.Floor), player.Cell.X, player.Cell.Y),
				"Arrows walk · click an NPC · P stops",
			}},
			{label: "CLOCK", lines: []string{state}},
			{label: "WATCHING", lines: []string{s.watchedLine()}},
		}
	case s.remote != nil:
		return []previewGroup{
			{label: labelPreview, lines: []string{"Running in another view of this zone."}},
			{label: "CLOCK", lines: []string{strings.TrimPrefix(s.remoteClock(), "Clock ")}},
		}
	}

	return []previewGroup{{label: labelPreview, lines: []string{
		"Starts at " + clock.Format(s.start) + ",", "set in the Clock window.",
	}}}
}

// drawPreview draws the preview's part of the inspector above its buttons.
func (s *State) drawPreview(screen *ebiten.Image, docs frame.Docs, left, foot int) {
	groups := s.previewGroups(docs)
	ink := ui.ColorMuted()
	if s.preview != nil || s.remote != nil {
		ink = ui.ColorLive()
	}
	height := (len(groups) - 1) * ui.Gap
	for _, group := range groups {
		height += (len(group.lines) + 1) * ui.LineHeight
	}
	top := foot - ui.ButtonH - ui.Inset - height
	for _, group := range groups {
		s.theme.DrawLabel(screen, group.label, left, top, ink)
		top += ui.LineHeight
		for _, line := range group.lines {
			s.theme.DrawText(screen, line, left, top, ui.ColorInk())
			top += ui.LineHeight
		}
		top += ui.Gap
	}
	s.clockBn.Draw(screen, s.theme)
	s.watchBn.Draw(screen, s.theme)
}

// clockShows reports whether this window's preview clock reads a time that
// starts with prefix, such as "09:" for any minute of nine o'clock.
func (s *State) clockShows(prefix string) bool {
	return s.preview != nil && strings.HasPrefix(clock.Format(s.preview.clock.Seconds()), prefix)
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
// "watching" while an NPC is watched, "npc:<name>=C,R" when the named NPC
// stands on that cell, and "heard:<cue>" once the preview has played a cue.
func (s *State) hasNPCState(state string, docs frame.Docs) bool {
	switch {
	case strings.HasPrefix(state, "heard:"):
		return s.preview != nil && s.preview.heard[strings.TrimPrefix(state, "heard:")] > 0
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
