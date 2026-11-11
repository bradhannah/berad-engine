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

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/editor/views/showdraw"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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
	// stepLimit is how many ticks a step runs before it gives up on the
	// watched NPC changing node: a day of them. With the clock stopped,
	// nothing that waits on it can end, and a step gives up after
	// stoppedStepLimit, ten minutes of them, room for a walk to finish.
	stepLimit        = 24 * 60 * stepBudget
	stoppedStepLimit = 10 * stepBudget
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

// refreshLoop keeps the zone loop's copy of its workflow current, as
// refreshWorkflow keeps an NPC's: an edit restarts it. A zone with no loop,
// or with one that is no zone loop or runs in another zone, runs none.
// Documents are read one at a time.
func (s *State) refreshLoop(ctx *frame.Context) {
	run := s.preview.loop
	loopID := ids.ULID{}
	ctx.Docs.View(s.zone, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			loopID = zoneDoc.Zone.Loop
		}
	})
	if loopID.IsZero() || s.names.Name(loopID, "") == "" || !s.takeLoop(ctx, loopID) {
		run.workflow, run.source = nil, ids.ULID{}
	}
}

// takeLoop keeps the zone loop's copy of a loop current, reporting whether
// the loop fits the zone: a zone loop running there or in any zone.
func (s *State) takeLoop(ctx *frame.Context, loopID ids.ULID) bool {
	run, fits := s.preview.loop, false
	s.subscribe(loopID, ctx)
	ctx.Docs.View(loopID, func(document client.Document) {
		workflowDoc, ok := document.Model.(*workflowplugin.Document)
		if !ok {
			return
		}
		loop := workflowDoc.Workflow
		fits = loop.Purpose() == flow.PurposeZoneLoop && (loop.RunsIn().IsZero() || loop.RunsIn() == s.zone)
		if !fits || (loopID == run.source && document.Revision == run.revision && run.workflow != nil) {
			return
		}
		restart := run.workflow != nil
		run.workflow, run.source, run.revision = loop.Clone(), loopID, document.Revision
		if restart {
			run.npc.Restart()
		}
		if cues := cuesOf(run.workflow); len(cues) > 0 {
			ctx.Sounds.Prepare(cues)
		}
	})

	return fits
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
	s.refreshLoop(ctx)
	s.refreshModules(ctx, append(slices.Clone(runners), s.preview.loop))
	s.nameAnchors(ctx.Docs)
	zone := docsZone{floors: s.floors, docs: ctx.Docs}
	s.declareValues(ctx.Docs)
	ticks := 1
	if s.preview.stepping {
		ticks = stepBudget
	}
	var asked []npcrun.Sound
	s.preview.named, _ = signals.Declared(ctx.Docs)
	base := npcrun.World{
		Names: places.NewNames(ctx.Docs), Zone: zone, Workflow: nil, Behaviour: ids.ULID{}, Workflows: s.module,
		Clock: s.preview.clock, Passed: 0, State: s.preview.values, Here: s.zone, Others: nil,
		Stage: s.preview.stage, CueTicks: s.cueTicks, Signals: s.preview.named,
	}
	for range ticks {
		asked = s.stepTick(runners, base)
		if s.preview.stepping && s.stepped() {
			s.hear(ctx, asked)

			return
		}
	}
	if !s.preview.stepping {
		s.hear(ctx, asked)

		return
	}
	s.preview.stepTicks += ticks
	s.giveUpStep(ctx)
}

// giveUpStep ends a step that has run its limit of ticks without the
// watched NPC changing node, pauses the NPCs, and says why it gave up.
func (s *State) giveUpStep(ctx *frame.Context) {
	stopped := s.preview.clock.Stopped()
	if s.preview.stepTicks < stepLimit && (!stopped || s.preview.stepTicks < stoppedStepLimit) {
		return
	}
	s.preview.paused, s.preview.stepping = true, false
	if stopped {
		ctx.Frame.Status("Step gave up: the clock is stopped, so what waits on it never ends. Start the clock, then step.")

		return
	}
	ctx.Frame.Status("Step gave up: the watched NPC stayed on the same node for a day of ticks.")
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
	watched, ok := s.watchedRun()
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
		s.startStep(ctx)
	}
}

// startStep steps the watched NPC to its next change of node. One with no
// behaviour has no node to change, so it is not stepped, and the status says
// why.
func (s *State) startStep(ctx *frame.Context) {
	watched, ok := s.watchedRun()
	if !ok {
		return
	}
	if watched.workflow == nil {
		ctx.Frame.Status("Cannot step: the watched NPC has no behaviour to follow.")

		return
	}
	s.preview.paused, s.preview.stepping, s.preview.step, s.preview.stepTicks = false, true, nil, 0
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
		Lane: change.Lane,
	}
}

