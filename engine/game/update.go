package game

import (
	"errors"
	"fmt"
	"image"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/show"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Why an NPC starts its workflow again when the project changes while the
// game runs.
const (
	// RestartedByEdit is why an NPC starts again when its behaviour is
	// changed, or it is given another.
	RestartedByEdit = "Started again: its workflow was changed"
	// RestartedByCall is why an NPC starts again when a workflow it is
	// inside a call to is changed, or is gone.
	RestartedByCall = "Started again: a workflow it called was changed"
)

// Update gives a running game its project as it is now, gathered again
// after an edit, and applies what changed (increment 63). The game goes on:
// the player stays where they are, the clock keeps its time and the values
// what they hold.
//
//   - The zone's floors are the project's, so a floor painted is walked as
//     painted, and the zone's sounds are the project's.
//   - A placement added starts running, and one removed stops.
//   - An NPC whose behaviour changed, or that was given another, starts
//     again, saying RestartedByEdit; one whose behaviour is gone follows
//     none. One that only now has a behaviour starts it without saying so.
//   - An NPC inside a call to a workflow that changed, or is gone, starts
//     again, saying RestartedByCall.
//   - The zone's loop follows the same rules.
//   - The values are declared anew, each keeping what it held; the signals
//     and the names are the project's.
//
// The project is of the zone being played; one with no zone changes nothing
// of the zone's. Its player start is not used: the game has started.
func (g *Game) Update(project Project) {
	g.settings.Columns, g.settings.Rows = project.Settings.Columns, project.Settings.Rows
	g.names, g.signals = project.Names, project.Signals
	g.declare(project.Values)
	changed := g.changedWorkflows(project.Workflows)
	g.workflows = project.Workflows
	if project.Zone != nil {
		g.levels = levelsOf(project)
		g.sounds = project.Zone.Sounds()
		g.syncRunners()
	}
	g.follow(project)
	g.followLoop(project)
	for _, character := range g.runners() {
		if !character.Module().IsZero() && slices.ContainsFunc(changed, func(target ids.ULID) bool {
			return inside(character, target)
		}) {
			character.Restart(RestartedByCall)
		}
	}
}

// levelsOf are a project's zone's floors, from the bottom up.
func levelsOf(project Project) play.Levels {
	levels := make(play.Levels, 0, len(project.Zone.Floors()))
	for _, floorID := range project.Zone.Floors() {
		levels = append(levels, play.Level{ID: floorID, Floor: project.Floors[floorID]})
	}

	return levels
}

// declare declares the values anew, each keeping what it held.
func (g *Game) declare(catalog *worldstate.Catalog) {
	switch {
	case catalog == nil:
	case g.values == nil:
		g.values = worldstate.NewStore(catalog)
	default:
		g.values.Declare(catalog)
	}
}

// changedWorkflows lists the workflows the game holds that are not what
// they were: changed, or gone.
func (g *Game) changedWorkflows(now map[ids.ULID]*flow.Workflow) []ids.ULID {
	var changed []ids.ULID
	for workflowID, held := range g.workflows {
		if held == nil {
			continue
		}
		if current := now[workflowID]; current == nil || !held.Equal(current) {
			changed = append(changed, workflowID)
		}
	}

	return changed
}

// syncRunners keeps a runner for every placement on the zone's floors: a
// placement added starts running, and one removed stops. Those that start
// together start in the order their placements were made, which their IDs
// keep, each seeded by its place among all the zone has started.
func (g *Game) syncRunners() {
	type placed struct {
		placement tilefloor.Placement
		floor     ids.ULID
	}
	present := map[ids.ULID]bool{}
	var added []placed
	for _, level := range g.levels {
		if level.Floor == nil {
			continue
		}
		for _, placement := range level.Floor.Placements() {
			present[placement.ID] = true
			if !slices.ContainsFunc(g.npcs, func(running *npcrun.NPC) bool { return running.Placement == placement.ID }) {
				added = append(added, placed{placement: placement, floor: level.ID})
			}
		}
	}
	slices.SortFunc(added, func(a, b placed) int { return a.placement.ID.Compare(b.placement.ID) })
	for _, each := range added {
		g.npcs = append(g.npcs, npcrun.Start(each.placement, each.floor, seed(g.seed, g.started)))
		g.started++
	}
	g.npcs = slices.DeleteFunc(g.npcs, func(running *npcrun.NPC) bool { return !present[running.Placement] })
	slices.SortFunc(g.npcs, func(a, b *npcrun.NPC) int { return a.Placement.Compare(b.Placement) })
}

// follow gives each NPC the behaviour the project names for it now.
func (g *Game) follow(project Project) {
	flows, behaviours := map[ids.ULID]*flow.Workflow{}, map[ids.ULID]ids.ULID{}
	restarted := map[ids.ULID]bool{}
	for _, character := range g.npcs {
		npcID := character.NPC
		if _, done := behaviours[npcID]; done {
			continue
		}
		behaviours[npcID] = ids.ULID{}
		definition, defined := project.NPCs[npcID]
		wanted := project.Workflows[definition.Behaviour]
		if !defined || definition.Behaviour.IsZero() || wanted == nil {
			continue
		}
		held := g.flows[npcID]
		if definition.Behaviour == g.behaviours[npcID] && held != nil && held.Equal(wanted) {
			flows[npcID], behaviours[npcID] = held, definition.Behaviour

			continue
		}
		flows[npcID], behaviours[npcID] = wanted, definition.Behaviour
		restarted[npcID] = held != nil
	}
	for npcID, behaviour := range behaviours {
		if behaviour.IsZero() {
			delete(behaviours, npcID)
		}
	}
	g.flows, g.behaviours = flows, behaviours
	for _, character := range g.npcs {
		if restarted[character.NPC] {
			character.Restart(RestartedByEdit)
		}
	}
}

// followLoop gives the zone its loop as the project has it now.
func (g *Game) followLoop(project Project) {
	if project.Zone == nil {
		return
	}
	loopID, loop := loopOf(project, g.zone)
	if loop == nil {
		g.loopFlow, g.loopID = nil, ids.ULID{}

		return
	}
	if g.loop == nil {
		g.loop = npcrun.StartLoop(g.zone, seed(g.seed, loopOrder))
	}
	if loopID == g.loopID && g.loopFlow != nil && g.loopFlow.Equal(loop) {
		return
	}
	restart := g.loopFlow != nil
	g.loopFlow, g.loopID = loop, loopID
	if restart {
		g.loop.Restart(RestartedByEdit)
	}
}

// inside reports whether a runner is inside a call to a workflow.
func inside(character *npcrun.NPC, target ids.ULID) bool {
	if character.Module() == target {
		return true
	}

	return slices.ContainsFunc(character.Callers(), func(call npcrun.Caller) bool { return call.Workflow == target })
}

// ErrPlace reports a cell the player cannot be moved to.
var ErrPlace = errors.New("the player cannot be moved there")

// Place moves the player to a cell of a floor of the zone being played
// (increment 63), as an author trying a game moves them, and as a portal
// that leads to an anchor of this zone takes them. Nothing else changes: the
// NPCs carry on, and the clock and the values are as they were. A floor the
// zone lacks, or a cell the player cannot stand on, is refused. A player who
// was leaving by a portal has arrived.
func (g *Game) Place(floorID ids.ULID, cell image.Point) error {
	if !slices.Contains(g.levels.Floors(), floorID) {
		return fmt.Errorf("%w: that floor is not in the zone being played", ErrPlace)
	}
	if !play.CanStand(g.levels, floorID, cell) {
		return fmt.Errorf("%w: nothing can stand on cell %d,%d", ErrPlace, cell.X, cell.Y)
	}
	g.run.Place(floorID, cell)
	g.arrived()

	return nil
}

// Enter moves the game into the zone a project is gathered for, with the
// player at the project's start (increment 63): the player leaves the zone
// being played, which stops, and the zone entered comes alive.
//
//   - The NPCs of the zone left stop. Those of the zone entered start at
//     their placements, as when a game begins, and so does its loop.
//   - The boxes and the shake are cleared, and signals not yet landed are
//     dropped: they belonged to the zone left.
//   - The clock keeps its time and the values what they hold, so anything
//     recorded as a value is remembered. Entering the zone being played
//     again starts its NPCs afresh in the same way.
//
// A project with no start or no zone, or a start the player cannot stand
// on, is refused, and the game goes on where it was. A player who was
// leaving by a portal has arrived.
func (g *Game) Enter(project Project) error {
	start := project.Settings.Start
	if start.IsZero() || project.Zone == nil {
		return fmt.Errorf("%w: its zone is not in the project", ErrPlace)
	}
	levels := levelsOf(project)
	cell := image.Pt(start.Col, start.Row)
	if !play.CanStand(levels, start.Floor, cell) {
		return fmt.Errorf("%w: nothing can stand on cell %d,%d", ErrPlace, start.Col, start.Row)
	}
	g.settings, g.zone, g.levels, g.sounds = project.Settings, start.Zone, levels, project.Zone.Sounds()
	g.run.Place(start.Floor, cell)
	g.names, g.signals, g.workflows = project.Names, project.Signals, project.Workflows
	g.declare(project.Values)
	g.npcs, g.flows, g.behaviours = nil, map[ids.ULID]*flow.Workflow{}, map[ids.ULID]ids.ULID{}
	g.started = 0
	g.loop, g.loopFlow, g.loopID = nil, nil, ids.ULID{}
	g.stage, g.sent = show.NewStage(), nil
	g.syncRunners()
	g.follow(project)
	g.startLoop(project)
	g.arrived()

	return nil
}