// stepTick runs one tick of every NPC and the zone's loop, the signals sent
// on the latest tick landing first, and returns the sounds they asked for.
func (s *State) stepTick(runners []*npcRun, base npcrun.World) []npcrun.Sound {
	base.Passed = s.preview.clock.Advance()
	s.preview.stage.Tick()
	base.Others = othersOf(runners)
	npcrun.Deliver(s.preview.signals, s.signalled(runners), s.zone)
	s.preview.signals = s.preview.signals[:0]
	var asked []npcrun.Sound
	step := func(run *npcRun, writer string) {
		world := base
		world.Workflow, world.Behaviour = run.workflow, run.source
		run.npc.Step(world, s.preview.run.Tick())
		s.preview.signals = append(s.preview.signals, run.npc.Sent...)
		asked = append(asked, run.npc.Sounds...)
		s.keepLines(run.npc, writer)
	}
	for _, run := range runners {
		step(run, run.npc.Placement.String())
	}
	if loop := s.preview.loop; loop.workflow != nil {
		step(loop, s.zone.String())
	}

	return asked
}

// signalEntity adds what an NPC's signals are doing to its entity: the one
// it is handling, from whom, what it will resume, and those held.
func (s *State) signalEntity(entity *protocol.PreviewEntity, run *npcRun) {
	if heard, handling := run.npc.Handling(); handling {
		entity.Handling = s.signalName(heard.Signal)
		if !heard.From.IsZero() {
			entity.HandlingFrom = s.names.Name(heard.From, "")
		}
		_, entity.Resumes = run.npc.Resumes(run.workflow, s.module)
	}
	for _, held := range run.npc.Held() {
		entity.Held = append(entity.Held, s.signalName(held.Signal))
	}
}

// signalName names a signal by its path in the world state, as last read.
func (s *State) signalName(signal ids.ULID) string {
	if path := worldstate.SignalPath(s.preview.named, signal); path != "" {
		return path
	}

	return signal.String()
}

// signalled lists the runners a signal can reach: the NPCs, then the
// zone's loop when it has one.
func (s *State) signalled(runners []*npcRun) []*npcrun.NPC {
	heard := make([]*npcrun.NPC, 0, len(runners)+1)
	for _, run := range runners {
		heard = append(heard, run.npc)
	}
	if loop := s.preview.loop; loop.workflow != nil {
		heard = append(heard, loop.npc)
	}

	return heard
}

// previewLanes are a Together's lanes as the preview's state carries them.
func previewLanes(lanes []npcrun.Lane) []protocol.PreviewLane {
	if len(lanes) == 0 {
		return nil
	}
	carried := make([]protocol.PreviewLane, 0, len(lanes))
	for _, lane := range lanes {
		carried = append(carried, protocol.PreviewLane{
			Port: lane.Port, Label: lane.Label, Node: lane.Node, Doing: lane.Doing, Done: lane.Done,
		})
	}

	return carried
}

// npcEntity is an NPC as the preview's state carries it.
func (s *State) npcEntity(run *npcRun) protocol.PreviewEntity {
	across, down := run.npc.At()

	workflow := run.source
	if module := run.npc.Module(); !module.IsZero() {
		workflow = module
	}
	callers := run.npc.Callers()
	calls := make([]protocol.PreviewCall, 0, len(callers))
	for _, call := range callers {
		caller := call.Workflow
		if caller.IsZero() {
			caller = run.source
		}
		calls = append(calls, protocol.PreviewCall{Workflow: caller, Call: call.Call})
	}

	if len(calls) == 0 {
		calls = nil
	}
	climbs := run.npc.Way()
	way := make([]protocol.PreviewClimb, 0, len(climbs))
	for _, climb := range climbs {
		way = append(way, protocol.PreviewClimb{
			From: climb.From, Col: climb.At.X, Row: climb.At.Y, To: climb.To,
			Up: indexOf(s.floors, climb.To) > indexOf(s.floors, climb.From),
		})
	}

	entity := protocol.PreviewEntity{
		ID: run.npc.Placement.String(), Floor: run.npc.Floor, X: across, Y: down,
		Face: [2]int{run.npc.Facing.X, run.npc.Facing.Y}, NPC: run.npc.NPC, Workflow: workflow,
		Node: run.npc.Node, Activity: run.npc.Activity(s.running(run)), Status: run.npc.Status,
		Last: previewChange(run.npc.Last), Changes: run.npc.Changes, Calls: calls, Way: way, Loop: ids.ULID{},
		Sprite: s.spriteNow(run.npc.NPC), Saying: "", SayingLeft: 0, Lanes: previewLanes(run.npc.Lanes(s.running(run))),
		Handling: "", HandlingFrom: "", Resumes: "", Held: nil,
	}
	s.signalEntity(&entity, run)
	if line, saying := run.npc.Saying(); saying {
		entity.Saying, entity.SayingLeft = line.Text, float64(line.Left)/float64(line.Ticks)
	}

	return entity
}

// spriteNow is the sprite an NPC shows in the preview now: its sprite value,
// which starts as the sprite in its details and its workflows can change.
func (s *State) spriteNow(npcID ids.ULID) ids.ULID {
	values := s.preview.values
	declared, known := values.Catalog().Lookup(worldstate.ScopeNPC, npcID, worldstate.SpriteID())
	if npcID.IsZero() || !known {
		return ids.ULID{}
	}

	return values.Get(declared, npcID).Ref
}

// loopEntity is the zone's loop as the preview's state carries it: an
// entity with no NPC, no floor and the zone's ID.
func (s *State) loopEntity() protocol.PreviewEntity {
	entity := s.npcEntity(s.preview.loop)
	entity.ID, entity.Loop = s.zone.String(), s.zone

	return entity
}

// watchedRun is the runner being watched: an NPC's, or the zone loop's,
// whose ID is the zone's.
func (s *State) watchedRun() (*npcRun, bool) {
	if run, ok := s.preview.npcs[s.preview.watched]; ok {
		return run, true
	}
	if loop := s.preview.loop; s.preview.watched == s.zone && loop.workflow != nil {
		return loop, true
	}

	return nil, false
}

// followWatched shows the floor the watched NPC is on when it takes
// stairs, as the map follows the player when they do.
func (s *State) followWatched(ctx *frame.Context) {
	watched := s.watched()
	for _, entity := range s.liveNPCs() {
		if entity.ID != watched.String() || watched.IsZero() {
			continue
		}
		moved := !s.watchedFloor.IsZero() && entity.Floor != s.watchedFloor
		s.watchedFloor = entity.Floor
		if moved && entity.Floor != s.floor && indexOf(s.floors, entity.Floor) >= 0 {
			s.switchFloor(entity.Floor, ctx)
			ctx.Frame.Status(s.names.Name(entity.NPC, "The NPC") + " took the stairs to " +
				s.floorName(ctx.Docs, entity.Floor))
		}

		return
	}
	s.watchedFloor = ids.ULID{}
}

// previewState is this window's preview as its state carries it.
func (s *State) previewState() protocol.PreviewState {
	runners := s.runners()
	entities := make([]protocol.PreviewEntity, 0, 1+len(runners))
	entities = append(entities, entityOf(s.preview.run.Player()))
	for _, run := range runners {
		entities = append(entities, s.npcEntity(run))
	}
	if s.preview.loop.workflow != nil {
		entities = append(entities, s.loopEntity())
	}
	game := s.preview.clock
	state := protocol.PreviewState{
		Run: s.preview.id, Tick: s.preview.run.Tick(), Entities: entities, Clock: game.Seconds(), Rate: int(game.Rate()),
		ClockStopped: game.Stopped(), NPCsPaused: s.preview.paused, Watched: s.preview.watched,
		Stepping: s.preview.stepping, Step: slices.Clone(s.preview.step), Values: s.watchedValues(),
		Log: slices.Clone(s.preview.lines), Box: showdraw.BoxOf(s.preview.stage), ShakeX: 0, ShakeY: 0,
	}
	state.ShakeX, state.ShakeY = s.preview.stage.Offset()

	return state
}

// keepLines numbers the lines an NPC's or the zone loop's Logs wrote this
// tick, at the game time now, keeping the latest protocol.RecentLines.
func (s *State) keepLines(runner *npcrun.NPC, entity string) {
	for _, line := range runner.Lines {
		s.preview.written++
		s.preview.lines = append(s.preview.lines, protocol.PreviewLine{
			Seq: s.preview.written, Clock: s.preview.clock.Seconds(), Entity: entity, NPC: runner.NPC, Loop: runner.Loop,
			Workflow: line.Workflow, Node: line.Node, Text: line.Text,
		})
	}
	if over := len(s.preview.lines) - protocol.RecentLines; over > 0 {
		s.preview.lines = slices.Delete(s.preview.lines, 0, over)
	}
}

// othersOf lists the NPCs as a tick begins, in placement order, for each to
// find the others.
func othersOf(runners []*npcRun) []npcrun.Other {
	others := make([]npcrun.Other, 0, len(runners))
	for _, run := range runners {
		others = append(others, npcrun.Other{
			Placement: run.npc.Placement, NPC: run.npc.NPC, Floor: run.npc.Floor, Cell: run.npc.Cell,
		})
	}

	return others
}

// declareValues gives the preview's values the project's declarations as
// they are now, keeping what has been set: the world state's, and each NPC's
// and zone's own and the templates applied to it.
func (s *State) declareValues(docs frame.Docs) {
	var revisions strings.Builder
	for _, document := range valuetree.Documents(docs) {
		docs.View(document, func(held client.Document) {
			revisions.WriteString(document.String() + ":" + strconv.FormatUint(held.Revision, 10) + ";")
		})
	}
	if revisions.String() == s.preview.declared {
		return
	}
	if catalog, ready := valuetree.Catalog(docs); ready {
		s.preview.values.Declare(catalog)
		s.preview.declared = revisions.String()
	}
}

// watchedValues lists the watched NPC's values: its workflow's locals and
// its own, then its zone's and the world's. A local holding an NPC shows
// that NPC's name.
func (s *State) watchedValues() []protocol.PreviewValue {
	run, ok := s.watchedRun()
	if !ok {
		return nil
	}
	var shown []protocol.PreviewValue
	if running := s.running(run); running != nil {
		locals := run.npc.Locals()
		for _, declared := range running.Locals().Values() {
			value, set := locals[declared.ID]
			if !set || !declared.Fits(value) {
				value = declared.Zero()
			}
			shown = append(shown, protocol.PreviewValue{
				Path: string(worldstate.ScopeLocal) + "." + declared.Name, Value: s.valueLabel(declared, value),
			})
		}
	}
	values := s.preview.values
	for _, part := range []struct {
		scope worldstate.Scope
		owner ids.ULID
	}{{worldstate.ScopeNPC, run.npc.NPC}, {worldstate.ScopeZone, s.zone}, {worldstate.ScopeWorld, ids.ULID{}}} {
		for _, held := range values.Catalog().Held(part.scope, part.owner) {
			label := s.valueLabel(held.Declaration, values.Get(held.Declaration, part.owner))
			shown = append(shown, protocol.PreviewValue{Path: held.Path, Value: label})
		}
	}

	return shown
}

// valueLabel is a value as Watch shows it: what it refers to by its name,
// an NPC, a place, a zone, a workflow or a sprite, where it has one, and
// otherwise as its type writes it.
func (s *State) valueLabel(declared worldstate.Declaration, value worldstate.Value) string {
	label := declared.Label(value)
	switch {
	case declared.Type == worldstate.TypeNPC && !value.NPC.IsZero():
		return s.names.Name(value.NPC, label)
	case declared.Type == worldstate.TypePlace:
		if name, named := s.preview.anchors[value.Place.Anchor]; named {
			return name
		}
	case declared.Type.IsRef() && !value.Ref.IsZero():
		return s.names.Name(value.Ref, label)
	}

	return label
}

// liveNPCs lists the NPCs of the preview this window shows, its own or
// another window's.
func (s *State) liveNPCs() []protocol.PreviewEntity {
	switch {
	case s.preview != nil:
		var npcs []protocol.PreviewEntity
		for _, entity := range s.previewState().Entities[1:] {
			if entity.Loop.IsZero() {
				npcs = append(npcs, entity)
			}
		}

		return npcs
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
// floor, as their sprites or dark tokens with their initials; the watched one
// has a blue ring.
func (s *State) drawLiveNPCs(area *ebiten.Image, grid image.Rectangle) {
	watched := s.watched()
	for _, entity := range s.liveNPCs() {
		if entity.Floor != s.floor {
			continue
		}
		s.drawNPC(area, s.entityRect(grid, entity.X, entity.Y), entity.NPC, entity.ID == watched.String())
	}
}

// entityRect is the cell-sized square an entity between cells fills, at a
// position in cells.
func (s *State) entityRect(grid image.Rectangle, x, y float64) image.Rectangle {
	size := float64(s.camera.cellSize())
	origin := grid.Min.Add(s.camera.offset)
	left := int(math.Round(float64(origin.X) + x*size))
	top := int(math.Round(float64(origin.Y) + y*size))

	return image.Rect(left, top, left+int(size), top+int(size))
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
		"Starts at " + clock.Format(s.start) + ",", "set on the window bar's clock.",
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
	if found, handled := s.hasShownState(state); handled {
		return found
	}
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
